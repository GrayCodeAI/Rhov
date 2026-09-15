package tool

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSleepSchemaProvider(t *testing.T) {
	var _ SchemaProvider = SleepTool{}
	params := SleepTool{}.Parameters()
	if params["type"] != "object" {
		t.Fatalf("Parameters type = %v, want object", params["type"])
	}
	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("properties missing")
	}
	sec, ok := props["seconds"].(map[string]interface{})
	if !ok || sec["type"] != "number" {
		t.Fatalf("seconds prop = %v, want number type", props["seconds"])
	}
	if req, ok := params["required"].([]string); !ok || len(req) != 1 || req[0] != "seconds" {
		t.Fatalf("required = %v, want [seconds]", params["required"])
	}
}

func TestSleepExecute(t *testing.T) {
	ctx := context.Background()
	out, err := (SleepTool{}).Execute(ctx, json.RawMessage(`{"seconds": 0.05}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(out, "Slept") {
		t.Fatalf("unexpected output %q", out)
	}
}

func TestSleepExecuteValidation(t *testing.T) {
	ctx := context.Background()
	if _, err := (SleepTool{}).Execute(ctx, json.RawMessage(`{"seconds": 0}`)); err == nil {
		t.Fatal("expected error for non-positive seconds")
	}
	if _, err := (SleepTool{}).Execute(ctx, json.RawMessage(`not json`)); err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if _, err := (SleepTool{}).Execute(ctx, nil); err == nil {
		t.Fatal("expected error for missing input")
	}
	// Cap: 300s max sleeps must not actually sleep 300s here; verify the cap
	// path via context cancellation instead of wall time.
	capped, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (SleepTool{}).Execute(capped, json.RawMessage(`{"seconds": 300}`)); err == nil {
		t.Fatal("expected cancellation error for capped sleep")
	}
}

func TestSleepRespectsContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := (SleepTool{}).Execute(ctx, json.RawMessage(`{"seconds": 30}`))
	if err == nil {
		t.Fatal("expected context error")
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("sleep did not respect context cancellation")
	}
}
