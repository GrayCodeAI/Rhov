package token

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	graphcontracts "github.com/GrayCodeAI/rho/internal/contracts/graph"
)

func TestChunkCodeEmpty(t *testing.T) {
	if got := ChunkCode("", ChunkOptions{}); got != nil {
		t.Errorf("ChunkCode(\"\") = %+v, want nil", got)
	}
	if got := ChunkCode("   \n  ", ChunkOptions{}); got != nil {
		t.Errorf("ChunkCode(whitespace) = %+v, want nil", got)
	}
}

func TestChunkCodeSplitsOnBudget(t *testing.T) {
	var lines []string
	for i := 0; i < 200; i++ {
		lines = append(lines, "func f"+itoa(i)+"() { return }")
	}
	source := strings.Join(lines, "\n")
	chunks := ChunkCode(source, ChunkOptions{MaxTokens: 50})
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	// Every chunk must respect the budget and carry line bounds.
	for i, c := range chunks {
		if c.Tokens > 50 {
			t.Errorf("chunk %d tokens = %d, want <= 50", i, c.Tokens)
		}
		if c.StartLine < 1 || c.EndLine < c.StartLine {
			t.Errorf("chunk %d line bounds = [%d,%d]", i, c.StartLine, c.EndLine)
		}
	}
	// Chunks must cover the source without gaps or overlap.
	if chunks[0].StartLine != 1 {
		t.Errorf("first chunk starts at line %d, want 1", chunks[0].StartLine)
	}
	if last := chunks[len(chunks)-1]; last.EndLine != len(lines) {
		t.Errorf("last chunk ends at line %d, want %d", last.EndLine, len(lines))
	}
}

func TestDefaultChunkOptions(t *testing.T) {
	opts := DefaultChunkOptions()
	if opts.MaxTokens <= 0 {
		t.Errorf("DefaultChunkOptions().MaxTokens = %d, want > 0", opts.MaxTokens)
	}
}

func TestJSONInvariantsConstantField(t *testing.T) {
	records := make([]json.RawMessage, 0, 4)
	for i := 0; i < 4; i++ {
		records = append(records, json.RawMessage(`{"kind":"note","n":`+itoa(i)+`}`))
	}
	got := JSONInvariants(records)
	if !strings.Contains(got, "kind=note") {
		t.Errorf("JSONInvariants = %q, want a constant-field fact", got)
	}
}

func TestJSONInvariantsTooFew(t *testing.T) {
	records := []json.RawMessage{json.RawMessage(`{"a":1}`), json.RawMessage(`{"a":2}`)}
	if got := JSONInvariants(records); got != "" {
		t.Errorf("JSONInvariants(2 records) = %q, want empty (below min run)", got)
	}
}

func TestJSONInvariantsWithholdsSensitiveFields(t *testing.T) {
	records := make([]json.RawMessage, 0, 4)
	for i := 0; i < 4; i++ {
		records = append(records, json.RawMessage(`{"token":"abc","n":`+itoa(i)+`}`))
	}
	got := JSONInvariants(records)
	if strings.Contains(got, "token") {
		t.Errorf("JSONInvariants leaked a sensitive field name: %q", got)
	}
}

func TestLogInvariants(t *testing.T) {
	lines := []string{
		"2026-01-01 INFO starting",
		"2026-01-01 INFO working",
		"2026-01-01 INFO done",
	}
	got := LogInvariants(lines)
	if !strings.Contains(got, "info") {
		t.Errorf("LogInvariants = %q, want a level distribution", got)
	}
}

func TestLogInvariantsTooFew(t *testing.T) {
	if got := LogInvariants([]string{"INFO a"}); got != "" {
		t.Errorf("LogInvariants(1 line) = %q, want empty", got)
	}
}

func TestShrinkToolCatalog(t *testing.T) {
	longDesc := strings.Repeat("This tool does a thing. ", 60) +
		"You must pass a valid path. It is optional otherwise."
	catalog := `[{"type":"function","function":{"name":"Read","description":` +
		mustJSON(longDesc) + `,"parameters":{"type":"object","properties":{"path":{"type":"string"}},"required":["path"],"title":"Read params"}}}]`

	shrunk, ok := ShrinkToolCatalog(catalog)
	if !ok {
		t.Fatal("ShrinkToolCatalog returned ok=false for a shrinkable catalog")
	}
	if len(shrunk) >= len(catalog) {
		t.Errorf("shrunk catalog is not smaller: %d -> %d", len(catalog), len(shrunk))
	}
	// Selection surface must survive byte-for-byte.
	for _, want := range []string{`"name":"Read"`, `"path"`, `"required"`, `"type":"string"`} {
		if !strings.Contains(shrunk, want) {
			t.Errorf("shrunk catalog lost selection surface %q", want)
		}
	}
	// Annotation-only keys are dropped.
	if strings.Contains(shrunk, `"title"`) {
		t.Errorf("shrunk catalog kept annotation-only key: %s", shrunk)
	}
	// Constraint-bearing sentence survives.
	if !strings.Contains(shrunk, "must pass") {
		t.Errorf("shrunk catalog dropped a constraint sentence: %s", shrunk)
	}
}

