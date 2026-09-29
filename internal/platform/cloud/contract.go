package cloud

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf16"
)

// opaqueIDPattern and the 16..128 length bound mirror the Worker's opaqueID
// schema used for project, device, event, session and sync identifiers.
var opaqueIDPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]+$`)

const (
	minOpaqueID = 16
	maxOpaqueID = 128
)

// ValidOpaqueID reports whether id satisfies GrayCode Cloud's opaque
// identifier schema: 16 to 128 letters, digits, '.', '_', ':' or '-'.
func ValidOpaqueID(id string) bool {
	return len(id) >= minOpaqueID && len(id) <= maxOpaqueID && opaqueIDPattern.MatchString(id)
}

// utf16Len counts s in UTF-16 code units, the unit zod's string length
// checks use.
func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if width := utf16.RuneLen(r); width > 0 {
			n += width
		} else {
			n++
		}
	}
	return n
}

// requireText checks a mandatory, already-trimmed text field of at most max
// UTF-16 code units.
func requireText(field, value string, maxUnits int) error {
	if value == "" {
		return fmt.Errorf("%s is required", field)
	}
	return optionalText(field, value, maxUnits)
}

// optionalText checks an optional, already-trimmed text field.
func optionalText(field, value string, maxUnits int) error {
	if n := utf16Len(value); n > maxUnits {
		return fmt.Errorf("%s is %d characters; GrayCode Cloud accepts at most %d", field, n, maxUnits)
	}
	return nil
}

// requireOneOf checks an enum field.
func requireOneOf(field, value string, allowed []string) error {
	if !slices.Contains(allowed, value) {
		return fmt.Errorf("%s %q is not one of %s", field, value, strings.Join(allowed, ", "))
	}
	return nil
}
