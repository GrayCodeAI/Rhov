// Package cloud contains Rho's optional HTTP integration with GrayCode Cloud,
// the opt-in hosted control plane at https://cloud.graycodeai.com. Automatic
// usage upload is fail-open; explicit commands report every failure.
package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"
)

type Config struct {
	Endpoint    string
	DeviceToken string
	HTTPClient  *http.Client
}

type UsageEvent struct {
	EventID             string `json:"eventId"`
	DeviceID            string `json:"deviceId"`
	ProjectID           string `json:"projectId"`
	SessionID           string `json:"sessionId,omitempty"`
	Capability          string `json:"capability"`
	Provider            string `json:"provider,omitempty"`
	Model               string `json:"model,omitempty"`
	InputTokens         int    `json:"inputTokens,omitempty"`
	OutputTokens        int    `json:"outputTokens,omitempty"`
	CachedInputTokens   int    `json:"cachedInputTokens,omitempty"`
	ReasoningTokens     int    `json:"reasoningTokens,omitempty"`
	TokensUsed          int    `json:"tokensUsed"`
	TokensSaved         int    `json:"tokensSaved,omitempty"`
	EstimatedCostMicros int    `json:"estimatedCostMicros,omitempty"`
	DurationMS          int    `json:"durationMs,omitempty"`
	Status              string `json:"status,omitempty"`
	ErrorCode           string `json:"errorCode,omitempty"`
	OccurredAt          string `json:"occurredAt"`
}

// DeliveryContext is bounded repository and delivery metadata Rho can sync
// with its project-scoped device token.
type DeliveryContext struct {
	ProjectID  string `json:"projectId"`
	Repository struct {
		Provider      string `json:"provider"`
		ExternalID    string `json:"externalId"`
		Name          string `json:"name"`
		URL           string `json:"url,omitempty"`
		DefaultBranch string `json:"defaultBranch,omitempty"`
	} `json:"repository"`
	Branch     string             `json:"branch,omitempty"`
	CommitSHA  string             `json:"commitSha,omitempty"`
	CIRun      *CIRunContext      `json:"ciRun,omitempty"`
	Deployment *DeploymentContext `json:"deployment,omitempty"`
}

type CIRunContext struct {
	Provider   string `json:"provider"`
	ExternalID string `json:"externalId"`
	Workflow   string `json:"workflow,omitempty"`
	Status     string `json:"status"`
}

type DeploymentContext struct {
	Provider    string `json:"provider"`
	ExternalID  string `json:"externalId"`
	Environment string `json:"environment"`
	Status      string `json:"status"`
}

// Bounds of POST /v1/auth/device/start (strict schema; lengths are counted in
// UTF-16 code units, as the Worker's zod schema does).
const (
	maxDeviceLabel     = 100
	maxDevicePlatform  = 50
	maxGraycodeVersion = 50
	maxUserCode        = 32
)

// deviceStartRequest is the exact body of POST /v1/auth/device/start. The
// schema is strict, so no other field may be sent.
type deviceStartRequest struct {
	Label           string `json:"label"`
	Platform        string `json:"platform"`
	GraycodeVersion string `json:"graycodeVersion"`
}

type DeviceLoginStart struct {
	DeviceCode      string `json:"deviceCode"`
	UserCode        string `json:"userCode"`
	VerificationURI string `json:"verificationUri"`
	ExpiresIn       int    `json:"expiresIn"`
	Interval        int    `json:"interval"`
}

// Device login states returned by POST /v1/auth/device/poll.
const (
	DeviceLoginPending  = "pending"
	DeviceLoginApproved = "approved"
	DeviceLoginExpired  = "expired"
	DeviceLoginConsumed = "consumed"
)

var (
	// ErrDeviceLoginExpired is the terminal "expired" poll state: the code was
	// not approved in time (or is unknown to the server).
	ErrDeviceLoginExpired = errors.New("the GrayCode Cloud device login expired before it was approved; run `rho cloud login` again")
	// ErrDeviceLoginConsumed is the terminal "consumed" poll state (HTTP 409):
	// the token for this login was already issued to an earlier poll.
	ErrDeviceLoginConsumed = errors.New("this GrayCode Cloud device login was already completed and its token was issued to an earlier request; run `rho cloud login` again")
)

