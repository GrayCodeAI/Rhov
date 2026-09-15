package cmd

import (
	"strings"
	"testing"

	"github.com/GrayCodeAI/rho/internal/engine/safety"

	rhoconfig "github.com/GrayCodeAI/rho/internal/config"
	"github.com/GrayCodeAI/rho/internal/engine"
)

func TestNormalizePermissionTier(t *testing.T) {
	level, label, ok := normalizePermissionTier("always_ask")
	if !ok || level != safety.AutonomySupervised || label != "Always Ask" {
		t.Fatalf("always_ask = (%v, %q, %v)", level, label, ok)
	}
	level, label, ok = normalizePermissionTier("operator")
	if !ok || level != safety.AutonomyFull || label != "Operator" {
		t.Fatalf("operator = (%v, %q, %v)", level, label, ok)
	}
	level, label, ok = normalizePermissionTier("auto")
	if !ok || level != safety.AutonomyYOLO || label != "Autonomous" {
		t.Fatalf("auto = (%v, %q, %v)", level, label, ok)
	}
}

func TestNormalizePermissionSandbox(t *testing.T) {
	mode, label, ok := normalizePermissionSandbox("workspace")
	if !ok || mode != "workspace" || label != "Workspace" {
		t.Fatalf("workspace = (%q, %q, %v)", mode, label, ok)
	}
	if _, _, ok := normalizePermissionSandbox("ghost"); ok {
		t.Fatal("expected invalid sandbox to fail")
	}
}

func TestEffectivePermissionRules(t *testing.T) {
	settings := rhoconfig.Settings{
		AutoAllow:       []string{"Read"},
		AllowedTools:    []string{"Bash(git:*)", "Read"},
		DisallowedTools: []string{"Bash(rm -rf *)"},
	}
	allow := effectiveAllowRules(settings)
	deny := effectiveDenyRules(settings)
	if len(allow) != 2 {
		t.Fatalf("allow len = %d, want 2 (%v)", len(allow), allow)
	}
	if len(deny) != 1 || deny[0] != "Bash(rm -rf *)" {
		t.Fatalf("deny = %v", deny)
	}
}

func TestAutonomyCenterSummary(t *testing.T) {
	sess := engine.NewSession("test", "test-model", "system", nil)
	sess.PermSvc().SetAutonomy(safety.AutonomySemi)
	sess.PermSvc().SetSpecStage(safety.SpecStageSpecify)
	model := &chatModel{
		session: sess,
		settings: rhoconfig.Settings{
			AllowedTools:    []string{"Bash(git:*)"},
			DisallowedTools: []string{"Bash(rm -rf *)"},
		},
	}
	out := autonomyCenterSummary(model)
	for _, fragment := range []string{"Autonomy Center", "Tier: Builder", "Spec stage: Specify", "Rules: 1 allow, 1 deny"} {
		if !strings.Contains(out, fragment) {
			t.Fatalf("summary %q missing %q", out, fragment)
		}
	}
}

func TestPermissionTierSettingValue(t *testing.T) {
	cases := []struct {
		level safety.AutonomyLevel
		want  int
	}{
		{safety.AutonomySupervised, 0},
		{safety.AutonomyBasic, 1},
		{safety.AutonomySemi, 2},
		{safety.AutonomyFull, 3},
		{safety.AutonomyYOLO, 4},
	}
	for _, c := range cases {
		if got := permissionTierSettingValue(c.level); got != c.want {
			t.Errorf("permissionTierSettingValue(%v) = %d, want %d", c.level, got, c.want)
		}
	}
}

