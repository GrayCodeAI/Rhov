package token

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	graphcontracts "github.com/GrayCodeAI/rho/internal/contracts/graph"
)

// RuntimeGraphSchemaVersion identifies the runtime-graph projection schema.
const RuntimeGraphSchemaVersion = "shrike.graph/v1"

// BudgetDecision summarizes a usage-budget evaluation.
type BudgetDecision struct {
	Allowed      bool
	Reason       string
	HourlyLimit  int
	DailyLimit   int
	SessionLimit int
	CostLimitUSD float64
}

// RedactionSummary summarizes a secret-redaction pass.
type RedactionSummary struct {
	MatchCount    int
	VerifiedCount int
	Types         map[string]int
}

// RuntimeGraphInput carries the summaries to project into the graph contract.
type RuntimeGraphInput struct {
	Compression     *Stats
	Usage           *UsageSummary
	Budget          *BudgetDecision
	Redaction       *RedactionSummary
	Source          string
	ObservedAt      time.Time
	Scope           graphcontracts.Scope
	CorrelationID   string
	ProducerVersion string
}

// RuntimeGraphExport is the projected graph payload.
type RuntimeGraphExport struct {
	SchemaVersion string                 `json:"schema_version"`
	GeneratedAt   time.Time              `json:"generated_at"`
	Scope         graphcontracts.Scope   `json:"scope,omitempty"`
	Nodes         []graphcontracts.Node  `json:"nodes"`
	Edges         []graphcontracts.Edge  `json:"edges"`
	Events        []graphcontracts.Event `json:"events"`
}

// BuildRuntimeGraph projects compression, usage, budget, and redaction
// summaries into the portable graph contract.
func BuildRuntimeGraph(input RuntimeGraphInput) (*RuntimeGraphExport, error) {
	if input.Compression == nil && input.Usage == nil && input.Budget == nil && input.Redaction == nil {
		return nil, errors.New("runtimegraph: at least one summary is required")
	}
	at := input.ObservedAt.UTC()
	if at.IsZero() {
		at = time.Now().UTC()
	}
	sourceDigest := runtimeDigest(input.Source)
	export := &RuntimeGraphExport{
		SchemaVersion: RuntimeGraphSchemaVersion, GeneratedAt: at, Scope: input.Scope,
		Nodes: []graphcontracts.Node{}, Edges: []graphcontracts.Edge{}, Events: []graphcontracts.Event{},
	}
	var usageRef graphcontracts.Ref
	if input.Compression != nil {
		attrs := map[string]string{
			"entity": "compression", "source_digest": sourceDigest,
			"original_tokens":   strconv.Itoa(input.Compression.OriginalTokens),
			"final_tokens":      strconv.Itoa(input.Compression.FinalTokens),
			"tokens_saved":      strconv.Itoa(input.Compression.TokensSaved),
			"reduction_percent": runtimeFormatFloat(input.Compression.ReductionPercent),
			"cost_savings_usd":  runtimeFormatFloat(input.Compression.CostSavings),
			"model_digest":      runtimeDigest(input.Compression.Model),
			"layer_count":       strconv.Itoa(len(input.Compression.Layers)),
		}
		if _, err := runtimeAddNode(export, graphcontracts.NodeOperations, "compression", attrs, input, at, sourceDigest); err != nil {
			return nil, err
		}
	}
	if input.Usage != nil {
		attrs := map[string]string{
			"entity": "usage", "hourly_tokens": strconv.Itoa(input.Usage.HourlyTokens),
			"hourly_remaining":   strconv.Itoa(input.Usage.HourlyRemaining),
			"daily_tokens":       strconv.Itoa(input.Usage.DailyTokens),
			"daily_remaining":    strconv.Itoa(input.Usage.DailyRemaining),
			"session_tokens":     strconv.Itoa(input.Usage.SessionTokens),
			"session_remaining":  strconv.Itoa(input.Usage.SessionRemaining),
			"daily_cost_usd":     runtimeFormatFloat(input.Usage.DailyCostUSD),
			"cost_remaining_usd": runtimeFormatFloat(input.Usage.CostRemaining),
			"hourly_percent":     runtimeFormatFloat(input.Usage.HourlyPct),
			"daily_percent":      runtimeFormatFloat(input.Usage.DailyPct),
		}
		ref, err := runtimeAddNode(export, graphcontracts.NodeOperations, "usage", attrs, input, at, sourceDigest)
		if err != nil {
			return nil, err
		}
		usageRef = ref
	}
	if input.Budget != nil {
		attrs := map[string]string{
			"entity": "budget_decision", "allowed": strconv.FormatBool(input.Budget.Allowed),
			"reason_digest":  runtimeDigest(input.Budget.Reason),
			"hourly_limit":   strconv.Itoa(input.Budget.HourlyLimit),
			"daily_limit":    strconv.Itoa(input.Budget.DailyLimit),
			"session_limit":  strconv.Itoa(input.Budget.SessionLimit),
			"cost_limit_usd": runtimeFormatFloat(input.Budget.CostLimitUSD),
		}
		ref, err := runtimeAddNode(export, graphcontracts.NodePolicy, "budget", attrs, input, at, sourceDigest)
		if err != nil {
			return nil, err
		}
		if usageRef.ID != "" {
			if err := runtimeAddEdge(export, ref, usageRef, graphcontracts.EdgeDependsOn, input, at); err != nil {
				return nil, err
			}
		}
	}
	if input.Redaction != nil {
		typeNames := make([]string, 0, len(input.Redaction.Types))
		for name := range input.Redaction.Types {
			typeNames = append(typeNames, name)
		}
		sort.Strings(typeNames)
		attrs := map[string]string{
			"entity": "redaction", "source_digest": sourceDigest,
			"match_count":    strconv.Itoa(runtimeMax(input.Redaction.MatchCount, 0)),
			"verified_count": strconv.Itoa(runtimeMax(input.Redaction.VerifiedCount, 0)),
			"type_count":     strconv.Itoa(len(typeNames)),
			"types_digest":   runtimeDigest(strings.Join(typeNames, "\x00")),
		}
		if _, err := runtimeAddNode(export, graphcontracts.NodeQuality, "redaction", attrs, input, at, sourceDigest); err != nil {
			return nil, err
		}
	}
	sort.Slice(export.Nodes, func(i, j int) bool { return export.Nodes[i].ID < export.Nodes[j].ID })
	sort.Slice(export.Edges, func(i, j int) bool { return export.Edges[i].ID < export.Edges[j].ID })
	sort.Slice(export.Events, func(i, j int) bool { return export.Events[i].ID < export.Events[j].ID })
	return export, nil
}

