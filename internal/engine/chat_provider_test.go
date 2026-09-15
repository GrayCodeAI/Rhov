package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/GrayCodeAI/flux/llm"
	"github.com/GrayCodeAI/rho/internal/types"
)

type recordingChatClient struct {
	chatOptions   types.ChatOptions
	streamOptions types.ChatOptions
}

func (c *recordingChatClient) Chat(_ context.Context, _ []types.FluxMessage, opts types.ChatOptions) (*types.FluxResponse, error) {
	c.chatOptions = opts
	return &types.FluxResponse{Content: "ok"}, nil
}

func (c *recordingChatClient) StreamChatContinue(_ context.Context, _ []types.FluxMessage, opts types.ChatOptions, _ types.ContinuationConfig) (*types.StreamResult, error) {
	c.streamOptions = opts
	events := make(chan types.FluxStreamEvent)
	close(events)
	return llm.NewStreamResult(events, "", nil), nil
}

func TestEngineChatProviderAppliesResolvedSelection(t *testing.T) {
	client := &recordingChatClient{}
	provider := &engineChatProvider{client: client, provider: "anthropic", model: "claude-test"}

	if _, err := provider.Chat(context.Background(), nil, types.ChatOptions{}); err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if client.chatOptions.Provider != "anthropic" || client.chatOptions.Model != "claude-test" {
		t.Fatalf("Chat() options = %#v, want resolved provider and model", client.chatOptions)
	}

	if _, err := provider.StreamChat(context.Background(), nil, types.ChatOptions{}); err != nil {
		t.Fatalf("StreamChat() error = %v", err)
	}
	if client.streamOptions.Provider != "anthropic" || client.streamOptions.Model != "claude-test" {
		t.Fatalf("StreamChat() options = %#v, want resolved provider and model", client.streamOptions)
	}
}

func TestEngineChatProviderPreservesRequestOverrides(t *testing.T) {
	client := &recordingChatClient{}
	provider := &engineChatProvider{client: client, provider: "anthropic", model: "claude-test"}

	opts := types.ChatOptions{Provider: "openai", Model: "gpt-test"}
	if _, err := provider.Chat(context.Background(), nil, opts); err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if client.chatOptions.Provider != "openai" || client.chatOptions.Model != "gpt-test" {
		t.Fatalf("Chat() options = %#v, want request overrides", client.chatOptions)
	}
}

func TestEngineChatProviderIdentityAndPing(t *testing.T) {
	provider := &engineChatProvider{client: &recordingChatClient{}, provider: "openai"}
	if got := provider.Name(); got != "openai" {
		t.Fatalf("Name() = %q, want openai", got)
	}
	if err := provider.Ping(context.Background()); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := provider.Ping(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Ping() error = %v, want context.Canceled", err)
	}
}