// TestEffectivePermissionTier_ReadsThroughRealSession exercises
// effectivePermissionTier against a session built via engine.NewSession +
// PermSvc().SetAutonomy, the actual production wiring — unlike
// TestAutonomyCenterSummary's `&engine.Session{Perm: perm}` literal, whose
// Perm field is a distinct, unwired pointer from the perms service that
// PermSvc() actually reads (session.go:106 perms vs session.go:112 Perm).
// That literal's assignment is dead for this codepath and only "worked"
// because AutonomySemi happens to equal DefaultContainerAutonomy.
func TestEffectivePermissionTier_ReadsThroughRealSession(t *testing.T) {
	if got := effectivePermissionTier(nil); got != DefaultContainerAutonomy {
		t.Fatalf("nil session: got %v, want default %v", got, DefaultContainerAutonomy)
	}

	sess := engine.NewSession("", "test-model", "you are helpful", nil)
	if got := effectivePermissionTier(sess); got != DefaultContainerAutonomy {
		t.Fatalf("unset autonomy: got %v, want default %v", got, DefaultContainerAutonomy)
	}

	sess.PermSvc().SetAutonomy(safety.AutonomyFull)
	if got := effectivePermissionTier(sess); got != safety.AutonomyFull {
		t.Fatalf("explicit Full: got %v, want %v", got, safety.AutonomyFull)
	}
	sess.PermSvc().SetAutonomy(safety.AutonomySupervised)
	if got := effectivePermissionTier(sess); got != safety.AutonomySupervised {
		t.Fatalf("explicit Supervised: got %v, want %v", got, safety.AutonomySupervised)
	}
}

func TestPermissionBehaviorSummary(t *testing.T) {
	seen := make(map[string]bool)
	for _, level := range []safety.AutonomyLevel{
		safety.AutonomyBasic, safety.AutonomySemi, safety.AutonomyFull, safety.AutonomyYOLO,
	} {
		summary := permissionBehaviorSummary(level)
		if summary == "" {
			t.Errorf("level %v: empty summary", level)
		}
		if seen[summary] {
			t.Errorf("level %v: summary %q duplicates another tier's", level, summary)
		}
		seen[summary] = true
	}
}

func TestResetPermissionCenter(t *testing.T) {
	sess := engine.NewSession("", "test-model", "you are helpful", nil)
	sess.PermSvc().SetAutonomy(safety.AutonomyYOLO)
	sess.PermSvc().SetSpecStage(safety.SpecStageTasks)
	sess.PermSvc().SetDryRun(true)
	model := &chatModel{
		session: sess,
		settings: rhoconfig.Settings{
			Autonomy:        permissionTierSettingValue(safety.AutonomyYOLO),
			AutoAllow:       []string{"Read"},
			AllowedTools:    []string{"Bash(git:*)"},
			DisallowedTools: []string{"Bash(rm -rf *)"},
		},
	}

	resetPermissionCenter(model)

	if got := sess.PermSvc().Autonomy(); got != DefaultContainerAutonomy {
		t.Errorf("autonomy = %v, want default %v", got, DefaultContainerAutonomy)
	}
	if got := sess.PermSvc().SpecStage(); got != safety.SpecStageNone {
		t.Errorf("spec stage = %v, want None", got)
	}
	if sess.PermSvc().DryRun() {
		t.Error("dry-run should be cleared")
	}
	if model.settings.AutoAllow != nil || model.settings.AllowedTools != nil || model.settings.DisallowedTools != nil {
		t.Error("rule lists should be cleared")
	}
}

func TestHandleAutonomyCommand_Tier(t *testing.T) {
	sess := engine.NewSession("", "test-model", "you are helpful", nil)
	model := &chatModel{session: sess}

	updated, _ := model.handleAutonomyCommand([]string{"autonomy", "tier", "operator"})
	if got := updated.session.PermSvc().Autonomy(); got != safety.AutonomyFull {
		t.Fatalf("after tier operator: autonomy = %v, want Full", got)
	}
	if updated.settings.Autonomy != permissionTierSettingValue(safety.AutonomyFull) {
		t.Fatalf("settings.Autonomy = %d, want %d", updated.settings.Autonomy, permissionTierSettingValue(safety.AutonomyFull))
	}

	updated, _ = updated.handleAutonomyCommand([]string{"autonomy", "tier", "not-a-tier"})
	last := updated.messages[len(updated.messages)-1]
	if last.role != "error" {
		t.Fatalf("invalid tier: expected error message, got role %q", last.role)
	}
	if got := updated.session.PermSvc().Autonomy(); got != safety.AutonomyFull {
		t.Fatalf("invalid tier should not change autonomy: got %v, want Full unchanged", got)
	}

	updated, _ = updated.handleAutonomyCommand([]string{"autonomy", "tier"})
	last = updated.messages[len(updated.messages)-1]
	if last.role != "error" || !strings.Contains(last.content, "Usage:") {
		t.Fatalf("missing tier arg: expected usage error, got %+v", last)
	}
}