type DeviceLoginPoll struct {
	Status      string `json:"status"`
	Token       string `json:"token"`
	DeviceID    string `json:"deviceId"`
	ProjectID   string `json:"projectId"`
	PrincipalID string `json:"principalId"`
}

// ApprovalURL returns the browser URL for approving this login: the
// verification URI with the user code as the "code" query parameter. The
// URI must satisfy the same TLS policy as the API endpoint, so a server
// response can never make rho open a non-web or plaintext remote URL.
func (s DeviceLoginStart) ApprovalURL() (string, error) {
	u, err := url.Parse(s.VerificationURI)
	if err != nil || u.Opaque != "" || u.Hostname() == "" || u.User != nil {
		return "", fmt.Errorf("GrayCode Cloud returned an invalid verification URI")
	}
	if err := requireSecureTransport(u); err != nil {
		return "", fmt.Errorf("GrayCode Cloud returned an unsafe verification URI: %w", err)
	}
	query := u.Query()
	query.Set("code", s.UserCode)
	u.RawQuery = query.Encode()
	u.Fragment = ""
	return u.String(), nil
}

func (s DeviceLoginStart) validate() error {
	if s.DeviceCode == "" || s.VerificationURI == "" {
		return fmt.Errorf("GrayCode Cloud returned an incomplete device login response")
	}
	if s.UserCode == "" || len(s.UserCode) > maxUserCode ||
		strings.ContainsFunc(s.UserCode, func(r rune) bool { return !strings.ContainsRune(userCodeAlphabet, r) }) {
		return fmt.Errorf("GrayCode Cloud returned an invalid device login user code")
	}
	_, err := s.ApprovalURL()
	return err
}

const userCodeAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-"

// boundedField prepares a free-text request field for a strict length check:
// non-printable characters are dropped, surrounding space is trimmed, the
// value is cut to at most maxUnits UTF-16 code units, and fallback is used
// when nothing remains.
func boundedField(value string, maxUnits int, fallback string) string {
	var b strings.Builder
	units := 0
	for _, r := range strings.TrimSpace(value) {
		if !unicode.IsPrint(r) {
			continue
		}
		width := utf16.RuneLen(r)
		if width < 0 {
			continue
		}
		if units+width > maxUnits {
			break
		}
		b.WriteRune(r)
		units += width
	}
	if out := strings.TrimSpace(b.String()); out != "" {
		return out
	}
	return fallback
}

type Client struct {
	endpoint, token string
	// endpointErr records why Config.Endpoint was refused (for example a
	// plaintext http:// URL to a remote host). A client with an endpoint
	// error never sends a request, so the device token cannot leak.
	endpointErr error
	http        *http.Client
}

// errRedirect is returned by the client's CheckRedirect hook. GrayCode Cloud
// API calls never redirect; following one could replay the device token and
// request body to another origin or downgrade the connection to plain HTTP.
var errRedirect = errors.New("GrayCode Cloud responded with a redirect; refusing to follow it")

// DefaultRequestTimeout bounds one GrayCode Cloud request when Config has no
// HTTPClient. It is sized for explicit commands (a 1 MiB graph upload or a
// first TLS handshake on a slow link); the automatic usage upload passes its
// own shorter context deadline, so it stays bounded regardless.
const DefaultRequestTimeout = 15 * time.Second

func New(cfg Config) *Client {
	var client http.Client
	if cfg.HTTPClient != nil {
		client = *cfg.HTTPClient
	} else {
		client.Timeout = DefaultRequestTimeout
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return errRedirect }
	c := &Client{token: cfg.DeviceToken, http: &client}
	if strings.TrimSpace(cfg.Endpoint) != "" {
		c.endpoint, c.endpointErr = NormalizeEndpoint(cfg.Endpoint)
	}
	return c
}

