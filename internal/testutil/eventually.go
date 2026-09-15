package testutil

import (
	"testing"
	"time"
)

// Eventually polls cond until it returns true or timeout elapses, then fails
// the test. It replaces fixed time.Sleep calls in asynchronous tests, which
// are the main source of CI flakiness.
func Eventually(t testing.TB, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !cond() {
		t.Fatalf("condition not met within %s", timeout)
	}
}

// Never asserts that cond stays false for the duration, then fails if it ever
// becomes true. Useful for asserting that a state does not regress.
func Never(t testing.TB, duration time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(duration)
	for time.Now().Before(deadline) {
		if cond() {
			t.Fatalf("condition became true within %s", duration)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
