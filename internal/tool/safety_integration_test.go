package tool

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These tests exercise the file tools' use of the sensitive-path policy, so
// they live in package tool (the policy itself is tested in toolsafety).

func TestFileToolsBlockFluxProviderConfig(t *testing.T) {
	fluxDir := filepath.Join(t.TempDir(), "flux")
	t.Setenv("FLUX_CONFIG_DIR", fluxDir)
	providerPath := filepath.Join(fluxDir, "provider.json")

	readInput, _ := json.Marshal(map[string]string{"path": providerPath})
	editInput, _ := json.Marshal(map[string]string{
		"path": providerPath, "old_str": "old", "new_str": "new",
	})
	writeInput, _ := json.Marshal(map[string]string{
		"path": providerPath, "content": "safe routing metadata",
	})
	tests := []struct {
		name string
		run  func() error
	}{
		{name: "Read", run: func() error {
			_, err := (FileReadTool{}).Execute(testCtx(), readInput)
			return err
		}},
		{name: "Edit", run: func() error {
			_, err := (FileEditTool{}).Execute(testCtx(), editInput)
			return err
		}},
		{name: "Write", run: func() error {
			_, err := (FileWriteTool{}).Execute(testCtx(), writeInput)
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			if err == nil || !strings.Contains(err.Error(), "blocked") {
				t.Fatalf("%s error = %v, want sensitive-path block", tt.name, err)
			}
		})
	}
}

// TestFileRead_BlocksSymlinkToSensitiveFile verifies the read tool resolves
// symlinks before opening (M13): reading through a symlink that points at a
// sensitive target is blocked, while a symlink to an ordinary file works.
func TestFileRead_BlocksSymlinkToSensitiveFile(t *testing.T) {
	fluxDir := filepath.Join(t.TempDir(), "flux")
	if err := os.MkdirAll(fluxDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FLUX_CONFIG_DIR", fluxDir)
	providerPath := filepath.Join(fluxDir, "provider.json")
	if err := os.WriteFile(providerPath, []byte(`{"key":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	workDir := t.TempDir()
	link := filepath.Join(workDir, "readme.md")
	if err := os.Symlink(providerPath, link); err != nil {
		t.Fatal(err)
	}
	in, _ := json.Marshal(map[string]string{"path": link})
	_, err := (FileReadTool{}).Execute(testCtx(), in)
	if err == nil || !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("expected sensitive-path block for symlinked provider config, got %v", err)
	}

	// A symlink to an ordinary file must still read fine.
	plain := filepath.Join(workDir, "plain.txt")
	if err := os.WriteFile(plain, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	link2 := filepath.Join(workDir, "link2.txt")
	if err := os.Symlink(plain, link2); err != nil {
		t.Fatal(err)
	}
	in2, _ := json.Marshal(map[string]string{"path": link2})
	out, err := (FileReadTool{}).Execute(testCtx(), in2)
	if err != nil {
		t.Fatalf("expected symlinked plain file to read, got %v", err)
	}
	if !strings.Contains(out, "hello") {
		t.Fatalf("expected content through symlink, got %q", out)
	}
}

// TestIsSensitivePath_SecretBasenames verifies the expanded basename blocklist
// (secrets.txt, .git-credentials, private keys, …) applies anywhere, not just
// under the home directory.

func TestBashSensitivePathIntegration(t *testing.T) {
	if !IsSuspicious("cat ~/.ssh/id_rsa") {
		t.Error("IsSuspicious should flag reads of SSH private keys")
	}
	if !isHardDeny("cat ~/.aws/credentials") {
		t.Error("isHardDeny should block credential reads when prompts are bypassed")
	}
	if isHardDeny("go build ./...") {
		t.Error("isHardDeny should not block ordinary commands")
	}
}

func TestCommandReferencesSensitivePath_FluxConfigDir(t *testing.T) {
	fluxDir := filepath.Join(t.TempDir(), "flux config with spaces")
	t.Setenv("FLUX_CONFIG_DIR", fluxDir)

	commands := []string{
		`cat "` + filepath.Join(fluxDir, "provider.json") + `"`,
		`cat "$FLUX_CONFIG_DIR/provider.json"`,
		`cat "${FLUX_CONFIG_DIR}/provider.json"`,
		`cat "${FLUX_CONFIG_DIR%/}/provider.json"`,
		`printf '%s\n' "$FLUX_CONFIG_DIR"`,
		"cat " + strings.ReplaceAll(filepath.Join(fluxDir, "provider.json"), " ", `\ `),
	}
	for _, command := range commands {
		if reason := CommandReferencesSensitivePath(command); reason == "" {
			t.Fatalf("CommandReferencesSensitivePath(%q) = empty, want blocked", command)
		}
		if !IsSuspicious(command) {
			t.Fatalf("IsSuspicious(%q) = false, want Bash prompt", command)
		}
		if !isHardDeny(command) {
			t.Fatalf("isHardDeny(%q) = false, want Bash block", command)
		}
	}
}
