package token

import (
	"encoding/json"
	"sort"
	"strings"
)

// Tool-catalog compression ("shrink"). A large tool catalog is paid on every
// request. ShrinkToolCatalog reduces that cost while preserving, byte-for-byte,
// every token the model needs to SELECT a tool and CONSTRUCT valid arguments.
// Fail-open: any parse problem returns the input unchanged.

const toolschemaShortDesc = 600

var constraintMarkers = []string{
	"must", "cannot", "required", "rejected", "invalid", "not allowed",
	"exactly one", "at least", "at most", "only ", "never",
	"max", "maximum", "min", "minimum", "limit", "between", "range",
	"greater than", "less than", "over ", "under ", "above", "below",
	"unique", "mutually exclusive", "either", "neither",
	"format", "iso", "rfc", "absolute", "relative path", "url", "utf-8",
	"default", "defaults to", "optional", "ignored", "deprecated",
}

// ToolShrinkStats reports one tool's reduction.
type ToolShrinkStats struct {
	Name       string `json:"name"`
	Before     int    `json:"bytes_before"`
	After      int    `json:"bytes_after"`
	DescBefore int    `json:"desc_bytes_before"`
	DescAfter  int    `json:"desc_bytes_after"`
}

// LintToolCatalog reports per-tool reductions without committing to them.
func LintToolCatalog(catalog string) ([]ToolShrinkStats, bool) {
	out, ok := shrinkCatalog([]byte(catalog), true)
	if !ok {
		return nil, false
	}
	stats, ok := out.([]ToolShrinkStats)
	if !ok {
		return nil, false
	}
	return stats, true
}

// ShrinkToolCatalog compresses an OpenAI-style function-tool catalog. Fail-open:
// on any problem the input is returned unchanged with ok=false.
func ShrinkToolCatalog(catalog string) (string, bool) {
	out, ok := shrinkCatalog([]byte(catalog), false)
	if !ok {
		return catalog, false
	}
	b, err := json.Marshal(out)
	if err != nil {
		return catalog, false
	}
	if len(b) >= len(catalog) {
		return catalog, false
	}
	return string(b), true
}

func shrinkCatalog(raw []byte, lintOnly bool) (interface{}, bool) {
	var tools []map[string]json.RawMessage
	if json.Unmarshal(raw, &tools) != nil || len(tools) == 0 {
		return nil, false
	}
	stats := make([]ToolShrinkStats, 0, len(tools))
	out := make([]map[string]json.RawMessage, 0, len(tools))
	changed := false

	for _, tool := range tools {
		fnRaw, has := tool["function"]
		if !has {
			out = append(out, tool)
			continue
		}
		before := len(fnRaw)
		fn, err := shrinkFunctionDef(fnRaw)
		if err != nil {
			out = append(out, tool)
			continue
		}
		after := len(fn)
		var name string
		var fnMap map[string]json.RawMessage
		if err := json.Unmarshal(fn, &fnMap); err == nil {
			if n, ok := fnMap["name"]; ok {
				_ = json.Unmarshal(n, &name)
			}
		}
		descB, descA := countDescBytes(fnRaw), countDescBytes(fn)
		stats = append(stats, ToolShrinkStats{Name: name, Before: before, After: after, DescBefore: descB, DescAfter: descA})
		if after < before {
			changed = true
		}
		if !lintOnly {
			newTool := map[string]json.RawMessage{}
			for k, v := range tool {
				if k == "function" {
					v = fn
				}
				newTool[k] = v
			}
			out = append(out, newTool)
		}
	}
	if lintOnly {
		return stats, true
	}
	if !changed {
		return nil, false
	}
	return out, true
}

func shrinkFunctionDef(raw json.RawMessage) (json.RawMessage, error) {
	var fn map[string]interface{}
	if err := json.Unmarshal(raw, &fn); err != nil {
		return nil, err
	}
	if desc, ok := fn["description"].(string); ok && len(desc) > toolschemaShortDesc {
		fn["description"] = shrinkDescription(desc)
	}
	params, ok := fn["parameters"].(map[string]interface{})
	if !ok {
		b, err := json.Marshal(fn)
		return b, err
	}
	cleanSchema(params)
	b, err := json.Marshal(fn)
	return b, err
}

func cleanSchema(node map[string]interface{}) {
	for k := range node {
		switch k {
		case "$schema", "$id", "$comment", "title", "examples":
			delete(node, k)
		default:
			if strings.HasPrefix(k, "x-") {
				delete(node, k)
			}
		}
	}
	for _, k := range sortedKeys(node) {
		switch tv := node[k].(type) {
		case map[string]interface{}:
			cleanSchema(tv)
		case []interface{}:
			for _, item := range tv {
				if m, ok := item.(map[string]interface{}); ok {
					cleanSchema(m)
				}
			}
		}
	}
	if props, ok := node["properties"].(map[string]interface{}); ok {
		for _, pk := range sortedKeys(props) {
			if pv, ok := props[pk].(map[string]interface{}); ok {
				if d, ok := pv["description"].(string); ok && len(d) > toolschemaShortDesc {
					pv["description"] = shrinkDescription(d)
				}
				if items, ok := pv["items"].(map[string]interface{}); ok {
					if d, ok := items["description"].(string); ok && len(d) > toolschemaShortDesc {
						items["description"] = shrinkDescription(d)
					}
				}
			}
		}
	}
}

func shrinkDescription(desc string) string {
	sentences := splitSentences(desc)
	if len(sentences) <= 1 {
		return desc
	}
	keep := []string{sentences[0]}
	for _, s := range sentences[1:] {
		low := strings.ToLower(s)
		for _, marker := range constraintMarkers {
			if strings.Contains(low, marker) {
				keep = append(keep, s)
				break
			}
		}
	}
	out := strings.Join(keep, " ")
	if len(out) >= len(desc) {
		return desc
	}
	return out
}

func splitSentences(s string) []string {
	var out []string
	cur := strings.Builder{}
	for _, r := range s {
		cur.WriteRune(r)
		if r == '.' || r == '!' || r == '?' {
			out = append(out, strings.TrimSpace(cur.String()))
			cur.Reset()
		}
	}
	if t := strings.TrimSpace(cur.String()); t != "" {
		out = append(out, t)
	}
	return out
}

func sortedKeys(m map[string]interface{}) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func countDescBytes(fnRaw json.RawMessage) int {
	var fn struct {
		Description string `json:"description"`
	}
	if json.Unmarshal(fnRaw, &fn) == nil {
		return len(fn.Description)
	}
	return 0
}
