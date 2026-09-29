package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"
)

// CapabilityRho is the canonical usage capability rho reports. GrayCode Cloud
// also accepts "graycode", but only as a legacy alias for pre-rename clients.
const CapabilityRho = "rho"

// Bounds of the POST /v1/usage schema.
const (
	// MaxUsageDurationMS is the durationMs ceiling (24 hours). Longer runs are
	// clamped so they are still recorded instead of rejected.
	MaxUsageDurationMS       = 86_400_000
	maxUsageTokens           = 10_000_000
	maxUsageCostMicros int64 = 10_000_000_000
	maxUsageText             = 100
)

var usageStatuses = []string{"completed", "failed", "cancelled"}

// RecordUsage uploads one usage event after clamping it into the /v1/usage
// schema. It serves the automatic, fail-open path (`rho exec`): callers must
// never let the returned error change local execution. The error exists so a
// caller can surface a server rejection (*APIError) rather than lose it.
func (c *Client) RecordUsage(ctx context.Context, event UsageEvent) error {
	if !c.Enabled() {
		return ErrNotConnected
	}
	body, err := json.Marshal(event.normalized(time.Now()))
	if err != nil {
		return fmt.Errorf("marshal usage event: %w", err)
	}
	req, err := c.newJSONRequest(ctx, "/v1/usage", body, true)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("upload usage to GrayCode Cloud: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return readAPIError("usage upload", resp)
	}
	return nil
}

// normalized clamps e into the /v1/usage schema: the capability defaults to
// rho, counters are clamped to their bounds, free text is cut to 100 UTF-16
// code units, an unknown status is dropped, and a missing timestamp is set.
func (e UsageEvent) normalized(now time.Time) UsageEvent {
	if e.Capability == "" {
		e.Capability = CapabilityRho
	}
	e.DurationMS = clamp(e.DurationMS, 0, MaxUsageDurationMS)
	for _, counter := range []*int{
		&e.InputTokens, &e.OutputTokens, &e.CachedInputTokens,
		&e.ReasoningTokens, &e.TokensUsed, &e.TokensSaved,
	} {
		*counter = clamp(*counter, 0, maxUsageTokens)
	}
	e.EstimatedCostMicros = int(min(max(int64(e.EstimatedCostMicros), 0), maxUsageCostMicros))
	e.Provider = boundedField(e.Provider, maxUsageText, "")
	e.Model = boundedField(e.Model, maxUsageText, "")
	e.ErrorCode = boundedField(e.ErrorCode, maxUsageText, "")
	if !slices.Contains(usageStatuses, e.Status) {
		e.Status = ""
	}
	if e.OccurredAt == "" {
		e.OccurredAt = now.UTC().Format(time.RFC3339)
	}
	return e
}

func clamp(value, lo, hi int) int {
	return min(max(value, lo), hi)
}
