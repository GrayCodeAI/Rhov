package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestUsageEventNormalizedClampsToContract(t *testing.T) {
	now := time.Date(2026, time.September, 27, 12, 0, 0, 0, time.UTC)
	got := UsageEvent{
		DurationMS:          MaxUsageDurationMS + 1,
		InputTokens:         maxUsageTokens * 2,
		OutputTokens:        -5,
		TokensUsed:          maxUsageTokens + 1,
		EstimatedCostMicros: -1,
		Model:               strings.Repeat("m", 150),
		Status:              "timeout",
	}.normalized(now)
	if got.Capability != CapabilityRho {
		t.Fatalf("capability = %q, want %q", got.Capability, CapabilityRho)
	}
	if got.DurationMS != MaxUsageDurationMS {
		t.Fatalf("durationMs = %d, want %d", got.DurationMS, MaxUsageDurationMS)
	}
	if got.InputTokens != maxUsageTokens || got.OutputTokens != 0 || got.TokensUsed != maxUsageTokens {
		t.Fatalf("token counters = %+v", got)
	}
	if got.EstimatedCostMicros != 0 || len(got.Model) != maxUsageText || got.Status != "" {
		t.Fatalf("normalized = %+v", got)
	}
	if got.OccurredAt != "2026-09-27T12:00:00Z" {
		t.Fatalf("occurredAt = %q", got.OccurredAt)
	}

	kept := UsageEvent{Capability: "rho", DurationMS: 1500, Status: "failed", OccurredAt: "2026-01-01T00:00:00Z"}.normalized(now)
	if kept.DurationMS != 1500 || kept.Status != "failed" || kept.OccurredAt != "2026-01-01T00:00:00Z" {
		t.Fatalf("in-range event changed: %+v", kept)
	}
}

func TestRecordUsageSendsClampedDurationAndReportsRejection(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"Invalid usage event"}`))
	}))
	defer server.Close()
	err := New(Config{Endpoint: server.URL, DeviceToken: "hwc_test"}).RecordUsage(context.Background(), UsageEvent{
		EventID: "exec-1727400000000-0123456789abcdef", DeviceID: "device_0123456789", ProjectID: "project_0123456789",
		DurationMS: 25 * 60 * 60 * 1000, TokensUsed: 1,
	})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Message != "Invalid usage event" {
		t.Fatalf("error = %v, want the Worker's rejection", err)
	}
	if body["durationMs"] != float64(MaxUsageDurationMS) || body["capability"] != CapabilityRho {
		t.Fatalf("body = %v", body)
	}
}
