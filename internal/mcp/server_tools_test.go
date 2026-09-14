package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// RegisterDefaultTools
// ---------------------------------------------------------------------------

func TestRegisterDefaultTools_RegistersAllTools(t *testing.T) {
	server := NewMCPServer(ServerInfo{Name: "rho", Version: "test"})
	RegisterDefaultTools(server, nil)

	// Send tools/list
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n"
	resp := sendRequest(t, server, req)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}

	result := resp.Result.(map[string]interface{})
	tools := result["tools"].([]interface{})

	expectedNames := map[string]bool{
		"rho_chat":          false,
		"rho_search":        false,
		"rho_memory_recall": false,
		"rho_memory_store":  false,
		"rho_compress":      false,
	}

	for _, raw := range tools {
		tm := raw.(map[string]interface{})
		name := tm["name"].(string)
		if _, ok := expectedNames[name]; ok {
			expectedNames[name] = true
		}
	}

	for name, found := range expectedNames {
		if !found {
			t.Errorf("tool %q not registered", name)
		}
	}
	if len(tools) != 5 {
		t.Errorf("expected 5 tools, got %d", len(tools))
	}
}

// ---------------------------------------------------------------------------
// rho_chat tool
// ---------------------------------------------------------------------------

func TestRhoChatTool_ValidPrompt(t *testing.T) {
	server := NewMCPServer(ServerInfo{Name: "rho", Version: "test"})
	executor := func(ctx context.Context, name string, params json.RawMessage) (string, error) {
		if name != "agent" {
			return "", fmt.Errorf("expected agent, got %s", name)
		}
		return "chat response", nil
	}
	RegisterDefaultTools(server, executor)

	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"rho_chat","arguments":{"prompt":"hello"}}}` + "\n"
	resp := sendRequest(t, server, req)
	assertNoError(t, resp)
	assertResponseContains(t, resp, "chat response")
}

func TestRhoChatTool_EmptyPrompt(t *testing.T) {
	server := NewMCPServer(ServerInfo{Name: "rho", Version: "test"})
	RegisterDefaultTools(server, nil)

	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"rho_chat","arguments":{"prompt":""}}}` + "\n"
	resp := sendRequest(t, server, req)
	assertIsError(t, resp)
}

func TestRhoChatTool_MissingPrompt(t *testing.T) {
	server := NewMCPServer(ServerInfo{Name: "rho", Version: "test"})
	RegisterDefaultTools(server, nil)

	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"rho_chat","arguments":{}}}` + "\n"
	resp := sendRequest(t, server, req)
	assertIsError(t, resp)
}

// ---------------------------------------------------------------------------
// rho_search tool
// ---------------------------------------------------------------------------

func TestRhoSearchTool_ValidQuery(t *testing.T) {
	server := NewMCPServer(ServerInfo{Name: "rho", Version: "test"})
	executor := func(ctx context.Context, name string, params json.RawMessage) (string, error) {
		if name != "code_search" {
			return "", fmt.Errorf("expected code_search, got %s", name)
		}
		return "search results", nil
	}
	RegisterDefaultTools(server, executor)

	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"rho_search","arguments":{"query":"auth middleware"}}}` + "\n"
	resp := sendRequest(t, server, req)
	assertNoError(t, resp)
	assertResponseContains(t, resp, "search results")
}

func TestRhoSearchTool_EmptyQuery(t *testing.T) {
	server := NewMCPServer(ServerInfo{Name: "rho", Version: "test"})
	RegisterDefaultTools(server, nil)

	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"rho_search","arguments":{"query":""}}}` + "\n"
	resp := sendRequest(t, server, req)
	assertIsError(t, resp)
}

// ---------------------------------------------------------------------------
// rho_memory_recall tool
// ---------------------------------------------------------------------------

func TestRhoMemoryRecallTool_EmptyQuery(t *testing.T) {
	server := NewMCPServer(ServerInfo{Name: "rho", Version: "test"})
	RegisterDefaultTools(server, nil)

	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"rho_memory_recall","arguments":{"query":""}}}` + "\n"
	resp := sendRequest(t, server, req)
	assertIsError(t, resp)
}

func TestRhoMemoryRecallTool_NoMatches(t *testing.T) {
	t.Setenv("RHO_STATE_DIR", t.TempDir())
	server := NewMCPServer(ServerInfo{Name: "rho", Version: "test"})
	RegisterDefaultTools(server, nil)

	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"rho_memory_recall","arguments":{"query":"definitely-not-stored"}}}` + "\n"
	resp := sendRequest(t, server, req)
	assertNoError(t, resp)
	assertResponseContains(t, resp, "No memories matched")
}

// ---------------------------------------------------------------------------
// rho_memory_store tool
// ---------------------------------------------------------------------------

func TestRhoMemoryStoreTool_RoundTrip(t *testing.T) {
	t.Setenv("RHO_STATE_DIR", t.TempDir())
	server := NewMCPServer(ServerInfo{Name: "rho", Version: "test"})
	RegisterDefaultTools(server, nil)

	storeReq := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"rho_memory_store","arguments":{"key":"decision","content":"use Go for the backend"}}}` + "\n"
	storeResp := sendRequest(t, server, storeReq)
	assertNoError(t, storeResp)
	assertResponseContains(t, storeResp, "Stored memory")

	recallReq := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"rho_memory_recall","arguments":{"query":"backend"}}}` + "\n"
	recallResp := sendRequest(t, server, recallReq)
	assertNoError(t, recallResp)
	assertResponseContains(t, recallResp, "use Go for the backend")
}

func TestRhoMemoryStoreTool_MissingKey(t *testing.T) {
	server := NewMCPServer(ServerInfo{Name: "rho", Version: "test"})
	RegisterDefaultTools(server, nil)

	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"rho_memory_store","arguments":{"content":"some content"}}}` + "\n"
	resp := sendRequest(t, server, req)
	assertIsError(t, resp)
}

