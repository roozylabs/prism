package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/roozylabs/prism/internal/authz"
	"github.com/roozylabs/prism/internal/config"
	"github.com/roozylabs/prism/internal/database"
	"github.com/roozylabs/prism/internal/handlers"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/proxy"
	"github.com/roozylabs/prism/internal/queue"
	goredis "github.com/roozylabs/prism/internal/redis"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/roozylabs/prism/internal/service"
	"github.com/roozylabs/prism/internal/telemetry"
	"github.com/roozylabs/prism/internal/workers"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title           RoozyLabs Prism AI Gateway API
// @version         0.4.0
// @description     Centralized AI API Gateway
// @host            localhost:8080
// @BasePath        /api/v1

func main() {
	migrateOnly := flag.Bool("migrate-only", false, "Run database migrations and exit without starting the HTTP server")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}
	// The migrator job does not handle secrets, so only the server validates them.
	if !*migrateOnly {
		if err := cfg.Validate(); err != nil {
			log.Fatal(err)
		}
	}

	// Run migrations. When --migrate-only is given (one-shot migrator job),
	// apply migrations and exit without binding the HTTP server so that
	// runtime API containers start fast and never race DDL locks.
	if cfg.RunMigrations || *migrateOnly {
		if err := database.RunMigrations(cfg.DatabaseURL, "./migrations"); err != nil {
			if *migrateOnly {
				log.Fatalf("Migration failed in migrate-only mode: %v", err)
			}
			log.Printf("[Migration Warning] Database migration step returned warning: %v", err)
		}
	}
	if *migrateOnly {
		log.Println("Migrations complete. Exiting (migrate-only mode).")
		return
	}

	// Connect to PostgreSQL
	db, err := database.NewPostgres(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer func() { _ = db.Close() }()

	// Connect to Redis
	rdb, err := database.NewRedis(cfg.RedisURL)
	if err != nil {
		log.Fatal("Failed to connect to Redis:", err)
	}
	defer func() { _ = rdb.Close() }()

	// OpenTelemetry Initialization
	otelShutdown, err := telemetry.InitOTel(context.Background())
	if err != nil {
		log.Printf("[OTel Warning] Failed to initialize OpenTelemetry: %v", err)
	} else {
		defer func() { _ = otelShutdown(context.Background()) }()
	}

	// Repositories (use underlying sql.DB from sqlx)
	sqlDB := db.DB
	_, _ = sqlDB.Exec("UPDATE credentials SET status = 'active' WHERE status = 'rate_limited'")

	userRepo := repository.NewUserRepository(sqlDB)
	sessionRepo := repository.NewSessionRepository(sqlDB)
	accountRepo := repository.NewAccountRepository(sqlDB)
	providerRepo := repository.NewProviderRepository(sqlDB)
	credentialRepo := repository.NewCredentialRepository(sqlDB)
	modelRepo := repository.NewModelRepository(sqlDB)
	gatewayKeyRepo := repository.NewGatewayKeyRepository(sqlDB)
	gatewayKeyCache := repository.NewGatewayKeyCache(gatewayKeyRepo)
	requestLogRepo := repository.NewRequestLogRepository(sqlDB)
	settingRepo := repository.NewSettingRepository(sqlDB)
	pricingRepo := repository.NewModelPricingRepository(sqlDB)
	routingRuleRepo := repository.NewRoutingRuleRepository(sqlDB)
	routingPolicyRepo := repository.NewRoutingPolicyRepository(sqlDB)
	budgetRepo := repository.NewBudgetRepository(sqlDB)
	decisionRepo := repository.NewRoutingDecisionRepository(sqlDB)
	payloadRepo := repository.NewPayloadRepository(sqlDB)
	toolInvocationRepo := repository.NewToolInvocationRepository(sqlDB)
	anomalyRepo := repository.NewCostAnomalyRepository(sqlDB)
	budgetAlertRepo := repository.NewBudgetAlertRepository(sqlDB)
	modelLatencyRepo := repository.NewModelLatencyHourlyRepository(sqlDB)
	toolRepo := repository.NewToolRepository(sqlDB)
	toolBackendRepo := repository.NewToolBackendRepository(sqlDB)
	resourceRepo := repository.NewResourceRepository(sqlDB)
	resourceBackendRepo := repository.NewResourceBackendRepository(sqlDB)
	mcpServerRepo := repository.NewMCPServerRepository(sqlDB)
	mcpToolRepo := repository.NewMCPToolRepository(sqlDB)
	mcpInvocationRepo := repository.NewMCPInvocationRepository(sqlDB)
	mcpRegistryRepo := repository.NewMCPRegistryRepository(sqlDB)
	agentRepo := repository.NewAgentRepository(sqlDB)
	agentTemplateRepo := repository.NewAgentTemplateRepository(sqlDB)
	governancePolicyRepo := repository.NewGovernancePolicyRepository(sqlDB)
	auditTrailRepo := repository.NewAuditTrailRepository(sqlDB)
	rbacRepo := repository.NewRBACRepository(sqlDB)
	quotaRepo := repository.NewQuotaRepository(sqlDB)
	billingRepo := repository.NewBillingRepository(sqlDB)

	// Services
	authService := service.NewAuthService(userRepo, sessionRepo, accountRepo)

	// Event system
	eventPublisher := goredis.NewEventPublisher(rdb)

	// Proxy
	cooldown := goredis.NewCooldownStore(rdb)
	telemetry := goredis.NewModelTelemetryStore(rdb)
	budgetMgr := proxy.NewBudgetManager(budgetRepo)
	healthStore := proxy.NewProviderHealthStore(requestLogRepo, 30*time.Second)
	idemStore := proxy.NewIdempotencyStore(rdb)
	router := proxy.NewRouter(modelRepo, providerRepo, credentialRepo, settingRepo, healthStore)
	engine := proxy.NewEngine(router, credentialRepo, cooldown, telemetry, eventPublisher, cfg.EncryptionKey, cfg.MaxRetries, cfg.CooldownSeconds, budgetMgr, routingPolicyRepo, decisionRepo, payloadRepo, toolInvocationRepo)
	toolGateway := proxy.NewToolGateway(toolRepo)
	resourceGateway := proxy.NewResourceGateway(resourceRepo)
	mcpGateway := proxy.NewMCPGateway(mcpServerRepo, mcpToolRepo, mcpInvocationRepo)
	mcpGateway.SetLocalStdioEnabled(cfg.MCPLocalStdioEnabled)
	agentGovernance := proxy.NewAgentGovernanceEngine(agentRepo)
	rbacEngine := proxy.NewRBACEngine(governancePolicyRepo)
	auditRecorder := proxy.NewAuditRecorder(auditTrailRepo)
	paperclipAdapter := proxy.NewPaperclipAdapter(agentRepo)

	// Handlers
	healthHandler := handlers.NewHealthHandler(db, rdb)
	// Sign-in and sign-up: 30 requests per address per 10 minutes, and 10
	// failed sign-ins per account per 15 minutes.
	authHandler := handlers.NewAuthHandler(authService,
		handlers.NewTurnstile(os.Getenv("CLOUDFLARE_SECRET_KEY"), cfg.IsDevelopment()),
		goredis.NewAttemptLimiter(rdb, "auth:attempts:ip:", 30, 10*time.Minute),
		goredis.NewAttemptLimiter(rdb, "auth:attempts:email:", 10, 15*time.Minute))
	providerHandler := handlers.NewProviderHandler(providerRepo, gatewayKeyRepo)
	credentialHandler := handlers.NewCredentialHandler(credentialRepo, providerRepo, gatewayKeyRepo, cooldown, eventPublisher, cfg.EncryptionKey)
	modelHandler := handlers.NewModelHandler(modelRepo, providerRepo, gatewayKeyRepo, cooldown)
	gatewayKeyHandler := handlers.NewGatewayKeyHandler(gatewayKeyRepo, credentialRepo)
	admissionCtrl := proxy.NewAdmissionController(rbacEngine, agentGovernance, proxy.NewCachedQuotaEvaluator(quotaRepo, proxy.DefaultQuotaCacheTTL), budgetMgr)
	orchestrator := service.NewExecutionOrchestrator(engine, admissionCtrl, gatewayKeyRepo, requestLogRepo, eventPublisher, pricingRepo, auditRecorder)
	gatewayHandler := handlers.NewGatewayHandler(engine, gatewayKeyRepo, requestLogRepo, eventPublisher, pricingRepo, idemStore, agentGovernance, rbacEngine, auditRecorder, modelRepo, agentRepo, orchestrator)
	gatewayHandler.SetMCPRepositories(mcpServerRepo, mcpToolRepo)
	gatewayHandler.SetMemberRoleResolver(rbacRepo)
	engine.SetMCPRepositories(mcpInvocationRepo, mcpServerRepo)

	// Async Side-Effect Post Processor
	postProcessor := service.NewAsyncPostProcessor(requestLogRepo, gatewayKeyRepo, eventPublisher, auditRecorder, 2000, 10)
	defer postProcessor.Stop()
	orchestrator.SetPostProcessor(postProcessor)

	// Async Job Queue & Worker Pool
	jobQueue := queue.NewJobQueue(rdb)
	jobQueue.SetEventPublisher(eventPublisher)
	gatewayHandler.SetJobQueue(jobQueue)

	jobWorkerCtx, jobWorkerStop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer jobWorkerStop()

	jobQueue.StartWorkerPool(jobWorkerCtx, 5, func(job *queue.AsyncJob) (*proxy.ProviderResponse, error) {
		res, execErr := orchestrator.ExecuteChatCompletions(
			nil,
			context.Background(),
			job.RequestPayload,
			job.GatewayKey,
			job.TenantCtx,
			job.AgentID,
			job.AgentName,
			job.UserRole,
			job.RequestID,
			job.ClientIP,
			job.UserAgent,
			job.ClientApp,
		)
		if execErr != nil {
			return nil, execErr
		}
		if res.Denied {
			return nil, fmt.Errorf("job denied by policy %s: %s", res.PolicyName, res.ErrorMessage)
		}
		return res.Response, nil
	})
	paperclipHandler := handlers.NewPaperclipHandler(paperclipAdapter, gatewayHandler)
	logsHandler := handlers.NewLogsHandler(requestLogRepo)
	dashboardHandler := handlers.NewDashboardHandler(requestLogRepo, healthStore)
	activeStreamsHandler := handlers.NewActiveStreamsHandler(cooldown)
	settingsHandler := handlers.NewSettingsHandler(settingRepo)
	sseHandler := handlers.NewSSEHandler(eventPublisher)
	googleOAuthHandler := handlers.NewGoogleOAuthHandler(credentialRepo, providerRepo, cfg.EncryptionKey)
	routingRuleHandler := handlers.NewRoutingRuleHandler(routingRuleRepo)
	routingPolicyHandler := handlers.NewRoutingPolicyHandler(routingPolicyRepo)
	budgetHandler := handlers.NewBudgetHandler(budgetRepo)
	budgetStatusHandler := handlers.NewBudgetStatusHandler(budgetMgr)
	routingDecisionHandler := handlers.NewRoutingDecisionHandler(decisionRepo)
	simulateHandler := handlers.NewSimulateHandler(modelRepo, providerRepo, credentialRepo, routingPolicyRepo, telemetry, healthStore)
	finopsHandler := handlers.NewFinOpsHandler(requestLogRepo, budgetRepo, modelRepo, settingRepo, anomalyRepo)
	finopsAnomaliesHandler := handlers.NewFinOpsAnomaliesHandler(anomalyRepo, budgetAlertRepo)
	toolHandler := handlers.NewToolHandler(toolRepo, toolBackendRepo, toolGateway, cfg.EncryptionKey)
	toolGatewayHandler := handlers.NewToolGatewayHandler(toolGateway, toolInvocationRepo, eventPublisher, cfg.EncryptionKey)
	resourceHandler := handlers.NewResourceHandler(resourceRepo, resourceBackendRepo, resourceGateway, cfg.EncryptionKey)
	resourceGatewayHandler := handlers.NewResourceGatewayHandler(resourceGateway, toolInvocationRepo, eventPublisher, cfg.EncryptionKey)
	mcpHandler := handlers.NewMCPHandler(mcpServerRepo, mcpToolRepo, mcpGateway, mcpInvocationRepo, agentRepo, cfg.EncryptionKey)
	mcpRegistryHandler := handlers.NewMCPRegistryHandler(mcpRegistryRepo)
	agentHandler := handlers.NewAgentHandler(agentRepo, agentGovernance, requestLogRepo)
	agentTemplateHandler := handlers.NewAgentTemplateHandler(agentTemplateRepo, agentRepo)
	governancePolicyHandler := handlers.NewGovernancePolicyHandler(governancePolicyRepo, rbacEngine)
	auditTrailHandler := handlers.NewAuditTrailHandler(auditTrailRepo, auditRecorder)
	authzEngine := authz.NewAuthorizationEngine(rbacRepo, rbacRepo)
	orgMemberHandler := handlers.NewOrganizationMemberHandler(rbacRepo)
	userPermissionsHandler := handlers.NewUserPermissionsHandler(rbacRepo, userRepo)
	onboardingHandler := handlers.NewOnboardingHandler(db.DB, userRepo, gatewayKeyRepo)
	oauthHandler := handlers.NewOAuthHandler(authService)
	quotaHandler := handlers.NewQuotaHandler(quotaRepo)
	billingHandler := handlers.NewBillingHandler(billingRepo)
	featuresHandler := handlers.NewFeaturesHandler()

	// Background workers
	workerCtx, workerStop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer workerStop()
	workerMgr := workers.NewManager()
	workerMgr.Register("cost-anomaly", 15*time.Minute, anomalyWorker(anomalyRepo, eventPublisher))
	workerMgr.Register("budget-alert", 2*time.Minute, budgetAlertWorker(budgetRepo, budgetAlertRepo, eventPublisher))
	workerMgr.Register("latency-flush", 1*time.Hour, latencyFlushWorker(telemetry, modelRepo, modelLatencyRepo))
	workerMgr.Register("retention-cleanup", 1*time.Hour, retentionWorker(payloadRepo, toolInvocationRepo))
	workerMgr.Register("credential-recovery", 1*time.Minute, workers.NewCredentialRecoveryWorker(credentialRepo, cooldown).Run)
	workerMgr.Start(workerCtx)

	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()
	if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		log.Fatalf("invalid TRUSTED_PROXIES: %v", err)
	}
	if cfg.TrustedPlatform == "cloudflare" {
		r.TrustedPlatform = gin.PlatformCloudflare
	}

	// CORS middleware
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.MaxBodyBytes(cfg.MaxRequestBodyBytes))

	// Health check, Readiness check & OTel Prometheus Metrics (public)
	r.GET("/health", healthHandler.Check)
	r.GET("/ready", healthHandler.Ready)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API routes group
	api := r.Group("/api")
	{
		// Public API routes
		api.GET("/health", healthHandler.Check)
		api.GET("/ready", healthHandler.Ready)
		api.GET("/features", featuresHandler.GetFeatures)
		api.POST("/auth/login", authHandler.Login)
		api.POST("/auth/signup", authHandler.Signup)
		api.GET("/auth/turnstile-config", authHandler.GetTurnstileConfig)
		api.GET("/auth/oauth/:provider", oauthHandler.InitiateOAuth)
		api.GET("/auth/oauth/:provider/callback", oauthHandler.OAuthCallback)
		api.GET("/auth/google/login", oauthHandler.GoogleLogin)
		api.GET("/auth/google/callback", oauthHandler.GoogleCallback)
		api.GET("/auth/github/login", oauthHandler.GitHubLogin)
		api.GET("/auth/github/callback", oauthHandler.GitHubCallback)

		// Protected User-Scoped API routes (Session auth required, no active tenant required)
		userProtected := api.Group("")
		userProtected.Use(middleware.AuthMiddleware(sessionRepo, gatewayKeyCache))
		{
			// Auth Session
			userProtected.POST("/auth/logout", authHandler.Logout)
			userProtected.GET("/auth/me", authHandler.Me)

			// User Permissions, Organization List & Onboarding
			userProtected.GET("/user/permissions", userPermissionsHandler.GetPermissions)
			userProtected.GET("/user/organizations", userPermissionsHandler.GetOrganizations)
			userProtected.GET("/organizations", userPermissionsHandler.GetOrganizations)
			userProtected.POST("/onboarding", onboardingHandler.Complete)
		}

		// Protected Tenant-Scoped API routes (Auth + Strict Tenant Membership required)
		protected := api.Group("")
		protected.Use(middleware.AuthMiddleware(sessionRepo, gatewayKeyCache))
		protected.Use(middleware.TenantMiddleware(accountRepo))
		requirePermission := func(code string) gin.HandlerFunc {
			return middleware.RequirePermission(authzEngine, code)
		}
		{
			// Organization Member Management
			protected.GET("/organizations/members", requirePermission("member:read"), orgMemberHandler.ListMembers)
			protected.POST("/organizations/invites", requirePermission("member:invite"), orgMemberHandler.InviteMember)
			protected.PUT("/organizations/members/:userId", requirePermission("member:update"), orgMemberHandler.UpdateMemberRole)
			protected.DELETE("/organizations/members/:userId", requirePermission("member:remove"), orgMemberHandler.RemoveMember)

			// Providers
			protected.GET("/providers", requirePermission("model:read"), providerHandler.List)
			protected.POST("/providers", requirePermission("model:create"), providerHandler.Create)
			protected.GET("/providers/:id", requirePermission("model:read"), providerHandler.Get)
			protected.PUT("/providers/:id", requirePermission("model:update"), providerHandler.Update)
			protected.DELETE("/providers/:id", requirePermission("model:delete"), providerHandler.Delete)

			// Credentials (nested under provider & top-level)
			protected.GET("/credentials", requirePermission("credential:read"), credentialHandler.List)
			protected.GET("/providers/:id/credentials", requirePermission("credential:read"), credentialHandler.List)
			protected.POST("/providers/:id/credentials", requirePermission("credential:create"), credentialHandler.Create)
			// Connecting a Google account opens these in a popup; the session
			// cookie authenticates both, so the credential gets an owner.
			protected.GET("/providers/google/oauth/login", requirePermission("credential:create"), googleOAuthHandler.Login)
			protected.GET("/providers/google/oauth/callback", requirePermission("credential:create"), googleOAuthHandler.Callback)
			protected.GET("/providers/:id/credentials/:credId", requirePermission("credential:read"), credentialHandler.Get)
			protected.PUT("/providers/:id/credentials/:credId", requirePermission("credential:update"), credentialHandler.Update)
			protected.DELETE("/providers/:id/credentials/:credId", requirePermission("credential:delete"), credentialHandler.Delete)
			protected.DELETE("/credentials/:credId", requirePermission("credential:delete"), credentialHandler.Delete)
			protected.POST("/providers/:id/credentials/:credId/test", requirePermission("credential:update"), credentialHandler.Test)
			protected.POST("/providers/:id/credentials/:credId/reveal", requirePermission("credential:update"), credentialHandler.Reveal)
			protected.POST("/providers/:id/credentials/:credId/reset-cooldown", requirePermission("credential:update"), credentialHandler.ResetCooldown)
			protected.POST("/credentials/:credId/reset-cooldown", requirePermission("credential:update"), credentialHandler.ResetCooldown)

			// Models (nested under provider & top-level)
			protected.GET("/models", requirePermission("model:read"), modelHandler.List)
			protected.GET("/providers/:id/models", requirePermission("model:read"), modelHandler.List)
			protected.POST("/providers/:id/models", requirePermission("model:create"), modelHandler.Create)
			protected.GET("/providers/:id/models/:modelId", requirePermission("model:read"), modelHandler.Get)
			protected.PUT("/providers/:id/models/:modelId", requirePermission("model:update"), modelHandler.Update)
			protected.PATCH("/providers/:id/models/:modelId/capabilities", requirePermission("model:update"), modelHandler.UpdateCapabilities)
			protected.DELETE("/providers/:id/models/:modelId", requirePermission("model:delete"), modelHandler.Delete)

			// Gateway API Keys
			protected.GET("/gateway-keys", requirePermission("api_key:read"), gatewayKeyHandler.List)
			protected.POST("/gateway-keys", requirePermission("api_key:create"), gatewayKeyHandler.Create)
			protected.DELETE("/gateway-keys/:id", requirePermission("api_key:revoke"), gatewayKeyHandler.Delete)

			// Request Logs & Analytics
			protected.GET("/logs", logsHandler.List)
			protected.GET("/analytics/logs", logsHandler.GetAnalytics)
			protected.GET("/analytics/finops", finopsHandler.GetSummary)
			protected.GET("/analytics/finops/anomalies", finopsAnomaliesHandler.ListAnomalies)
			protected.GET("/analytics/finops/budget-alerts", finopsAnomaliesHandler.ListBudgetAlerts)
			protected.POST("/analytics/finops/budget-alerts/:id/acknowledge", finopsAnomaliesHandler.AcknowledgeAlert)

			// Quotas & Budgets
			protected.GET("/quotas", requirePermission("quota:read"), quotaHandler.List)
			protected.GET("/quotas/:target_type/:target_id", requirePermission("quota:read"), quotaHandler.Get)
			protected.PUT("/quotas/:target_type/:target_id", requirePermission("quota:update"), quotaHandler.Update)

			// Multi-Tier Billing & Subscriptions
			protected.GET("/billing/plans", requirePermission("billing:read"), billingHandler.ListPlans)
			protected.GET("/billing/subscription", requirePermission("billing:read"), billingHandler.GetSubscription)
			protected.POST("/billing/subscription/upgrade", requirePermission("billing:manage"), billingHandler.UpgradeSubscription)
			protected.GET("/billing/invoices", requirePermission("billing:read"), billingHandler.ListInvoices)
			protected.GET("/billing/usage", requirePermission("billing:read"), billingHandler.GetUsage)

			// Dashboard
			protected.GET("/dashboard/stats", dashboardHandler.GetStats)
			protected.GET("/dashboard/usage", dashboardHandler.GetUsageChart)
			protected.GET("/dashboard/health", dashboardHandler.GetProviderHealth)
			protected.GET("/dashboard/active-streams", activeStreamsHandler.GetActiveStreams)

			// Settings
			protected.GET("/settings", requirePermission("organization:read"), settingsHandler.List)
			protected.PUT("/settings", requirePermission("organization:update"), settingsHandler.Update)

			// SSE
			protected.GET("/sse", sseHandler.Stream)

			// Routing Rules
			protected.GET("/routing-rules", routingRuleHandler.List)
			protected.GET("/routing-rules/:id", routingRuleHandler.Get)
			protected.POST("/routing-rules", routingRuleHandler.Create)
			protected.PUT("/routing-rules/:id", routingRuleHandler.Update)
			protected.DELETE("/routing-rules/:id", routingRuleHandler.Delete)

			// Routing Policies (with aliases for /policies and /routing/policies)
			protected.GET("/routing-policies", routingPolicyHandler.List)
			protected.GET("/routing-policies/:id", routingPolicyHandler.Get)
			protected.POST("/routing-policies", routingPolicyHandler.Create)
			protected.PUT("/routing-policies/:id", routingPolicyHandler.Update)
			protected.PUT("/routing-policies/:id/default", routingPolicyHandler.SetDefault)
			protected.DELETE("/routing-policies/:id", routingPolicyHandler.Delete)

			protected.GET("/policies", routingPolicyHandler.List)
			protected.GET("/policies/:id", routingPolicyHandler.Get)
			protected.POST("/policies", routingPolicyHandler.Create)
			protected.PUT("/policies/:id", routingPolicyHandler.Update)
			protected.PUT("/policies/:id/default", routingPolicyHandler.SetDefault)
			protected.DELETE("/policies/:id", routingPolicyHandler.Delete)

			protected.GET("/routing/policies", routingPolicyHandler.List)
			protected.POST("/routing/policies", routingPolicyHandler.Create)
			protected.PUT("/routing/policies/:id", routingPolicyHandler.Update)
			protected.PUT("/routing/policies/:id/default", routingPolicyHandler.SetDefault)
			protected.DELETE("/routing/policies/:id", routingPolicyHandler.Delete)

			// Routing Decisions Audit Logs & Simulation
			protected.GET("/routing/decisions", routingDecisionHandler.List)
			protected.GET("/routing-decisions", routingDecisionHandler.List)
			protected.POST("/routing/simulate", simulateHandler.Simulate)
			protected.POST("/routing-simulate", simulateHandler.Simulate)

			// Budgets
			protected.GET("/budgets", requirePermission("budget:read"), budgetHandler.List)
			protected.GET("/budgets/status", requirePermission("budget:read"), budgetStatusHandler.GetStatus)
			protected.GET("/budgets/:id", requirePermission("budget:read"), budgetHandler.Get)
			protected.POST("/budgets", requirePermission("budget:create"), budgetHandler.Create)
			protected.PUT("/budgets/:id", requirePermission("budget:update"), budgetHandler.Update)
			protected.DELETE("/budgets/:id", requirePermission("budget:delete"), budgetHandler.Delete)

			// Tool Gateway (admin CRUD)
			protected.GET("/tools", requirePermission("tool:read"), toolHandler.List)
			protected.POST("/tools", requirePermission("tool:create"), toolHandler.Create)
			protected.GET("/tools/:id", requirePermission("tool:read"), toolHandler.Get)
			protected.PUT("/tools/:id", requirePermission("tool:update"), toolHandler.Update)
			protected.DELETE("/tools/:id", requirePermission("tool:delete"), toolHandler.Delete)
			protected.POST("/tools/:id/test", requirePermission("tool:execute"), toolHandler.TestTool)

			// Resource Gateway (admin CRUD)
			protected.GET("/resources", requirePermission("resource:read"), resourceHandler.List)
			protected.POST("/resources", requirePermission("resource:create"), resourceHandler.Create)
			protected.GET("/resources/:id", requirePermission("resource:read"), resourceHandler.Get)
			protected.PUT("/resources/:id", requirePermission("resource:update"), resourceHandler.Update)
			protected.DELETE("/resources/:id", requirePermission("resource:delete"), resourceHandler.Delete)
			protected.POST("/resources/:id/test", requirePermission("resource:query"), resourceHandler.TestResource)

			// MCP Gateway
			protected.GET("/mcp/servers", requirePermission("mcp:read"), mcpHandler.List)
			protected.GET("/mcp/servers/:id", requirePermission("mcp:read"), mcpHandler.Get)
			protected.POST("/mcp/servers", requirePermission("mcp:create"), mcpHandler.Create)
			protected.PUT("/mcp/servers/:id", requirePermission("mcp:update"), mcpHandler.Update)
			protected.DELETE("/mcp/servers/:id", requirePermission("mcp:delete"), mcpHandler.Delete)
			protected.POST("/mcp/servers/:id/sync", requirePermission("mcp:update"), mcpHandler.Sync)
			protected.POST("/mcp/servers/:id/test", requirePermission("mcp:execute"), mcpHandler.TestTool)
			protected.GET("/mcp/servers/:id/tools", requirePermission("mcp:read"), mcpHandler.ListTools)
			protected.GET("/mcp/servers/:id/stats", requirePermission("mcp:read"), mcpHandler.Stats)

			// MCP Registry Catalog
			protected.GET("/mcp/registry", requirePermission("mcp:read"), mcpRegistryHandler.ListCatalog)
			protected.GET("/mcp/registry/:slug", requirePermission("mcp:read"), mcpRegistryHandler.GetCatalogBySlug)
			protected.POST("/mcp/registry", requirePermission("mcp:create"), mcpRegistryHandler.RegisterServer)

			// Agent Gateway & Infrastructure
			protected.GET("/agents", requirePermission("agent:read"), agentHandler.List)
			protected.GET("/agents/:id", requirePermission("agent:read"), agentHandler.Get)
			protected.GET("/agents/:id/stats", requirePermission("agent:read"), agentHandler.Stats)
			protected.POST("/agents", requirePermission("agent:create"), agentHandler.Create)
			protected.PUT("/agents/:id", requirePermission("agent:update"), agentHandler.Update)
			protected.DELETE("/agents/:id", requirePermission("agent:delete"), agentHandler.Delete)

			// Agent Templates Engine
			protected.GET("/agent-templates", requirePermission("agent:read"), agentTemplateHandler.ListTemplates)
			protected.POST("/agent-templates", requirePermission("agent:create"), agentTemplateHandler.CreateTemplate)
			protected.POST("/agent-templates/:id/instantiate", requirePermission("agent:create"), agentTemplateHandler.InstantiateTemplate)

			// Enterprise Governance & RBAC
			protected.GET("/governance/policies", requirePermission("governance:read"), governancePolicyHandler.List)
			protected.GET("/governance/policies/:id", requirePermission("governance:read"), governancePolicyHandler.Get)
			protected.POST("/governance/policies", requirePermission("governance:create"), governancePolicyHandler.Create)
			protected.PUT("/governance/policies/:id", requirePermission("governance:update"), governancePolicyHandler.Update)
			protected.DELETE("/governance/policies/:id", requirePermission("governance:delete"), governancePolicyHandler.Delete)
			protected.POST("/governance/evaluate", requirePermission("governance:evaluate"), governancePolicyHandler.Evaluate)

			// End-to-End AI Audit Trail
			protected.GET("/audit-trail", requirePermission("audit:read"), auditTrailHandler.List)
			protected.GET("/audit-trail/logs", requirePermission("audit:read"), auditTrailHandler.ListLogs)
			protected.GET("/audit-trail/export", requirePermission("audit:export"), auditTrailHandler.ExportLogs)
			protected.GET("/audit-trail/:id", requirePermission("audit:read"), auditTrailHandler.Get)
			protected.POST("/audit-trail/:id/verify", requirePermission("audit:verify"), auditTrailHandler.Verify)

			// Sandbox
			protected.POST("/sandbox/chat/completions", requirePermission("playground:execute"), gatewayHandler.SandboxChatCompletions)
			protected.POST("/sandbox/chat/completions/async", requirePermission("playground:execute"), gatewayHandler.AsyncSandboxChatCompletions)
			protected.GET("/jobs/:jobId", gatewayHandler.GetJobStatus)
		}
	}

	// Gateway routes (authenticated with gw_sk_* keys) - accessible at /v1 and /api/v1
	registerGatewayRoutes := func(rg *gin.RouterGroup) {
		rg.Use(middleware.GatewayAuthMiddleware(gatewayKeyCache))
		rg.Use(middleware.TenantMiddleware(accountRepo))
		rg.Use(middleware.GatewayRateLimitMiddleware(rdb, cfg.RateLimitPerKey))
		rg.Use(middleware.AgentPolicyMiddleware(agentRepo))
		rg.Use(proxy.IdempotencyMiddleware(idemStore))
		rg.POST("/chat/completions", gatewayHandler.ChatCompletions)
		rg.POST("/chat/completions/async", gatewayHandler.AsyncChatCompletions)
		rg.GET("/jobs/:jobId", gatewayHandler.GetJobStatus)
		rg.GET("/models", gatewayHandler.Models)
		rg.POST("/tools/:toolName/execute", toolGatewayHandler.ExecuteTool)
		rg.POST("/resources/:resourceName/query", resourceGatewayHandler.ExecuteQuery)
		rg.POST("/adapters/paperclip/agent/register", paperclipHandler.RegisterAgent)
		rg.POST("/adapters/paperclip/chat/completions", paperclipHandler.ChatCompletions)
	}

	registerGatewayRoutes(r.Group("/v1"))
	registerGatewayRoutes(api.Group("/v1"))

	// Swagger endpoint (dev only)
	if cfg.AppEnv != "production" {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	srv := &http.Server{
		Addr:              ":" + cfg.ServerPort,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		// Bounds reading the request, body included; Go clears the deadline
		// once the body is read, so long streams are not affected.
		ReadTimeout: 60 * time.Second,
		IdleTimeout: 120 * time.Second,
	}
	srv.RegisterOnShutdown(sseHandler.Shutdown)

	shutdownSignal, stopSignal := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignal()

	log.Printf("Server starting on port %s", cfg.ServerPort)
	if err := runServer(shutdownSignal, srv, time.Duration(cfg.ShutdownTimeoutSeconds)*time.Second); err != nil {
		log.Fatal("Failed to start server:", err)
	}
	log.Println("HTTP server stopped")
}

