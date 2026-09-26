package cmd

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"

	rhoconfig "github.com/GrayCodeAI/rho/internal/config"
	"github.com/GrayCodeAI/rho/internal/provider/gateway"
)

func TestDisplayVersion_FromVERSIONFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("0.0.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	SetVersion("dev")
	if got := DisplayVersion(); got != "0.0.1" {
		t.Fatalf("DisplayVersion() = %q, want 0.0.1", got)
	}
}

func stubBuildInfo(t *testing.T, mainVersion string) {
	t.Helper()
	prev := readBuildInfo
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{Main: debug.Module{Path: "github.com/GrayCodeAI/rho", Version: mainVersion}}, true
	}
	t.Cleanup(func() { readBuildInfo = prev })
}

func TestDisplayVersion_GoInstallTaggedModule(t *testing.T) {
	t.Chdir(t.TempDir()) // no VERSION file nearby
	SetVersion("dev")
	stubBuildInfo(t, "v0.3.0")
	if got := DisplayVersion(); got != "0.3.0" {
		t.Fatalf("DisplayVersion() = %q, want 0.3.0 for go install ...@v0.3.0", got)
	}
}

func TestDisplayVersion_DevelopmentModuleVersionsFallBack(t *testing.T) {
	for _, mv := range []string{
		"(devel)",
		"v0.0.0-20260921005105-13e05ceb15e5",   // go install ...@main
		"v0.3.1-0.20260930120000-abcdefabcdef", // untagged commit after v0.3.0
		"v0.3.0+dirty",                         // modified checkout at the tag
		"",
	} {
		t.Run(mv, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("0.3.0\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			t.Chdir(dir)
			SetVersion("dev")
			stubBuildInfo(t, mv)
			if got := DisplayVersion(); got != "0.3.0" {
				t.Fatalf("DisplayVersion() = %q, want the VERSION file value for module version %q", got, mv)
			}
		})
	}
}

func TestDisplayVersion_LdflagsWinOverModuleVersion(t *testing.T) {
	SetVersion("1.4.2")
	t.Cleanup(func() { SetVersion("dev") })
	stubBuildInfo(t, "v0.3.0")
	if got := DisplayVersion(); got != "1.4.2" {
		t.Fatalf("DisplayVersion() = %q, want the ldflags version 1.4.2", got)
	}
}

func TestDisplayVersion_ReleaseBuild(t *testing.T) {
	SetVersion("1.4.2")
	if got := DisplayVersion(); got != "1.4.2" {
		t.Fatalf("DisplayVersion() = %q, want 1.4.2", got)
	}
}

func TestChatConnectionStatus_NoCredentials(t *testing.T) {
	rhoconfig.InvalidateConfigUICache()
	store := &gateway.MapStore{}
	gateway.SetDefaultStore(store)
	t.Cleanup(func() {
		gateway.SetDefaultStore(nil)
		rhoconfig.InvalidateConfigUICache()
	})

	m := chatModel{session: nil}
	got := m.chatConnectionStatus()
	if got != "" {
		t.Fatalf("connection status = %q, want empty when unconfigured", got)
	}
}

func TestChatBottomRightStatus_NoCredentials(t *testing.T) {
	rhoconfig.InvalidateConfigUICache()
	store := &gateway.MapStore{}
	gateway.SetDefaultStore(store)
	t.Cleanup(func() {
		gateway.SetDefaultStore(nil)
		rhoconfig.InvalidateConfigUICache()
	})

	m := chatModel{inputIndicator: &InputIndicator{}}
	got := m.chatBottomRightStatus()
	if got != "" {
		t.Fatalf("status = %q, want empty when no keys", got)
	}
}
