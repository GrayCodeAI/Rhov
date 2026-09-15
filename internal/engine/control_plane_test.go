package engine

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/GrayCodeAI/rho/internal/tool"
	"github.com/GrayCodeAI/rho/internal/types"
)

func TestWorkModePlanFiltersToolsAndBash(t *testing.T) {
	reg := tool.NewRegistry(
		tool.BashTool{}, tool.FileReadTool{}, tool.FileWriteTool{}, tool.GrepTool{},
		tool.ToolSearchTool{},
	)
	reg.EnableLazyModelSurface([]string{"Bash", "Read", "Write", "Grep", "ToolSearch"})
	sess := NewSession("test", "test", "sys", reg)
	if err := sess.SetWorkMode(WorkModePlan); err != nil {
		t.Fatal(err)
	}
	if !sess.Tools().ReadOnlyBash() {
		t.Fatal("plan mode should set read-only bash")
	}
	names := reg.ModelVisibleNames()
	for _, n := range names {
		if n == "Write" {
			t.Fatalf("Write should not be model-visible in plan mode: %v", names)
		}
	}
	if !reg.IsModelVisible("Read") {
		t.Fatal("Read should be visible in plan")
	}
	if sess.WorkMode() != WorkModePlan {
		t.Fatalf("WorkMode = %s", sess.WorkMode())
	}
	if addon := sess.workModeSystemAddon(); !strings.Contains(addon, "PLAN") {
		t.Fatalf("plan addon missing: %q", addon)
	}
}

func TestLazyFluxToolsAndPromote(t *testing.T) {
	reg := tool.NewRegistry(tool.FileReadTool{}, tool.ImpactTool{})
	reg.EnableLazyModelSurface([]string{"Read"})
	flux := reg.FluxTools()
	if len(flux) != 1 || flux[0].Name != "Read" {
		t.Fatalf("FluxTools = %#v, want only Read", flux)
	}
	if !reg.PromoteModelTool("Impact") {
		t.Fatal("promote Impact failed")
	}
	flux = reg.FluxTools()
	if len(flux) != 2 {
		t.Fatalf("after promote FluxTools len = %d", len(flux))
	}
}

func TestToolSearchSelectPromotes(t *testing.T) {
	reg := tool.NewRegistry(tool.FileReadTool{}, tool.ImpactTool{}, tool.ToolSearchTool{})
	reg.EnableLazyModelSurface([]string{"Read", "ToolSearch"})
	sess := NewSession("test", "test", "sys", reg)
	sess.PermSvc().SetAutonomy(AutonomyYOLO)
	ch := make(chan StreamEvent, 8)
	input, _ := json.Marshal(map[string]interface{}{"query": "select:Impact"})
	res := sess.executeSingleTool(context.Background(), types.ToolCall{
		Name: "ToolSearch", ID: "ts1",
		Arguments: map[string]interface{}{"query": "select:Impact"},
	}, ch, 0, "")
	_ = input
	if res.isErr {
		t.Fatalf("ToolSearch failed: %v", res.err)
	}
	if !reg.IsModelVisible("Impact") {
		t.Fatalf("Impact should be promoted; visible=%v", reg.ModelVisibleNames())
	}
}

func TestSpawnControllerStatus(t *testing.T) {
	sess := NewSession("test", "test", "sys", tool.NewRegistry())
	sess.WireAgentTool()
	sc := sess.SpawnController()
	if sc.Status() == "" {
		t.Fatal("empty status")
	}
	if sc.Tasks() == nil {
		t.Fatal("tasks registry nil after ensure")
	}
}
