package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

const SharedMatrixPassword = "PrismMatrix_7x9k2m4p!"

type PlanSpec struct {
	Slug             string
	Name             string
	Tier             string
	PriceUSD         float64
	SpendCapMonthly  float64
	SpendCapDaily    float64
	DailyReqLimit    int
	MaxConcurrent    int
	MaxWorkspaces    int
	MaxProjectsPerWs int
}

var PlanSpecs = []PlanSpec{
	{
		Slug:             "free",
		Name:             "Matrix Labs Free",
		Tier:             "free",
		PriceUSD:         0.0,
		SpendCapMonthly:  50.0,
		SpendCapDaily:    5.0,
		DailyReqLimit:    10000,
		MaxConcurrent:    5,
		MaxWorkspaces:    1,
		MaxProjectsPerWs: 2,
	},
	{
		Slug:             "pro",
		Name:             "Matrix Labs Pro",
		Tier:             "pro",
		PriceUSD:         15.0,
		SpendCapMonthly:  300.0,
		SpendCapDaily:    30.0,
		DailyReqLimit:    50000,
		MaxConcurrent:    20,
		MaxWorkspaces:    3,
		MaxProjectsPerWs: 5,
	},
	{
		Slug:             "team",
		Name:             "Matrix Labs Team",
		Tier:             "team",
		PriceUSD:         49.0,
		SpendCapMonthly:  1500.0,
		SpendCapDaily:    150.0,
		DailyReqLimit:    250000,
		MaxConcurrent:    50,
		MaxWorkspaces:    10,
		MaxProjectsPerWs: 10,
	},
	{
		Slug:             "enterprise",
		Name:             "Matrix Labs Enterprise",
		Tier:             "enterprise",
		PriceUSD:         199.0,
		SpendCapMonthly:  5000.0,
		SpendCapDaily:    500.0,
		DailyReqLimit:    1000000,
		MaxConcurrent:    200,
		MaxWorkspaces:    50,
		MaxProjectsPerWs: 50,
	},
}

type RoleSpec struct {
	Slug        string
	Name        string
	Description string
	Prefix      string
}

var RoleSpecs = []RoleSpec{
	{Slug: "owner", Name: "Owner / Org Admin", Description: "Full administrative, billing, policy & member management", Prefix: "owner"},
	{Slug: "developer", Name: "Developer / AI Engineer", Description: "CRUD for API Keys, Prompts, MCP Servers, Tools, Playground & Logs", Prefix: "dev"},
	{Slug: "agent_manager", Name: "Agent Administrator", Description: "Manage AI Agent identities, model rules, tool execution & budgets", Prefix: "agent"},
	{Slug: "finops_manager", Name: "FinOps / Budget Manager", Description: "Manage spend limits, cost analysis, subscriptions & invoices", Prefix: "finops"},
	{Slug: "auditor", Name: "Security Auditor", Description: "Inspect cryptographic audit logs, compliance rules & request traces", Prefix: "auditor"},
	{Slug: "viewer", Name: "Viewer / Guest", Description: "Read-only telemetry and dashboard monitoring access", Prefix: "viewer"},
}

func main() {
	// Every account this creates shares a password published in this
	// repository, so it must never reach a production database.
	if strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") {
		log.Fatal("Refusing to seed the role/plan matrix with APP_ENV=production: its accounts share a published password")
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/prism?sslmode=disable"
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Printf("Warning: Database ping failed (%v). Attempting to run seed operations...", err)
	}

	if err := SeedMatrix(ctx, db); err != nil {
		log.Fatalf("Matrix seeding failed: %v", err)
	}

	PrintMatrixTable()
}

