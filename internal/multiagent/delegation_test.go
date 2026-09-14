package mission

import (
	"context"
	"strings"
	"testing"
)

func TestDelegatedPolicy_DenyAllGateDeterministicRejection(t *testing.T) {
	gate := DenyAllGate()
	if gate == nil {
		t.Fatal("DenyAllGate() returned nil")
	}

	ctx := context.Background()
	err := gate.Check(ctx, "Bash", "destructive command rm -rf /")
	if err == nil {
		t.Fatal("DenyAllGate.Check() succeeded, want deterministic rejection")
	}
	if !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("error = %v, want tool rejected error", err)
	}
}
