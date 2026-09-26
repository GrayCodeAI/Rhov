package cloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
)

// contractWorker is an httptest stand-in for the GrayCode Cloud Worker that
// enforces the request schemas of the rho <-> GrayCode Cloud wire contract the
// way the Worker's zod schemas do: strict objects (unknown keys rejected),
// enums, string lengths in UTF-16 code units, integer bounds, RFC 3339
// timestamps and the opaqueID pattern. A schema violation is a 400 with the
// violation in the error, so a test failure names the offending field.
type contractWorker struct {
	t      *testing.T
	token  string
	server *httptest.Server
}

func newContractWorker(t *testing.T) *contractWorker {
	t.Helper()
	w := &contractWorker{t: t, token: "hwc_contract_token"}
	w.server = httptest.NewServer(http.HandlerFunc(w.serve))
	t.Cleanup(w.server.Close)
	return w
}

func (w *contractWorker) client() *Client {
	return New(Config{Endpoint: w.server.URL, DeviceToken: w.token})
}

func (w *contractWorker) serve(rw http.ResponseWriter, r *http.Request) {
	reply := func(status int, body any) {
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(status)
		_ = json.NewEncoder(rw).Encode(body)
	}
	if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
		reply(http.StatusBadRequest, map[string]string{"error": "expected a JSON POST"})
		return
	}
	authenticated := map[string]bool{"/v1/usage": true, "/v1/delivery-context": true, "/v1/graph/sync": true}
	if authenticated[r.URL.Path] && r.Header.Get("Authorization") != "Bearer "+w.token {
		reply(http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	var body map[string]any
	if err := decoder.Decode(&body); err != nil {
		reply(http.StatusBadRequest, map[string]string{"error": "body is not a JSON object: " + err.Error()})
		return
	}
	var err error
	switch r.URL.Path {
	case "/v1/auth/device/start":
		if err = validateDeviceStart(body); err == nil {
			reply(http.StatusCreated, map[string]any{
				"deviceCode": "dc_0123456789abcdefghijklmnop", "userCode": "ABCD-EFGH",
				"verificationUri": "https://graycodeai.com/cli/approve", "expiresIn": 600, "interval": 5,
			})
			return
		}
	case "/v1/auth/device/poll":
		if err = validateDevicePoll(body); err == nil {
			reply(http.StatusOK, map[string]any{
				"status": "approved", "token": w.token, "deviceId": "0123456789abcdef0123456789abcdef",
				"projectId": "fedcba9876543210fedcba9876543210", "principalId": "user_0123456789",
			})
			return
		}
	case "/v1/usage":
		if err = validateUsage(body); err == nil {
			reply(http.StatusAccepted, map[string]any{"accepted": true})
			return
		}
	case "/v1/delivery-context":
		if err = validateDeliveryContext(body); err == nil {
			reply(http.StatusAccepted, map[string]any{"accepted": true, "repositoryId": "repo_0123456789abcdef"})
			return
		}
	case "/v1/graph/sync":
		if err = validateGraphSync(body); err == nil {
			reply(http.StatusOK, map[string]any{"accepted": true, "duplicate": false, "graphDigest": strings.Repeat("a", 64), "facts": 1})
			return
		}
	default:
		err = fmt.Errorf("unknown route %s", r.URL.Path)
	}
	reply(http.StatusBadRequest, map[string]string{"error": "contract violation: " + err.Error()})
}

// --- zod-equivalent schema helpers ---

type field struct {
	obj  map[string]any
	path string
}

func (f field) sub(key string) string { return f.path + "." + key }

// strict rejects keys outside required+optional and requires the required
// keys, like z.object({...}).strict().
func (f field) strict(required, optional []string) error {
	allowed := map[string]bool{}
	for _, key := range append(append([]string{}, required...), optional...) {
		allowed[key] = true
	}
	for key := range f.obj {
		if !allowed[key] {
			return fmt.Errorf("%s: unrecognized key %q", f.path, key)
		}
	}
	for _, key := range required {
		if _, ok := f.obj[key]; !ok {
			return fmt.Errorf("%s: required", f.sub(key))
		}
	}
	return nil
}

func units(s string) int { return len(utf16.Encode([]rune(s))) }

// str checks z.string().min(lo).max(hi); trim mirrors .trim() before checks.
func (f field) str(key string, lo, hi int, trim bool) error {
	value, present := f.obj[key]
	if !present {
		return nil
	}
	text, ok := value.(string)
	if !ok {
		return fmt.Errorf("%s: expected string", f.sub(key))
	}
	if trim {
		text = strings.TrimSpace(text)
	}
	if n := units(text); n < lo || n > hi {
		return fmt.Errorf("%s: length %d outside %d..%d", f.sub(key), n, lo, hi)
	}
	return nil
}

var workerOpaqueID = regexp.MustCompile(`^[A-Za-z0-9._:-]+$`)

func (f field) opaque(key string) error {
	if err := f.str(key, 16, 128, false); err != nil {
		return err
	}
	if value, ok := f.obj[key].(string); ok && !workerOpaqueID.MatchString(value) {
		return fmt.Errorf("%s: invalid opaque ID %q", f.sub(key), value)
	}
	return nil
}

func (f field) integer(key string, lo, hi int64) error {
	value, present := f.obj[key]
	if !present {
		return nil
	}
	number, ok := value.(json.Number)
	if !ok {
		return fmt.Errorf("%s: expected number", f.sub(key))
	}
	n, err := number.Int64()
	if err != nil {
		return fmt.Errorf("%s: expected integer, got %s", f.sub(key), number)
	}
	if n < lo || n > hi {
		return fmt.Errorf("%s: %d outside %d..%d", f.sub(key), n, lo, hi)
	}
	return nil
}

func (f field) enum(key string, values ...string) error {
	value, present := f.obj[key]
	if !present {
		return nil
	}
	for _, allowed := range values {
		if value == allowed {
			return nil
		}
	}
	return fmt.Errorf("%s: %v is not one of %v", f.sub(key), value, values)
}

func (f field) datetime(key string) error {
	value, present := f.obj[key]
	if !present {
		return nil
	}
	text, ok := value.(string)
	if !ok {
		return fmt.Errorf("%s: expected datetime string", f.sub(key))
	}
	if _, err := time.Parse(time.RFC3339Nano, text); err != nil {
		return fmt.Errorf("%s: invalid datetime %q", f.sub(key), text)
	}
	return nil
}

func (f field) regex(key string, pattern *regexp.Regexp) error {
	value, present := f.obj[key]
	if !present {
		return nil
	}
	text, ok := value.(string)
	if !ok || !pattern.MatchString(text) {
		return fmt.Errorf("%s: %v does not match %s", f.sub(key), value, pattern)
	}
	return nil
}

func (f field) object(key string) (field, bool, error) {
	value, present := f.obj[key]
	if !present {
		return field{}, false, nil
	}
	obj, ok := value.(map[string]any)
	if !ok {
		return field{}, false, fmt.Errorf("%s: expected object", f.sub(key))
	}
	return field{obj: obj, path: f.sub(key)}, true, nil
}

func (f field) array(key string, maxItems int) ([]field, error) {
	value, ok := f.obj[key].([]any)
	if !ok {
		return nil, fmt.Errorf("%s: expected array", f.sub(key))
	}
	if len(value) > maxItems {
		return nil, fmt.Errorf("%s: %d items, max %d", f.sub(key), len(value), maxItems)
	}
	out := make([]field, 0, len(value))
	for i, item := range value {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s[%d]: expected object", f.sub(key), i)
		}
		out = append(out, field{obj: obj, path: fmt.Sprintf("%s[%d]", f.sub(key), i)})
	}
	return out, nil
}

