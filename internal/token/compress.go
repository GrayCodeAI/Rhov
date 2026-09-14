package token

import (
	"strings"
)

// Compress applies a deterministic, budget-bounded compression pipeline to
// text. It removes redundant blank lines, collapses runs of repeated and
// near-duplicate lines, and finally enforces the token budget by eliding the
// middle of the text while preserving the head (goal/context) and the tail
// (most recent turns). The returned Stats report original/final token counts
// and the per-layer savings.
func Compress(text string, budget int) (string, Stats) {
	if text == "" {
		return "", Stats{}
	}
	original := EstimateTokens(text)
	stats := Stats{
		OriginalTokens: original,
		Layers:         map[string]LayerStat{},
	}

	// Layer 1: normalize whitespace (collapse 3+ blank lines, trim trailing
	// spaces). Structural, lossless for content.
	deduped := normalizeWhitespace(text)
	stats.Layers["whitespace"] = LayerStat{TokensSaved: max0(original - EstimateTokens(deduped))}

	// Layer 2: collapse runs of identical consecutive lines.
	collapsed := collapseRepeatedLines(deduped)
	stats.Layers["dedupe"] = LayerStat{TokensSaved: max0(EstimateTokens(deduped) - EstimateTokens(collapsed))}

	// Layer 3: collapse repeating line cycles (e.g. alternating
	// user/assistant boilerplate) into one instance plus a marker.
	cycled := collapseRepeatingCycles(collapsed)
	stats.Layers["cycle"] = LayerStat{TokensSaved: max0(EstimateTokens(collapsed) - EstimateTokens(cycled))}

	// Layer 4: collapse runs of near-duplicate lines (same shape, differing
	// only in trailing detail). This is what catches repeated conversational
	// boilerplate that is not byte-identical.
	compacted := collapseNearDuplicates(cycled)
	stats.Layers["near_dupe"] = LayerStat{TokensSaved: max0(EstimateTokens(cycled) - EstimateTokens(compacted))}

	out := compacted
	if budget > 0 {
		out = enforceBudget(compacted, budget)
		stats.Layers["budget"] = LayerStat{TokensSaved: max0(EstimateTokens(compacted) - EstimateTokens(out))}
	}

	stats.FinalTokens = EstimateTokens(out)
	stats.TokensSaved = stats.OriginalTokens - stats.FinalTokens
	if stats.OriginalTokens > 0 {
		stats.ReductionPercent = float64(stats.TokensSaved) / float64(stats.OriginalTokens) * 100
	}
	return out, stats
}

// CompressForContext compresses text to fit within a token budget, returning
// the compressed text and the final token count.
func CompressForContext(text string, budget int) (string, int) {
	compressed, stats := Compress(text, budget)
	return compressed, stats.FinalTokens
}

func normalizeWhitespace(text string) string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	blank := 0
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(trimmed) == "" {
			blank++
			if blank > 2 {
				continue
			}
		} else {
			blank = 0
		}
		out = append(out, trimmed)
	}
	return strings.Join(out, "\n")
}

func collapseRepeatedLines(text string) string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	i := 0
	for i < len(lines) {
		j := i + 1
		for j < len(lines) && lines[j] == lines[i] {
			j++
		}
		run := j - i
		if run >= 3 && strings.TrimSpace(lines[i]) != "" {
			out = append(out, lines[i])
			out = append(out, "[repeated line x"+itoa(run-2)+"]")
			out = append(out, lines[i])
		} else {
			out = append(out, lines[i:j]...)
		}
		i = j
	}
	return strings.Join(out, "\n")
}

// collapseRepeatingCycles detects a repeating block of lines (period 1-8) that
// occurs at least three times consecutively and replaces the repeats after the
// first with a single marker. This catches alternating boilerplate such as a
// repeated user/assistant exchange that byte-level dedupe misses.
func collapseRepeatingCycles(text string) string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	i := 0
	for i < len(lines) {
		period, repeats := detectCycle(lines, i)
		if period == 0 {
			out = append(out, lines[i])
			i++
			continue
		}
		// Emit the first cycle verbatim, then one marker for the rest.
		out = append(out, lines[i:i+period]...)
		out = append(out, "[... "+itoa(repeats-1)+" repeated blocks of "+itoa(period)+" lines]")
		i += period * repeats
	}
	return strings.Join(out, "\n")
}

