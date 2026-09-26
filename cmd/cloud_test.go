package cmd

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/GrayCodeAI/rho/internal/executiongraph"
	cloud "github.com/GrayCodeAI/rho/internal/platform/cloud"
	"github.com/spf13/cobra"
)

// runCloudCommand executes a freshly constructed cloud subcommand with args
// and returns its combined output.
func runCloudCommand(t *testing.T, command *cobra.Command, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	command.SetOut(&out)
	command.SetErr(&out)
	command.SetArgs(args)
	command.SilenceUsage = true
	command.SilenceErrors = true
	err := command.Execute()
	return out.String(), err
}

func TestCloudLoginHelpNamesDefaultEndpoint(t *testing.T) {
	command := newCloudLoginCmd()
	if !strings.Contains(command.Long, cloud.DefaultEndpoint) {
		t.Fatalf("login help does not name %s:\n%s", cloud.DefaultEndpoint, command.Long)
	}
	usage := command.Flags().Lookup("endpoint").Usage
	if !strings.Contains(usage, cloud.DefaultEndpoint) || !strings.Contains(usage, cloud.EndpointEnv) {
		t.Fatalf("--endpoint usage = %q, want default and %s", usage, cloud.EndpointEnv)
	}
}

func TestCloudLoginUsesRHOCloudURL(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/auth/device/start" {
			hits.Add(1)
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	t.Setenv(cloud.EndpointEnv, server.URL)
	if _, err := runCloudCommand(t, newCloudLoginCmd(), "--label", "ci"); err == nil {
		t.Fatal("login succeeded against a failing server")
	}
	if hits.Load() != 1 {
		t.Fatalf("device start hits = %d, want 1 (RHO_CLOUD_URL was not used)", hits.Load())
	}
}

func TestCloudLoginRejectsPlaintextEndpoint(t *testing.T) {
	t.Setenv(cloud.EndpointEnv, "")
	_, err := runCloudCommand(t, newCloudLoginCmd(), "--endpoint", "http://cloud.graycodeai.com")
	if !errors.Is(err, cloud.ErrInsecureEndpoint) {
		t.Fatalf("error = %v, want ErrInsecureEndpoint", err)
	}
}

func TestCloudConnectResolvesEndpointFromEnvironment(t *testing.T) {
	t.Setenv(cloud.EndpointEnv, "http://cloud.graycodeai.com")
	_, err := runCloudCommand(t, newCloudConnectCmd(), "--device-id", "device_0123456789", "--project-id", "project_0123456789")
	if !errors.Is(err, cloud.ErrInsecureEndpoint) || !strings.Contains(err.Error(), cloud.EndpointEnv) {
		t.Fatalf("error = %v, want the insecure %s to be refused", err, cloud.EndpointEnv)
	}

	t.Setenv(cloud.EndpointEnv, "")
	_, err = runCloudCommand(t, newCloudConnectCmd())
	if err == nil || strings.Contains(err.Error(), "endpoint") || !strings.Contains(err.Error(), "device-id") {
		t.Fatalf("error = %v, want only the missing device-id/project-id/token (endpoint has a default)", err)
	}
}

// useCloudClient points the cloud commands at client/cfg for one test.
func useCloudClient(t *testing.T, client *cloud.Client, cfg cloud.DeviceConfig, err error) {
	t.Helper()
	original := loadCloudClient
	t.Cleanup(func() { loadCloudClient = original })
	loadCloudClient = func() (*cloud.Client, cloud.DeviceConfig, error) { return client, cfg, err }
}

// noGitRemote makes git context detection fail so tests pass --repository.
func noGitRemote(t *testing.T) {
	t.Helper()
	original := runCloudGit
	t.Cleanup(func() { runCloudGit = original })
	runCloudGit = func(context.Context, ...string) (string, error) { return "", errors.New("no git") }
	t.Setenv("GITHUB_RUN_ID", "")
	t.Setenv("GITHUB_WORKFLOW", "")
}