func (c *Client) Enabled() bool { return c.endpoint != "" && c.endpointErr == nil && c.token != "" }

// checkEndpoint reports whether the client may send requests at all.
func (c *Client) checkEndpoint() error {
	if c.endpointErr != nil {
		return c.endpointErr
	}
	if c.endpoint == "" {
		return fmt.Errorf("GrayCode Cloud endpoint is not configured")
	}
	return nil
}

// newJSONRequest builds a POST to path on the validated endpoint. It is the
// single place requests are created, so the TLS policy in NormalizeEndpoint
// applies to every call, and the device token is attached only when
// authenticated is true.
func (c *Client) newJSONRequest(ctx context.Context, path string, body []byte, authenticated bool) (*http.Request, error) {
	if err := c.checkEndpoint(); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if authenticated {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return req, nil
}

func (c *Client) startDeviceLogin(ctx context.Context, label, platform, version string) (DeviceLoginStart, error) {
	var result DeviceLoginStart
	body, err := json.Marshal(deviceStartRequest{
		Label:           boundedField(label, maxDeviceLabel, "rho"),
		Platform:        boundedField(platform, maxDevicePlatform, "unknown"),
		GraycodeVersion: boundedField(version, maxGraycodeVersion, "dev"),
	})
	if err != nil {
		return result, err
	}
	req, err := c.newJSONRequest(ctx, "/v1/auth/device/start", body, false)
	if err != nil {
		return result, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return result, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, readAPIError("device login start", resp)
	}
	if err := decodeResponse("device login start", resp, &result); err != nil {
		return result, err
	}
	return result, result.validate()
}

func (c *Client) pollDeviceLogin(ctx context.Context, deviceCode string) (DeviceLoginPoll, error) {
	var result DeviceLoginPoll
	body, err := json.Marshal(map[string]string{"deviceCode": deviceCode})
	if err != nil {
		return result, err
	}
	req, err := c.newJSONRequest(ctx, "/v1/auth/device/poll", body, false)
	if err != nil {
		return result, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return result, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := readAPIError("device login poll", resp)
		if resp.StatusCode == http.StatusConflict && apiErr.State == DeviceLoginConsumed {
			return DeviceLoginPoll{Status: DeviceLoginConsumed}, ErrDeviceLoginConsumed
		}
		return result, apiErr
	}
	if err := decodeResponse("device login poll", resp, &result); err != nil {
		return result, err
	}
	switch result.Status {
	case DeviceLoginPending:
		return result, nil
	case DeviceLoginApproved:
		if result.Token == "" || result.DeviceID == "" || result.ProjectID == "" ||
			strings.ContainsFunc(result.Token, func(r rune) bool { return !unicode.IsGraphic(r) || unicode.IsSpace(r) }) {
			return result, fmt.Errorf("GrayCode Cloud returned an incomplete device authorization")
		}
		return result, nil
	case DeviceLoginExpired:
		return result, ErrDeviceLoginExpired
	case DeviceLoginConsumed:
		return result, ErrDeviceLoginConsumed
	default:
		return result, fmt.Errorf("GrayCode Cloud returned an unknown device login status %q", sanitizeServerText(result.Status))
	}
}

// StartDeviceLogin begins a browser device login. version is rho's own version
// string, sent as the contract's graycodeVersion field; label, platform and
// version are cut to the contract's bounds.
func (c *Client) StartDeviceLogin(ctx context.Context, label, platform, version string) (DeviceLoginStart, error) {
	if err := c.checkEndpoint(); err != nil {
		return DeviceLoginStart{}, err
	}
	return c.startDeviceLogin(ctx, label, platform, version)
}

func (c *Client) PollDeviceLogin(ctx context.Context, deviceCode string) (DeviceLoginPoll, error) {
	if err := c.checkEndpoint(); err != nil {
		return DeviceLoginPoll{}, err
	}
	return c.pollDeviceLogin(ctx, deviceCode)
}
