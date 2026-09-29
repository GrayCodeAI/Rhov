package testaudit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/mod/modfile"
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

// The hawk-era tags v0.1.0, v0.1.1 and v0.2.0 are on the Go module proxy under
// this module path but declare module github.com/GrayCodeAI/hawk; go.mod must
// keep retracting them so `go list -m -versions` and tooling skip them.
func TestGoModRetractsHawkEraVersions(t *testing.T) {
	f, err := modfile.Parse("go.mod", readRepoFile(t, "go.mod"), nil)
	if err != nil {
		t.Fatalf("parse go.mod: %v", err)
	}
	for _, v := range []string{"v0.1.0", "v0.1.1", "v0.2.0"} {
		retracted := false
		for _, r := range f.Retract {
			if semver.Compare(r.Low, v) <= 0 && semver.Compare(v, r.High) <= 0 {
				retracted = true
			}
		}
		if !retracted {
			t.Errorf("go.mod does not retract hawk-era %s", v)
		}
	}
	current := "v" + repoVersion(t)
	for _, r := range f.Retract {
		if semver.Compare(r.Low, current) <= 0 && semver.Compare(current, r.High) <= 0 {
			t.Errorf("go.mod retracts the current VERSION %s", current)
		}
	}
}
