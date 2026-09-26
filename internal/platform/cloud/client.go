// Package cloud contains Rho's optional, fail-open HTTP integration with Rho Cloud.
package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
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

type DeviceLoginStart struct {
	DeviceCode      string `json:"deviceCode"`
	UserCode        string `json:"userCode"`
	VerificationURI string `json:"verificationUri"`
	ExpiresIn       int    `json:"expiresIn"`
	Interval        int    `json:"interval"`
}

type DeviceLoginPoll struct {
	Status      string `json:"status"`
	Token       string `json:"token"`
	DeviceID    string `json:"deviceId"`
	ProjectID   string `json:"projectId"`
	PrincipalID string `json:"principalId"`
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

func New(cfg Config) *Client {
	var client http.Client
	if cfg.HTTPClient != nil {
		client = *cfg.HTTPClient
	} else {
		client.Timeout = 3 * time.Second
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
		return fmt.Errorf("rho cloud endpoint is not configured")
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

func (c *Client) startDeviceLogin(ctx context.Context, label, platform, rhoVersion string) (DeviceLoginStart, error) {
	var result DeviceLoginStart
	body, err := json.Marshal(map[string]string{"label": label, "platform": platform, "rhoVersion": rhoVersion})
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
		return result, fmt.Errorf("rho cloud device login start: %s", resp.Status)
	}
	return result, json.NewDecoder(resp.Body).Decode(&result)
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
		return result, fmt.Errorf("rho cloud device login poll: %s", resp.Status)
	}
	return result, json.NewDecoder(resp.Body).Decode(&result)
}

func (c *Client) StartDeviceLogin(ctx context.Context, label, platform, rhoVersion string) (DeviceLoginStart, error) {
	if err := c.checkEndpoint(); err != nil {
		return DeviceLoginStart{}, err
	}
	return c.startDeviceLogin(ctx, label, platform, rhoVersion)
}

func (c *Client) PollDeviceLogin(ctx context.Context, deviceCode string) (DeviceLoginPoll, error) {
	if err := c.checkEndpoint(); err != nil {
		return DeviceLoginPoll{}, err
	}
	return c.pollDeviceLogin(ctx, deviceCode)
}

// RecordUsage intentionally discards transport failures. Cloud sync must not alter local execution.
func (c *Client) RecordUsage(ctx context.Context, event UsageEvent) {
	if !c.Enabled() {
		return
	}
	body, err := json.Marshal(event)
	if err != nil {
		return
	}
	req, err := c.newJSONRequest(ctx, "/v1/usage", body, true)
	if err != nil {
		return
	}
	resp, err := c.http.Do(req)
	if err == nil && resp != nil {
		_ = resp.Body.Close()
	}
}

// RecordDeliveryContext is fail-open: delivery metadata must not affect local execution.
func (c *Client) RecordDeliveryContext(ctx context.Context, event DeliveryContext) {
	if !c.Enabled() {
		return
	}
	body, err := json.Marshal(event)
	if err != nil {
		return
	}
	req, err := c.newJSONRequest(ctx, "/v1/delivery-context", body, true)
	if err != nil {
		return
	}
	resp, err := c.http.Do(req)
	if err == nil && resp != nil {
		_ = resp.Body.Close()
	}
}
