package mcp

import (
	"encoding/json"
	"testing"
)

// FuzzParseToolCallResult ensures the MCP tool-result parser never panics on
// arbitrary server responses.
func FuzzParseToolCallResult(f *testing.F) {
	f.Add([]byte(`{"content":[{"type":"text","text":"hi"}]}`))
	f.Add([]byte(`{"content":[{"type":"image","data":"..."}],"isError":true}`))
	f.Add([]byte(`not json`))
	f.Add([]byte(`{"content":[{"type":"text","text":"a"},{"type":"text","text":"b"}]}`))

	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = parseToolCallResult(json.RawMessage(b))
	})
}
