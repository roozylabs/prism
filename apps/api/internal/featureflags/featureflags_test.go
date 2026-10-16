package featureflags

import (
	"testing"
)

func TestFeatureFlags_DefaultsAndPlanEntitlements(t *testing.T) {
	mgr := GetManager()

	// 1. SmartRouterAuto should be enabled for all plans
	if !mgr.IsEnabled(FlagSmartRouterAuto, "free") {
		t.Errorf("expected smart_router_auto to be enabled for free tier")
	}
	if !mgr.IsEnabled(FlagSmartRouterAuto, "enterprise") {
		t.Errorf("expected smart_router_auto to be enabled for enterprise tier")
	}

	// 2. PaperclipOrchestrator should be disabled for free/pro, enabled for team/enterprise
	if mgr.IsEnabled(FlagPaperclipOrchestrator, "free") {
		t.Errorf("expected paperclip_orchestrator to be disabled for free tier")
	}
	if mgr.IsEnabled(FlagPaperclipOrchestrator, "pro") {
		t.Errorf("expected paperclip_orchestrator to be disabled for pro tier")
	}
	if !mgr.IsEnabled(FlagPaperclipOrchestrator, "team") {
		t.Errorf("expected paperclip_orchestrator to be enabled for team tier")
	}
	if !mgr.IsEnabled(FlagPaperclipOrchestrator, "enterprise") {
		t.Errorf("expected paperclip_orchestrator to be enabled for enterprise tier")
	}

	// 3. MerkleAuditVerification should only be enabled for enterprise
	if mgr.IsEnabled(FlagMerkleAuditVerification, "team") {
		t.Errorf("expected merkle_audit_verification to be disabled for team tier")
	}
	if !mgr.IsEnabled(FlagMerkleAuditVerification, "enterprise") {
		t.Errorf("expected merkle_audit_verification to be enabled for enterprise tier")
	}
}

func TestFeatureFlags_EnvVarOverride(t *testing.T) {
	mgr := GetManager()

	// Override Paperclip for free tier via t.Setenv
	t.Setenv("FEATURE_FLAG_PAPERCLIP_ORCHESTRATOR", "true")

	if !mgr.IsEnabled(FlagPaperclipOrchestrator, "free") {
		t.Errorf("expected paperclip_orchestrator to be overridden to true by env var")
	}

	// Disable SmartRouter for all plans via t.Setenv
	t.Setenv("FEATURE_FLAG_SMART_ROUTER", "false")

	if mgr.IsEnabled(FlagSmartRouterAuto, "enterprise") {
		t.Errorf("expected smart_router_auto to be overridden to false by env var")
	}
}

func TestBillingIsOffUnlessSwitchedOn(t *testing.T) {
	m := GetManager()
	for _, plan := range []string{"", "free", "pro", "team", "enterprise"} {
		if m.IsEnabled(FlagBilling, plan) {
			t.Fatalf("billing enabled for plan %q without FEATURE_FLAG_BILLING", plan)
		}
	}
	t.Setenv("FEATURE_FLAG_BILLING", "true")
	if !m.IsEnabled(FlagBilling, "") {
		t.Fatal("FEATURE_FLAG_BILLING=true does not enable billing")
	}
}