func TestCloudContextReportsWorkerRejection(t *testing.T) {
	noGitRemote(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"Device not found"}`))
	}))
	defer server.Close()
	useCloudClient(t, cloud.New(cloud.Config{Endpoint: server.URL, DeviceToken: "hwc_test"}), cloud.DeviceConfig{ProjectID: "project_0123456789"}, nil)

	out, err := runCloudCommand(t, newCloudContextCmd(), "--repository", "GrayCodeAI/rho")
	if err == nil || !strings.Contains(err.Error(), "Device not found") {
		t.Fatalf("error = %v, want the Worker's rejection", err)
	}
	if strings.Contains(out, "synced") || strings.Contains(out, "queued") {
		t.Fatalf("output claims success after a rejection: %q", out)
	}
}

func TestCloudContextReportsSuccessOnlyWhenAccepted(t *testing.T) {
	noGitRemote(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"accepted":true}`))
	}))
	defer server.Close()
	useCloudClient(t, cloud.New(cloud.Config{Endpoint: server.URL, DeviceToken: "hwc_test"}), cloud.DeviceConfig{ProjectID: "project_0123456789"}, nil)

	out, err := runCloudCommand(t, newCloudContextCmd(), "--repository", "GrayCodeAI/rho", "--ci-run", "42", "--ci-status", "succeeded")
	if err != nil || !strings.Contains(out, "synced to GrayCode Cloud") {
		t.Fatalf("out = %q, err = %v", out, err)
	}
}

func TestCloudContextRejectsUnknownStatusFlags(t *testing.T) {
	noGitRemote(t)
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	useCloudClient(t, cloud.New(cloud.Config{Endpoint: server.URL, DeviceToken: "hwc_test"}), cloud.DeviceConfig{ProjectID: "project_0123456789"}, nil)

	_, err := runCloudCommand(t, newCloudContextCmd(), "--repository", "GrayCodeAI/rho", "--ci-run", "42", "--ci-status", "passed")
	if err == nil || !strings.Contains(err.Error(), "--ci-status") {
		t.Fatalf("error = %v, want a --ci-status error", err)
	}
	_, err = runCloudCommand(t, newCloudContextCmd(), "--repository", "GrayCodeAI/rho", "--deployment", "d1", "--deployment-environment", "prod", "--deployment-status", "done")
	if err == nil || !strings.Contains(err.Error(), "--deployment-status") || !strings.Contains(err.Error(), "rolled_back") {
		t.Fatalf("error = %v, want a --deployment-status error listing rolled_back", err)
	}
	if hits.Load() != 0 {
		t.Fatalf("server received %d requests for invalid flags", hits.Load())
	}
}

func TestCloudStatusNotConnected(t *testing.T) {
	useCloudClient(t, nil, cloud.DeviceConfig{}, cloud.ErrNotConnected)
	out, err := runCloudCommand(t, newCloudStatusCmd())
	if err != nil || !strings.Contains(out, "not connected") {
		t.Fatalf("out = %q, err = %v", out, err)
	}
}

func TestCloudStatusReportsBrokenConnection(t *testing.T) {
	broken := errors.New("read GrayCode Cloud device token from the credential store: security: exit status 51")
	useCloudClient(t, nil, cloud.DeviceConfig{}, broken)
	out, err := runCloudCommand(t, newCloudStatusCmd())
	if !errors.Is(err, broken) {
		t.Fatalf("error = %v, want the load failure", err)
	}
	if strings.Contains(out, "not connected") {
		t.Fatalf("a broken connection was reported as not connected: %q", out)
	}
}

func TestCloudStatusConnected(t *testing.T) {
	cfg := cloud.DeviceConfig{Endpoint: cloud.DefaultEndpoint, DeviceID: "device_0123456789", ProjectID: "project_0123456789"}
	useCloudClient(t, cloud.New(cloud.Config{Endpoint: cfg.Endpoint, DeviceToken: "hwc_test"}), cfg, nil)
	out, err := runCloudCommand(t, newCloudStatusCmd())
	if err != nil || !strings.Contains(out, cloud.DefaultEndpoint) || !strings.Contains(out, "project_0123456789") {
		t.Fatalf("out = %q, err = %v", out, err)
	}
}

func TestExplicitCloudCommandsReportLoadErrors(t *testing.T) {
	noGitRemote(t)
	broken := errors.New("GrayCode Cloud configuration is incomplete")
	useCloudClient(t, nil, cloud.DeviceConfig{}, broken)
	if _, err := runCloudCommand(t, newCloudContextCmd(), "--repository", "GrayCodeAI/rho"); !errors.Is(err, broken) {
		t.Fatalf("context error = %v, want the load failure", err)
	}
	missionDir := t.TempDir()
	graph := `{"schema_version":"` + executiongraph.SchemaVersion + `","generated_at":"2026-09-27T12:00:00Z","scope":{},"nodes":[],"edges":[],"events":[]}`
	if err := os.WriteFile(filepath.Join(missionDir, "mission-graph.json"), []byte(graph), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := runCloudCommand(t, newCloudGraphCmd(), "sync", "--mission-dir", missionDir); !errors.Is(err, broken) {
		t.Fatalf("graph sync error = %v, want the load failure", err)
	}
}
