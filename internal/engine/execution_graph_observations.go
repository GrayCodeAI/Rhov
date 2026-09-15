package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	fluxengine "github.com/GrayCodeAI/flux/engine"
	fluxgraph "github.com/GrayCodeAI/flux/graph"
	graphcontracts "github.com/GrayCodeAI/rho/internal/contracts/graph"
	policycontracts "github.com/GrayCodeAI/rho/internal/contracts/policy"
	"github.com/GrayCodeAI/rho/internal/engine/token"
	"github.com/GrayCodeAI/rho/internal/graphjournal"
	"github.com/GrayCodeAI/rho/internal/types"
)

func (s *Session) recordPolicyObservation(tc types.ToolCall, stage string, allowed bool, reason string) {
	// Tamper-evident security event log: record every denial regardless of
	// whether graph observation is available, so enforcement history is
	// auditable even when the session transcript is gone.
	if !allowed {
		s.recordSecurityDenial(tc, stage, reason)
	}
	sessionID := s.executionGraphSessionID()
	if sessionID == "" {
		return
	}
	verdict := policycontracts.Allow(reason)
	verdict.Rule = strings.TrimSpace(stage)
	verdict.Source = "rho." + strings.TrimSpace(stage)
	if !allowed {
		verdict = policycontracts.Deny(reason, strings.TrimSpace(stage))
		verdict.Source = "rho." + strings.TrimSpace(stage)
	}
	if err := graphjournal.AppendPolicy(sessionID, tc.ID, stage, verdict, time.Now()); err != nil {
		s.Logger().Warn("graph observation append failed", map[string]interface{}{
			"kind":  graphjournal.KindPolicy,
			"stage": stage,
		})
	}
}

func (s *Session) recordVerificationObservation(tc types.ToolCall, output string, isErr bool) {
	if canonicalToolName(tc.Name) != "VerifyPlanExecution" {
		return
	}
	sessionID := s.executionGraphSessionID()
	if sessionID == "" {
		return
	}

	var result struct {
		AllVerified bool `json:"allVerified"`
		TotalSteps  int  `json:"totalSteps"`
		Verified    int  `json:"verified"`
	}
	failed := isErr
	findingCount := 0
	if !isErr {
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			failed = true
			findingCount = 1
		} else {
			failed = !result.AllVerified
			findingCount = result.TotalSteps - result.Verified
			if findingCount < 0 {
				findingCount = 0
			}
		}
	} else {
		findingCount = 1
	}

	maxSeverity := "info"
	if failed {
		maxSeverity = "medium"
	}
	if err := graphjournal.AppendVerification(
		sessionID,
		tc.ID,
		"verify-plan-execution",
		failed,
		findingCount,
		maxSeverity,
		"plan-execution",
		time.Now(),
	); err != nil {
		s.Logger().Warn("graph observation append failed", map[string]interface{}{
			"kind":  graphjournal.KindVerify,
			"stage": "verify-plan-execution",
		})
	}
}

func (s *Session) executionGraphSessionID() string {
	if s == nil {
		return ""
	}
	if p := s.Persistence(); p != nil {
		return strings.TrimSpace(p.PersistID())
	}
	return ""
}

func (s *Session) configuredWorkingDir() string {
	if s == nil || s.Tools() == nil {
		return ""
	}
	return strings.TrimSpace(s.Tools().WorkingDir())
}

// SessionID returns the persistence ID of this session, or "" before one is
// assigned. Used by lifecycle bookkeeping to attribute cost entries to the
// real session instead of fabricated IDs.
func (s *Session) SessionID() string {
	return s.executionGraphSessionID()
}

func (s *Session) recordCompressionObservation(source, stage string, stats token.Stats) {
	sessionID := s.executionGraphSessionID()
	if sessionID == "" || stats.OriginalTokens <= 0 {
		return
	}
	repositoryDir := s.configuredWorkingDir()
	if repositoryDir == "" {
		repositoryDir, _ = os.Getwd()
	}
	repositoryID := ""
	if repositoryDir != "" {
		repositoryID = filepath.Base(filepath.Clean(repositoryDir))
	}
	observedAt := time.Now().UTC()
	export, err := token.BuildRuntimeGraph(token.RuntimeGraphInput{
		Compression:   &stats,
		Source:        source,
		ObservedAt:    observedAt,
		Scope:         graphcontracts.Scope{RepositoryID: repositoryID},
		CorrelationID: sessionID,
	})
	if err == nil {
		err = graphjournal.AppendRuntimeGraph(
			sessionID, "", stage, "token",
			export.Nodes, export.Edges, export.Events, observedAt,
		)
	}
	if err != nil {
		s.Logger().Warn("graph observation append failed", map[string]interface{}{
			"kind":  graphjournal.KindRuntime,
			"stage": stage,
		})
	}
}