func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// --- endpoint schemas (graycode-platform apps/worker/src) ---

func validateDeviceStart(body map[string]any) error {
	f := field{obj: body, path: "deviceStart"}
	return firstError(
		f.strict([]string{"label", "platform", "graycodeVersion"}, nil),
		f.str("label", 1, 100, false),
		f.str("platform", 1, 50, false),
		f.str("graycodeVersion", 1, 50, false),
	)
}

func validateDevicePoll(body map[string]any) error {
	f := field{obj: body, path: "devicePoll"}
	return firstError(f.strict([]string{"deviceCode"}, nil), f.str("deviceCode", 20, 128, false))
}

func validateUsage(body map[string]any) error {
	f := field{obj: body, path: "usage"}
	errs := []error{
		f.strict(
			[]string{"eventId", "deviceId", "projectId", "capability", "tokensUsed", "occurredAt"},
			[]string{"sessionId", "provider", "model", "inputTokens", "outputTokens", "cachedInputTokens", "reasoningTokens", "tokensSaved", "estimatedCostMicros", "durationMs", "status", "errorCode"},
		),
		f.opaque("eventId"), f.opaque("deviceId"), f.opaque("projectId"), f.opaque("sessionId"),
		f.enum("capability", "rho", "graycode"),
		f.str("provider", 0, 100, false), f.str("model", 0, 100, false), f.str("errorCode", 0, 100, false),
		f.integer("tokensUsed", 0, 10_000_000),
		f.integer("durationMs", 0, 86_400_000),
		f.integer("estimatedCostMicros", 0, 10_000_000_000),
		f.enum("status", "completed", "failed", "cancelled"),
		f.datetime("occurredAt"),
	}
	for _, counter := range []string{"inputTokens", "outputTokens", "cachedInputTokens", "reasoningTokens", "tokensSaved"} {
		errs = append(errs, f.integer(counter, 0, 10_000_000))
	}
	return firstError(errs...)
}

