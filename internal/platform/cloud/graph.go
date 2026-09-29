package cloud

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
)

const MaxGraphSyncBodySize = 1 << 20

// The cloud sensitive-attribute policy. It must match graphHasUnsafeCloudData
// in GrayCode Cloud (graycode-platform apps/worker/src/domain/graph.ts) and
// the daemon mirror in internal/daemon; testdata/graph_attribute_policy.json
// pins the shared expectations:
//   - a key naming sensitive content is hashed behind "<key>_sha256";
//   - the safe-suffix check is case-insensitive on both sides, so FILE_COUNT
//     or Model_Tokens are counts, not content;
//   - "sast_source" (exact key) is exempt on both sides: it marks whether a
//     finding came from static analysis and carries no source text or path;
//   - any scope with a tenant_id is rejected on both sides (see
//     rejectTenantScopes).
var (
	sensitiveGraphAttribute = regexp.MustCompile(`(?i)(content|prompt|secret|credential|password|api[_-]?key|query|reason|url|path|command|provider|model|repository|branch|commit|source|target|message|evidence|element|file|fix)`)
	safeGraphAttribute      = regexp.MustCompile(`(?i)(_sha256|_digest|_count|_tokens?|token_count)$`)
)

const (
	sastSourceAttribute = "sast_source"
	// Attribute bounds of the portable graph schema (UTF-16 code units).
	maxGraphAttributes     = 64
	maxGraphAttributeKey   = 64
	maxGraphAttributeValue = 512
)

// isSensitiveGraphAttribute reports whether key must be hashed before upload.
func isSensitiveGraphAttribute(key string) bool {
	return key != sastSourceAttribute && sensitiveGraphAttribute.MatchString(key) && !safeGraphAttribute.MatchString(key)
}

// PreparedGraph is a bounded, cloud-safe graph document and its deterministic
// upload identity.
type PreparedGraph struct {
	Graph  json.RawMessage
	SyncID string
	Facts  int
}

type GraphSyncRequest struct {
	SyncID    string          `json:"syncId"`
	ProjectID string          `json:"projectId"`
	SessionID string          `json:"sessionId,omitempty"`
	Graph     json.RawMessage `json:"graph"`
}

type GraphSyncResult struct {
	Accepted    bool   `json:"accepted"`
	Duplicate   bool   `json:"duplicate"`
	GraphDigest string `json:"graphDigest"`
	Facts       int    `json:"facts,omitempty"`
}

// PrepareGraph converts a portable graph to deterministic JSON, hashes values
// behind cloud-sensitive attribute names, and enforces GrayCode Cloud's fact caps.
// It does not mutate the caller's graph.
func PrepareGraph(graph any) (PreparedGraph, error) {
	raw, err := json.Marshal(graph)
	if err != nil {
		return PreparedGraph{}, fmt.Errorf("marshal graph: %w", err)
	}
	var document map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err = decoder.Decode(&document); err != nil {
		return PreparedGraph{}, fmt.Errorf("decode graph: %w", err)
	}

	nodes, err := graphFacts(document, "nodes", 250)
	if err != nil {
		return PreparedGraph{}, err
	}
	edges, err := graphFacts(document, "edges", 500)
	if err != nil {
		return PreparedGraph{}, err
	}
	events, err := graphFacts(document, "events", 500)
	if err != nil {
		return PreparedGraph{}, err
	}
	facts := len(nodes) + len(edges) + len(events)
	if facts > 900 {
		return PreparedGraph{}, fmt.Errorf("graph has %d facts; GrayCode Cloud accepts at most 900", facts)
	}
	if err := rejectTenantScopes(document, nodes, edges, events); err != nil {
		return PreparedGraph{}, err
	}

	for _, collection := range [][]any{nodes, edges} {
		for _, fact := range collection {
			item, ok := fact.(map[string]any)
			if !ok {
				return PreparedGraph{}, fmt.Errorf("graph fact must be an object")
			}
			if sanitizeErr := sanitizeGraphAttributes(item); sanitizeErr != nil {
				return PreparedGraph{}, sanitizeErr
			}
		}
	}

	digestInput, err := json.Marshal(document)
	if err != nil {
		return PreparedGraph{}, fmt.Errorf("encode graph digest input: %w", err)
	}
	document["query_sha256"] = sha256Hex(digestInput)
	prepared, err := json.Marshal(document)
	if err != nil {
		return PreparedGraph{}, fmt.Errorf("encode prepared graph: %w", err)
	}
	return PreparedGraph{
		Graph:  prepared,
		SyncID: "graph_" + sha256Hex(prepared),
		Facts:  facts,
	}, nil
}

