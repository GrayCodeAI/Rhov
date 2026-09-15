package engine

import (
	"strings"

	"github.com/GrayCodeAI/rho/internal/permissions"
)

// externalContentSource maps a tool name to the untrusted content source used
// to wrap its output before it enters the model context. It covers every tool
// whose output originates outside the local workspace: web fetches/searches,
// the browser, MCP servers (which may be remote and third-party), and remote
// API tools.
func externalContentSource(toolName string) (permissions.ContentSource, bool) {
	switch toolName {
	case "WebFetch", "web_fetch", "webfetch":
		return permissions.SourceWebFetch, true
	case "WebSearch", "web_search", "websearch", "SearchX", "AgenticFetch":
		return permissions.SourceWebSearch, true
	case "Browser", "browser":
		return permissions.SourceBrowser, true
	case "GitHub", "github":
		return permissions.SourceAPI, true
	}
	// MCP tools are named mcp__<server>__<tool> (current) or mcp_<server>_<tool>
	// (legacy). Their output is controlled by the MCP server, not the workspace.
	if strings.HasPrefix(toolName, "mcp__") || strings.HasPrefix(toolName, "mcp_") {
		return permissions.SourceAPI, true
	}
	return "", false
}

// wrapExternalToolResult marks output from network-facing tools as untrusted
// so the model treats it as data rather than instructions. Without this, a
// fetched page or MCP result containing "ignore previous instructions…" enters
// the context as ordinary text with no boundary marker.
func wrapExternalToolResult(toolName, content string) string {
	src, ok := externalContentSource(toolName)
	if !ok {
		return content
	}
	return permissions.WrapWebContent(content, src)
}