func (s *Session) recordRedactionObservation(source string, matchCount int, types map[string]int) {
	sessionID := s.executionGraphSessionID()
	if sessionID == "" || matchCount <= 0 {
		return
	}
	repositoryDir := s.configuredWorkingDir()
	if repositoryDir == "" {
		repositoryDir, _ = os.Getwd()
	}
	repositoryID := ""
	if repositoryDir != "" {
		repositoryID = filepath.Base(filepath.Clean(repositoryDir))
	}
	observedAt := time.Now().UTC()
	export, err := token.BuildRuntimeGraph(token.RuntimeGraphInput{
		Redaction: &token.RedactionSummary{
			MatchCount: matchCount,
			Types:      types,
		},
		Source:        source,
		ObservedAt:    observedAt,
		Scope:         graphcontracts.Scope{RepositoryID: repositoryID},
		CorrelationID: sessionID,
	})
	if err == nil {
		err = graphjournal.AppendRuntimeGraph(
			sessionID, "", "response-redaction", "token",
			export.Nodes, export.Edges, export.Events, observedAt,
		)
	}
	if err != nil {
		s.Logger().Warn("graph observation append failed", map[string]interface{}{
			"kind":  graphjournal.KindRuntime,
			"stage": "response-redaction",
		})
	}
}

func (s *Session) recordUsageBudgetObservation(
	tokens int,
	costUSD float64,
	provider, model string,
) {
	if s == nil || tokens <= 0 {
		return
	}
	tracker := s.ensureUsageTracker()
	tracker.Record(tokens, costUSD, provider, model)
	allowed, reason := tracker.CanProceed()
	usage := tracker.GetUsage()
	limits := tracker.GetLimits()

	sessionID := s.executionGraphSessionID()
	if sessionID == "" {
		return
	}
	repositoryDir := s.configuredWorkingDir()
	if repositoryDir == "" {
		repositoryDir, _ = os.Getwd()
	}
	repositoryID := ""
	if repositoryDir != "" {
		repositoryID = filepath.Base(filepath.Clean(repositoryDir))
	}

	observedAt := time.Now().UTC()
	export, err := token.BuildRuntimeGraph(token.RuntimeGraphInput{
		Usage: &usage,
		Budget: &token.BudgetDecision{
			Allowed:      allowed,
			Reason:       reason,
			HourlyLimit:  limits.HourlyTokens,
			DailyLimit:   limits.DailyTokens,
			SessionLimit: limits.SessionTokens,
			CostLimitUSD: limits.CostUSD,
		},
		Source:          provider + "\x00" + model,
		ObservedAt:      observedAt,
		Scope:           graphcontracts.Scope{RepositoryID: repositoryID},
		CorrelationID:   sessionID,
		ProducerVersion: "",
	})
	if err == nil {
		err = graphjournal.AppendRuntimeGraph(
			sessionID, "", "usage-budget", "token",
			export.Nodes, export.Edges, export.Events, observedAt,
		)
	}
	if err != nil {
		s.Logger().Warn("graph observation append failed", map[string]interface{}{
			"kind":  graphjournal.KindRuntime,
			"stage": "usage-budget",
		})
	}
}

func (s *Session) ensureUsageTracker() *token.UsageTracker {
	if s == nil || s.LifecycleSvc() == nil {
		return nil
	}
	return s.LifecycleSvc().EnsureUsageTracker()
}

func (s *Session) currentUsageTracker() *token.UsageTracker {
	if s == nil || s.LifecycleSvc() == nil {
		return nil
	}
	return s.LifecycleSvc().UsageTracker()
}

func (s *Session) usageCanProceed() (bool, string) {
	tracker := s.currentUsageTracker()
	if tracker == nil {
		return true, ""
	}
	return tracker.CanProceed()
}