// runServer serves until ctx is done, then stops accepting connections and
// lets in-flight requests (including LLM streams) finish for up to timeout
// before closing whatever is still open. It returns once the server has
// stopped, so the caller's deferred cleanup (post-processor flush, DB close)
// runs after the last request. It returns an error only if serving fails.
func runServer(ctx context.Context, srv *http.Server, timeout time.Duration) error {
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}

	log.Printf("Shutdown signal received; draining in-flight requests for up to %s", timeout)
	drainCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := srv.Shutdown(drainCtx); err != nil {
		log.Printf("Drain timed out (%v); closing remaining connections", err)
		_ = srv.Close()
	}
	return nil
}

func anomalyWorker(anomalies *repository.CostAnomalyRepository, publisher *goredis.EventPublisher) func(context.Context) {
	d := workers.NewAnomalyDetector(anomalies, publisher)
	return d.Run
}

func budgetAlertWorker(budgets *repository.BudgetRepository, alerts *repository.BudgetAlertRepository, publisher *goredis.EventPublisher) func(context.Context) {
	s := workers.NewBudgetAlertScanner(budgets, alerts, publisher)
	return s.Run
}

func latencyFlushWorker(telemetry *goredis.ModelTelemetryStore, models *repository.ModelRepository, latency *repository.ModelLatencyHourlyRepository) func(context.Context) {
	w := workers.NewLatencyFlushWorker(telemetry, models, latency)
	return w.Run
}

func retentionWorker(payloads *repository.PayloadRepository, toolCalls *repository.ToolInvocationRepository) func(context.Context) {
	w := workers.NewPayloadRetentionWorker(payloads, toolCalls)
	return w.Run
}