func TestRhoMemoryStoreTool_MissingContent(t *testing.T) {
	server := NewMCPServer(ServerInfo{Name: "rho", Version: "test"})
	RegisterDefaultTools(server, nil)

	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"rho_memory_store","arguments":{"key":"some-key"}}}` + "\n"
	resp := sendRequest(t, server, req)
	assertIsError(t, resp)
}

// ---------------------------------------------------------------------------
// rho_compress tool
// ---------------------------------------------------------------------------

func TestRhoCompressTool_ValidText(t *testing.T) {
	server := NewMCPServer(ServerInfo{Name: "rho", Version: "test"})
	RegisterDefaultTools(server, nil)

	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"rho_compress","arguments":{"text":"the quick brown fox jumps over the lazy dog. the quick brown fox jumps over the lazy dog."}}}` + "\n"
	resp := sendRequest(t, server, req)
	assertNoError(t, resp)
	assertResponseContains(t, resp, "original_tokens")
}

func TestRhoCompressTool_EmptyText(t *testing.T) {
	server := NewMCPServer(ServerInfo{Name: "rho", Version: "test"})
	RegisterDefaultTools(server, nil)

	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"rho_compress","arguments":{"text":""}}}` + "\n"
	resp := sendRequest(t, server, req)
	assertIsError(t, resp)
}

// ---------------------------------------------------------------------------
// delegateToExecutor
// ---------------------------------------------------------------------------

func TestDelegateToExecutor_NilExecutor(t *testing.T) {
	_, err := delegateToExecutor(context.Background(), nil, "test", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error with nil executor")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected 'not configured' in error, got: %s", err.Error())
	}
}

func TestDelegateToExecutor_CallsExecutor(t *testing.T) {
	executor := func(ctx context.Context, name string, params json.RawMessage) (string, error) {
		return "executed:" + name, nil
	}
	result, err := delegateToExecutor(context.Background(), executor, "my_tool", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if result != "executed:my_tool" {
		t.Errorf("expected 'executed:my_tool', got %q", result)
	}
}

func TestDelegateToExecutor_ExecutorError(t *testing.T) {
	executor := func(ctx context.Context, name string, params json.RawMessage) (string, error) {
		return "", fmt.Errorf("executor failed")
	}
	_, err := delegateToExecutor(context.Background(), executor, "failing", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error from executor")
	}
}

// ---------------------------------------------------------------------------
// Tool schema validation
// ---------------------------------------------------------------------------

func TestToolSchemas_HaveRequiredFields(t *testing.T) {
	server := NewMCPServer(ServerInfo{Name: "rho", Version: "test"})
	RegisterDefaultTools(server, nil)

	req := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n"
	resp := sendRequest(t, server, req)

	result := resp.Result.(map[string]interface{})
	tools := result["tools"].([]interface{})

	for _, raw := range tools {
		tm := raw.(map[string]interface{})
		name := tm["name"].(string)

		if tm["description"] == nil || tm["description"] == "" {
			t.Errorf("tool %q has empty description", name)
		}
		schema, ok := tm["inputSchema"].(map[string]interface{})
		if !ok {
			t.Errorf("tool %q has no inputSchema", name)
			continue
		}
		if schema["type"] != "object" {
			t.Errorf("tool %q inputSchema type = %v, want object", name, schema["type"])
		}
		if schema["properties"] == nil {
			t.Errorf("tool %q inputSchema has no properties", name)
		}
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func assertNoError(t *testing.T, resp JSONRPCResponse) {
	t.Helper()
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %+v", resp.Error)
	}
}

func assertIsError(t *testing.T, resp JSONRPCResponse) {
	t.Helper()
	if resp.Error != nil {
		return // RPC error is also acceptable
	}
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("expected result map")
	}
	if result["isError"] != true {
		t.Errorf("expected isError=true in result, got: %v", result)
	}
}

func assertResponseContains(t *testing.T, resp JSONRPCResponse, substr string) {
	t.Helper()
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected result map, got %T", resp.Result)
	}
	content, ok := result["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatal("expected content array")
	}
	item := content[0].(map[string]interface{})
	text := item["text"].(string)
	if !strings.Contains(text, substr) {
		t.Errorf("expected response to contain %q, got %q", substr, text)
	}
}
