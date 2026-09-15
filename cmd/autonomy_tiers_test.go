package cmd

import (
	"fmt"
	"testing"

	"github.com/GrayCodeAI/rho/internal/engine/safety"
)

func TestAutonomyTierNames(t *testing.T) {
	if got := autonomyTierName(safety.AutonomyBasic); got != "Scout" {
		t.Fatalf("Basic = %q, want Scout", got)
	}
	if got := autonomyTierName(safety.AutonomySemi); got != "Builder" {
		t.Fatalf("Semi = %q, want Builder", got)
	}
	if got := autonomyTierName(safety.AutonomyFull); got != "Operator" {
		t.Fatalf("Full = %q, want Operator", got)
	}
	if got := autonomyTierName(safety.AutonomyYOLO); got != "Autonomous" {
		t.Fatalf("YOLO = %q, want Autonomous", got)
	}
}

func TestNextAutonomyTier(t *testing.T) {
	if nextAutonomyTier(safety.AutonomyYOLO) != safety.AutonomyBasic {
		t.Fatal("expected Autonomous -> Scout wrap")
	}
	if nextAutonomyTier(safety.AutonomySemi) != safety.AutonomyFull {
		t.Fatal("expected Builder -> Operator")
	}
}

func TestAutonomyFromSettings(t *testing.T) {
	if autonomyFromSettings(2) != safety.AutonomySemi {
		t.Fatal("settings autonomy 2 should map to Builder/Semi")
	}
	if autonomyFromSettings(0) != 0 {
		t.Fatal("settings 0 should leave unset")
	}
}

func TestAutonomyTierColorsDistinct(t *testing.T) {
	levels := []safety.AutonomyLevel{
		safety.AutonomyBasic,
		safety.AutonomySemi,
		safety.AutonomyFull,
		safety.AutonomyYOLO,
	}
	seen := make(map[string]bool)
	for _, l := range levels {
		c := fmt.Sprint(autonomyTierColor(l))
		if seen[c] {
			t.Fatalf("duplicate color for tier %v", l)
		}
		seen[c] = true
	}
}
