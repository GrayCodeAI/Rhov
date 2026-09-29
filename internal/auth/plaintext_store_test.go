package auth

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestUsesPlaintextTokenFileOnlyWithoutOSStore(t *testing.T) {
	for goos, want := range map[string]bool{
		"darwin":  false,
		"windows": false,
		"linux":   true,
		"freebsd": true,
	} {
		if got := usesPlaintextTokenFile(goos); got != want {
			t.Errorf("usesPlaintextTokenFile(%q) = %v, want %v", goos, got, want)
		}
	}
	if UsesPlaintextTokenFile() != usesPlaintextTokenFile(runtime.GOOS) {
		t.Fatal("UsesPlaintextTokenFile disagrees with the current platform")
	}
}

func TestCredentialStoreNameNamesThePlaintextFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("RHO_CONFIG_DIR", dir)
	if TokenFilePath() != filepath.Join(dir, ".tokens") {
		t.Fatalf("TokenFilePath = %q", TokenFilePath())
	}
	name := CredentialStoreName()
	if UsesPlaintextTokenFile() != strings.Contains(name, TokenFilePath()) {
		t.Fatalf("CredentialStoreName = %q; it must name the plaintext file exactly when it is used", name)
	}
}
