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
	UserID    string
	Role      string
	AgentID   string
	AgentName string
	// Agent is the agent the gateway already resolved for this request. When
	// it is nil but AgentID or AgentName is set, admission resolves it.
	Agent         *models.Agent
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
	if res, err := a.evaluateAgent(ctx, req); err != nil || res != nil {
		return res, err
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
					Reason:       budgetExceededReason(budgetStatus),
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

// evaluateAgent denies a request whose agent may not use the model or tool,
// or whose named agent does not exist. It returns nil when the request may
// continue.
func (a *AdmissionController) evaluateAgent(ctx context.Context, req AdmissionRequest) (*AdmissionResult, error) {
	ref := req.AgentID
	if ref == "" {
		ref = req.AgentName
	}
	if req.Agent == nil && ref == "" {
		return nil, nil
	}

	agent := req.Agent
	if agent == nil {
		if a.agentGovernance == nil {
			return nil, nil
		}
		found, reason, err := a.agentGovernance.resolve(ctx, req.UserID, ref)
		if err != nil {
			return nil, fmt.Errorf("agent governance: %w", err)
		}
		if found == nil {
			return agentDenied("agent_not_found", reason), nil
		}
		agent = found
	}

	if req.ModelSlug != "" {
		if res := CheckAgentModelAccess(agent, req.ModelSlug); !res.ModelAllowed {
			return agentDenied("agent_model_denied", res.Reason), nil
		}
	}
	if req.ToolName != "" {
		if res := CheckAgentToolAccess(agent, req.ToolName); !res.ToolAllowed {
			return agentDenied("agent_tool_denied", res.Reason), nil
		}
	}
	return nil, nil
}

func agentDenied(code, reason string) *AdmissionResult {
	return &AdmissionResult{
		Decision:   AdmissionDeny,
		HTTPStatus: http.StatusForbidden,
		ErrorCode:  code,
		Reason:     reason,
	}
}

func budgetExceededReason(st *models.BudgetStatus) string {
	limit := st.Budget.MonthlyLimit
	if st.LimitPeriod == "daily" {
		limit = st.Budget.DailyLimit
	}
	return fmt.Sprintf("%s budget limit ($%.2f) of %q exceeded", st.LimitPeriod, limit, st.Budget.Name)
}
