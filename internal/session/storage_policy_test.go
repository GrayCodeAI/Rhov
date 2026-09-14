package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSessionPathDoesNotCreateProjectRho(t *testing.T) {
	project := t.TempDir()
	state := filepath.Join(t.TempDir(), "state")
	t.Setenv("RHO_STATE_DIR", state)

	path := SessionPath(project, "abc123")
	if strings.Contains(path, filepath.Join(project, ".rho")) {
		t.Fatalf("SessionPath leaked project .rho: %q", path)
	}
	if !strings.HasPrefix(path, state) {
		t.Fatalf("SessionPath = %q, want under %q", path, state)
	}
	if _, err := os.Stat(filepath.Join(project, ".rho")); !os.IsNotExist(err) {
		t.Fatalf("SessionPath created project .rho, stat err=%v", err)
	}
}