func (s *Session) recordFluxOperationObservation(
	provider, model, finishReason, content string,
	toolCallCount int,
	usage *types.FluxUsage,
) {
	sessionID := s.executionGraphSessionID()
	if sessionID == "" || usage == nil {
		return
	}
	repositoryDir := s.configuredWorkingDir()
	if repositoryDir == "" {
		repositoryDir, _ = os.Getwd()
	}
	repositoryID := ""
	if repositoryDir != "" {
		repositoryID = filepath.Base(filepath.Clean(repositoryDir))
	}
	observedAt := time.Now().UTC()
	route := types.ResolvedRoute{Provider: provider, Model: model}
	export, err := fluxengine.BuildOperationsGraph(fluxengine.OperationsGraphInput{
		Route:         &route,
		Usage:         usage,
		FinishReason:  finishReason,
		Content:       content,
		ToolCallCount: toolCallCount,
		ObservedAt:    observedAt,
		Scope:         fluxgraph.Scope{RepositoryID: repositoryID},
		CorrelationID: sessionID,
	})
	if err == nil {
		err = graphjournal.AppendRuntimeGraph(
			sessionID, "", "model-generation", "flux",
			toContractNodes(export.Nodes), toContractEdges(export.Edges), toContractEvents(export.Events), observedAt,
		)
	}
	if err != nil {
		s.Logger().Warn("graph observation append failed", map[string]interface{}{
			"kind":  graphjournal.KindRuntime,
			"stage": "model-generation",
		})
	}
}

// The following helpers convert Flux's vendored graph contract types into
// Rho's contracts/graph contract types. The definitions are byte-identical, so
// conversion is a field-by-field copy at the sibling boundary.

func toContractNodes(nodes []fluxgraph.Node) []graphcontracts.Node {
	out := make([]graphcontracts.Node, len(nodes))
	for i, n := range nodes {
		out[i] = toContractNode(n)
	}
	return out
}

func toContractNode(n fluxgraph.Node) graphcontracts.Node {
	return graphcontracts.Node{
		ID:          n.ID,
		Kind:        graphcontracts.NodeKind(n.Kind),
		Scope:       toContractScope(n.Scope),
		CreatedAt:   n.CreatedAt,
		EffectiveAt: n.EffectiveAt,
		Provenance:  toContractProvenance(n.Provenance),
		Attributes:  n.Attributes,
	}
}

func toContractEdges(edges []fluxgraph.Edge) []graphcontracts.Edge {
	out := make([]graphcontracts.Edge, len(edges))
	for i, e := range edges {
		out[i] = toContractEdge(e)
	}
	return out
}

func toContractEdge(e fluxgraph.Edge) graphcontracts.Edge {
	return graphcontracts.Edge{
		ID:          e.ID,
		Kind:        graphcontracts.EdgeKind(e.Kind),
		From:        toContractRef(e.From),
		To:          toContractRef(e.To),
		Scope:       toContractScope(e.Scope),
		CreatedAt:   e.CreatedAt,
		EffectiveAt: e.EffectiveAt,
		Provenance:  toContractProvenance(e.Provenance),
		Attributes:  e.Attributes,
	}
}

func toContractEvents(events []fluxgraph.Event) []graphcontracts.Event {
	out := make([]graphcontracts.Event, len(events))
	for i, ev := range events {
		out[i] = toContractEvent(ev)
	}
	return out
}

func toContractEvent(ev fluxgraph.Event) graphcontracts.Event {
	return graphcontracts.Event{
		ID:             ev.ID,
		Type:           graphcontracts.EventType(ev.Type),
		Subject:        toContractRef(ev.Subject),
		Scope:          toContractScope(ev.Scope),
		OccurredAt:     ev.OccurredAt,
		CorrelationID:  ev.CorrelationID,
		CausationID:    ev.CausationID,
		IdempotencyKey: ev.IdempotencyKey,
		Provenance:     toContractProvenance(ev.Provenance),
	}
}

func toContractRef(r fluxgraph.Ref) graphcontracts.Ref {
	return graphcontracts.Ref{Kind: graphcontracts.NodeKind(r.Kind), ID: r.ID}
}

func toContractScope(s fluxgraph.Scope) graphcontracts.Scope {
	return graphcontracts.Scope{TenantID: s.TenantID, ProjectID: s.ProjectID, RepositoryID: s.RepositoryID}
}

func toContractProvenance(p fluxgraph.Provenance) graphcontracts.Provenance {
	evidence := make([]graphcontracts.ArtifactRef, len(p.Evidence))
	for i, a := range p.Evidence {
		evidence[i] = graphcontracts.ArtifactRef{URI: a.URI, Digest: a.Digest, MediaType: a.MediaType}
	}
	return graphcontracts.Provenance{Producer: p.Producer, Version: p.Version, SourceID: p.SourceID, Evidence: evidence}
}
