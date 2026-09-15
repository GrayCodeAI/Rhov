package types

import (
	"context"

	"github.com/GrayCodeAI/flux/llm"
)

// ContentPart is a provider-neutral multimodal message part. Rho owns this
// conversation shape; the Flux engine adapter translates it at the transport
// boundary. It aliases the canonical contract type.
type ContentPart = llm.ContentPart

// ImageURLPart describes an image URL or data URI.
type ImageURLPart = llm.ImageURLPart

// InputAudioPart describes base64-encoded audio content.
type InputAudioPart = llm.InputAudioPart

// ChatProvider is Rho's transport-provider interface.
type ChatProvider interface {
	Chat(ctx context.Context, messages []FluxMessage, opts ChatOptions) (*FluxResponse, error)
	StreamChat(ctx context.Context, messages []FluxMessage, opts ChatOptions) (*StreamResult, error)
	Ping(ctx context.Context) error
	Name() string
}

// ChatClient is the session-level agent-loop client interface.
type ChatClient interface {
	Chat(ctx context.Context, messages []FluxMessage, opts ChatOptions) (*FluxResponse, error)
	StreamChatContinue(ctx context.Context, messages []FluxMessage, opts ChatOptions, cfg ContinuationConfig) (*StreamResult, error)
}

// ResponseFormat specifies the desired output format for a Rho runtime request.
type ResponseFormat = llm.ResponseFormat

// ToolChoiceOption controls how the model uses tools.
type ToolChoiceOption = llm.ToolChoiceOption

// FluxTool is Rho's runtime tool definition DTO.
type FluxTool = llm.FluxTool

// ChatOptions holds Rho-owned request options for an engine chat call.
type ChatOptions = llm.ChatOptions

// ContinuationConfig controls output continuation behavior for Rho runtime calls.
type ContinuationConfig = llm.ContinuationConfig

// DefaultContinuationConfig is Rho's agent-loop continuation policy. Flux
// receives these limits through its engine request rather than owning the
// product policy.
func DefaultContinuationConfig() ContinuationConfig {
	return ContinuationConfig{MaxContinuations: 3, MaxTotalTokens: 32000}
}

// ToolCall is Rho's runtime tool invocation DTO.
type ToolCall = llm.ToolCall

// ToolResult is Rho's runtime tool result DTO.
type ToolResult = llm.ToolResult

// FluxUsage tracks token usage for Rho runtime responses and streams.
type FluxUsage = llm.FluxUsage

// ResolvedRoute is Rho's view of the concrete provider/model route selected
// by the provider engine. Keeping this projection Rho-owned prevents Flux's
// transport DTOs from leaking into product state and observability payloads.
type ResolvedRoute = llm.ResolvedRoute

// FluxResponse is Rho's runtime chat response DTO.
type FluxResponse = llm.FluxResponse

// FluxStreamEvent is Rho's runtime stream event DTO.
type FluxStreamEvent = llm.FluxStreamEvent

// StreamResult wraps a Rho-owned streaming response with cleanup. It aliases
// the canonical contract type; its Close() method and canonical constructor
// (NewStreamResult) live in github.com/GrayCodeAI/flux/llm.
type StreamResult = llm.StreamResult

// FluxMessage is Rho's runtime conversation DTO.
// It intentionally mirrors the engine boundary shape while remaining Rho-owned.
type FluxMessage = llm.FluxMessage
