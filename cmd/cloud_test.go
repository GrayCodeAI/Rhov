package cmd

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

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
