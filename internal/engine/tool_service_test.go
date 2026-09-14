package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GrayCodeAI/rho/internal/tool"
	"github.com/GrayCodeAI/rho/internal/types"
)

func TestToolServiceNormalizeOutputKeepsSmallResults(t *testing.T) {
	service := NewToolService(nil)
	const want = "short tool result"
	if got := service.NormalizeOutput(want, "Read", "call-1", 128_000); got != want {
		t.Fatalf("NormalizeOutput changed a small result: got %q, want %q", got, want)
	}
}

func TestToolServicePostProcessRunsPostExecuteStage(t *testing.T) {
	service := NewToolService(nil)
	p := service.Pipeline()
	if p == nil {
		t.Fatal("lazy Pipeline() returned nil; SetPipeline/DefaultToolPipeline seam broken")
	}
	p.Register(tool.StagePostExecute, func(ctx context.Context, req tool.ToolRequest, res *tool.ToolResult, next func() error) error {
		if err := next(); err != nil {
			return err
		}
		if req.Tool == nil {
			t.Fatal("StagePostExecute saw a nil resolved Tool")
		}
		res.Output = "rewritten by post-execute stage"
		res.IsError = false
		return nil
	})

	stub := stubTool{}
	got := service.PostProcess(context.Background(), toolExecResult{
		tc:     types.ToolCall{Name: "Read", ID: "r1"},
		tool:   stub,
		output: "raw output",
		isErr:  true,
	}, 0, "", 128_000)

	if got.isErr {
		t.Fatal("post-execute stage did not clear error")
	}
	if got.output != "rewritten by post-execute stage" {
		t.Fatalf("output = %q, want rewritten", got.output)
	}
}

type stubTool struct{}

func (stubTool) Name() string                                             { return "Stub" }
func (stubTool) Description() string                                      { return "stub" }
func (stubTool) Parameters() map[string]interface{}                       { return map[string]interface{}{} }
func (stubTool) Execute(context.Context, json.RawMessage) (string, error) { return "", nil }

func TestDefaultToolPipelineIsEmptyPassThrough(t *testing.T) {
	p := DefaultToolPipeline()
	if p == nil {
		t.Fatal("DefaultToolPipeline returned nil")
	}
	// An empty pipeline must be a strict pass-through: no registered interceptor,
	// so Run delegates immediately without modifying the provider result.
	res := &tool.ToolResult{Output: "unchanged", IsError: false}
	if err := p.Run(tool.StagePostExecute, context.Background(), tool.ToolRequest{}, res); err != nil {
		t.Fatalf("empty pipeline Run: %v", err)
	}
	if res.Output != "unchanged" {
		t.Fatalf("output = %q, want unchanged", res.Output)
	}
}

func TestToolServiceSemanticIndexSearchesWorkingDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "auth.go"), []byte(
		"package auth\n\nfunc ValidateToken(token string) bool { return token != \"\" }\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "unrelated.go"), []byte(
		"package other\n\nfunc AddNumbers(a, b int) int { return a + b }\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}

	service := NewToolService(nil).WithExecutionDeps(toolExecutionDeps{workingDir: dir})
	idx, err := service.semanticIndex()
	if err != nil {
		t.Fatalf("semanticIndex: %v", err)
	}
	if idx.Size() == 0 {
		t.Fatal("semantic index is empty")
	}
	results := idx.Search("validate token", 5)
	if len(results) == 0 {
		t.Fatal("semantic search returned no results")
	}
	if !strings.Contains(results[0].Content, "ValidateToken") {
		t.Errorf("top result = %q, want the token validator", results[0].Content)
	}

	// Refresh must rebuild cleanly.
	if err := service.RefreshCodeIndex(); err != nil {
		t.Fatalf("RefreshCodeIndex: %v", err)
	}
}
