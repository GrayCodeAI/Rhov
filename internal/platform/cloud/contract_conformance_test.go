package cloud

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	graphcontracts "github.com/GrayCodeAI/rho/internal/contracts/graph"
	"github.com/GrayCodeAI/rho/internal/executiongraph"
)

// The IDs GrayCode Cloud issues are 32 lowercase hex characters (newId()).
const (
	contractDeviceID  = "0123456789abcdef0123456789abcdef"
	contractProjectID = "fedcba9876543210fedcba9876543210"
)

// TestContractWorkerIsStrict proves the stand-in Worker rejects what the real
// schemas reject, so the conformance tests below are meaningful.
func TestContractWorkerIsStrict(t *testing.T) {
	worker := newContractWorker(t)
	validUsage := `"eventId":"exec-1727400000000-0123456789abcdef","deviceId":"` + contractDeviceID + `","projectId":"` + contractProjectID + `","tokensUsed":1,"occurredAt":"2026-09-27T12:00:00Z"`
	rejected := map[string][2]string{
		"start with legacy rhoVersion": {"/v1/auth/device/start", `{"label":"l","platform":"linux","rhoVersion":"0.3.0"}`},
		"start label too long":         {"/v1/auth/device/start", `{"label":"` + strings.Repeat("x", 101) + `","platform":"linux","graycodeVersion":"0.3.0"}`},
		"poll unknown key":             {"/v1/auth/device/poll", `{"deviceCode":"dc_0123456789abcdefghij","extra":1}`},
		"usage retired capability":     {"/v1/usage", `{` + validUsage + `,"capability":"swift"}`},
		"usage duration over 24h":      {"/v1/usage", `{` + validUsage + `,"capability":"rho","durationMs":86400001}`},
		"usage short opaque id":        {"/v1/usage", strings.Replace(`{`+validUsage+`,"capability":"rho"}`, "exec-1727400000000-0123456789abcdef", "exec-1", 1)},
		"usage fractional tokens":      {"/v1/usage", strings.Replace(`{`+validUsage+`,"capability":"rho"}`, `"tokensUsed":1`, `"tokensUsed":1.5`, 1)},
		"usage unknown key":            {"/v1/usage", `{` + validUsage + `,"capability":"rho","rhoVersion":"0.3.0"}`},
		"delivery bad ci status":       {"/v1/delivery-context", `{"projectId":"` + contractProjectID + `","repository":{"provider":"github","externalId":"1","name":"a/b"},"ciRun":{"provider":"github","externalId":"1","status":"passed"}}`},
		"graph tenant scope":           {"/v1/graph/sync", `{"syncId":"graph_0123456789abcdef","projectId":"` + contractProjectID + `","graph":{"schema_version":"rho.graph/v1","generated_at":"2026-09-27T12:00:00Z","scope":{"tenant_id":"t"},"nodes":[],"edges":[],"events":[]}}`},
		"graph unknown key":            {"/v1/graph/sync", `{"syncId":"graph_0123456789abcdef","projectId":"` + contractProjectID + `","graph":{"schema_version":"rho.graph/v1","generated_at":"2026-09-27T12:00:00Z","nodes":[],"edges":[],"events":[],"extra":true}}`},
	}
	for name, request := range rejected {
		if status := worker.postRaw(t, request[0], request[1]); status != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, status)
		}
	}
	if status := worker.postRaw(t, "/v1/usage", `{`+validUsage+`,"capability":"rho"}`); status != http.StatusAccepted {
		t.Fatalf("valid usage status = %d, want 202", status)
	}
}

func TestDeviceLoginConformsToContract(t *testing.T) {
	worker := newContractWorker(t)
	client := worker.client()
	for _, args := range [][3]string{
		{"lakshman-mbp.local", "darwin", "0.3.0"},
		{strings.Repeat("\U0001F680", 80), "linux", "v0.3.0-12-g13e05ceb-dirty+" + strings.Repeat("x", 60)},
		{"", "", ""},
	} {
		start, err := client.StartDeviceLogin(context.Background(), args[0], args[1], args[2])
		if err != nil {
			t.Fatalf("StartDeviceLogin(%q) = %v", args, err)
		}
		poll, err := client.PollDeviceLogin(context.Background(), start.DeviceCode)
		if err != nil || poll.Status != DeviceLoginApproved {
			t.Fatalf("PollDeviceLogin = %+v, %v", poll, err)
		}
	}
}

