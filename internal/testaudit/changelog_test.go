package testaudit

import (
	"regexp"
	"strings"
	"testing"

	"golang.org/x/mod/semver"
)

// CHANGELOG.md follows Keep a Changelog: one [Unreleased] section on top,
// released rho versions newest first, each with unique subsection headings.
// Entries from before the rename to rho are kept verbatim under "[hawk …]"
// headings so they cannot be mistaken for rho versions.
var (
	changelogReleaseHeading    = regexp.MustCompile(`^## \[(\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?)\] — \d{4}-\d{2}-\d{2}$`)
	changelogHistoricalHeading = regexp.MustCompile(`^## \[hawk [^\]]+\]`)
)

func TestChangelogStructure(t *testing.T) {
	version := repoVersion(t)
	lines := strings.Split(string(readRepoFile(t, "CHANGELOG.md")), "\n")

	var sections []string
	subsections := map[string]map[string]bool{}
	var current string
	var lastRelease string
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "## "):
			current = line
			sections = append(sections, line)
			subsections[line] = map[string]bool{}
			switch {
			case line == "## [Unreleased]":
			case changelogReleaseHeading.MatchString(line):
				v := "v" + changelogReleaseHeading.FindStringSubmatch(line)[1]
				if semver.Compare(v, "v"+version) > 0 {
					t.Errorf("CHANGELOG.md:%d: %s is newer than VERSION %s", i+1, line, version)
				}
				if lastRelease != "" && semver.Compare(v, lastRelease) >= 0 {
					t.Errorf("CHANGELOG.md:%d: %s is not older than the section above it (%s)", i+1, line, lastRelease)
				}
				lastRelease = v
			case changelogHistoricalHeading.MatchString(line):
			default:
				t.Errorf("CHANGELOG.md:%d: unexpected section heading %q (want [Unreleased], [X.Y.Z] — YYYY-MM-DD, or a [hawk …] historical entry)", i+1, line)
			}
		case strings.HasPrefix(line, "### ") && current != "":
			if subsections[current][line] {
				t.Errorf("CHANGELOG.md:%d: duplicate %q inside %q; merge the lists", i+1, line, current)
			}
			subsections[current][line] = true
		}
	}
	if len(sections) == 0 || sections[0] != "## [Unreleased]" {
		t.Fatalf("CHANGELOG.md must start with a single ## [Unreleased] section, got %q", sections)
	}
	unreleased := 0
	for _, s := range sections {
		if s == "## [Unreleased]" {
			unreleased++
		}
	}
	if unreleased != 1 {
		t.Errorf("CHANGELOG.md has %d [Unreleased] sections, want 1", unreleased)
	}
}
