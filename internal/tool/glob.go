package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type GlobTool struct{}

func (GlobTool) Name() string        { return "Glob" }
func (GlobTool) RiskLevel() string   { return "low" }
func (GlobTool) Aliases() []string   { return []string{"glob"} }
func (GlobTool) Description() string { return "Find files matching a glob pattern." }

// GlobInput is the typed input for GlobTool.
type GlobInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"`
}

// Schema returns the typed input schema. Parameters() delegates to it so the
// two cannot diverge.
func (GlobTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]SchemaProperty{
			"pattern": {Type: "string", Description: "Glob pattern (e.g. **/*.go)"},
			"path":    {Type: "string", Description: "Root directory (default: current dir)"},
		},
		Required: []string{"pattern"},
	}
}

func (GlobTool) Parameters() map[string]interface{} {
	return globSchema.ToJSONSchema()
}

// globSchema is the single source of truth for Glob's input schema.
var globSchema = GlobTool{}.Schema()

func (GlobTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	p, err := DecodeInput[GlobInput]("Glob", input)
	if err != nil {
		return "", err
	}
	root := p.Path
	if root == "" {
		root = "."
	}
	if err := validatePathAllowed(ctx, root); err != nil {
		return "", err
	}
	var matches []string
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == "dist") {
			return filepath.SkipDir
		}
		matched, _ := filepath.Match(p.Pattern, filepath.Base(path))
		if matched {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "No files found", nil
	}
	return fmt.Sprintf("%d files:\n%s", len(matches), strings.Join(matches, "\n")), nil
}
