package token

import (
	"fmt"
	"strings"
	"testing"
)

func TestCompressEmpty(t *testing.T) {
	out, stats := Compress("", 100)
	if out != "" {
		t.Errorf("Compress(\"\") = %q, want empty", out)
	}
	if stats.OriginalTokens != 0 || stats.FinalTokens != 0 {
		t.Errorf("Compress(\"\") stats = %+v, want zero", stats)
	}
}

func TestCompressLargeBudgetReturnsContent(t *testing.T) {
	text := strings.Repeat("the quick brown fox jumps over the lazy dog. ", 10)
	out, stats := Compress(text, len(text)*4)
	if out == "" {
		t.Error("Compress with a large budget returned empty text")
	}
	if stats.OriginalTokens <= 0 {
		t.Errorf("stats.OriginalTokens = %d, want > 0", stats.OriginalTokens)
	}
	if stats.FinalTokens <= 0 {
		t.Errorf("stats.FinalTokens = %d, want > 0", stats.FinalTokens)
	}
}

func TestCompressTinyBudgetReduces(t *testing.T) {
	text := strings.Repeat("the quick brown fox jumps over the lazy dog. ", 10)
	out, stats := Compress(text, 1)
	if len(out) >= len(text) {
		t.Errorf("Compress(budget=1) did not reduce: %d -> %d chars", len(text), len(out))
	}
	if stats.FinalTokens > stats.OriginalTokens {
		t.Errorf("FinalTokens %d > OriginalTokens %d", stats.FinalTokens, stats.OriginalTokens)
	}
	if stats.ReductionPercent <= 0 {
		t.Errorf("ReductionPercent = %f, want > 0", stats.ReductionPercent)
	}
}

func TestCompressCollapsesRepeatedLines(t *testing.T) {
	text := strings.Join([]string{
		"start",
		"repeat",
		"repeat",
		"repeat",
		"repeat",
		"end",
	}, "\n")
	out, _ := Compress(text, 10000)
	if !strings.Contains(out, "repeated line") {
		t.Errorf("expected repeated-line marker, got:\n%s", out)
	}
}

func TestCompressForContext(t *testing.T) {
	text := strings.Repeat("alpha beta gamma delta ", 50)
	out, n := CompressForContext(text, 5)
	if n != EstimateTokens(out) {
		t.Errorf("CompressForContext final count %d != EstimateTokens(out) %d", n, EstimateTokens(out))
	}
}

func TestStatsHardTruncated(t *testing.T) {
	// No savings: never hard-truncated.
	if (Stats{}).HardTruncated() {
		t.Error("zero Stats should not be hard-truncated")
	}
	// Budget layer dominates the savings.
	budget := Stats{
		TokensSaved: 100,
		Layers:      map[string]LayerStat{"budget": {TokensSaved: 60}},
	}
	if !budget.HardTruncated() {
		t.Error("budget-dominated Stats should be hard-truncated")
	}
	// Structural layers dominate.
	structural := Stats{
		TokensSaved: 100,
		Layers:      map[string]LayerStat{"budget": {TokensSaved: 10}},
	}
	if structural.HardTruncated() {
		t.Error("structurally-compressed Stats should not be hard-truncated")
	}
}

func TestCompressCollapsesRepeatingCycle(t *testing.T) {
	text := strings.Repeat("user: explain the auth flow\nassistant: it uses JWT tokens.\n", 20)
	out, stats := Compress(text, 10000)
	if !strings.Contains(out, "repeated blocks") {
		t.Errorf("expected a cycle marker, got:\n%s", out)
	}
	if stats.Layers["cycle"].TokensSaved <= 0 {
		t.Errorf("cycle layer saved nothing: %+v", stats.Layers)
	}
	if stats.HardTruncated() {
		t.Error("cycle compression should be structural, not hard truncation")
	}
}

func TestCompressPreservesHeadAndTail(t *testing.T) {
	var b strings.Builder
	b.WriteString("HEAD: the goal is to refactor auth\n")
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&b, "filler line number %d with distinct content %d\n", i, i*17)
	}
	b.WriteString("TAIL: most recent instruction\n")
	out, _ := Compress(b.String(), 200)
	if !strings.Contains(out, "HEAD: the goal is to refactor auth") {
		t.Errorf("head was dropped:\n%s", out)
	}
	if !strings.Contains(out, "TAIL: most recent instruction") {
		t.Errorf("tail was dropped:\n%s", out)
	}
	if !strings.Contains(out, "middle elided") {
		t.Errorf("expected a middle-elision marker:\n%s", out)
	}
}

func TestCompressCycleDoesNotCollapseDistinctLines(t *testing.T) {
	text := "alpha one\nalpha two\nalpha three\nalpha four\n"
	out, _ := Compress(text, 10000)
	if strings.Contains(out, "repeated blocks") {
		t.Errorf("distinct lines were collapsed as a cycle:\n%s", out)
	}
}