// detectCycle returns the period and repeat count of the block starting at
// index start, or (0, 0) when no cycle of period 1-8 repeats at least 3 times.
func detectCycle(lines []string, start int) (period, repeats int) {
	maxPeriod := 8
	for p := 1; p <= maxPeriod; p++ {
		if start+2*p > len(lines) {
			break
		}
		// Require the first two blocks to match, then extend.
		if !blocksEqual(lines, start, start+p, p) {
			continue
		}
		n := 2
		for start+(n+1)*p <= len(lines) && blocksEqual(lines, start, start+n*p, p) {
			n++
		}
		if n >= 3 {
			return p, n
		}
	}
	return 0, 0
}

// blocksEqual reports whether the p-line block at a equals the block at b.
// Empty blocks are never considered equal so blank-line runs are not collapsed
// as cycles (whitespace normalization already handles those).
func blocksEqual(lines []string, a, b, p int) bool {
	for k := 0; k < p; k++ {
		if lines[a+k] != lines[b+k] {
			return false
		}
	}
	return strings.TrimSpace(lines[a]) != ""
}

// collapseNearDuplicates collapses runs of lines that share a long common
// prefix (same speaker/indentation/shape) into the first line plus a marker.
// It is deliberately conservative: only runs of 3+ lines whose normalized
// shape is identical are collapsed, so distinct content is never dropped.
func collapseNearDuplicates(text string) string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	i := 0
	for i < len(lines) {
		shape := lineShape(lines[i])
		j := i + 1
		if shape != "" {
			for j < len(lines) && lineShape(lines[j]) == shape {
				j++
			}
		}
		run := j - i
		if shape != "" && run >= 3 {
			out = append(out, lines[i])
			out = append(out, "[... "+itoa(run-1)+" similar lines]")
			// Keep the last line of the run: it is usually the most specific.
			out = append(out, lines[j-1])
		} else {
			out = append(out, lines[i:j]...)
		}
		i = j
	}
	return strings.Join(out, "\n")
}

// lineShape reduces a line to a coarse shape used for near-duplicate
// detection: the leading token (speaker/indent) plus a fixed-length prefix of
// the remaining content. Lines shorter than the prefix keep their full text,
// so short distinct lines never collide.
func lineShape(line string) string {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) < 24 {
		return ""
	}
	const prefix = 24
	if len(trimmed) > prefix {
		trimmed = trimmed[:prefix]
	}
	return trimmed
}

func enforceBudget(text string, budget int) string {
	if budget <= 0 || EstimateTokens(text) <= budget {
		return text
	}
	lines := strings.Split(text, "\n")
	if len(lines) <= 1 {
		// Single line: hard-truncate at a rune boundary that fits the budget.
		return truncateToBudget(text, budget)
	}

	// Preserve the head and tail, eliding the middle. The head carries the
	// goal/context and the tail carries the most recent turns, so cutting the
	// middle loses the least.
	marker := "[... middle elided to fit context budget ...]"
	markerTokens := EstimateTokens(marker)
	remaining := budget - markerTokens
	if remaining < 1 {
		return truncateToBudget(text, budget)
	}

	headBudget := remaining / 2
	tailBudget := remaining - headBudget

	head := lines[:linePrefixWithin(lines, headBudget)]
	tail := lines[len(lines)-lineSuffixWithin(lines, tailBudget):]
	if len(head) >= len(lines)-len(tail) {
		// Head and tail overlap: the whole text fits after the marker, so just
		// truncate rather than duplicate content.
		return truncateToBudget(text, budget)
	}
	return strings.Join(head, "\n") + "\n" + marker + "\n" + strings.Join(tail, "\n")
}

// linePrefixWithin returns the number of leading lines whose joined token
// count is within budget (at least 1 when the text is non-empty).
func linePrefixWithin(lines []string, budget int) int {
	used := 0
	for i, line := range lines {
		used += EstimateTokens(line) + 1
		if used > budget {
			if i == 0 {
				return 1
			}
			return i
		}
	}
	return len(lines)
}

// lineSuffixWithin returns the number of trailing lines whose joined token
// count is within budget (at least 1 when the text is non-empty).
func lineSuffixWithin(lines []string, budget int) int {
	used := 0
	for i := len(lines) - 1; i >= 0; i-- {
		used += EstimateTokens(lines[i]) + 1
		if used > budget {
			n := len(lines) - 1 - i
			if n == 0 {
				return 1
			}
			return n
		}
	}
	return len(lines)
}

// truncateToBudget cuts text at a rune boundary that fits the budget.
func truncateToBudget(text string, budget int) string {
	if budget <= 0 {
		return ""
	}
	runes := []rune(text)
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if EstimateTokensFast(string(runes[:mid])) <= budget {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return string(runes[:lo])
}

func max0(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
