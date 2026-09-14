package env

import "testing"

func TestGetenvRhoPrefersCurrentName(t *testing.T) {
	t.Setenv("RHO_STATE_DIR", "/current")
	t.Setenv("HAWK_STATE_DIR", "/legacy")
	if got := GetenvRho("STATE_DIR"); got != "/current" {
		t.Fatalf("GetenvRho = %q, want /current", got)
	}
}

func TestGetenvRhoFallsBackToLegacy(t *testing.T) {
	t.Setenv("RHO_STATE_DIR", "")
	t.Setenv("HAWK_STATE_DIR", "/legacy")
	if got := GetenvRho("STATE_DIR"); got != "/legacy" {
		t.Fatalf("GetenvRho = %q, want /legacy", got)
	}
}

func TestGetenvRhoEmptyWhenUnset(t *testing.T) {
	t.Setenv("RHO_STATE_DIR", "")
	t.Setenv("HAWK_STATE_DIR", "")
	if got := GetenvRho("STATE_DIR"); got != "" {
		t.Fatalf("GetenvRho = %q, want empty", got)
	}
}

func TestLookupRhoReportsPresence(t *testing.T) {
	t.Setenv("RHO_FEATURE", "")
	t.Setenv("HAWK_FEATURE", "1")
	v, ok := LookupRho("FEATURE")
	if !ok || v != "1" {
		t.Fatalf("LookupRho = (%q, %v), want (1, true)", v, ok)
	}

	t.Setenv("HAWK_FEATURE", "")
	if _, ok := LookupRho("FEATURE"); ok {
		t.Fatal("LookupRho reported presence for an unset variable")
	}
}