func TestShrinkToolCatalogFailOpen(t *testing.T) {
	for _, bad := range []string{"", "not json", "{}", "[]"} {
		got, ok := ShrinkToolCatalog(bad)
		if ok {
			t.Errorf("ShrinkToolCatalog(%q) ok = true, want false", bad)
		}
		if got != bad {
			t.Errorf("ShrinkToolCatalog(%q) = %q, want unchanged input", bad, got)
		}
	}
}

func TestLintToolCatalog(t *testing.T) {
	longDesc := strings.Repeat("Filler sentence that carries no constraint. ", 40) +
		"You must supply a valid path."
	catalog := `[{"type":"function","function":{"name":"X","description":` + mustJSON(longDesc) + `,"parameters":{"type":"object","properties":{}}}}]`
	stats, ok := LintToolCatalog(catalog)
	if !ok || len(stats) != 1 {
		t.Fatalf("LintToolCatalog = (%+v, %v), want one entry", stats, ok)
	}
	if stats[0].Name != "X" {
		t.Errorf("stats name = %q, want X", stats[0].Name)
	}
	if stats[0].DescAfter >= stats[0].DescBefore {
		t.Errorf("description not reduced: %d -> %d", stats[0].DescBefore, stats[0].DescAfter)
	}
}

func mustJSON(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func TestBuildRuntimeGraphRequiresSummary(t *testing.T) {
	if _, err := BuildRuntimeGraph(RuntimeGraphInput{}); err == nil {
		t.Error("BuildRuntimeGraph with no summaries should error")
	}
}

func TestBuildRuntimeGraphCompression(t *testing.T) {
	at := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	out, err := BuildRuntimeGraph(RuntimeGraphInput{
		Compression:   &Stats{OriginalTokens: 100, FinalTokens: 40, TokensSaved: 60, ReductionPercent: 60},
		Source:        "conversation",
		ObservedAt:    at,
		CorrelationID: "sess-1",
	})
	if err != nil {
		t.Fatalf("BuildRuntimeGraph: %v", err)
	}
	if out == nil || len(out.Nodes) != 1 {
		t.Fatalf("export = %+v, want one node", out)
	}
	if out.SchemaVersion != RuntimeGraphSchemaVersion {
		t.Errorf("SchemaVersion = %q, want %q", out.SchemaVersion, RuntimeGraphSchemaVersion)
	}
	if out.Nodes[0].Kind != graphcontracts.NodeOperations {
		t.Errorf("node kind = %q, want operations", out.Nodes[0].Kind)
	}
	// Privacy: the source text must never appear, only its digest.
	payload, _ := json.Marshal(out)
	if strings.Contains(string(payload), "conversation") {
		t.Errorf("runtime graph leaked the source text: %s", payload)
	}
	if len(out.Events) != 1 || out.Events[0].Type != graphcontracts.EventObserved {
		t.Errorf("events = %+v, want one observed event", out.Events)
	}
}

func TestBuildRuntimeGraphUsageAndBudget(t *testing.T) {
	out, err := BuildRuntimeGraph(RuntimeGraphInput{
		Usage:  &UsageSummary{HourlyTokens: 10, DailyTokens: 20, SessionTokens: 30},
		Budget: &BudgetDecision{Allowed: true, Reason: "within limits", HourlyLimit: 100},
		Source: "provider\x00model",
	})
	if err != nil {
		t.Fatalf("BuildRuntimeGraph: %v", err)
	}
	if len(out.Nodes) != 2 {
		t.Fatalf("nodes = %d, want 2 (usage + budget)", len(out.Nodes))
	}
	if len(out.Edges) != 1 {
		t.Fatalf("edges = %d, want 1 (budget depends_on usage)", len(out.Edges))
	}
	if out.Edges[0].Kind != graphcontracts.EdgeDependsOn {
		t.Errorf("edge kind = %q, want depends_on", out.Edges[0].Kind)
	}
	// Every node must pass the shared contract validators.
	for _, n := range out.Nodes {
		if err := n.Validate(); err != nil {
			t.Errorf("node %q invalid: %v", n.ID, err)
		}
	}
}

func TestBuildRuntimeGraphRedaction(t *testing.T) {
	out, err := BuildRuntimeGraph(RuntimeGraphInput{
		Redaction: &RedactionSummary{MatchCount: 3, Types: map[string]int{"AWS Access Key": 3}},
		Source:    "response",
	})
	if err != nil {
		t.Fatalf("BuildRuntimeGraph: %v", err)
	}
	if len(out.Nodes) != 1 || out.Nodes[0].Kind != graphcontracts.NodeQuality {
		t.Fatalf("export = %+v, want one quality node", out.Nodes)
	}
}

func TestBuildRuntimeGraphDeterministic(t *testing.T) {
	in := RuntimeGraphInput{
		Compression: &Stats{OriginalTokens: 10, FinalTokens: 5, TokensSaved: 5},
		Source:      "s",
		ObservedAt:  time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
	}
	a, err := BuildRuntimeGraph(in)
	if err != nil {
		t.Fatal(err)
	}
	b, err := BuildRuntimeGraph(in)
	if err != nil {
		t.Fatal(err)
	}
	if a.Nodes[0].ID != b.Nodes[0].ID {
		t.Errorf("node IDs not deterministic: %q vs %q", a.Nodes[0].ID, b.Nodes[0].ID)
	}
}
