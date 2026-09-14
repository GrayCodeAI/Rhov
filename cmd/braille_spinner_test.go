package cmd

import (
	"strings"
	"testing"

	"github.com/GrayCodeAI/rho/internal/ui/icons"
)

func TestBrailleSpinner_Tick(t *testing.T) {
	s := NewBrailleSpinner(SpinnerBraille, "Thinking")
	f1 := s.Frame()
	f2 := s.Tick()
	if f1 == f2 {
		t.Error("expected different frames after tick")
	}
	// Glyph uses the 20-color wave (not fixed orange).
	if !frameContainsSpinnerWave(f1) {
		t.Error("expected wave-colored spinner glyph")
	}
	// Verb + dots use the 20-color wave palette.
	if !frameContainsSpinnerWave(f1) {
		t.Error("expected wave-colored verb label")
	}
	if !strings.Contains(f1, icons.CircleFilled()) {
		t.Error("expected filled typing dot")
	}
}

func TestBrailleSpinner_AllStyles(t *testing.T) {
	styles := []SpinnerStyle{
		SpinnerBraille, SpinnerBrailleWave, SpinnerRho, SpinnerDNA,
		SpinnerScan, SpinnerPulse, SpinnerSnake, SpinnerOrbit,
		SpinnerWing, SpinnerTalons,
	}
	for _, style := range styles {
		s := NewBrailleSpinner(style, "test")
		f := s.Frame()
		if f == "" {
			t.Errorf("style %s produced empty frame", style)
		}
	}
}

func TestBrailleSpinner_Random(t *testing.T) {
	s := NewBrailleSpinner(SpinnerRandom, "Loading")
	f := s.Frame()
	if f == "" {
		t.Error("random spinner produced empty frame")
	}
}

func TestRhoSpinner_Frames(t *testing.T) {
	if len(rhoSpinnerGlyphs) != 4 {
		t.Fatalf("expected 4 compass glyphs, got %d", len(rhoSpinnerGlyphs))
	}
	if rhoSpinnerGlyphs[0] != "◐" {
		t.Fatalf("expected first compass frame ◐, got %q", rhoSpinnerGlyphs[0])
	}
	s := NewBrailleSpinner(SpinnerRho, "Working")
	f0 := s.Frame()
	if !strings.Contains(f0, "◐") {
		t.Fatalf("expected compass glyph, got %q", f0)
	}
	s.Tick()
	s.Tick()
	s.Tick()
	if !strings.Contains(s.Frame(), "◒") {
		t.Fatalf("expected frame cycle to reach ◒, got %q", s.Frame())
	}
}

func TestRhoQuadBlock_LegacyFrames(t *testing.T) {
	s := NewBrailleSpinner(SpinnerRhoQuad, "Working")
	if !strings.Contains(s.Frame(), "▛") {
		t.Fatalf("expected QuadBlock glyph, got %q", s.Frame())
	}
}

func TestRhoAnimatedDots_PresentInFrame(t *testing.T) {
	s := NewBrailleSpinner(SpinnerRho, "Crafting")
	f := s.Frame()
	// Three progress dots ride after the verb: one bright, two dim.
	total := strings.Count(f, icons.CircleFilled()) + strings.Count(f, icons.CircleOutline())
	if total != rhoTypingDots {
		t.Errorf("expected %d trailing circle-dots, got %d in %q", rhoTypingDots, total, f)
	}
	// Tick advances the highlighted dot position.
	idxBefore := s.dots
	s.Tick()
	if s.dots == idxBefore {
		t.Error("expected dot phase to advance on tick")
	}
}
