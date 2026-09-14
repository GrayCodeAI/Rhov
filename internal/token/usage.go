package token

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// UsageEntry records a single usage event.
type UsageEntry struct {
	Tokens    int
	CostUSD   float64
	Timestamp time.Time
	Provider  string
	Model     string
}

// Alert represents a usage threshold alert.
type Alert struct {
	Level     string // "warning", "critical", "limit_reached"
	Message   string
	Timestamp time.Time
	Threshold float64 // what % triggered it
}

// UsageSummary provides a snapshot of current usage across all windows.
type UsageSummary struct {
	HourlyTokens     int
	HourlyRemaining  int
	DailyTokens      int
	DailyRemaining   int
	SessionTokens    int
	SessionRemaining int
	DailyCostUSD     float64
	CostRemaining    float64
	HourlyPct        float64
	DailyPct         float64
}

// UsageLimits configures the ceilings enforced by a UsageTracker.
type UsageLimits struct {
	DailyTokens   int
	HourlyTokens  int
	SessionTokens int
	CostUSD       float64
}

// UsageTracker tracks API usage across sessions and prevents surprise bills.
type UsageTracker struct {
	DailyLimit   int
	HourlyLimit  int
	SessionLimit int
	CostLimitUSD float64

	hourlyUsage  []UsageEntry
	dailyUsage   []UsageEntry
	sessionUsage int
	mu           sync.Mutex
	Alerts       []Alert

	firedThresholds map[string]bool
}

// NewUsageTracker creates an in-memory usage tracker with sensible defaults.
func NewUsageTracker() *UsageTracker {
	return &UsageTracker{firedThresholds: map[string]bool{}}
}

// SetLimits applies usage ceilings.
func (u *UsageTracker) SetLimits(limits UsageLimits) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.HourlyLimit = limits.HourlyTokens
	u.DailyLimit = limits.DailyTokens
	u.SessionLimit = limits.SessionTokens
	u.CostLimitUSD = limits.CostUSD
}

// GetLimits returns the current ceilings.
func (u *UsageTracker) GetLimits() UsageLimits {
	u.mu.Lock()
	defer u.mu.Unlock()
	return UsageLimits{
		DailyTokens:   u.DailyLimit,
		HourlyTokens:  u.HourlyLimit,
		SessionTokens: u.SessionLimit,
		CostUSD:       u.CostLimitUSD,
	}
}

// Record logs a usage event and emits threshold alerts.
func (u *UsageTracker) Record(tokens int, costUSD float64, provider, model string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	now := time.Now()
	entry := UsageEntry{Tokens: tokens, CostUSD: costUSD, Timestamp: now, Provider: provider, Model: model}
	u.hourlyUsage = append(u.hourlyUsage, entry)
	u.dailyUsage = append(u.dailyUsage, entry)
	u.sessionUsage += tokens
	u.pruneOldLocked(now)
	u.checkThresholdsLocked()
}

// CanProceed reports whether usage is within all configured limits.
func (u *UsageTracker) CanProceed() (bool, string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	now := time.Now()
	u.pruneOldLocked(now)
	if u.HourlyLimit > 0 && u.hourlyTokensLocked(now) >= u.HourlyLimit {
		return false, "hourly token limit reached"
	}
	if u.DailyLimit > 0 && u.dailyTokensLocked(now) >= u.DailyLimit {
		return false, "daily token limit reached"
	}
	if u.SessionLimit > 0 && u.sessionUsage >= u.SessionLimit {
		return false, "session token limit reached"
	}
	if u.CostLimitUSD > 0 && u.dailyCostLocked(now) >= u.CostLimitUSD {
		return false, "daily cost limit reached"
	}
	return true, ""
}

// GetUsage returns a snapshot of current usage.
func (u *UsageTracker) GetUsage() UsageSummary {
	u.mu.Lock()
	defer u.mu.Unlock()
	now := time.Now()
	u.pruneOldLocked(now)
	hourly := u.hourlyTokensLocked(now)
	daily := u.dailyTokensLocked(now)
	cost := u.dailyCostLocked(now)
	s := UsageSummary{
		HourlyTokens:  hourly,
		DailyTokens:   daily,
		SessionTokens: u.sessionUsage,
		DailyCostUSD:  cost,
	}
	if u.HourlyLimit > 0 {
		s.HourlyRemaining = max0(u.HourlyLimit - hourly)
		s.HourlyPct = float64(hourly) / float64(u.HourlyLimit) * 100
	}
	if u.DailyLimit > 0 {
		s.DailyRemaining = max0(u.DailyLimit - daily)
		s.DailyPct = float64(daily) / float64(u.DailyLimit) * 100
	}
	if u.SessionLimit > 0 {
		s.SessionRemaining = max0(u.SessionLimit - u.sessionUsage)
	}
	if u.CostLimitUSD > 0 {
		s.CostRemaining = u.CostLimitUSD - cost
	}
	return s
}

