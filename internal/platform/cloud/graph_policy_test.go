package cloud

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	graphcontracts "github.com/GrayCodeAI/rho/internal/contracts/graph"
	"github.com/GrayCodeAI/rho/internal/executiongraph"
)

type graphPolicyCase struct {
	Key       string `json:"key"`
	CloudSafe bool   `json:"cloudSafe"`
}

func loadGraphPolicy(t *testing.T) []graphPolicyCase {
	t.Helper()
	raw, err := os.ReadFile("testdata/graph_attribute_policy.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []graphPolicyCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture.Cases
}

func testGraph(attributes map[string]string) executiongraph.Export {
	at := time.Date(2026, time.September, 27, 12, 0, 0, 0, time.UTC)
	return executiongraph.Export{
		SchemaVersion: executiongraph.SchemaVersion,
		GeneratedAt:   at,
		Nodes: []graphcontracts.Node{{
			ID: "rho/session/session-0123456789", Kind: graphcontracts.NodeExecution, CreatedAt: at,
			Provenance: graphcontracts.Provenance{Producer: "rho"}, Attributes: attributes,
		}},
		Edges:  []graphcontracts.Edge{},
		Events: []graphcontracts.Event{},
	}
}

func TestGraphAttributePolicyMatchesSharedFixture(t *testing.T) {
	for _, tc := range loadGraphPolicy(t) {
		if got := !isSensitiveGraphAttribute(tc.Key); got != tc.CloudSafe {
			t.Errorf("key %q: cloud-safe = %v, want %v", tc.Key, got, tc.CloudSafe)
		}
		prepared, err := PrepareGraph(testGraph(map[string]string{tc.Key: "value"}))
		if err != nil {
			t.Fatalf("key %q: %v", tc.Key, err)
		}
		var document struct {
			Nodes []struct {
				Attributes map[string]string `json:"attributes"`
			} `json:"nodes"`
		}
		if err := json.Unmarshal(prepared.Graph, &document); err != nil {
			t.Fatal(err)
		}
		attributes := document.Nodes[0].Attributes
		if tc.CloudSafe {
			if attributes[tc.Key] != "value" {
				t.Errorf("key %q: cloud-safe attribute was altered: %v", tc.Key, attributes)
			}
			continue
		}
		if _, leaked := attributes[tc.Key]; leaked || attributes[tc.Key+"_sha256"] != sha256Hex([]byte("value")) {
			t.Errorf("key %q: sensitive attribute was not hashed: %v", tc.Key, attributes)
		}
	}
}

func TestPrepareGraphRejectsTenantScopes(t *testing.T) {
	cases := map[string]func(*executiongraph.Export){
		"document": func(g *executiongraph.Export) { g.Scope.TenantID = "tenant-1" },
		"node":     func(g *executiongraph.Export) { g.Nodes[0].Scope.TenantID = "tenant-1" },
		"event": func(g *executiongraph.Export) {
			g.Events = []graphcontracts.Event{{
				ID: "event-1", Type: graphcontracts.EventCreated, Subject: graphcontracts.Ref{Kind: graphcontracts.NodeExecution, ID: g.Nodes[0].ID},
				Scope: graphcontracts.Scope{TenantID: "tenant-1"}, OccurredAt: g.GeneratedAt, Provenance: graphcontracts.Provenance{Producer: "rho"},
			}}
		},
	}
	for name, mutate := range cases {
		graph := testGraph(nil)
		mutate(&graph)
		if _, err := PrepareGraph(graph); err == nil || !strings.Contains(err.Error(), "tenant_id") {
			t.Errorf("%s: error = %v, want a tenant_id rejection", name, err)
		}
	}
	graph := testGraph(nil)
	graph.Scope.ProjectID = "project_0123456789"
	if _, err := PrepareGraph(graph); err != nil {
		t.Fatalf("project-scoped graph rejected: %v", err)
	}
}

func TestPrepareGraphEnforcesAttributeBounds(t *testing.T) {
	longSensitiveKey := strings.Repeat("k", 55) + "_model" // 61 chars, 68 after _sha256
	if _, err := PrepareGraph(testGraph(map[string]string{longSensitiveKey: "x"})); err == nil || !strings.Contains(err.Error(), "1 to 64") {
		t.Fatalf("long key error = %v", err)
	}
	if _, err := PrepareGraph(testGraph(map[string]string{"note": strings.Repeat("v", maxGraphAttributeValue+1)})); err == nil {
		t.Fatal("oversized attribute value accepted")
	}
	many := map[string]string{}
	for i := 0; i <= maxGraphAttributes; i++ {
		many["attr_"+strings.Repeat("x", i%5)+string(rune('a'+i%26))+string(rune('a'+i/26))] = "v"
	}
	if _, err := PrepareGraph(testGraph(many)); err == nil || !strings.Contains(err.Error(), "attributes") {
		t.Fatalf("too many attributes error = %v", err)
	}
}
