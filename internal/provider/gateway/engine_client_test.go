package gateway

import (
	"testing"

	fluxengine "github.com/GrayCodeAI/flux/engine"
	"github.com/GrayCodeAI/rho/internal/types"
)

func TestEngineAdapterPreservesRhoRequestOptions(t *testing.T) {
	topP := 0.75
	thinking := true
	request := toEngineRequest(
		[]types.FluxMessage{{
			Role:         "user",
			Content:      "inspect",
			ContentParts: []types.ContentPart{{Type: "image_url", ImageURL: &types.ImageURLPart{URL: "data:image/png;base64,abc"}}},
		}},
		types.ChatOptions{
			Provider: "openrouter", Model: "openrouter/auto", MaxTokens: 2048,
			Tools:  []types.FluxTool{{Name: "read_file", Description: "read", Parameters: map[string]interface{}{"type": "object"}}},
			System: "system", EnableCaching: true, ReasoningEffort: "high",
			GLMThinkingEnabled: &thinking, TopP: &topP, ServiceTier: "priority",
			MetadataUserID: "rho-user-1",
			ResponseFormat: &types.ResponseFormat{Type: "json_schema", Schema: `{"type":"object"}`},
		},
		types.ContinuationConfig{MaxContinuations: 2, MaxTotalTokens: 9000},
	)
	if request.Preference.PreferredProvider != "openrouter" || request.Preference.PreferredModelID != "openrouter/auto" {
		t.Fatalf("selection lost: %+v", request.Preference)
	}
	if !request.Requirements.Tools || !request.Requirements.Vision || !request.Requirements.StructuredJSON || !request.Requirements.Reasoning {
		t.Fatalf("requirements lost: %+v", request.Requirements)
	}
	if request.Limits.MaxContinuations != 2 || request.Limits.MaxTotalOutputTokens != 9000 {
		t.Fatalf("continuation lost: %+v", request.Limits)
	}
	if !request.Options.EnableCaching || request.Options.ReasoningEffort != "high" || request.Options.ThinkingEnabled == nil || request.Options.GLMThinkingEnabled == nil || request.Options.TopP == nil || request.Options.ServiceTier != "priority" {
		t.Fatalf("advanced options lost: %+v", request.Options)
	}
	if request.Metadata.UserID != "rho-user-1" {
		t.Fatalf("metadata user ID lost: %+v", request.Metadata)
	}
	if len(request.Messages) != 1 || len(request.Messages[0].ContentParts) != 1 || request.Messages[0].ContentParts[0].ImageURL == nil || request.Messages[0].ContentParts[0].ImageURL.URL == "" {
		t.Fatalf("multimodal message lost: %+v", request.Messages)
	}
}

func TestEngineAdapterOnlyRequiresGLMReasoningWhenEnabled(t *testing.T) {
	disabled := false
	enabled := true

	if request := toEngineRequest(nil, types.ChatOptions{ThinkingEnabled: &disabled}, types.ContinuationConfig{}); request.Requirements.Reasoning {
		t.Fatalf("ThinkingEnabled=false unexpectedly requires reasoning: %+v", request.Requirements)
	}
	if request := toEngineRequest(nil, types.ChatOptions{ThinkingEnabled: &enabled}, types.ContinuationConfig{}); !request.Requirements.Reasoning {
		t.Fatalf("ThinkingEnabled=true did not require reasoning: %+v", request.Requirements)
	}
	if request := toEngineRequest(nil, types.ChatOptions{GLMThinkingEnabled: &enabled}, types.ContinuationConfig{}); !request.Requirements.Reasoning {
		t.Fatalf("deprecated GLMThinkingEnabled=true did not require reasoning: %+v", request.Requirements)
	}
}

func TestEngineAdapterNormalizesEventsForRhoLoop(t *testing.T) {
	tests := []struct {
		in       fluxengine.Event
		wantType string
		emit     bool
		provider string
		model    string
	}{
		{
			in: fluxengine.Event{Type: fluxengine.EventRouteSelected, Route: &fluxengine.Route{
				Provider: "openai", Model: "openai/gpt-5", DeploymentRouting: true,
			}},
			wantType: "route_selected", emit: true, provider: "openai", model: "openai/gpt-5",
		},
		{
			in: fluxengine.Event{Type: fluxengine.EventRouteChanged, Route: &fluxengine.Route{
				Provider: "anthropic", Model: "anthropic/claude-sonnet-4-6", DeploymentRouting: true,
			}},
			wantType: "route_changed", emit: true, provider: "anthropic", model: "anthropic/claude-sonnet-4-6",
		},
		{in: fluxengine.Event{Type: fluxengine.EventContentDelta, Content: "x"}, wantType: "content", emit: true},
		{in: fluxengine.Event{Type: fluxengine.EventThinkingDelta}, wantType: "thinking", emit: true},
		{in: fluxengine.Event{Type: fluxengine.EventToolCallDone, ToolCall: &fluxengine.ToolCall{Name: "read"}}, wantType: "tool_call", emit: true},
		{in: fluxengine.Event{Type: fluxengine.EventUsage, Usage: &fluxengine.Usage{TotalTokens: 8}}, wantType: "usage", emit: true},
		{in: fluxengine.Event{Type: fluxengine.EventDone, StopReason: "end_turn", Usage: &fluxengine.Usage{PromptTokens: 5, CompletionTokens: 3, TotalTokens: 8}}, wantType: "done", emit: true},
	}
	for _, tt := range tests {
		got, emit := fromEngineEvent(tt.in)
		if emit != tt.emit || got.Type != tt.wantType {
			t.Fatalf("event %q => type %q emit %v, want %q %v", tt.in.Type, got.Type, emit, tt.wantType, tt.emit)
		}
		if tt.provider != "" && (got.Route == nil || got.Route.Provider != tt.provider || got.Route.Model != tt.model || !got.Route.DeploymentRouting) {
			t.Fatalf("event %q lost resolved route: %+v", tt.in.Type, got.Route)
		}
		if tt.in.Type == fluxengine.EventDone && (got.Usage == nil || got.Usage.PromptTokens != 5 || got.Usage.CompletionTokens != 3 || got.Usage.TotalTokens != 8) {
			t.Fatalf("terminal usage lost: %+v", got.Usage)
		}
	}
}

func TestEngineAdapterPreservesResolvedRouteInBlockingResponse(t *testing.T) {
	got := fromEngineResponse(&fluxengine.GenerateResponse{
		Content: "ok",
		Route: &fluxengine.Route{
			Provider: "openai", Model: "openai/gpt-5", DeploymentRouting: true,
		},
	})
	if got.Route == nil || got.Route.Provider != "openai" || got.Route.Model != "openai/gpt-5" || !got.Route.DeploymentRouting {
		t.Fatalf("blocking response lost resolved route: %+v", got.Route)
	}
}