func TestUsageEventConformsToContract(t *testing.T) {
	worker := newContractWorker(t)
	started := time.Date(2026, time.September, 27, 12, 0, 0, 0, time.UTC)
	events := []UsageEvent{
		{
			// Shape built by `rho exec` (cmd/exec.go).
			EventID: fmt.Sprintf("exec-%d-%s", started.UnixMilli(), "0123456789abcdef"), DeviceID: contractDeviceID, ProjectID: contractProjectID,
			SessionID: fmt.Sprintf("exec-%d-%s", started.UnixMilli(), "01234567"), Capability: CapabilityRho,
			Model: "claude-sonnet-4-5", InputTokens: 1200, OutputTokens: 300, TokensUsed: 1500,
			DurationMS: 42_000, Status: "completed", OccurredAt: started.Format(time.RFC3339),
		},
		{
			// A 30-hour mission with oversized counters and text.
			EventID: "exec-1727400000000-fedcba9876543210", DeviceID: contractDeviceID, ProjectID: contractProjectID,
			Model: strings.Repeat("m", 300), InputTokens: 50_000_000, TokensUsed: 60_000_000,
			DurationMS: 30 * 60 * 60 * 1000, Status: "failed", ErrorCode: strings.Repeat("e", 200),
		},
	}
	for _, event := range events {
		if err := worker.client().RecordUsage(context.Background(), event); err != nil {
			t.Fatalf("RecordUsage(%s) = %v", event.EventID, err)
		}
	}
}

func TestDeliveryContextConformsToContract(t *testing.T) {
	worker := newContractWorker(t)
	event := DeliveryContext{ProjectID: contractProjectID, Branch: "main", CommitSHA: strings.Repeat("a", 40)}
	event.Repository.Provider, event.Repository.ExternalID, event.Repository.Name = "github", "GrayCodeAI/rho", "GrayCodeAI/rho"
	event.CIRun = &CIRunContext{Provider: "github", ExternalID: "1234567890", Workflow: "CI", Status: "succeeded"}
	event.Deployment = &DeploymentContext{Provider: "github", ExternalID: "deploy-1", Environment: "production", Status: "rolled_back"}
	if err := worker.client().SendDeliveryContext(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	minimal := DeliveryContext{ProjectID: contractProjectID}
	minimal.Repository.Provider, minimal.Repository.ExternalID, minimal.Repository.Name = "git", "local", "local"
	if err := worker.client().SendDeliveryContext(context.Background(), minimal); err != nil {
		t.Fatal(err)
	}
}

func TestGraphSyncConformsToContract(t *testing.T) {
	worker := newContractWorker(t)
	at := time.Date(2026, time.September, 27, 12, 0, 0, 0, time.UTC)
	provenance := graphcontracts.Provenance{Producer: "rho", Version: "0.3.0"}
	session := graphcontracts.Ref{Kind: graphcontracts.NodeExecution, ID: "rho/session/session-0123456789"}
	tool := graphcontracts.Ref{Kind: graphcontracts.NodeExecution, ID: "rho/tool/tool-0123456789"}
	export := executiongraph.Export{
		SchemaVersion: executiongraph.SchemaVersion,
		GeneratedAt:   at,
		Scope:         graphcontracts.Scope{ProjectID: contractProjectID, RepositoryID: "GrayCodeAI/rho"},
		Nodes: []graphcontracts.Node{
			{ID: session.ID, Kind: session.Kind, CreatedAt: at, Provenance: provenance, Attributes: map[string]string{
				"entity_type": "rho_session", "provider": "private-provider", "model": "private-model",
				"MESSAGE_COUNT": "12", "sast_source": "true", "Prompt_Tokens": "1200",
			}},
			{ID: tool.ID, Kind: tool.Kind, CreatedAt: at, Provenance: provenance, Attributes: map[string]string{
				"tool_name": "Bash", "command": "go test ./...", "file_path": "/home/dev/src/main.go",
			}},
		},
		Edges: []graphcontracts.Edge{{
			ID: "rho/edge/contains-0123456789", Kind: graphcontracts.EdgeContains, From: session, To: tool,
			CreatedAt: at, Provenance: provenance, Attributes: map[string]string{"reason": "tool call"},
		}},
		Events: []graphcontracts.Event{{
			ID: "rho/event/created-0123456789", Type: graphcontracts.EventCreated, Subject: session,
			OccurredAt: at, Provenance: provenance,
		}},
	}
	prepared, err := PrepareGraph(export)
	if err != nil {
		t.Fatal(err)
	}
	result, err := worker.client().SyncGraph(context.Background(), GraphSyncRequest{SyncID: prepared.SyncID, ProjectID: contractProjectID, Graph: prepared.Graph})
	if err != nil || !result.Accepted {
		t.Fatalf("SyncGraph = %+v, %v", result, err)
	}
	for _, leaked := range []string{"private-provider", "private-model", "go test ./...", "/home/dev/src/main.go", "tool call"} {
		if strings.Contains(string(prepared.Graph), leaked) {
			t.Fatalf("prepared graph leaked %q", leaked)
		}
	}
}
