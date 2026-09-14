package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSpecForLinks creates an active spec with the given spec.md content and
// returns the working directory.
func writeSpecForLinks(t *testing.T, ctx context.Context, specContent string) string {
	t.Helper()
	dir := withTempCwd(t)
	// Create the spec through the real tool so the slug is session-scoped.
	input, _ := json.Marshal(map[string]string{"title": "links test", "spec": specContent})
	if _, err := (SpecifyTool{}).Execute(ctx, input); err != nil {
		t.Fatalf("SpecifyTool: %v", err)
	}
	return dir
}

func TestSpecLinksCheckReportsCoverage(t *testing.T) {
	ctx := withSpecSession(context.Background())
	dir := writeSpecForLinks(t, ctx, "REQ-1.1.1 must work\nREQ-1.1.2 must also work\n")

	// Cite REQ-1.1.1 from a source file and a test file.
	if err := os.WriteFile(filepath.Join(dir, "impl.go"), []byte("// REQ-1.1.1\npackage main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "impl_test.go"), []byte("// REQ-1.1.1\npackage main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := SpecLinksTool{}.Execute(ctx, json.RawMessage(`{"action":"check"}`))
	if err != nil {
		t.Fatalf("SpecLinks check: %v", err)
	}
	if !strings.Contains(out, "REQ-1.1.1") || !strings.Contains(out, "REQ-1.1.2") {
		t.Fatalf("output missing requirements:\n%s", out)
	}
	if !strings.Contains(out, "1/2 requirements cited") {
		t.Errorf("expected coverage summary 1/2 cited, got:\n%s", out)
	}
	if !strings.Contains(out, "1/2 covered by a test") {
		t.Errorf("expected test coverage summary 1/2, got:\n%s", out)
	}
}

func TestSpecLinksAddWritesTraceability(t *testing.T) {
	ctx := withSpecSession(context.Background())
	dir := writeSpecForLinks(t, ctx, "REQ-2.1.1 alpha\nREQ-2.1.2 beta\n")

	out, err := SpecLinksTool{}.Execute(ctx, json.RawMessage(`{"action":"add"}`))
	if err != nil {
		t.Fatalf("SpecLinks add: %v", err)
	}
	if !strings.Contains(out, "Added 2 requirement link(s)") {
		t.Errorf("unexpected add result: %q", out)
	}

	entries, err := os.ReadDir(filepath.Join(dir, ".rho", "specs"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected one spec dir, got %v (%v)", entries, err)
	}
	tasks, err := os.ReadFile(filepath.Join(dir, ".rho", "specs", entries[0].Name(), "tasks.md"))
	if err != nil {
		t.Fatalf("read tasks.md: %v", err)
	}
	for _, want := range []string{"REQ-2.1.1", "REQ-2.1.2", "Traceability"} {
		if !strings.Contains(string(tasks), want) {
			t.Errorf("tasks.md missing %q:\n%s", want, tasks)
		}
	}

	// Second run must be idempotent.
	out2, err := SpecLinksTool{}.Execute(ctx, json.RawMessage(`{"action":"add"}`))
	if err != nil {
		t.Fatalf("SpecLinks add (2): %v", err)
	}
	if !strings.Contains(out2, "already linked") {
		t.Errorf("expected idempotent add, got: %q", out2)
	}
}

func TestSpecLinksNoActiveSpec(t *testing.T) {
	withTempCwd(t)
	ctx := withSpecSession(context.Background())
	if _, err := (SpecLinksTool{}).Execute(ctx, json.RawMessage(`{"action":"check"}`)); err == nil {
		t.Fatal("expected error with no active spec")
	}
}

func TestSpecLinksUnknownAction(t *testing.T) {
	ctx := withSpecSession(context.Background())
	writeSpecForLinks(t, ctx, "REQ-3.1.1 x\n")
	if _, err := (SpecLinksTool{}).Execute(ctx, json.RawMessage(`{"action":"bogus"}`)); err == nil {
		t.Fatal("expected error for unknown action")
	}
}
