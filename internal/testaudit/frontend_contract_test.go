package testaudit

import (
	"os"
	"strings"
	"testing"
)

// TestFrontendContract enforces the frontend boundary documented in
// docs/architecture/frontend-contract.md: frontends consume engine events
// through the public facade and must not reach into the agent-loop internals.
//
// Shared engine *utility* packages (io, git, project, branching, lifecycle,
// token, safety, diff) are allowed — they carry no agent-loop state. The
// denied set below is the agent brain itself.
func TestFrontendContract(t *testing.T) {
	root := repoRoot(t)
	files := parseGoFiles(t, root+"/cmd")

	// Agent-loop internals a frontend must never import.
	denied := []string{
		"github.com/GrayCodeAI/rho/internal/engine/agent",
		"github.com/GrayCodeAI/rho/internal/engine/code",
		"github.com/GrayCodeAI/rho/internal/engine/prompt",
		"github.com/GrayCodeAI/rho/internal/engine/async",
		"github.com/GrayCodeAI/rho/internal/engine/trajectory",
	}

	var violations []string
	for _, pf := range files {
		for _, imp := range pf.File.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			for _, d := range denied {
				if path == d || strings.HasPrefix(path, d+"/") {
					violations = append(violations, relPath(root, pf.Path)+": "+path)
				}
			}
		}
	}
	if len(violations) > 0 {
		t.Errorf("frontends must not import engine internals (see docs/architecture/frontend-contract.md):\n  %s",
			strings.Join(violations, "\n  "))
	}
}

// TestFrontendContractDocsExist ensures the contract document is present so the
// guard above has a discoverable rationale.
func TestFrontendContractDocsExist(t *testing.T) {
	root := repoRoot(t)
	if !fileExists(root + "/docs/architecture/frontend-contract.md") {
		t.Fatal("docs/architecture/frontend-contract.md is missing")
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
