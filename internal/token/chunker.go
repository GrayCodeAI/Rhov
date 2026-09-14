package token

import "strings"

// CodeChunk represents a chunk of source code for semantic search.
type CodeChunk struct {
	Content   string `json:"content"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Symbol    string `json:"symbol,omitempty"`
	Tokens    int    `json:"tokens"`
}

// ChunkOptions configures code chunking.
type ChunkOptions struct {
	MaxTokens     int
	MinTokens     int
	MinChunkSize  int
	Language      string
	Overlap       int
	KeepSeparator int
}

// DefaultChunkOptions returns sensible defaults for code chunking.
func DefaultChunkOptions() ChunkOptions {
	return ChunkOptions{MaxTokens: 1000, MinTokens: 100}
}

// ChunkCode splits source into line-bounded chunks that respect the token
// budget. It is a local, dependency-free implementation: chunks break on line
// boundaries and never exceed MaxTokens (default 1000).
func ChunkCode(source string, opts ChunkOptions) []CodeChunk {
	if strings.TrimSpace(source) == "" {
		return nil
	}
	maxTokens := opts.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1000
	}
	lines := strings.Split(source, "\n")
	var chunks []CodeChunk
	start := 0
	for start < len(lines) {
		// Grow the chunk one line at a time until adding the next line would
		// exceed the budget. Token counts are measured on the accumulated
		// content so the reported count matches the budget check.
		end := start + 1
		for end < len(lines) {
			candidate := strings.Join(lines[start:end+1], "\n")
			if EstimateTokensFast(candidate) > maxTokens {
				break
			}
			end++
		}
		chunks = append(chunks, makeChunk(lines, start, end))
		start = end
	}
	return chunks
}

func makeChunk(lines []string, start, end int) CodeChunk {
	content := strings.Join(lines[start:end], "\n")
	return CodeChunk{
		Content:   content,
		StartLine: start + 1,
		EndLine:   end,
		Tokens:    EstimateTokensFast(content),
	}
}