func validateDeliveryContext(body map[string]any) error {
	f := field{obj: body, path: "deliveryContext"}
	if err := firstError(
		f.strict([]string{"projectId", "repository"}, []string{"branch", "commitSha", "ciRun", "deployment"}),
		f.opaque("projectId"), f.str("branch", 1, 200, true), f.str("commitSha", 1, 128, true),
	); err != nil {
		return err
	}
	repository, _, err := f.object("repository")
	if err != nil {
		return err
	}
	if err := firstError(
		repository.strict([]string{"provider", "externalId", "name"}, []string{"url", "defaultBranch"}),
		repository.str("provider", 1, 50, true), repository.str("externalId", 1, 200, true),
		repository.str("name", 1, 300, true), repository.str("defaultBranch", 1, 200, true),
	); err != nil {
		return err
	}
	optionalRun := []string{"url", "startedAt", "completedAt"}
	if ciRun, ok, err := f.object("ciRun"); err != nil {
		return err
	} else if ok {
		if err := firstError(
			ciRun.strict([]string{"provider", "externalId", "status"}, append([]string{"workflow"}, optionalRun...)),
			ciRun.str("provider", 1, 50, true), ciRun.str("externalId", 1, 200, true), ciRun.str("workflow", 1, 200, true),
			ciRun.enum("status", "queued", "running", "succeeded", "failed", "cancelled"),
		); err != nil {
			return err
		}
	}
	if deployment, ok, err := f.object("deployment"); err != nil {
		return err
	} else if ok {
		return firstError(
			deployment.strict([]string{"provider", "externalId", "environment", "status"}, optionalRun),
			deployment.str("provider", 1, 50, true), deployment.str("externalId", 1, 200, true), deployment.str("environment", 1, 100, true),
			deployment.enum("status", "queued", "running", "succeeded", "failed", "cancelled", "rolled_back"),
		)
	}
	return nil
}

var (
	workerSchemaVersion = regexp.MustCompile(`^[a-z0-9-]+\.graph/v1$`)
	workerSHA256        = regexp.MustCompile(`^[a-f0-9]{64}$`)
	workerSensitive     = regexp.MustCompile(`(?i)(?:content|prompt|secret|credential|password|api[_-]?key|query|reason|url|path|command|provider|model|repository|branch|commit|source|target|message|evidence|element|file|fix)`)
	workerSafeSuffix    = regexp.MustCompile(`(?i)(?:_sha256|_digest|_count|_tokens?|token_count)$`)
)

