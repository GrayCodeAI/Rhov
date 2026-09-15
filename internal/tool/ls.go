package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type LSTool struct{}

func (LSTool) Name() string      { return "LS" }
func (LSTool) RiskLevel() string { return "low" }
func (LSTool) Aliases() []string { return []string{"ls"} }
func (LSTool) Description() string {
	return "List files and directories in a directory."
}

// LSInput is the typed input for LSTool.
type LSInput struct {
	Path   string   `json:"path"`
	Ignore []string `json:"ignore"`
}

// Schema returns the typed input schema. Parameters() delegates to it so the
// two cannot diverge.
func (LSTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]SchemaProperty{
			"path":   {Type: "string", Description: "Directory path to list (default: current directory)"},
			"ignore": {Type: "array", Description: "Optional file or glob patterns to exclude", Items: &SchemaProperty{Type: "string"}},
		},
	}
}

func (LSTool) Parameters() map[string]interface{} {
	return lsSchema.ToJSONSchema()
}

// lsSchema is the single source of truth for LS's input schema.
var lsSchema = LSTool{}.Schema()

func (LSTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	// Empty input lists the current directory; only decode when present.
	var p LSInput
	if len(input) > 0 {
		var err error
		p, err = DecodeInput[LSInput]("LS", input)
		if err != nil {
			return "", err
		}
	}
	path := p.Path
	if path == "" {
		path = "."
	}
	if err := validatePathAllowed(ctx, path); err != nil {
		return "", err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("ls %s: %w", path, err)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})

	var lines []string
	for _, entry := range entries {
		name := entry.Name()
		fullPath := filepath.Join(path, name)
		if ignoredByLS(name, fullPath, p.Ignore) {
			continue
		}
		if entry.IsDir() {
			name += "/"
		}
		lines = append(lines, name)
	}
	if len(lines) == 0 {
		return fmt.Sprintf("%s: no entries", path), nil
	}
	return fmt.Sprintf("%s:\n%s", path, strings.Join(lines, "\n")), nil
}

func ignoredByLS(name, path string, patterns []string) bool {
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if pattern == name || pattern == path {
			return true
		}
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
	}
	return false
}
