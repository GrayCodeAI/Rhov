package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/GrayCodeAI/rho/internal/intelligence/skillcurator"
	"github.com/GrayCodeAI/rho/internal/plugin"
)

type SkillTool struct{}

func (SkillTool) Name() string      { return "Skill" }
func (SkillTool) Aliases() []string { return []string{"skill"} }
func (SkillTool) Description() string {
	return "Load instructions from a local Rho skill. Use without a skill name to list available skills."
}

// SkillInput is the typed input for SkillTool.
type SkillInput struct {
	Skill string `json:"skill"`
}

// Schema returns the typed input schema. Parameters() delegates to it so the
// two cannot diverge.
func (SkillTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]SchemaProperty{
			"skill": {Type: "string", Description: "Skill name to load"},
		},
	}
}

func (SkillTool) Parameters() map[string]interface{} {
	return skillSchema.ToJSONSchema()
}

// skillSchema is the single source of truth for Skill's input schema.
var skillSchema = SkillTool{}.Schema()

func (SkillTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	// Empty input lists available skills; only decode when present.
	var p SkillInput
	if len(input) > 0 {
		var err error
		p, err = DecodeInput[SkillInput]("Skill", input)
		if err != nil {
			return "", err
		}
	}

	cwd, _ := os.Getwd()
	if p.Skill == "" {
		allSkills, err := plugin.DefaultRegistry.List(ctx, cwd)
		if err != nil {
			return "", err
		}
		var modelSkills []plugin.SkillEntry
		for _, s := range allSkills {
			if s.Invocation.IsModelInvocable() {
				modelSkills = append(modelSkills, s)
			}
		}

		if len(modelSkills) == 0 {
			return "No skills found in Rho user state, .agents/skills, or .codex/skills.", nil
		}
		names := make([]string, 0, len(modelSkills))
		for _, s := range modelSkills {
			names = append(names, s.Name)
		}
		sort.Strings(names)
		return "Available skills:\n" + strings.Join(names, "\n"), nil
	}

	entry, err := plugin.DefaultRegistry.Get(ctx, cwd, p.Skill)
	if err != nil {
		return "", err
	}
	if !entry.Invocation.IsModelInvocable() {
		return "", fmt.Errorf("skill %q is not invocable by the model (policy restricted)", p.Skill)
	}

	// Canonical rendering (port of renderSkillContent)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Skill: %s", entry.Name))
	if entry.Provider != "" {
		sb.WriteString(fmt.Sprintf(" (Provider: %s)", entry.Provider))
	}
	sb.WriteString("\n")
	if entry.ResourceBase != "" {
		sb.WriteString(fmt.Sprintf("Resource Base: %s\n", entry.ResourceBase))
	}
	if entry.Path != "" {
		sb.WriteString(fmt.Sprintf("Source: %s\n", entry.Path))
	}
	sb.WriteString("\n")
	sb.WriteString(entry.Content)
	// Best-effort skill-usage recording for the curator (opt-in). Never
	// interrupts skill execution.
	if strings.EqualFold(os.Getenv("RHO_SKILL_CURATOR"), "1") {
		skillcurator.RecordSkillUsage(p.Skill)
	}
	return sb.String(), nil
}
