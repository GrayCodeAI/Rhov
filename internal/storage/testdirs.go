package storage

import (
	"path/filepath"
	"testing"
)

// SetTestDirs configures isolated Rho config/state/cache directories for tests.
func SetTestDirs(t *testing.T, root string) {
	t.Helper()
	t.Setenv(envConfigDir, filepath.Join(root, "config"))
	t.Setenv(envFluxConfigDir, filepath.Join(root, "flux"))
	t.Setenv(envStateDir, filepath.Join(root, "state"))
	t.Setenv(envCacheDir, filepath.Join(root, "cache"))
}
