package engine

import (
	"context"
	"time"

	"github.com/GrayCodeAI/rho/internal/engine/token"

	"github.com/GrayCodeAI/rho/internal/types"

	"github.com/GrayCodeAI/rho/internal/engine/compact"
)

type MicroCompactStrategy struct{}

func (s *MicroCompactStrategy) Name() string { return "micro" }

func (s *MicroCompactStrategy) ShouldTrigger(msgs []types.FluxMessage, tokenCount, threshold int) bool {
	if tokenCount < threshold/2 {
		return false
	}
	compactableCount := 0
	for _, m := range msgs {
		if len(m.ToolResults) > 0 && compact.IsCompactableTool(compact.ToolNameForResult(m, msgs)) {
			compactableCount++
		}
	}
	if compactableCount < 5 {
		return false
	}
	return compact.HasTimeGap(msgs, 60*time.Minute)
}

func (s *MicroCompactStrategy) Compact(ctx context.Context, sess *Session) (*compact.CompactResult, error) {
	messages := sess.Persistence().RawMessages()
	tokensBefore := token.EstimateTokens(messages)
	result := compact.MicrocompactMessages(messages, compact.DefaultMicroCompactConfig())
	tokensAfter := token.EstimateTokens(result)

	return &compact.CompactResult{
		Messages:     result,
		TokensBefore: tokensBefore,
		TokensAfter:  tokensAfter,
		Strategy:     "micro",
	}, nil
}
