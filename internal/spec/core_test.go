package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleDelta = `# Delta

## ADDED Requirements

### Requirement: Widget creation
The system SHALL create a widget when requested.

#### Scenario: Create succeeds
- **WHEN** the user requests a widget
- **THEN** the system creates it

## REMOVED Requirements

### Requirement: Legacy widget
**Reason**: superseded
**Migration**: use Widget creation

## RENAMED Requirements

### Requirement: Widget
- FROM: Old Widget
- TO: New Widget
`

func TestParseDeltaSpec(t *testing.T) {
	ds, err := ParseDeltaSpec(sampleDelta)
	if err != nil {
		t.Fatalf("ParseDeltaSpec: %v", err)
	}
	if len(ds.Requirements) != 3 {
		t.Fatalf("expected 3 requirements, got %d", len(ds.Requirements))
	}

	added := ds.Requirements[0]
	if added.Section != DeltaAdded || added.Name != "Widget creation" {
		t.Fatalf("unexpected added requirement: %+v", added)
	}
	if len(added.Scenarios) != 1 || added.Scenarios[0].When != "the user requests a widget" {
		t.Fatalf("scenario not parsed: %+v", added.Scenarios)
	}
	if !strings.Contains(added.Description, "SHALL") {
		t.Fatalf("description missing SHALL: %q", added.Description)
	}

	removed := ds.Requirements[1]
	if removed.Section != DeltaRemoved || removed.Reason != "superseded" || removed.Migration != "use Widget creation" {
		t.Fatalf("unexpected removed requirement: %+v", removed)
	}

	renamed := ds.Requirements[2]
	if renamed.Section != DeltaRenamed || renamed.OldName != "Old Widget" || renamed.NewName != "New Widget" {
		t.Fatalf("unexpected renamed requirement: %+v", renamed)
	}
}

func TestParseDeltaSpec_Errors(t *testing.T) {
	if _, err := ParseDeltaSpec("no headers here"); err == nil {
		t.Fatal("expected error for content with no section headers")
	}
	if _, err := ParseDeltaSpec("## ADDED Requirements\n\n(no requirements)"); err == nil {
		t.Fatal("expected error for section headers without requirements")
	}
}

func TestValidateDeltaSpec(t *testing.T) {
	ds, err := ParseDeltaSpec(sampleDelta)
	if err != nil {
		t.Fatal(err)
	}
	res := ValidateDeltaSpec(ds)
	if !res.Valid {
		t.Fatalf("expected valid delta, issues: %+v", res.Issues)
	}

	// A requirement without SHALL/MUST is an error.
	bad := &DeltaSpec{Requirements: []DeltaRequirement{{
		Name:        "No normative word",
		Description: "the system does something",
		Section:     DeltaAdded,
	}}}
	res = ValidateDeltaSpec(bad)
	if res.Valid {
		t.Fatal("expected invalid delta for missing SHALL/MUST")
	}
	found := false
	for _, iss := range res.Issues {
		if iss.Code == "NO_SHALL_MUST" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected NO_SHALL_MUST issue, got %+v", res.Issues)
	}

	// Duplicate requirement in the same section is an error.
	dup := &DeltaSpec{Requirements: []DeltaRequirement{
		{Name: "Dup", Description: "SHALL do a thing", Section: DeltaAdded},
		{Name: "Dup", Description: "SHALL do a thing", Section: DeltaAdded},
	}}
	res = ValidateDeltaSpec(dup)
	if res.Valid {
		t.Fatal("expected invalid delta for duplicate requirement")
	}
}

func TestApplyDelta_AddAndRemove(t *testing.T) {
	ds, err := ParseDeltaSpec(sampleDelta)
	if err != nil {
		t.Fatal(err)
	}

	// Apply to an empty main spec: the added requirement must appear.
	merged, err := ApplyDelta("", ds)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(merged, "Widget creation") {
		t.Fatalf("added requirement missing from merged spec:\n%s", merged)
	}

	// Remove a requirement from a main spec that contains it.
	main := "# Requirements\n\n### Requirement: Legacy widget\nThe system SHALL be old.\n"
	removeOnly := &DeltaSpec{Requirements: []DeltaRequirement{{
		Name:    "Legacy widget",
		Section: DeltaRemoved,
		Reason:  "gone",
	}}}
	merged, err = ApplyDelta(main, removeOnly)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(merged, "Legacy widget") {
		t.Fatalf("removed requirement still present:\n%s", merged)
	}
}

func TestGraph_TopologicalOrderAndActionable(t *testing.T) {
	g := NewGraph(&DefaultSchema, t.TempDir())

	order, err := g.TopologicalOrder()
	if err != nil {
		t.Fatalf("TopologicalOrder: %v", err)
	}
	pos := map[string]int{}
	for i, id := range order {
		pos[id] = i
	}
	if pos["proposal"] >= pos["specs"] || pos["specs"] >= pos["tasks"] {
		t.Fatalf("dependency order violated: %v", order)
	}

	// With no output files, only proposal (no deps) is actionable.
	actionable := g.NextActionable()
	if len(actionable) != 1 || actionable[0] != "proposal" {
		t.Fatalf("expected [proposal] actionable, got %v", actionable)
	}

	// Creating proposal.md makes specs and design actionable.
	dir := g.changeDir
	if err := os.WriteFile(filepath.Join(dir, "proposal.md"), []byte("p"), 0o600); err != nil {
		t.Fatal(err)
	}
	actionable = g.NextActionable()
	got := map[string]bool{}
	for _, id := range actionable {
		got[id] = true
	}
	if !got["specs"] || !got["design"] || got["tasks"] {
		t.Fatalf("unexpected actionable set after proposal: %v", actionable)
	}

	done, total := g.Progress()
	if done != 1 || total != 4 {
		t.Fatalf("progress = %d/%d, want 1/4", done, total)
	}
}

func TestGraph_CycleDetection(t *testing.T) {
	schema := &Schema{
		Name: "cyclic",
		Artifacts: []Artifact{
			{ID: "a", Generates: "a.md", Requires: []string{"b"}},
			{ID: "b", Generates: "b.md", Requires: []string{"a"}},
		},
	}
	g := NewGraph(schema, t.TempDir())
	if _, err := g.TopologicalOrder(); err == nil {
		t.Fatal("expected cycle detection error")
	}
}

func TestSpecConfig_Format(t *testing.T) {
	empty := SpecConfig{}
	if !empty.IsEmpty() {
		t.Fatal("zero SpecConfig should be empty")
	}
	if empty.HasAIDecide() {
		t.Fatal("empty SpecConfig should not report explicit AI-decide")
	}
	if empty.Format() == "" {
		t.Fatal("Format should render a default message for an empty config")
	}
	if !(SpecConfig{Language: "ai"}).HasAIDecide() {
		t.Fatal("explicit 'ai' field should report AI-decide")
	}
}

func BenchmarkParseDeltaSpec(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := ParseDeltaSpec(sampleDelta); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGraphTopologicalOrder(b *testing.B) {
	g := NewGraph(&DefaultSchema, b.TempDir())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := g.TopologicalOrder(); err != nil {
			b.Fatal(err)
		}
	}
}
