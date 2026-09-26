package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestNormalizeEndpointAcceptsTLSAndLoopbackHTTP(t *testing.T) {
	cases := map[string]string{
		"https://cloud.graycodeai.com":         "https://cloud.graycodeai.com",
		"https://cloud.graycodeai.com/":        "https://cloud.graycodeai.com",
		" https://cloud.graycodeai.com/api/ ":  "https://cloud.graycodeai.com/api",
		"HTTPS://cloud.graycodeai.com":         "https://cloud.graycodeai.com",
		"http://localhost:8787":                "http://localhost:8787",
		"http://LOCALHOST:8787/":               "http://LOCALHOST:8787",
		"http://127.0.0.1:1":                   "http://127.0.0.1:1",
		"http://127.0.0.1":                     "http://127.0.0.1",
		"http://[::1]:8787":                    "http://[::1]:8787",
		"https://cloud.graycodeai.com:8443/v1": "https://cloud.graycodeai.com:8443/v1",
	}
	for raw, want := range cases {
		got, err := NormalizeEndpoint(raw)
		if err != nil {
			t.Errorf("NormalizeEndpoint(%q) error = %v", raw, err)
			continue
		}
		if got != want {
			t.Errorf("NormalizeEndpoint(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestNormalizeEndpointRejectsPlaintextRemoteAndMalformedURLs(t *testing.T) {
	insecure := []string{
		"http://cloud.graycodeai.com",
		"http://127.0.0.2:8787",
		"http://localhost.attacker.example",
		"http://0.0.0.0:8787",
		"http://[::2]:8787",
		"ftp://cloud.graycodeai.com",
		"ws://localhost:8787",
	}
	for _, raw := range insecure {
		if _, err := NormalizeEndpoint(raw); !errors.Is(err, ErrInsecureEndpoint) {
			t.Errorf("NormalizeEndpoint(%q) error = %v, want ErrInsecureEndpoint", raw, err)
		}
	}
	malformed := []string{
		"",
		"   ",
		"cloud.graycodeai.com",
		"https://",
		"https://:8443",
		"https://user:secret@cloud.graycodeai.com",
		"https://cloud.graycodeai.com?token=x",
		"https://cloud.graycodeai.com?",
		"https://cloud.graycodeai.com#frag",
		"mailto:ops@graycodeai.com",
	}
	for _, raw := range malformed {
		if got, err := NormalizeEndpoint(raw); err == nil {
			t.Errorf("NormalizeEndpoint(%q) = %q, want error", raw, got)
		}
	}
	if _, err := NormalizeEndpoint("https://user:secret@cloud.graycodeai.com"); err != nil && strings.Contains(err.Error(), "secret") {
		t.Fatalf("error leaks URL credentials: %v", err)
	}
}

// countingTransport records every request so tests can prove none was sent.
type countingTransport struct{ calls atomic.Int32 }

func (c *countingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	c.calls.Add(1)
	return nil, errors.New("unexpected request")
}

func TestInsecureEndpointNeverSendsTheDeviceToken(t *testing.T) {
	transport := &countingTransport{}
	client := New(Config{
		Endpoint:    "http://cloud.graycodeai.com",
		DeviceToken: "hwc_secret",
		HTTPClient:  &http.Client{Transport: transport},
	})
	if client.Enabled() {
		t.Fatal("client with a plaintext remote endpoint must not be enabled")
	}
	ctx := context.Background()
	client.RecordUsage(ctx, UsageEvent{EventID: "event_0123456789", Capability: "rho"})
	if _, err := client.SyncGraph(ctx, GraphSyncRequest{SyncID: "graph_0123456789abcdef", Graph: json.RawMessage(`{}`)}); err == nil {
		t.Fatal("SyncGraph succeeded against a plaintext endpoint")
	}
	if _, err := client.StartDeviceLogin(ctx, "laptop", "darwin", "0.3.0"); !errors.Is(err, ErrInsecureEndpoint) {
		t.Fatalf("StartDeviceLogin error = %v, want ErrInsecureEndpoint", err)
	}
	if _, err := client.PollDeviceLogin(ctx, "device-code-0123456789"); !errors.Is(err, ErrInsecureEndpoint) {
		t.Fatalf("PollDeviceLogin error = %v, want ErrInsecureEndpoint", err)
	}
	if n := transport.calls.Load(); n != 0 {
		t.Fatalf("transport saw %d requests, want 0", n)
	}
}

func TestClientRefusesRedirects(t *testing.T) {
	var leaked atomic.Value
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		leaked.Store(r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusAccepted)
	}))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	defer origin.Close()

	_, err := New(Config{Endpoint: origin.URL, DeviceToken: "hwc_secret"}).SyncGraph(
		context.Background(),
		GraphSyncRequest{SyncID: "graph_0123456789abcdef", ProjectID: "project_0123456789", Graph: json.RawMessage(`{}`)},
	)
	if err == nil || !strings.Contains(err.Error(), "redirect") {
		t.Fatalf("SyncGraph error = %v, want redirect refusal", err)
	}
	if got, _ := leaked.Load().(string); got != "" {
		t.Fatalf("redirect target received Authorization %q", got)
	}
}

func TestSaveDeviceConfigRefusesPlaintextEndpoint(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("RHO_CONFIG_DIR", dir)
	err := SaveDeviceConfig(DeviceConfig{Endpoint: "http://cloud.graycodeai.com", DeviceID: "device_0123456789", ProjectID: "project_0123456789"}, "hwc_secret")
	if !errors.Is(err, ErrInsecureEndpoint) {
		t.Fatalf("SaveDeviceConfig error = %v, want ErrInsecureEndpoint", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "cloud.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("cloud.json was written for a refused endpoint (stat err %v)", statErr)
	}
}

func TestLoadClientRefusesSavedPlaintextEndpoint(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("RHO_CONFIG_DIR", dir)
	saved := `{"endpoint":"http://cloud.graycodeai.com","device_id":"device_0123456789","project_id":"project_0123456789"}`
	if err := os.WriteFile(filepath.Join(dir, "cloud.json"), []byte(saved), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadClient(); !errors.Is(err, ErrInsecureEndpoint) {
		t.Fatalf("LoadClient error = %v, want ErrInsecureEndpoint", err)
	}
}

func TestResolveEndpointPrecedence(t *testing.T) {
	t.Setenv(EndpointEnv, "")
	if got, err := ResolveEndpoint(""); err != nil || got != DefaultEndpoint {
		t.Fatalf("default = %q, %v; want %q", got, err, DefaultEndpoint)
	}
	t.Setenv(EndpointEnv, "http://127.0.0.1:8787/")
	if got, err := ResolveEndpoint(""); err != nil || got != "http://127.0.0.1:8787" {
		t.Fatalf("env = %q, %v", got, err)
	}
	if got, err := ResolveEndpoint("https://staging.cloud.graycodeai.com"); err != nil || got != "https://staging.cloud.graycodeai.com" {
		t.Fatalf("flag = %q, %v; the flag must win over the environment", got, err)
	}
	t.Setenv(EndpointEnv, "http://cloud.graycodeai.com")
	_, err := ResolveEndpoint("")
	if !errors.Is(err, ErrInsecureEndpoint) || !strings.Contains(err.Error(), EndpointEnv) {
		t.Fatalf("insecure env error = %v, want ErrInsecureEndpoint naming %s", err, EndpointEnv)
	}
	if _, err := ResolveEndpoint("http://cloud.graycodeai.com"); err == nil || !strings.Contains(err.Error(), "--endpoint") {
		t.Fatalf("insecure flag error = %v, want it to name --endpoint", err)
	}
}
