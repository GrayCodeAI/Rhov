package token

import (
	"encoding/json"

	rhotoken "github.com/GrayCodeAI/rho/internal/token"
)

// Stats is the compression result consumed by Rho's runtime observations.
// The alias preserves the local token engine schema while keeping token imports
// inside this package.
type Stats = rhotoken.Stats

// UsageTracker and UsageLimits expose the session budget API through Rho's
// token boundary without changing the local engine's accounting behavior.
type (
	UsageTracker       = rhotoken.UsageTracker
	UsageLimits        = rhotoken.UsageLimits
	CodeChunk          = rhotoken.CodeChunk
	ChunkOptions       = rhotoken.ChunkOptions
	SecretMatch        = rhotoken.SecretMatch
	SecretDetector     = rhotoken.SecretDetector
	BudgetDecision     = rhotoken.BudgetDecision
	RedactionSummary   = rhotoken.RedactionSummary
	RuntimeGraphInput  = rhotoken.RuntimeGraphInput
	RuntimeGraphExport = rhotoken.RuntimeGraphExport
)

// NewUsageTracker creates an in-memory usage tracker with the engine's defaults.
func NewUsageTracker() *UsageTracker { return rhotoken.NewUsageTracker() }

// ChunkCode splits source into semantically meaningful token-bounded chunks.
func ChunkCode(source string, opts ChunkOptions) []CodeChunk {
	return rhotoken.ChunkCode(source, opts)
}

// DefaultSecretDetector returns the engine's concurrency-safe built-in detector.
func DefaultSecretDetector() *SecretDetector { return rhotoken.DefaultSecretDetector() }

func BuildRuntimeGraph(input RuntimeGraphInput) (*RuntimeGraphExport, error) {
	return rhotoken.BuildRuntimeGraph(input)
}

// Compress applies context compression with a fixed token budget.
func Compress(text string, budget int) (string, Stats) {
	return rhotoken.Compress(text, budget)
}

// JSONInvariants renders verified-fact summaries for elided JSON records.
func JSONInvariants(dropped []json.RawMessage) string { return rhotoken.JSONInvariants(dropped) }

// ShrinkToolCatalog compresses an OpenAI-style function-tool catalog,
// preserving the selection surface byte-for-byte. Fail-open: unchanged input
// with ok=false when nothing can be safely reduced.
func ShrinkToolCatalog(catalog string) (string, bool) {
	return rhotoken.ShrinkToolCatalog(catalog)
}

// LintToolCatalog reports per-tool reductions without committing.
func LintToolCatalog(catalog string) ([]rhotoken.ToolShrinkStats, bool) {
	return rhotoken.LintToolCatalog(catalog)
}

// LogInvariants renders the level distribution of elided log lines.
func LogInvariants(lines []string) string { return rhotoken.LogInvariants(lines) }
