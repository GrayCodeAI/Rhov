package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type GrepTool struct{}

func (GrepTool) Name() string        { return "Grep" }
func (GrepTool) RiskLevel() string   { return "low" }
func (GrepTool) Aliases() []string   { return []string{"grep"} }
func (GrepTool) Description() string { return "Search for a regex pattern in files." }

// GrepInput is the typed input for GrepTool.
type GrepInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"`
	Include string `json:"include"`
}

// Schema returns the typed input schema. Parameters() delegates to it so the
// two cannot diverge.
func (GrepTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]SchemaProperty{
			"pattern": {Type: "string", Description: "Regex pattern to search for"},
			"path":    {Type: "string", Description: "Directory to search (default: current dir)"},
			"include": {Type: "string", Description: "File glob filter (e.g. *.go)"},
		},
		Required: []string{"pattern"},
	}
}

func (GrepTool) Parameters() map[string]interface{} {
	return grepSchema.ToJSONSchema()
}

// grepSchema is the single source of truth for Grep's input schema.
var grepSchema = GrepTool{}.Schema()

func (GrepTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	p, err := DecodeInput[GrepInput]("Grep", input)
	if err != nil {
		return "", err
	}
	re, err := regexp.Compile(p.Pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex: %w", err)
	}
	root := p.Path
	if root == "" {
		root = "."
	}
	if err := validatePathAllowed(ctx, root); err != nil {
		return "", err
	}
	rootAbs, err := guardedAbs(root)
	if err != nil {
		return "", err
	}
	rootHandle, err := os.OpenRoot(rootAbs)
	if err != nil {
		return "", fmt.Errorf("open search root: %w", err)
	}
	defer func() { _ = rootHandle.Close() }()

	var results []string
	_ = fs.WalkDir(rootHandle.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if p.Include != "" {
			if matched, _ := filepath.Match(p.Include, d.Name()); !matched {
				return nil
			}
		}
		data, err := fs.ReadFile(rootHandle.FS(), path)
		if err != nil {
			return nil
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if re.MatchString(line) {
				results = append(results, fmt.Sprintf("%s:%d: %s", filepath.Join(rootAbs, filepath.FromSlash(path)), i+1, line))
				if len(results) >= 200 {
					return fmt.Errorf("limit")
				}
			}
		}
		return nil
	})
	if len(results) == 0 {
		return "No matches found", nil
	}
	return strings.Join(results, "\n"), nil
}
