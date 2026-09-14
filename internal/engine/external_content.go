package engine

import "github.com/GrayCodeAI/rho/internal/permissions"

// externalContentSource maps a tool name to the untrusted content source used
// to wrap its output before it enters the model context.
func externalContentSource(toolName string) (permissions.ContentSource, bool) {
	switch toolName {
	case "WebFetch", "web_fetch", "webfetch":
		return permissions.SourceWebFetch, true
	case "WebSearch", "web_search", "websearch":
		return permissions.SourceWebSearch, true
	case "Browser", "browser":
		return permissions.SourceBrowser, true
	default:
		return "", false
	}
}

// wrapExternalToolResult marks output from network-facing tools as untrusted
// so the model treats it as data rather than instructions. Without this, a
// fetched page containing "ignore previous instructions…" enters the context
// as ordinary text with no boundary marker.
func wrapExternalToolResult(toolName, content string) string {
	src, ok := externalContentSource(toolName)
	if !ok {
		return content
	}
	return permissions.WrapWebContent(content, src)
}
