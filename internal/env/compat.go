package env

import "os"

// legacyPrefix is the pre-rename environment-variable prefix. Rho was formerly
// named Hawk; reads fall back to HAWK_* so existing shell profiles, CI configs,
// and service units keep working without edits.
const (
	prefix       = "RHO_"
	legacyPrefix = "HAWK_"
)

// GetenvRho reads a Rho environment variable by its suffix (the part after
// "RHO_"), falling back to the legacy HAWK_<suffix> name. The RHO_ form wins
// when both are set, so users can migrate incrementally.
//
// Example: GetenvRho("STATE_DIR") reads RHO_STATE_DIR, then HAWK_STATE_DIR.
func GetenvRho(suffix string) string {
	if v := os.Getenv(prefix + suffix); v != "" {
		return v
	}
	return os.Getenv(legacyPrefix + suffix)
}

// LookupRho is GetenvRho with an explicit presence flag. A variable set to an
// empty value counts as absent, matching GetenvRho's fallback behavior.
func LookupRho(suffix string) (string, bool) {
	if v := os.Getenv(prefix + suffix); v != "" {
		return v, true
	}
	if v := os.Getenv(legacyPrefix + suffix); v != "" {
		return v, true
	}
	return "", false
}