func runtimeAddNode(export *RuntimeGraphExport, kind graphcontracts.NodeKind, entity string, attrs map[string]string, input RuntimeGraphInput, at time.Time, sourceDigest string) (graphcontracts.Ref, error) {
	id := "shrike/" + entity + "/" + runtimeDigest(sourceDigest, at.Format(time.RFC3339Nano))
	ref := graphcontracts.Ref{Kind: kind, ID: id}
	provenance := graphcontracts.Provenance{
		Producer: "shrike", Version: strings.TrimSpace(input.ProducerVersion), SourceID: sourceDigest,
		Evidence: []graphcontracts.ArtifactRef{{URI: "shrike://" + entity + "/" + runtimeDigest(sourceDigest)}},
	}
	node := graphcontracts.Node{ID: id, Kind: kind, Scope: input.Scope, CreatedAt: at, Provenance: provenance, Attributes: attrs}
	if err := node.Validate(); err != nil {
		return graphcontracts.Ref{}, fmt.Errorf("runtimegraph: %s node: %w", entity, err)
	}
	event := graphcontracts.Event{
		ID: "shrike/observed/" + runtimeDigest(id, at.Format(time.RFC3339Nano)), Type: graphcontracts.EventObserved,
		Subject: ref, Scope: input.Scope, OccurredAt: at, CorrelationID: strings.TrimSpace(input.CorrelationID),
		IdempotencyKey: runtimeDigest(id, at.Format(time.RFC3339Nano)), Provenance: provenance,
	}
	export.Nodes = append(export.Nodes, node)
	export.Events = append(export.Events, event)
	return ref, nil
}

func runtimeAddEdge(export *RuntimeGraphExport, from, to graphcontracts.Ref, kind graphcontracts.EdgeKind, input RuntimeGraphInput, at time.Time) error {
	edge := graphcontracts.Edge{
		ID: "shrike/edge/" + runtimeDigest(from.ID, to.ID, string(kind)), Kind: kind, From: from, To: to,
		Scope: input.Scope, CreatedAt: at,
		Provenance: graphcontracts.Provenance{Producer: "shrike", Version: strings.TrimSpace(input.ProducerVersion), SourceID: runtimeDigest(input.Source)},
	}
	if err := edge.Validate(); err != nil {
		return fmt.Errorf("runtimegraph: edge: %w", err)
	}
	export.Edges = append(export.Edges, edge)
	return nil
}

func runtimeFormatFloat(value float64) string { return strconv.FormatFloat(value, 'f', -1, 64) }

func runtimeMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func runtimeDigest(parts ...string) string {
	hash := sha256.New()
	for _, part := range parts {
		_, _ = hash.Write([]byte(strconv.Itoa(len(part))))
		_, _ = hash.Write([]byte{':'})
		_, _ = hash.Write([]byte(part))
	}
	return hex.EncodeToString(hash.Sum(nil))
}