func graphFacts(document map[string]any, field string, limit int) ([]any, error) {
	value, ok := document[field].([]any)
	if !ok {
		return nil, fmt.Errorf("graph %s must be an array", field)
	}
	if len(value) > limit {
		return nil, fmt.Errorf("graph has %d %s; GrayCode Cloud accepts at most %d", len(value), field, limit)
	}
	return value, nil
}

// rejectTenantScopes refuses a graph whose document scope or any fact scope
// sets tenant_id. GrayCode Cloud rejects such graphs because the connected
// project already scopes every upload; failing here gives a specific message
// instead of the Worker's generic 400.
func rejectTenantScopes(document map[string]any, collections ...[]any) error {
	if hasTenantScope(document) {
		return fmt.Errorf("graph scope sets tenant_id; GrayCode Cloud rejects tenant-scoped graphs because the connected project already scopes the upload")
	}
	for _, collection := range collections {
		for _, fact := range collection {
			if item, ok := fact.(map[string]any); ok && hasTenantScope(item) {
				return fmt.Errorf("graph fact %v scope sets tenant_id; GrayCode Cloud rejects tenant-scoped facts because the connected project already scopes the upload", item["id"])
			}
		}
	}
	return nil
}

func hasTenantScope(item map[string]any) bool {
	scope, ok := item["scope"].(map[string]any)
	if !ok {
		return false
	}
	tenant, present := scope["tenant_id"]
	return present && tenant != nil && tenant != ""
}

func sanitizeGraphAttributes(fact map[string]any) error {
	value, exists := fact["attributes"]
	if !exists {
		return nil
	}
	attributes, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("graph attributes must be an object")
	}
	sanitized := make(map[string]any, len(attributes))
	for key, value := range attributes {
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("graph attribute %q must be a string", key)
		}
		safeKey := key
		safeValue := text
		if isSensitiveGraphAttribute(key) {
			safeKey = key + "_sha256"
			safeValue = sha256Hex([]byte(text))
		}
		if n := utf16Len(safeKey); n == 0 || n > maxGraphAttributeKey {
			return fmt.Errorf("graph attribute key %q is %d characters after cloud sanitization; GrayCode Cloud accepts 1 to %d", safeKey, n, maxGraphAttributeKey)
		}
		if n := utf16Len(safeValue); n > maxGraphAttributeValue {
			return fmt.Errorf("graph attribute %q value is %d characters; GrayCode Cloud accepts at most %d", safeKey, n, maxGraphAttributeValue)
		}
		if _, duplicate := sanitized[safeKey]; duplicate {
			return fmt.Errorf("graph attributes collide after cloud sanitization at %q", safeKey)
		}
		sanitized[safeKey] = safeValue
	}
	if len(sanitized) > maxGraphAttributes {
		return fmt.Errorf("graph fact has %d attributes; GrayCode Cloud accepts at most %d", len(sanitized), maxGraphAttributes)
	}
	fact["attributes"] = sanitized
	return nil
}

func sha256Hex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

// SyncGraph uploads an explicitly prepared graph and reports server failures.
// Unlike automatic usage accounting, this method is called by an explicit user
// command, so errors are returned rather than discarded.
func (c *Client) SyncGraph(ctx context.Context, request GraphSyncRequest) (GraphSyncResult, error) {
	var result GraphSyncResult
	if err := c.checkEndpoint(); err != nil {
		return result, err
	}
	if c.token == "" {
		return result, ErrNotConnected
	}
	body, err := json.Marshal(request)
	if err != nil {
		return result, fmt.Errorf("marshal graph sync: %w", err)
	}
	if len(body) > MaxGraphSyncBodySize {
		return result, fmt.Errorf("graph sync body is %d bytes; GrayCode Cloud accepts at most %d", len(body), MaxGraphSyncBodySize)
	}
	req, err := c.newJSONRequest(ctx, "/v1/graph/sync", body, true)
	if err != nil {
		return result, fmt.Errorf("create graph sync request: %w", err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return result, fmt.Errorf("sync graph: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, readAPIError("graph sync", resp)
	}
	if err := decodeResponse("graph sync", resp, &result); err != nil {
		return result, err
	}
	if !result.Accepted {
		return result, fmt.Errorf("GrayCode Cloud did not accept the graph sync")
	}
	return result, nil
}
