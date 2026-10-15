package proxy

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/roozylabs/prism/internal/models"
)

type AdmissionDecision string

const (
	AdmissionAllow     AdmissionDecision = "ALLOW"
	AdmissionDeny      AdmissionDecision = "DENY"
	AdmissionDowngrade AdmissionDecision = "DOWNGRADE"
	AdmissionWarn      AdmissionDecision = "WARN"
)

type AdmissionResult struct {
	Decision     AdmissionDecision
	HTTPStatus   int
	ErrorCode    string
	Reason       string
	PolicyID     string
	PolicyName   string
	BudgetStatus string
}

type AdmissionRequest struct {
	UserID        string
	Role          string
	AgentID       string
	AgentName     string
	ModelSlug     string
	ToolName      string
	ResourceName  string
	PromptPayload string
	TenantCtx     models.TenantContext
}

type AdmissionController struct {
	rbacEngine      *RBACEngine
	agentGovernance *AgentGovernanceEngine
	quotaRepo       QuotaEvaluator
	budgetManager   *BudgetManager
}

func NewAdmissionController(
	rbacEngine *RBACEngine,
	agentGovernance *AgentGovernanceEngine,
	quotaRepo QuotaEvaluator,
	budgetManager *BudgetManager,
) *AdmissionController {
	return &AdmissionController{
		rbacEngine:      rbacEngine,
		agentGovernance: agentGovernance,
		quotaRepo:       quotaRepo,
		budgetManager:   budgetManager,
	}
}

func (a *AdmissionController) Evaluate(ctx context.Context, req AdmissionRequest) (*AdmissionResult, error) {
	// 0. Prompt Safety & System Boundary Leak Check
	if req.PromptPayload != "" {
		lowerPrompt := strings.ToLower(req.PromptPayload)
		restrictedKeywords := []string{
			"dump database password",
			"show env jwt_secret",
			"reveal backend encryption_key",
			"explain internal gateway architecture behind the scene",
			"show system environment variables",
			"reveal database connection string",
		}
		for _, kw := range restrictedKeywords {
			if strings.Contains(lowerPrompt, kw) {
				return &AdmissionResult{
					Decision:   AdmissionDeny,
					HTTPStatus: http.StatusBadRequest,
					ErrorCode:  "security_prompt_denied",
					Reason:     "prompt payload contains restricted system boundary probe query",
				}, nil
			}
		}
	}

	// 1. RBAC Governance Check
	if a.rbacEngine != nil {
		evalReq := models.RBACEvaluationRequest{
			Role:         req.Role,
			AgentName:    req.AgentName,
			ModelSlug:    req.ModelSlug,
			ToolName:     req.ToolName,
			ResourceName: req.ResourceName,
			UserPrompt:   req.PromptPayload,
		}
		rbacRes, err := a.rbacEngine.Evaluate(ctx, req.UserID, evalReq)
		if err != nil {
			return nil, fmt.Errorf("rbac evaluate: %w", err)
		}
		if !rbacRes.Allowed {
			policyID := ""
			policyName := ""
			if rbacRes.MatchedPolicy != nil {
				policyID = rbacRes.MatchedPolicy.ID
				policyName = rbacRes.MatchedPolicy.Name
			}
			return &AdmissionResult{
				Decision:   AdmissionDeny,
				HTTPStatus: http.StatusForbidden,
				ErrorCode:  "governance_policy_denied",
				Reason:     rbacRes.Reason,
				PolicyID:   policyID,
				PolicyName: policyName,
			}, nil
		}
	}

	// 2. Agent Governance Check (Model & Tool access)
	if a.agentGovernance != nil && req.AgentName != "" {
		if req.ModelSlug != "" {
			agRes, err := a.agentGovernance.ValidateAgentModelAccess(ctx, req.UserID, req.AgentName, req.ModelSlug)
			if err != nil {
				return nil, fmt.Errorf("agent model governance: %w", err)
			}
			if !agRes.ModelAllowed {
				return &AdmissionResult{
					Decision:   AdmissionDeny,
					HTTPStatus: http.StatusForbidden,
					ErrorCode:  "agent_model_denied",
					Reason:     agRes.Reason,
				}, nil
			}
		}

		if req.ToolName != "" {
			agToolRes, err := a.agentGovernance.ValidateAgentToolAccess(ctx, req.UserID, req.AgentName, req.ToolName)
			if err != nil {
				return nil, fmt.Errorf("agent tool governance: %w", err)
			}
			if !agToolRes.ToolAllowed {
				return &AdmissionResult{
					Decision:   AdmissionDeny,
					HTTPStatus: http.StatusForbidden,
					ErrorCode:  "agent_tool_denied",
					Reason:     agToolRes.Reason,
				}, nil
			}
		}
	}

	// 3. Tenant Quota Check (Organization & Workspace quotas)
	if a.quotaRepo != nil {
		orgID := req.TenantCtx.OrgID
		if orgID != "" {
			quotaRes, err := a.quotaRepo.EvaluateQuota(ctx, "organization", orgID)
			if err == nil && !quotaRes.Allowed {
				return &AdmissionResult{
					Decision:   AdmissionDeny,
					HTTPStatus: http.StatusTooManyRequests,
					ErrorCode:  "quota_exceeded_error",
					Reason:     quotaRes.Reason,
				}, nil
			}
		}
	}

	// 4. Multi-Level Budget Check
	var budgetStatusStr string
	if a.budgetManager != nil {
		budgetStatus, err := a.budgetManager.GetStatus(ctx, req.UserID)
		if err == nil && budgetStatus != nil {
			budgetStatusStr = budgetStatus.Status
			if budgetStatus.Status == "exceeded" && budgetStatus.Budget != nil && budgetStatus.Budget.HardLimit {
				return &AdmissionResult{
					Decision:     AdmissionDeny,
					HTTPStatus:   http.StatusTooManyRequests,
					ErrorCode:    "budget_exceeded_error",
					Reason:       fmt.Sprintf("Monthly or daily budget limit ($%.2f) exceeded", budgetStatus.Budget.MonthlyLimit),
					BudgetStatus: budgetStatus.Status,
				}, nil
			}

			if budgetStatus.Status == "critical" {
				return &AdmissionResult{
					Decision:     AdmissionDowngrade,
					HTTPStatus:   http.StatusOK,
					Reason:       "Budget usage critical; model downgrade penalty applied",
					BudgetStatus: budgetStatus.Status,
				}, nil
			}

			if budgetStatus.Status == "warning" {
				return &AdmissionResult{
					Decision:     AdmissionWarn,
					HTTPStatus:   http.StatusOK,
					Reason:       "Budget usage approaching limit threshold",
					BudgetStatus: budgetStatus.Status,
				}, nil
			}
		}
	}

	return &AdmissionResult{
		Decision:     AdmissionAllow,
		HTTPStatus:   http.StatusOK,
		BudgetStatus: budgetStatusStr,
	}, nil
}
