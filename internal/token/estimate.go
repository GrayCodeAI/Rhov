// Package token is Hawk's self-contained token/context engine. It replaces the
// former external shrike dependency with local implementations of token
// estimation, compression, usage tracking, secret detection, code chunking,
// and runtime-graph projection. The public surface is unchanged so existing
// callers (engine, repomap, lsp, config) keep working.
package token

import (
	"container/list"
	"hash/maphash"
	"strings"
	"sync"
	"sync/atomic"

	tiktoken "github.com/tiktoken-go/tokenizer"
)

// Stats contains compression statistics.
type Stats struct {
	OriginalTokens   int
	FinalTokens      int
	TokensSaved      int
	ReductionPercent float64
	Layers           map[string]LayerStat
	Model            string  // Model used for cost calculation
	CostSavings      float64 // Dollar savings from compression
}

// LayerStat contains per-layer statistics.
type LayerStat struct {
	TokensSaved int
	DurationMs  int64
}

// HardTruncated reports whether the final budget enforcer did the bulk of the
// token reduction — i.e. the output is mostly the input cut short at the
// budget boundary — rather than structural compression. A hard-truncated
// output is NOT a rewrite of the content and must not be treated as a summary.
func (s Stats) HardTruncated() bool {
	if s.TokensSaved <= 0 {
		return false
	}
	budgetSaved := s.Layers["budget"].TokensSaved
	return budgetSaved > 0 && budgetSaved*2 > s.TokensSaved
}

// --- BPE tokenizer with a bounded LRU cache ---

// BPETokenizer wraps tiktoken for accurate BPE token counting.
type BPETokenizer struct {
	codec tiktoken.Codec
	cache *tokenCache
}

type cacheEntry struct {
	count  int
	length int
	elem   *list.Element
}

var cacheSeed = maphash.MakeSeed()

const maxCacheableTextBytes = 1 << 20 // 1 MiB

type tokenCacheShard struct {
	mu    sync.Mutex
	items map[uint64]*cacheEntry
	ll    *list.List
	max   int
}

type tokenCache struct {
	shards [16]*tokenCacheShard
}

func newTokenCache(max int) *tokenCache {
	c := &tokenCache{}
	for i := range c.shards {
		c.shards[i] = &tokenCacheShard{
			items: make(map[uint64]*cacheEntry),
			ll:    list.New(),
			max:   max,
		}
	}
	return c
}

func (c *tokenCache) hashText(text string) uint64 {
	var h maphash.Hash
	h.SetSeed(cacheSeed)
	_, _ = h.WriteString(text)
	return h.Sum64()
}

func (c *tokenCache) get(text string) (int, bool) {
	if len(text) > maxCacheableTextBytes {
		return 0, false
	}
	key := c.hashText(text)
	sh := c.shards[key%uint64(len(c.shards))]
	sh.mu.Lock()
	defer sh.mu.Unlock()
	entry, ok := sh.items[key]
	if !ok || entry.length != len(text) {
		return 0, false
	}
	sh.ll.MoveToFront(entry.elem)
	return entry.count, true
}

func (c *tokenCache) put(text string, count int) {
	if len(text) > maxCacheableTextBytes {
		return
	}
	key := c.hashText(text)
	sh := c.shards[key%uint64(len(c.shards))]
	sh.mu.Lock()
	defer sh.mu.Unlock()
	if entry, ok := sh.items[key]; ok {
		entry.count = count
		entry.length = len(text)
		sh.ll.MoveToFront(entry.elem)
		return
	}
	elem := sh.ll.PushFront(&lruItem{key: key})
	sh.items[key] = &cacheEntry{count: count, length: len(text), elem: elem}
	for len(sh.items) > sh.max {
		back := sh.ll.Back()
		if back == nil {
			break
		}
		item, _ := back.Value.(*lruItem)
		if item == nil {
			break
		}
		delete(sh.items, item.key)
		sh.ll.Remove(back)
	}
}

type lruItem struct{ key uint64 }

var (
	bpeOnce      sync.Once
	bpeTokenizer *BPETokenizer
	bpeFailed    atomic.Bool
)

func getBPETokenizer() (*BPETokenizer, error) {
	bpeOnce.Do(func() {
		codec, err := tiktoken.Get(tiktoken.Cl100kBase)
		if err != nil {
			bpeFailed.Store(true)
			return
		}
		bpeTokenizer = &BPETokenizer{codec: codec, cache: newTokenCache(4096)}
	})
	if bpeFailed.Load() {
		return nil, errBPEFailed
	}
	return bpeTokenizer, nil
}

var errBPEFailed = errString("token: BPE tokenizer unavailable")

type errString string

func (e errString) Error() string { return string(e) }

// fastEstimateTokens is a self-contained BPE-style token estimate used when
// the BPE tokenizer is unavailable. It lands in the 3-7 chars-per-token band
// for English prose.
func fastEstimateTokens(text string) int {
	length := len(text)
	if length == 0 {
		return 0
	}
	// Very short strings: assume 1 token per 3 chars.
	if length < 30 {
		return (length + 2) / 3
	}
	// Short strings: standard heuristic.
	if length < 100 {
		return (length + 3) / 4
	}
	spaces := 0
	sample := length
	if sample > 200 {
		sample = 200
	}
	for i := 0; i < sample; i++ {
		if text[i] == ' ' || text[i] == '\n' || text[i] == '\t' {
			spaces++
		}
	}
	spaceRatio := float64(spaces) / float64(sample)
	nonSpaceChars := float64(length) * (1 - spaceRatio)
	spaceTokens := float64(length) * spaceRatio
	return int(nonSpaceChars/3.5 + spaceTokens)
}

// EstimateTokens is the single source of truth for token estimation.
func EstimateTokens(text string) int { return EstimateTokensPrecise(text) }

// EstimateTokensFast provides a fast estimate without BPE.
func EstimateTokensFast(text string) int { return fastEstimateTokens(text) }

// EstimateTokensPrecise always uses BPE, skipping the short-string heuristic.
func EstimateTokensPrecise(text string) int {
	if text == "" {
		return 0
	}
	if bpeFailed.Load() {
		return fastEstimateTokens(text)
	}
	tok, err := getBPETokenizer()
	if err != nil || tok == nil {
		return fastEstimateTokens(text)
	}
	if n, ok := tok.cache.get(text); ok {
		return n
	}
	n, err := tok.codec.Count(text)
	if err != nil {
		return fastEstimateTokens(text)
	}
	tok.cache.put(text, n)
	return n
}

// CountTokens returns a precise BPE-based token count.
func CountTokens(text string) int { return EstimateTokensPrecise(text) }

// CountTokensFast returns a token count. Historically this forwarded to the
// engine's BPE estimate (not the heuristic), so it is kept behavior-identical
// to CountTokens. Use EstimateTokensFast for the cheap heuristic path.
func CountTokensFast(text string) int { return EstimateTokensPrecise(text) }

// ShrikeAvailable reports whether a real tokenizer is linked. Retained for
// status surfaces; the local engine always reports true.
func ShrikeAvailable() bool { return true }

// CalculateTokensSaved computes token savings between original and filtered.
func CalculateTokensSaved(original, filtered string) int {
	origTokens := EstimateTokens(original)
	filterTokens := EstimateTokens(filtered)
	if origTokens > filterTokens {
		return origTokens - filterTokens
	}
	return 0
}

var _ = strings.TrimSpace