func SeedMatrix(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// 1. Generate bcrypt hash for shared password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(SharedMatrixPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("generate bcrypt hash: %w", err)
	}

	// 2. Iterate through all Plan Tiers
	for _, plan := range PlanSpecs {
		orgSlug := fmt.Sprintf("org-matrix-%s", plan.Slug)
		wsEngSlug := fmt.Sprintf("ws-%s-eng", plan.Slug)
		wsFinSlug := fmt.Sprintf("ws-%s-finance", plan.Slug)
		projEngSlug := fmt.Sprintf("proj-%s-eng", plan.Slug)
		projFinSlug := fmt.Sprintf("proj-%s-finance", plan.Slug)

		// 3a. Upsert Organization with natural generated UUID
		var orgID string
		err = tx.QueryRowContext(ctx, `
			INSERT INTO organizations (id, name, slug, plan_tier, max_workspaces, max_projects_per_workspace, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
			ON CONFLICT (slug) DO UPDATE SET
				name = EXCLUDED.name,
				plan_tier = EXCLUDED.plan_tier,
				max_workspaces = EXCLUDED.max_workspaces,
				max_projects_per_workspace = EXCLUDED.max_projects_per_workspace,
				updated_at = NOW()
			RETURNING id
		`, uuid.New().String(), plan.Name, orgSlug, plan.Tier, plan.MaxWorkspaces, plan.MaxProjectsPerWs).Scan(&orgID)
		if err != nil {
			return fmt.Errorf("upsert organization %s: %w", orgSlug, err)
		}

		// 3b. Upsert Workspaces with natural generated UUID
		var wsEngID, wsFinID string
		err = tx.QueryRowContext(ctx, `
			INSERT INTO workspaces (id, org_id, name, slug, created_at, updated_at)
			VALUES ($1, $2, $3, $4, NOW(), NOW())
			ON CONFLICT (org_id, slug) DO UPDATE SET
				name = EXCLUDED.name,
				updated_at = NOW()
			RETURNING id
		`, uuid.New().String(), orgID, fmt.Sprintf("%s Engineering", plan.Name), wsEngSlug).Scan(&wsEngID)
		if err != nil {
			return fmt.Errorf("upsert ws eng for %s: %w", orgSlug, err)
		}

		err = tx.QueryRowContext(ctx, `
			INSERT INTO workspaces (id, org_id, name, slug, created_at, updated_at)
			VALUES ($1, $2, $3, $4, NOW(), NOW())
			ON CONFLICT (org_id, slug) DO UPDATE SET
				name = EXCLUDED.name,
				updated_at = NOW()
			RETURNING id
		`, uuid.New().String(), orgID, fmt.Sprintf("%s Finance & Ops", plan.Name), wsFinSlug).Scan(&wsFinID)
		if err != nil {
			return fmt.Errorf("upsert ws fin for %s: %w", orgSlug, err)
		}

		// 3c. Upsert Projects with natural generated UUID
		_, err = tx.ExecContext(ctx, `
			INSERT INTO projects (id, workspace_id, name, slug, created_at, updated_at)
			VALUES
				($1, $2, 'AI Core Services', $3, NOW(), NOW()),
				($4, $5, 'FinOps & Invoicing', $6, NOW(), NOW())
			ON CONFLICT (workspace_id, slug) DO UPDATE SET
				name = EXCLUDED.name,
				updated_at = NOW()
		`, uuid.New().String(), wsEngID, projEngSlug, uuid.New().String(), wsFinID, projFinSlug)
		if err != nil {
			return fmt.Errorf("upsert projects for %s: %w", orgID, err)
		}

		// 3d. Upsert Quota for Organization
		_, err = tx.ExecContext(ctx, `
			INSERT INTO tenant_quotas (organization_id, monthly_spend_limit_usd, daily_spend_limit_usd, daily_request_limit, max_concurrent_streams, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
			ON CONFLICT (target_type, target_id) DO UPDATE SET
				monthly_spend_limit_usd = EXCLUDED.monthly_spend_limit_usd,
				daily_spend_limit_usd = EXCLUDED.daily_spend_limit_usd,
				daily_request_limit = EXCLUDED.daily_request_limit,
				max_concurrent_streams = EXCLUDED.max_concurrent_streams,
				updated_at = NOW()
		`, orgID, plan.SpendCapMonthly, plan.SpendCapDaily, plan.DailyReqLimit, plan.MaxConcurrent)
		if err != nil {
			return fmt.Errorf("upsert quota for %s: %w", orgID, err)
		}

		// 3e. Seed 6 Users per Organization (Total 24 Matrix Users)
		for _, role := range RoleSpecs {
			email := fmt.Sprintf("%s.%s@prism.local", role.Prefix, plan.Slug)
			name := fmt.Sprintf("%s (%s Tier)", role.Name, strings.ToUpper(plan.Slug))

			// Insert / update "user" table
			var userID string
			err = tx.QueryRowContext(ctx, `
				INSERT INTO "user" (id, name, email, email_verified, is_onboarded, primary_role, auth_provider, org_id, created_at, updated_at)
				VALUES ($1, $2, $3, true, true, $4, 'credential', $5, NOW(), NOW())
				ON CONFLICT (email) DO UPDATE SET
					name = EXCLUDED.name,
					org_id = EXCLUDED.org_id,
					primary_role = EXCLUDED.primary_role,
					is_onboarded = true,
					updated_at = NOW()
				RETURNING id
			`, uuid.New().String(), name, email, role.Slug, orgID).Scan(&userID)
			if err != nil {
				return fmt.Errorf("upsert user %s: %w", email, err)
			}

			// Clean existing credential account and re-insert with natural UUID
			_, _ = tx.ExecContext(ctx, `DELETE FROM account WHERE account_id = $1`, email)
			_, err = tx.ExecContext(ctx, `
				INSERT INTO account (id, account_id, provider_id, user_id, password, created_at, updated_at)
				VALUES ($1, $2, 'credential', $3, $4, NOW(), NOW())
			`, uuid.New().String(), email, userID, string(passwordHash))
			if err != nil {
				return fmt.Errorf("insert account %s: %w", email, err)
			}

			// Insert / update "organization_members" with the system role of that slug
			_, err = tx.ExecContext(ctx, `
				INSERT INTO organization_members (id, org_id, user_id, role_id, created_at, updated_at)
				VALUES ($1, $2, $3, (SELECT id FROM roles WHERE slug = $4 AND is_system), NOW(), NOW())
				ON CONFLICT (org_id, user_id) DO UPDATE SET
					role_id = EXCLUDED.role_id,
					updated_at = NOW()
			`, uuid.New().String(), orgID, userID, role.Slug)
			if err != nil {
				return fmt.Errorf("upsert org member %s (%s): %w", email, role.Slug, err)
			}

			// Insert workspace memberships (Omit ID so DB uses gen_random_uuid()):
			targetWsID := wsEngID
			if role.Slug == "finops_manager" {
				targetWsID = wsFinID
			}

			_, err = tx.ExecContext(ctx, `
				INSERT INTO workspace_members (workspace_id, user_id, role, created_at, updated_at)
				VALUES ($1, $2, $3, NOW(), NOW())
				ON CONFLICT (workspace_id, user_id) DO UPDATE SET
					role = EXCLUDED.role,
					updated_at = NOW()
			`, targetWsID, userID, role.Slug)
			if err != nil {
				return fmt.Errorf("upsert workspace member %s (%s): %w", email, targetWsID, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func PrintMatrixTable() {
	fmt.Println("\n=========================================================================================================")
	fmt.Println(" ROOZYLABS PRISM (v2.2.0) - ROLE & PLAN AUDIT MATRIX SEED COMPLETE")
	fmt.Println("=========================================================================================================")
	fmt.Printf("%-24s | %-12s | %-16s | %-12s | %-24s\n", "Email", "Plan Tier", "Role", "Spend Cap", "Password")
	fmt.Println("---------------------------------------------------------------------------------------------------------")

	for _, plan := range PlanSpecs {
		for _, role := range RoleSpecs {
			email := fmt.Sprintf("%s.%s@prism.local", role.Prefix, plan.Slug)
			spendCap := fmt.Sprintf("$%.0f/mo", plan.SpendCapMonthly)
			fmt.Printf("%-24s | %-12s | %-16s | %-12s | %-24s\n", email, plan.Tier, role.Slug, spendCap, SharedMatrixPassword)
		}
	}
	fmt.Println("=========================================================================================================")
}
