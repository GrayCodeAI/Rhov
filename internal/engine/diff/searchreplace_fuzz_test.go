package diff

import "testing"

// FuzzParseSearchReplace ensures the SEARCH/REPLACE parser and applier never
// panic on arbitrary model output.
func FuzzParseSearchReplace(f *testing.F) {
	f.Add("<<<<<<< SEARCH\nold\n=======\nnew\n>>>>>>> REPLACE")
	f.Add("")
	f.Add("<<<<<<< SEARCH\nonly a search\n>>>>>>> REPLACE")
	f.Add("prose before\n<<<<<<< SEARCH\na\n=======\nb\n>>>>>>> REPLACE\nprose after")

	f.Fuzz(func(t *testing.T, text string) {
		blocks := ParseSearchReplace(text)
		_, _ = ApplySearchReplace("old\ncontent\n", blocks)
		_, _ = ApplySearchReplace("", blocks)
	})
}

func BenchmarkParseSearchReplace(b *testing.B) {
	const in = "prose\n<<<<<<< SEARCH\nold\n=======\nnew\n>>>>>>> REPLACE\n"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = ParseSearchReplace(in)
	}
}
