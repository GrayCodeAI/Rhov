package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/GrayCodeAI/rho/internal/intelligence/memory"
)

func TestMemoryServiceRecallContextFallsBackToRecaller(t *testing.T) {
	mem := &mockMemoryRecaller{}
	service := NewMemoryService(nil).WithMemory(mem)

	got := service.RecallContext(context.Background(), "question", 128)
	if got != "## Relevant Memories\nrecalled: question" {
		t.Fatalf("RecallContext() = %q", got)
	}
}

func TestMemoryServiceRecallContextIsEmptyWithoutBackends(t *testing.T) {
	if got := NewMemoryService(nil).RecallContext(context.Background(), "question", 128); got != "" {
		t.Fatalf("RecallContext() = %q, want empty", got)
	}
}

// TestMemoryServiceRoundTripThroughLocalManager proves the post-harrier memory
// path works end to end: a value remembered through the real EnhancedMemoryManager
// is recalled back through MemoryService.
func TestMemoryServiceRoundTripThroughLocalManager(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("RHO_STATE_DIR", t.TempDir())

	mgr := memory.NewEnhancedMemoryManager(t.TempDir())
	service := NewMemoryService(nil).WithMemory(mgr).WithEnhanced(mgr)

	const fact = "the staging deploy uses the eu-west cluster"
	service.Remember(context.Background(), fact, "core")

	got := service.RecallContext(context.Background(), "staging deploy cluster", 2000)
	if got == "" {
		t.Fatal("RecallContext() returned empty after Remember()")
	}
	if !strings.Contains(got, "eu-west") {
		t.Fatalf("RecallContext() = %q, want it to contain the remembered fact", got)
	}
}