func validateGraphSync(body map[string]any) error {
	f := field{obj: body, path: "graphSync"}
	if err := firstError(
		f.strict([]string{"syncId", "projectId", "graph"}, []string{"sessionId"}),
		f.opaque("syncId"), f.opaque("projectId"), f.opaque("sessionId"),
	); err != nil {
		return err
	}
	graph, _, err := f.object("graph")
	if err != nil {
		return err
	}
	if err := firstError(
		graph.strict([]string{"schema_version", "generated_at", "nodes", "edges", "events"}, []string{"query_sha256", "scope"}),
		graph.regex("schema_version", workerSchemaVersion), graph.datetime("generated_at"), graph.regex("query_sha256", workerSHA256),
		validateScope(graph),
	); err != nil {
		return err
	}
	nodes, err := graph.array("nodes", 250)
	if err != nil {
		return err
	}
	edges, err := graph.array("edges", 500)
	if err != nil {
		return err
	}
	events, err := graph.array("events", 500)
	if err != nil {
		return err
	}
	if len(nodes)+len(edges)+len(events) > 900 {
		return fmt.Errorf("graph exceeds the 900 fact limit")
	}
	nodeKinds := []string{"system", "knowledge", "execution", "policy", "quality", "operations"}
	for _, node := range nodes {
		if err := firstError(
			node.strict([]string{"id", "kind", "created_at", "provenance"}, []string{"scope", "effective_at", "attributes"}),
			node.str("id", 1, 256, true), node.enum("kind", nodeKinds...), node.datetime("created_at"), node.datetime("effective_at"),
			validateScope(node), validateProvenance(node), validateAttributes(node),
		); err != nil {
			return err
		}
	}
	for _, edge := range edges {
		if err := firstError(
			edge.strict([]string{"id", "kind", "from", "to", "created_at", "provenance"}, []string{"scope", "effective_at", "attributes"}),
			edge.str("id", 1, 256, true),
			edge.enum("kind", "contains", "depends_on", "references", "produced", "governed_by", "validated_by"),
			validateRef(edge, "from", nodeKinds), validateRef(edge, "to", nodeKinds),
			edge.datetime("created_at"), edge.datetime("effective_at"),
			validateScope(edge), validateProvenance(edge), validateAttributes(edge),
		); err != nil {
			return err
		}
	}
	for _, event := range events {
		if err := firstError(
			event.strict([]string{"id", "type", "subject", "occurred_at", "provenance"}, []string{"scope", "correlation_id", "causation_id", "idempotency_key"}),
			event.str("id", 1, 256, true), event.enum("type", "created", "updated", "transitioned", "observed", "deleted"),
			validateRef(event, "subject", nodeKinds), event.datetime("occurred_at"),
			event.str("correlation_id", 0, 256, false), event.str("causation_id", 0, 256, false), event.str("idempotency_key", 0, 256, false),
			validateScope(event), validateProvenance(event),
		); err != nil {
			return err
		}
	}
	return nil
}

func validateScope(parent field) error {
	scope, ok, err := parent.object("scope")
	if err != nil || !ok {
		return err
	}
	if tenant, present := scope.obj["tenant_id"]; present && tenant != "" {
		return fmt.Errorf("%s: tenant-scoped graphs are rejected", scope.path)
	}
	return firstError(
		scope.strict(nil, []string{"tenant_id", "project_id", "repository_id"}),
		scope.str("tenant_id", 0, 128, false), scope.str("project_id", 0, 128, false), scope.str("repository_id", 0, 256, false),
	)
}

func validateRef(parent field, key string, kinds []string) error {
	ref, ok, err := parent.object(key)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%s: required", parent.sub(key))
	}
	return firstError(ref.strict([]string{"kind", "id"}, nil), ref.enum("kind", kinds...), ref.str("id", 1, 256, true))
}

func validateProvenance(parent field) error {
	provenance, _, err := parent.object("provenance")
	if err != nil {
		return err
	}
	if err := firstError(
		provenance.strict([]string{"producer"}, []string{"version", "source_id", "evidence"}),
		provenance.str("producer", 1, 100, true), provenance.str("version", 0, 100, false), provenance.str("source_id", 0, 256, false),
	); err != nil {
		return err
	}
	if _, present := provenance.obj["evidence"]; !present {
		return nil
	}
	evidence, err := provenance.array("evidence", 16)
	if err != nil {
		return err
	}
	for _, item := range evidence {
		if err := firstError(
			item.strict([]string{"uri"}, []string{"digest", "media_type"}),
			item.str("uri", 1, 2048, false), item.str("digest", 0, 256, false), item.str("media_type", 0, 128, false),
		); err != nil {
			return err
		}
	}
	return nil
}

func validateAttributes(parent field) error {
	attributes, ok, err := parent.object("attributes")
	if err != nil || !ok {
		return err
	}
	if len(attributes.obj) > 64 {
		return fmt.Errorf("%s: too many graph attributes", attributes.path)
	}
	for key, value := range attributes.obj {
		text, isString := value.(string)
		if n := units(key); n < 1 || n > 64 || !isString || units(text) > 512 {
			return fmt.Errorf("%s: attribute %q violates the record schema", attributes.path, key)
		}
		if workerSensitive.MatchString(key) && key != "sast_source" && !workerSafeSuffix.MatchString(key) {
			return fmt.Errorf("%s: graph contains non-portable or sensitive metadata (%q)", attributes.path, key)
		}
	}
	return nil
}

// postRaw sends a hand-written body to the contract worker.
func (w *contractWorker) postRaw(t *testing.T, path, body string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, w.server.URL+path, bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+w.token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}
