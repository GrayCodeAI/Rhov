package tool

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkIsSensitivePath(b *testing.B) {
	// Use realistic paths under the user's home / temp dir. Literal "/home/..."
	// paths hit macOS's autofs automounter and would measure the wrong thing.
	home, _ := os.UserHomeDir()
	paths := []string{
		filepath.Join(home, "project", "main.go"),
		filepath.Join(home, ".ssh", "id_rsa"),
		filepath.Join(home, "project", ".env"),
		filepath.Join(os.TempDir(), "notes.txt"),
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = IsSensitivePath(paths[i%len(paths)])
	}
}

func BenchmarkIsDestructiveCommand(b *testing.B) {
	cmds := []string{
		"go test ./...",
		"git status",
		"rm -rf /",
		"echo hello",
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = IsDestructiveCommand(cmds[i%len(cmds)])
	}
}

func BenchmarkValidateShellCommand(b *testing.B) {
	ctx := context.Background()
	cmds := []string{
		"go build ./...",
		"git diff --stat",
		"ls -la",
		"cat README.md",
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = validateShellCommand(ctx, cmds[i%len(cmds)])
	}
}
