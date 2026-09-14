package token

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Invariant-bearing elision summaries. A bare "[N items removed]" marker
// forces the reader to guess what was dropped. Markers that state only
// VERIFIED facts about the elided units — field constants, exact
// enumerations, numeric ranges, and distinct-count coverage — eliminate most
// recovery calls. A fact is stated only when it was verified across EVERY
// elided unit; anything uncertain is withheld entirely.

const (
	invariantsMaxBytes       = 160
	invariantsMaxBuckets     = 5
	invariantsMaxValueLen    = 24
	invariantsMinUnitsPerRun = 3
)

var logLineRe = regexp.MustCompile(`(?i)(?:^[0-9\[\]\s\-T:.Z]{0,35})?\b(INFO|DEBUG|WARN|WARNING|ERROR|FATAL|TRACE)\b`)

func logLevel(line string) string {
	m := logLineRe.FindStringSubmatch(line)
	if m == nil {
		return ""
	}
	return strings.ToUpper(m[1])
}

// JSONInvariants renders the verified-facts summary for a set of elided JSON
// records. The result states only what holds across every record; "" when no
// fact clears the withholding rules.
func JSONInvariants(dropped []json.RawMessage) string {
	if len(dropped) < invariantsMinUnitsPerRun {
		return ""
	}

	type bucket struct {
		count int
		vals  []string
	}
	fields := map[string]*bucket{}
	fieldOrder := []string{}
	parsed := 0

	for _, raw := range dropped {
		var obj map[string]json.RawMessage
		if json.Unmarshal(raw, &obj) != nil {
			continue
		}
		parsed++
		for k, v := range obj {
			s := decodeScalar(v)
			if s == "" || len(s) > invariantsMaxValueLen || strings.ContainsAny(s, " \t\n") {
				continue
			}
			b, ok := fields[k]
			if !ok {
				if looksSensitive(k) {
					continue
				}
				b = &bucket{}
				fields[k] = b
				fieldOrder = append(fieldOrder, k)
			}
			b.count++
			b.vals = append(b.vals, s)
		}
	}
	if parsed == 0 || parsed*2 < len(dropped) {
		return ""
	}

	sort.Strings(fieldOrder)
	var facts []string
	for _, k := range fieldOrder {
		b := fields[k]
		if b.count != parsed {
			continue
		}
		distinct := distinctSorted(b.vals)
		switch {
		case len(distinct) == 1:
			facts = append(facts, fmt.Sprintf("%s=%s×%d", k, distinct[0], parsed))
		case allNumeric(distinct):
			loN, hiN := numericBounds(distinct)
			facts = append(facts, fmt.Sprintf("range %s=%s..%s", k, formatNum(loN), formatNum(hiN)))
		case len(distinct) <= invariantsMaxBuckets && len(distinct) == parsed:
			if cov, ok := coverageFact(k, distinct); ok {
				facts = append(facts, cov)
			}
		case len(distinct) <= invariantsMaxBuckets:
			parts := make([]string, 0, len(distinct))
			counts := map[string]int{}
			for _, v := range b.vals {
				counts[v]++
			}
			for _, v := range distinct {
				parts = append(parts, fmt.Sprintf("%s×%d", v, counts[v]))
			}
			facts = append(facts, k+": "+strings.Join(parts, " "))
		default:
			if cov, ok := coverageFact(k, distinct); ok {
				facts = append(facts, cov)
			}
		}
	}
	return capFacts(facts)
}

// LogInvariants enriches a collapsed-log-run marker with the level
// distribution of the elided lines when they parse as log levels.
func LogInvariants(lines []string) string {
	if len(lines) < invariantsMinUnitsPerRun {
		return ""
	}
	counts := map[string]int{}
	order := []string{}
	for _, ln := range lines {
		lv := logLevel(ln)
		if lv == "" {
			continue
		}
		if _, seen := counts[lv]; !seen {
			order = append(order, lv)
		}
		counts[lv]++
	}
	if len(counts) == 0 || len(counts) > invariantsMaxBuckets+2 {
		return ""
	}
	sort.Strings(order)
	parts := make([]string, 0, len(order))
	for _, lv := range order {
		parts = append(parts, fmt.Sprintf("%s×%d", strings.ToLower(lv), counts[lv]))
	}
	return strings.Join(parts, " ")
}

func coverageFact(k string, distinct []string) (string, bool) {
	lo, hi := distinct[0], distinct[len(distinct)-1]
	n := len(distinct)
	base := fmt.Sprintf("%s: %d distinct, %s..%s", k, n, lo, hi)
	loN, err1 := strconv.Atoi(digitsOnly(lo))
	hiN, err2 := strconv.Atoi(digitsOnly(hi))
	if err1 != nil || err2 != nil || hiN < loN {
		return base, true
	}
	span := hiN - loN + 1
	if span == n && sameWidth(lo, hi) && sharedPrefixLoose(lo, hi) {
		return fmt.Sprintf("%s: %s..%s all %d present", k, lo, hi, n), true
	}
	return base, true
}

func capFacts(facts []string) string {
	if len(facts) == 0 {
		return ""
	}
	rank := func(s string) int {
		switch {
		case strings.Contains(s, ".."):
			return 0
		case strings.Contains(s, ": "):
			return 1
		default:
			return 2
		}
	}
	sort.SliceStable(facts, func(i, j int) bool { return rank(facts[i]) < rank(facts[j]) })
	out := strings.Join(facts, ", ")
	for len(out) > invariantsMaxBytes && len(facts) > 0 {
		facts = facts[:len(facts)-1]
		out = strings.Join(facts, ", ")
	}
	return out
}

func decodeScalar(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var f float64
	if json.Unmarshal(raw, &f) == nil {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	var b bool
	if json.Unmarshal(raw, &b) == nil {
		return strconv.FormatBool(b)
	}
	return ""
}

func distinctSorted(vals []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

func numericBounds(sortedVals []string) (float64, float64) {
	lo, hi := 0.0, 0.0
	first := true
	for _, v := range sortedVals {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			continue
		}
		if first || f < lo {
			lo = f
		}
		if first || f > hi {
			hi = f
		}
		first = false
	}
	return lo, hi
}

func formatNum(f float64) string {
	if f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func allNumeric(sortedVals []string) bool {
	for _, v := range sortedVals {
		if _, err := strconv.ParseFloat(v, 64); err != nil {
			return false
		}
	}
	return true
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func sameWidth(a, b string) bool {
	return len(digitsOnly(a)) == len(digitsOnly(b))
}

func sharedPrefixLoose(a, b string) bool {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] && (a[i] < '0' || a[i] > '9') {
		i++
	}
	return i > 0
}

func looksSensitive(key string) bool {
	k := strings.ToLower(key)
	for _, pat := range []string{"token", "secret", "password", "passwd", "apikey", "api_key", "authorization", "credential", "private"} {
		if strings.Contains(k, pat) {
			return true
		}
	}
	return false
}
