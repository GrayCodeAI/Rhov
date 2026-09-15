package diff

import (
	"strconv"
	"strings"
)

// AddedLine is one added line of a unified diff with its new-file location.
type AddedLine struct {
	File string
	Line int
	Text string
}

// ParseHunkNewStart extracts the new-file start line from a hunk header like
// "@@ -a,b +c,d @@". ok is false when no parseable number is present, and the
// caller must fall through to normal line handling.
func ParseHunkNewStart(header string) (int, bool) {
	parts := strings.Split(header, "+")
	if len(parts) < 2 {
		return 0, false
	}
	numStr := strings.Split(parts[1], ",")[0]
	n, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, false
	}
	return n, true
}

// ParseUnifiedAddedLines walks a unified diff and returns every added line
// with its file path and new-file line number.
//
// File tracking follows "+++ b/<path>" headers; hunk headers reset the
// counter to the header's new-file start. Context (" ") and added ("+")
// lines advance the new-file counter; removed lines, "---"/"+++" headers,
// and "diff --git" markers do not. A malformed hunk header leaves the counter
// untouched and is itself skipped as a non-body line.
func ParseUnifiedAddedLines(diff string) []AddedLine {
	var out []AddedLine
	currentFile := ""
	currentLine := 0

	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+++ b/") {
			currentFile = strings.TrimPrefix(line, "+++ b/")
			currentLine = 0
			continue
		}
		if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") ||
			strings.HasPrefix(line, "diff --git") {
			continue
		}

		if strings.HasPrefix(line, "@@") {
			if n, ok := ParseHunkNewStart(line); ok {
				currentLine = n - 1
				continue
			}
			// Malformed header: not a body line, keep counting state.
			continue
		}

		switch {
		case strings.HasPrefix(line, "+"):
			currentLine++
			out = append(out, AddedLine{File: currentFile, Line: currentLine, Text: line[1:]})
		case strings.HasPrefix(line, " "):
			currentLine++
		case line == "":
			// A bare empty line is an empty context line (git renders
			// context line "" for an empty source line); it occupies a
			// new-file line.
			currentLine++
		default:
			// Removed ("-") and unknown lines do not advance the counter.
		}
	}

	return out
}
