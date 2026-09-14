package memory

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// EnhancedMemoryManager extends MemoryManager with the local memory
// subsystems: retrieval metrics, continuity scoring, and skill distillation.
// It no longer carries graph-backed subsystems (auto-capture, proactive
// context, confidence, code links, session diff, cross-project, graph budget,
// shared mission memory) — those were built on the removed harrier bridge.
type EnhancedMemoryManager struct {
	*MemoryManager

	Retrieval  *RetrievalMetrics
	Continuity *ContinuityTracker

	sessionID string
	mu        sync.Mutex
}

// NewEnhancedMemoryManager creates a fully-integrated local memory system.
func NewEnhancedMemoryManager(projectDir string) *EnhancedMemoryManager {
	base := NewMemoryManager(projectDir)

	return &EnhancedMemoryManager{
		MemoryManager: base,
		Retrieval:     NewRetrievalMetrics(projectDir),
		Continuity:    NewContinuityTracker(projectDir),
	}
}

// StartSession initializes all session-level tracking.
func (em *EnhancedMemoryManager) StartSession(sessionID string) {
	em.mu.Lock()
	defer em.mu.Unlock()

	em.sessionID = sessionID

	if em.Continuity != nil {
		em.Continuity.StartSession(sessionID, false)
	}
}

// EndSession performs end-of-session processing: metric persistence and
// continuity scoring.
func (em *EnhancedMemoryManager) EndSession(success bool) {
	em.mu.Lock()
	defer em.mu.Unlock()

	if em.Continuity != nil {
		em.Continuity.EndSession(success)
	}

	if em.Retrieval != nil {
		em.Retrieval.Save()
	}
}

// Recall performs a recall that tracks metrics and continuity.
// Implements engine.MemoryRecaller.
func (em *EnhancedMemoryManager) Recall(query string, tokenBudget int) (string, error) {
	result, err := em.MemoryManager.Recall(query, tokenBudget)
	if err != nil {
		return "", err
	}

	if em.Retrieval != nil {
		resultCount := 0
		if result != "" {
			resultCount = strings.Count(result, "\n") + 1
		}
		em.Retrieval.RecordRecall(query, resultCount, len(result)/4, "base")
	}
	if em.Continuity != nil && result != "" {
		em.Continuity.RecordMemoryUse(1, len(result)/4)
	}

	return result, nil
}

// Remember stores memory through the base manager.
// Implements engine.MemoryRecaller.
func (em *EnhancedMemoryManager) Remember(ctx context.Context, content, category string) error {
	return em.MemoryManager.Remember(ctx, content, category)
}

// OnToolResult is retained as a no-op hook for the tool pipeline.
func (em *EnhancedMemoryManager) OnToolResult(string, map[string]interface{}, string, bool) {}

// ProactiveContextForFile is retained for the tool pipeline; no graph backend
// means there is nothing file-scoped to inject.
func (em *EnhancedMemoryManager) ProactiveContextForFile(string) string { return "" }

// GlobalContext is retained for the prompt builder; no cross-project backend.
func (em *EnhancedMemoryManager) GlobalContext(int) string { return "" }

// FormatForPrompt builds the memory context for prompt injection.
func (em *EnhancedMemoryManager) FormatForPrompt() string {
	return em.MemoryManager.FormatForPrompt()
}

// StatusSummary returns a concise status line for the memory system.
func (em *EnhancedMemoryManager) StatusSummary() string {
	var parts []string

	if em.Retrieval != nil {
		if s := em.Retrieval.FormatSummary(); s != "" {
			parts = append(parts, s)
		}
	}
	if em.Continuity != nil {
		if s := em.Continuity.FormatSummary(); s != "" {
			parts = append(parts, s)
		}
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " | ")
}

// Close shuts down all subsystems gracefully.
func (em *EnhancedMemoryManager) Close() {
	if em.Retrieval != nil {
		em.Retrieval.Save()
	}
	if em.Continuity != nil {
		em.Continuity.Save()
	}
}

// HealthCheck verifies the memory system is working correctly.
func (em *EnhancedMemoryManager) HealthCheck() map[string]interface{} {
	health := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
	}

	if em.Retrieval != nil {
		health["hit_rate"] = em.Retrieval.HitRate()
		health["total_recalls"] = em.Retrieval.TotalRecalls()
	}
	if em.Continuity != nil {
		r := em.Continuity.Report()
		health["continuity_score"] = r.AvgScore
		health["total_sessions"] = r.TotalSessions
		health["tokens_saved"] = r.TotalTokensSaved
	}

	return health
}

// DiagnosticReport generates a detailed diagnostic for troubleshooting.
func (em *EnhancedMemoryManager) DiagnosticReport(_ context.Context) string {
	var sb strings.Builder
	sb.WriteString("=== Memory System Diagnostic ===\n\n")

	if em.Retrieval != nil {
		r := em.Retrieval.Report()
		sb.WriteString("\nRetrieval:\n")
		sb.WriteString(fmt.Sprintf("  Total recalls: %d\n", r.TotalRecalls))
		sb.WriteString(fmt.Sprintf("  Hit rate: %.1f%%\n", r.HitRate*100))
		sb.WriteString(fmt.Sprintf("  Avg results: %.1f\n", r.AvgResultCount))
		sb.WriteString(fmt.Sprintf("  Tokens saved: %d\n", r.TotalTokensSaved))
	}

	if em.Continuity != nil {
		r := em.Continuity.Report()
		sb.WriteString("\nContinuity:\n")
		sb.WriteString(fmt.Sprintf("  Sessions tracked: %d\n", r.TotalSessions))
		sb.WriteString(fmt.Sprintf("  Avg score: %.0f/100\n", r.AvgScore))
		sb.WriteString(fmt.Sprintf("  Memory contribution: %.1f%%\n", r.MemoryContribution*100))
	}

	return sb.String()
}
