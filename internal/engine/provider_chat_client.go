package engine

import (
	"context"
	"errors"

	"github.com/GrayCodeAI/rho/internal/types"
)

// NewUnavailableChatClient preserves Session construction while surfacing
// Flux transport setup failures at the first chat call.
func NewUnavailableChatClient(err error) ChatClient {
	if err == nil {
		err = errors.New("rho: chat transport unavailable")
	}
	return &unavailableChatClient{err: err}
}

type unavailableChatClient struct {
	err error
}

func (c *unavailableChatClient) Chat(context.Context, []types.FluxMessage, types.ChatOptions) (*types.FluxResponse, error) {
	return nil, c.err
}

func (c *unavailableChatClient) StreamChatContinue(context.Context, []types.FluxMessage, types.ChatOptions, types.ContinuationConfig) (*types.StreamResult, error) {
	return nil, c.err
}