// CheckThresholds evaluates and records threshold alerts.
func (u *UsageTracker) CheckThresholds() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.checkThresholdsLocked()
}

// DrainAlerts returns and clears pending alerts.
func (u *UsageTracker) DrainAlerts() []Alert {
	u.mu.Lock()
	defer u.mu.Unlock()
	out := u.Alerts
	u.Alerts = nil
	return out
}

func (u *UsageTracker) checkThresholdsLocked() {
	now := time.Now()
	u.emitAlert("hourly", u.pct(u.hourlyTokensLocked(now), u.HourlyLimit), "hourly")
	u.emitAlert("daily", u.pct(u.dailyTokensLocked(now), u.DailyLimit), "daily")
	u.emitAlert("session", u.pct(u.sessionUsage, u.SessionLimit), "session")
	u.emitAlert("cost", u.pctFloat(u.dailyCostLocked(now), u.CostLimitUSD), "cost")
}

func (u *UsageTracker) pct(used, limit int) float64 {
	if limit <= 0 {
		return 0
	}
	return float64(used) / float64(limit) * 100
}

func (u *UsageTracker) pctFloat(used, limit float64) float64 {
	if limit <= 0 {
		return 0
	}
	return used / limit * 100
}

func (u *UsageTracker) emitAlert(category string, pct float64, label string) {
	if pct <= 0 {
		return
	}
	level := ""
	switch {
	case pct >= 100:
		level = "limit_reached"
	case pct >= 90:
		level = "critical"
	case pct >= 75:
		level = "warning"
	default:
		return
	}
	key := fmt.Sprintf("%s_%d", category, int(pct/10)*10)
	if u.firedThresholds[key] {
		return
	}
	u.firedThresholds[key] = true
	u.Alerts = append(u.Alerts, Alert{
		Level:     level,
		Message:   fmt.Sprintf("%s usage at %.0f%% of limit", label, pct),
		Timestamp: time.Now(),
		Threshold: pct,
	})
}

// Reset clears all recorded usage.
func (u *UsageTracker) Reset() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.hourlyUsage = nil
	u.dailyUsage = nil
	u.sessionUsage = 0
	u.Alerts = nil
	u.firedThresholds = map[string]bool{}
}

// PruneOld drops usage entries outside the rolling windows.
func (u *UsageTracker) PruneOld() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.pruneOldLocked(time.Now())
}

func (u *UsageTracker) pruneOldLocked(now time.Time) {
	hourAgo := now.Add(-time.Hour)
	dayAgo := now.Add(-24 * time.Hour)
	u.hourlyUsage = pruneEntries(u.hourlyUsage, hourAgo)
	u.dailyUsage = pruneEntries(u.dailyUsage, dayAgo)
}

func pruneEntries(entries []UsageEntry, cutoff time.Time) []UsageEntry {
	out := entries[:0]
	for _, e := range entries {
		if e.Timestamp.After(cutoff) {
			out = append(out, e)
		}
	}
	return out
}

func (u *UsageTracker) hourlyTokensLocked(now time.Time) int {
	total := 0
	for _, e := range u.hourlyUsage {
		total += e.Tokens
	}
	return total
}

func (u *UsageTracker) dailyTokensLocked(now time.Time) int {
	total := 0
	for _, e := range u.dailyUsage {
		total += e.Tokens
	}
	return total
}

func (u *UsageTracker) dailyCostLocked(now time.Time) float64 {
	total := 0.0
	for _, e := range u.dailyUsage {
		total += e.CostUSD
	}
	return total
}

// FormatUsageBar renders a simple percentage bar.
func FormatUsageBar(pct float64, width int) string {
	if width <= 0 {
		width = 20
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := int(float64(width) * pct / 100)
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + "]"
}

// FormatSummary renders a human-readable usage summary.
func (u *UsageTracker) FormatSummary() string {
	s := u.GetUsage()
	var b strings.Builder
	fmt.Fprintf(&b, "hourly: %d tokens", s.HourlyTokens)
	if u.HourlyLimit > 0 {
		fmt.Fprintf(&b, " (%.0f%%)", s.HourlyPct)
	}
	fmt.Fprintf(&b, " · daily: %d tokens", s.DailyTokens)
	if u.DailyLimit > 0 {
		fmt.Fprintf(&b, " (%.0f%%)", s.DailyPct)
	}
	fmt.Fprintf(&b, " · session: %d tokens", s.SessionTokens)
	return b.String()
}
