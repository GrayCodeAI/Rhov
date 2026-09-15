package tool

import (
	"context"
	"testing"
)

func BenchmarkIsSensitivePath(b *testing.B) {
	paths := []string{
		"/home/user/project/main.go",
		"/home/user/.ssh/id_rsa",
		"/home/user/project/.env",
		"/tmp/notes.txt",
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
