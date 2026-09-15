package tool

import (
	"context"
	"net/http"
	"time"

	"github.com/GrayCodeAI/rho/internal/toolsafety"
)

// This file re-exports the low-level safety helpers that now live in
// internal/toolsafety. Keeping the tool-package names stable lets existing
// callers migrate incrementally; new code should import toolsafety directly.

// BinaryIndicator is returned instead of binary file content.
const BinaryIndicator = toolsafety.BinaryIndicator

// MaxOutputBytes is the output cap applied by TruncateOutput.
const MaxOutputBytes = toolsafety.MaxOutputBytes

// ToolTimeout returns the default timeout for a given tool name.
func ToolTimeout(toolName string) time.Duration { return toolsafety.ToolTimeout(toolName) }

// TruncateOutput trims output to the safety cap and appends an indicator.
func TruncateOutput(s string) string { return toolsafety.TruncateOutput(s) }

// IsDestructiveCommand reports whether a shell command is destructive.
func IsDestructiveCommand(command string) bool { return toolsafety.IsDestructiveCommand(command) }

// DetectCredentials returns a description when content looks like a credential.
func DetectCredentials(content string) string { return toolsafety.DetectCredentials(content) }

// IsSensitivePath returns a reason when path is a blocked sensitive file.
func IsSensitivePath(path string) string { return toolsafety.IsSensitivePath(path) }

// CommandReferencesSensitivePath returns a reason when a shell command
// references a blocked sensitive path.
func CommandReferencesSensitivePath(command string) string {
	return toolsafety.CommandReferencesSensitivePath(command)
}

// ResolvePath returns the absolute, symlink-resolved path.
func ResolvePath(path string) (string, error) { return toolsafety.ResolvePath(path) }

// IsBinaryContent reports whether data appears to be binary.
func IsBinaryContent(data []byte) bool { return toolsafety.IsBinaryContent(data) }

// SegmentCommand splits a command string on shell operators.
func SegmentCommand(cmd string) []string { return toolsafety.SegmentCommand(cmd) }

// WithSSRFSkip returns a context that skips SSRF URL validation.
func WithSSRFSkip(ctx context.Context) context.Context { return toolsafety.WithSSRFSkip(ctx) }

// ValidateURLPublic rejects private/link-local URLs and pins the resolved IP.
func ValidateURLPublic(ctx context.Context, rawURL string) (pinnedURL, originalHost string, err error) {
	return toolsafety.ValidateURLPublic(ctx, rawURL)
}

// SSRFSafeClient returns an http.Client that validates redirect targets.
func SSRFSafeClient(ctx context.Context, timeout time.Duration) *http.Client {
	return toolsafety.SSRFSafeClient(ctx, timeout)
}
