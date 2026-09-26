package testaudit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/mod/semver"
)

// readRepoFile reads a file relative to the repository root.
func readRepoFile(t *testing.T, rel string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return data
}

// repoVersion returns the trimmed contents of the VERSION file.
func repoVersion(t *testing.T) string {
	t.Helper()
	return strings.TrimSpace(string(readRepoFile(t, "VERSION")))
}

func TestVersionFileIsSemVer(t *testing.T) {
	v := repoVersion(t)
	if strings.HasPrefix(v, "v") || !semver.IsValid("v"+v) {
		t.Fatalf("VERSION = %q, want a SemVer version without a leading v (e.g. 0.3.0)", v)
	}
	// v0.1.0–v0.2.0 are hawk-era tags already on the Go module proxy; a lower
	// VERSION would never become @latest.
	if semver.Compare("v"+v, "v0.3.0") < 0 {
		t.Fatalf("VERSION = %q is below 0.3.0, the first rho-named release", v)
	}
}
