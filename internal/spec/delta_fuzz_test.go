package spec

import "testing"

// FuzzParseDeltaSpec ensures the delta parser and validator never panic on
// arbitrary input. The delta spec is model/user-authored, so malformed input
// must be handled gracefully.
func FuzzParseDeltaSpec(f *testing.F) {
	f.Add(sampleDelta)
	f.Add("")
	f.Add("## ADDED Requirements\n")
	f.Add("## REMOVED Requirements\n### Requirement: x\n**Reason**: y")
	f.Add("## RENAMED Requirements\n### Requirement: x\n- FROM: a\n- TO: b")
	// Regression: a RENAMED requirement name with invalid UTF-8 used to panic
	// in applyRename's regexp.MustCompile.
	f.Add("## RENAMED Requirements\n### Requirement: x\n- FROM: \xff0\n- TO: y")

	f.Fuzz(func(t *testing.T, content string) {
		ds, err := ParseDeltaSpec(content)
		if err != nil {
			return
		}
		_ = ValidateDeltaSpec(ds)
		_, _ = ApplyDelta("# Requirements\n\n### Requirement: x\nSHALL be.\n", ds)
	})
}
