package token

import (
	"strings"
	"testing"
	"time"
)

func TestUsageTrackerRecordAndUsage(t *testing.T) {
	ut := NewUsageTracker()
	ut.SetLimits(UsageLimits{HourlyTokens: 1000, DailyTokens: 10_000, SessionTokens: 5000, CostUSD: 10})
	ut.Record(100, 0.5, "provider", "model")
	ut.Record(200, 0.25, "provider", "model")

	s := ut.GetUsage()
	if s.HourlyTokens != 300 {
		t.Errorf("HourlyTokens = %d, want 300", s.HourlyTokens)
	}
	if s.DailyTokens != 300 {
		t.Errorf("DailyTokens = %d, want 300", s.DailyTokens)
	}
	if s.SessionTokens != 300 {
		t.Errorf("SessionTokens = %d, want 300", s.SessionTokens)
	}
	if s.DailyCostUSD != 0.75 {
		t.Errorf("DailyCostUSD = %f, want 0.75", s.DailyCostUSD)
	}
	if s.HourlyRemaining != 700 {
		t.Errorf("HourlyRemaining = %d, want 700", s.HourlyRemaining)
	}
}

func TestUsageTrackerCanProceed(t *testing.T) {
	ut := NewUsageTracker()
	if ok, _ := ut.CanProceed(); !ok {
		t.Error("unlimited tracker should allow")
	}
	ut.SetLimits(UsageLimits{SessionTokens: 100})
	ut.Record(100, 0, "p", "m")
	if ok, reason := ut.CanProceed(); ok || reason == "" {
		t.Errorf("CanProceed at session limit = (%v, %q), want (false, reason)", ok, reason)
	}
}

func TestUsageTrackerThresholdAlerts(t *testing.T) {
	ut := NewUsageTracker()
	ut.SetLimits(UsageLimits{HourlyTokens: 100, DailyTokens: 10_000, SessionTokens: 10_000, CostUSD: 10})
	ut.Record(80, 0, "provider", "model")
	alerts := ut.DrainAlerts()
	if len(alerts) == 0 {
		t.Fatal("expected threshold alert after crossing 75%")
	}
	if len(ut.DrainAlerts()) != 0 {
		t.Fatal("expected DrainAlerts to clear pending alerts")
	}
}

func TestUsageTrackerReset(t *testing.T) {
	ut := NewUsageTracker()
	ut.SetLimits(UsageLimits{SessionTokens: 1000})
	ut.Record(500, 1, "p", "m")
	ut.Reset()
	s := ut.GetUsage()
	if s.SessionTokens != 0 || s.HourlyTokens != 0 || s.DailyCostUSD != 0 {
		t.Errorf("Reset did not clear usage: %+v", s)
	}
}

func TestUsageTrackerPruneOld(t *testing.T) {
	ut := NewUsageTracker()
	ut.mu.Lock()
	ut.hourlyUsage = append(ut.hourlyUsage, UsageEntry{Tokens: 100, Timestamp: time.Now().Add(-2 * time.Hour)})
	ut.dailyUsage = append(ut.dailyUsage, UsageEntry{Tokens: 100, Timestamp: time.Now().Add(-48 * time.Hour)})
	ut.mu.Unlock()
	ut.PruneOld()
	s := ut.GetUsage()
	if s.HourlyTokens != 0 {
		t.Errorf("HourlyTokens after prune = %d, want 0", s.HourlyTokens)
	}
	if s.DailyTokens != 0 {
		t.Errorf("DailyTokens after prune = %d, want 0", s.DailyTokens)
	}
}

func TestUsageTrackerFormatSummary(t *testing.T) {
	ut := NewUsageTracker()
	ut.SetLimits(UsageLimits{HourlyTokens: 1000, DailyTokens: 10_000})
	ut.Record(100, 0, "p", "m")
	out := ut.FormatSummary()
	for _, want := range []string{"hourly:", "daily:", "session:"} {
		if !strings.Contains(out, want) {
			t.Errorf("FormatSummary missing %q: %s", want, out)
		}
	}
}

func TestFormatUsageBar(t *testing.T) {
	if got := FormatUsageBar(0, 10); got != "[░░░░░░░░░░]" {
		t.Errorf("FormatUsageBar(0,10) = %q", got)
	}
	if got := FormatUsageBar(100, 10); got != "[██████████]" {
		t.Errorf("FormatUsageBar(100,10) = %q", got)
	}
	// Out-of-range values clamp.
	if got := FormatUsageBar(-5, 10); !strings.HasPrefix(got, "[░") {
		t.Errorf("FormatUsageBar(-5,10) = %q, want clamped low", got)
	}
	if got := FormatUsageBar(500, 10); got != "[██████████]" {
		t.Errorf("FormatUsageBar(500,10) = %q, want clamped high", got)
	}
}

func TestUsageTrackerGetLimitsRoundTrip(t *testing.T) {
	ut := NewUsageTracker()
	want := UsageLimits{DailyTokens: 1, HourlyTokens: 2, SessionTokens: 3, CostUSD: 4.5}
	ut.SetLimits(want)
	if got := ut.GetLimits(); got != want {
		t.Errorf("GetLimits = %+v, want %+v", got, want)
	}
}
