package token

import (
	"strings"
	"testing"
)

func TestCountTokens(t *testing.T) {
	if got := CountTokens(""); got != 0 {
		t.Errorf("CountTokens(\"\") = %d, want 0", got)
	}
	short := CountTokens("hello world")
	long := CountTokens(strings.Repeat("the quick brown fox jumps over the lazy dog. ", 20))
	if short <= 0 {
		t.Errorf("CountTokens(short) = %d, want > 0", short)
	}
	if long <= short {
		t.Errorf("CountTokens(long) = %d must exceed short = %d", long, short)
	}
}

func TestCountTokensFast(t *testing.T) {
	if got := CountTokensFast(""); got != 0 {
		t.Errorf("CountTokensFast(\"\") = %d, want 0", got)
	}
	if n := CountTokensFast(strings.Repeat("word ", 100)); n <= 0 {
		t.Errorf("CountTokensFast = %d, want > 0", n)
	}
}

func TestFastEstimateTokens(t *testing.T) {
	if got := fastEstimateTokens(""); got != 0 {
		t.Errorf("fastEstimateTokens(\"\") = %d, want 0", got)
	}
	short := fastEstimateTokens("hello world")
	if short <= 0 {
		t.Errorf("fastEstimateTokens(short) = %d, want > 0", short)
	}
	long := fastEstimateTokens(strings.Repeat("the quick brown fox jumps over the lazy dog. ", 20))
	if long <= short {
		t.Errorf("fastEstimateTokens(long) = %d must exceed short = %d", long, short)
	}
	// Very short strings use the (len+2)/3 branch.
	if got := fastEstimateTokens("abcd"); got != 2 {
		t.Errorf("fastEstimateTokens(\"abcd\") = %d, want 2", got)
	}
	// Medium strings use the (len+3)/4 branch.
	if got := fastEstimateTokens(strings.Repeat("x", 40)); got != 10 {
		t.Errorf("fastEstimateTokens(40 x) = %d, want 10", got)
	}
}

func TestEstimateTokensPreciseMonotonic(t *testing.T) {
	prev := 0
	for _, n := range []int{1, 5, 20, 100, 500} {
		got := EstimateTokensPrecise(strings.Repeat("token ", n))
		if got < prev {
			t.Fatalf("EstimateTokensPrecise not monotonic at n=%d: %d < %d", n, got, prev)
		}
		prev = got
	}
}

func TestCalculateTokensSaved(t *testing.T) {
	if got := CalculateTokensSaved("", ""); got != 0 {
		t.Errorf("CalculateTokensSaved(empty) = %d, want 0", got)
	}
	long := strings.Repeat("the quick brown fox ", 50)
	if got := CalculateTokensSaved(long, "short"); got <= 0 {
		t.Errorf("CalculateTokensSaved(long, short) = %d, want > 0", got)
	}
	if got := CalculateTokensSaved("short", long); got != 0 {
		t.Errorf("CalculateTokensSaved(short, long) = %d, want 0", got)
	}
}

func TestShrikeAvailable(t *testing.T) {
	if !ShrikeAvailable() {
		t.Error("ShrikeAvailable() = false, want true (local engine always available)")
	}
}
