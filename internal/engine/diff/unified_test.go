package diff

import (
	"reflect"
	"testing"
)

func TestParseUnifiedAddedLines_MultiFileMultiHunk(t *testing.T) {
	diff := "diff --git a/a.go b/a.go\n" +
		"--- a/a.go\n" +
		"+++ b/a.go\n" +
		"@@ -1,2 +1,3 @@\n" +
		" ctx\n" +
		"-old\n" +
		"+new1\n" +
		"+new2\n" +
		"@@ -10,1 +11,2 @@\n" +
		"+new3\n" +
		"diff --git a/b.py b/b.py\n" +
		"--- a/b.py\n" +
		"+++ b/b.py\n" +
		"@@ -5,2 +5,2 @@\n" +
		"-x = 1\n" +
		"+x = 2\n"

	got := ParseUnifiedAddedLines(diff)
	want := []AddedLine{
		{File: "a.go", Line: 2, Text: "new1"},
		{File: "a.go", Line: 3, Text: "new2"},
		{File: "a.go", Line: 11, Text: "new3"},
		{File: "b.py", Line: 5, Text: "x = 2"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseUnifiedAddedLines() = %+v, want %+v", got, want)
	}
}

func TestParseUnifiedAddedLines_DevNullAndMalformedHeader(t *testing.T) {
	diff := "diff --git a/old.go b/new.go\n" +
		"--- /dev/null\n" +
		"+++ b/new.go\n" +
		"@@ -0,0 +1,1 @@\n" +
		"+hello\n" +
		"@@ malformed header\n" +
		"+after-bad-header\n"

	got := ParseUnifiedAddedLines(diff)
	// The malformed @@ line is skipped without touching the counter, so the
	// next added line continues from the previous hunk.
	want := []AddedLine{
		{File: "new.go", Line: 1, Text: "hello"},
		{File: "new.go", Line: 2, Text: "after-bad-header"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseUnifiedAddedLines() = %+v, want %+v", got, want)
	}
}

func TestParseUnifiedAddedLines_BareEmptyContextLine(t *testing.T) {
	diff := "+++ b/f.txt\n" +
		"@@ -1,3 +1,3 @@\n" +
		" a\n" +
		"\n" +
		"+b\n"

	got := ParseUnifiedAddedLines(diff)
	want := []AddedLine{{File: "f.txt", Line: 3, Text: "b"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseUnifiedAddedLines() = %+v, want %+v", got, want)
	}
}

func TestParseUnifiedAddedLines_Empty(t *testing.T) {
	if got := ParseUnifiedAddedLines(""); len(got) != 0 {
		t.Fatalf("expected no added lines, got %+v", got)
	}
	if got := ParseUnifiedAddedLines("just some prose\n"); len(got) != 0 {
		t.Fatalf("expected no added lines, got %+v", got)
	}
}

func TestParseUnifiedAddedLines_NoFileHeader(t *testing.T) {
	// Lines outside any file header are still reported with an empty path,
	// matching the historical review pass.
	got := ParseUnifiedAddedLines("+const apiKey = \"x\"\n")
	want := []AddedLine{{File: "", Line: 1, Text: `const apiKey = "x"`}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseUnifiedAddedLines() = %+v, want %+v", got, want)
	}
}

func TestParseHunkNewStart(t *testing.T) {
	n, ok := ParseHunkNewStart("@@ -1,2 +3,4 @@")
	if !ok || n != 3 {
		t.Fatalf("ParseHunkNewStart = %d,%v, want 3,true", n, ok)
	}
	if _, ok := ParseHunkNewStart("@@ malformed"); ok {
		t.Fatal("expected ok=false for malformed header")
	}
	if _, ok := ParseHunkNewStart("not a header"); ok {
		t.Fatal("expected ok=false for non-header")
	}
}
