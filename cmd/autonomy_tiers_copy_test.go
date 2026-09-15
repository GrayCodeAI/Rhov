package cmd

import (
	"strings"
	"testing"

	"github.com/GrayCodeAI/rho/internal/engine/safety"
)

func TestAutonomyTierDescriptions_PlainLanguage(t *testing.T) {
	cases := []struct {
		level safety.AutonomyLevel
		need  []string
	}{
		{safety.AutonomyBasic, []string{"Explore only", "commands ask"}},
		{safety.AutonomySemi, []string{"File changes auto-approve", "commands ask"}},
		{safety.AutonomyFull, []string{"Commands auto-run", "risky"}},
		{safety.AutonomyYOLO, []string{"Minimal prompts", "highest-risk"}},
	}
	for _, tc := range cases {
		desc := autonomyTierDescription(tc.level)
		for _, fragment := range tc.need {
			if !strings.Contains(desc, fragment) {
				t.Fatalf("level %v description %q missing %q", tc.level, desc, fragment)
			}
		}
	}
}

func TestFormatAutonomyTierMessage_NoArrowJargon(t *testing.T) {
	msg := formatAutonomyTierMessage(safety.AutonomyFull)
	if strings.Contains(msg, "→") {
		t.Fatalf("expected no arrow jargon, got %q", msg)
	}
	if len(msg) > 120 {
		t.Fatalf("message too long (%d chars): %q", len(msg), msg)
	}
	if !strings.Contains(msg, "Operator") {
		t.Fatalf("expected tier name in message, got %q", msg)
	}
}
