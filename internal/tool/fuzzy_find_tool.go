package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/GrayCodeAI/rho/internal/fuzzyfind"
)

// FuzzyFindTool searches a project tree for files matching a fuzzy query,
// returning ranked results (exact basename > basename prefix > contains >
// multi-word > camel-case abbreviation). Complements Glob (exact patterns)
// and Grep (exact text) when the agent knows roughly what it's looking for
// but not the exact path.
type FuzzyFindTool struct{}

func (FuzzyFindTool) Name() string      { return "FuzzyFind" }
func (FuzzyFindTool) RiskLevel() string { return "low" }
func (FuzzyFindTool) Aliases() []string { return []string{"fuzzy_find", "ffind"} }
func (FuzzyFindTool) Description() string {
	return "Fuzzy-search a project for files by name. Handles partial names, camel-case abbreviations (CP→CachePlanner), and multi-word queries (cache gate→cache_gate.go). Returns ranked paths."
}

// FuzzyFindInput is the typed input for FuzzyFindTool.
type FuzzyFindInput struct {
	Query string `json:"query"`
	Path  string `json:"path"`
	Limit int    `json:"limit"`
}

// Schema returns the typed input schema. Parameters() delegates to it so the
// two cannot diverge. The limit bounds are preserved verbatim.
func (FuzzyFindTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]SchemaProperty{
			"query": {
				Type:        "string",
				Description: "Fuzzy query: full or partial file name, camel-case abbreviation, or space-separated terms.",
			},
			"path": {
				Type:        "string",
				Description: "Project directory (default: session working directory).",
			},
			"limit": {
				Type:        "integer",
				Minimum:     1,
				Maximum:     100,
				Description: "Maximum results (default 20).",
			},
		},
		Required: []string{"query"},
	}
}

func (FuzzyFindTool) Parameters() map[string]interface{} {
	return fuzzyFindSchema.ToJSONSchema()
}

// fuzzyFindSchema is the single source of truth for FuzzyFind's input schema.
var fuzzyFindSchema = FuzzyFindTool{}.Schema()

func (FuzzyFindTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	params, err := DecodeInput[FuzzyFindInput]("FuzzyFind", input)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(params.Query) == "" {
		return "", fmt.Errorf("query is required")
	}

	root := params.Path
	if root == "" {
		if tc := GetToolContext(ctx); tc != nil && tc.WorkingDir != "" {
			root = tc.WorkingDir
		} else {
			var err error
			root, err = os.Getwd()
			if err != nil {
				return "", fmt.Errorf("resolve working directory: %w", err)
			}
		}
	}
	if err := validatePathAllowed(ctx, root); err != nil {
		return "", err
	}
	if params.Limit <= 0 {
		params.Limit = 20
	}
	if params.Limit > 100 {
		params.Limit = 100
	}

	finder, err := fuzzyfind.New(root)
	if err != nil {
		return "", err
	}
	matches := finder.Search(params.Query, params.Limit)
	if len(matches) == 0 {
		return fmt.Sprintf("No files matching %q.", params.Query), nil
	}
	out, _ := json.MarshalIndent(map[string]interface{}{
		"query":   params.Query,
		"matches": len(matches),
		"results": matches,
	}, "", "  ")
	return string(out), nil
}
