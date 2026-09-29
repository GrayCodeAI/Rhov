package cmd

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	cloud "github.com/GrayCodeAI/rho/internal/platform/cloud"
)

var testUsageConfig = cloud.DeviceConfig{Endpoint: "https://cloud.graycodeai.com", DeviceID: "device_0123456789", ProjectID: "project_0123456789"}

func buildTestUsage(cfg cloud.DeviceConfig) cloud.UsageEvent {
	return cloud.UsageEvent{EventID: "exec-1727400000000-0123456789abcdef", DeviceID: cfg.DeviceID, ProjectID: cfg.ProjectID, TokensUsed: 1}
}

func TestStartCloudUsageWaitsForDelivery(t *testing.T) {
	var received atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(150 * time.Millisecond) // slower than returning from the command
		received.Store(true)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	useCloudClient(t, cloud.New(cloud.Config{Endpoint: server.URL, DeviceToken: "hwc_test"}), testUsageConfig, nil)

	var stderr bytes.Buffer
	wait := startCloudUsage(&stderr, buildTestUsage)
	wait()
	if !received.Load() {
		t.Fatal("wait returned before the usage event reached GrayCode Cloud")
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected output: %q", stderr.String())
	}
}

func TestStartCloudUsageWaitIsBounded(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	defer close(release)
	useCloudClient(t, cloud.New(cloud.Config{Endpoint: server.URL, DeviceToken: "hwc_test"}), testUsageConfig, nil)
	original := cloudUsageWait
	t.Cleanup(func() { cloudUsageWait = original })
	cloudUsageWait = 100 * time.Millisecond

	var stderr bytes.Buffer
	began := time.Now()
	startCloudUsage(&stderr, buildTestUsage)()
	if elapsed := time.Since(began); elapsed > 2*time.Second {
		t.Fatalf("wait took %v; a hung endpoint must not hold the process open", elapsed)
	}
	if stderr.Len() != 0 {
		t.Fatalf("a transport timeout should stay silent, got %q", stderr.String())
	}
}

func TestStartCloudUsageReportsRejection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"Invalid usage event"}`))
	}))
	defer server.Close()
	useCloudClient(t, cloud.New(cloud.Config{Endpoint: server.URL, DeviceToken: "hwc_test"}), testUsageConfig, nil)

	var stderr bytes.Buffer
	startCloudUsage(&stderr, buildTestUsage)()
	if !strings.Contains(stderr.String(), "Invalid usage event") {
		t.Fatalf("stderr = %q, want the rejection reported", stderr.String())
	}
}

func TestStartCloudUsageSkipsWhenNotConnected(t *testing.T) {
	useCloudClient(t, nil, cloud.DeviceConfig{}, cloud.ErrNotConnected)
	built := false
	startCloudUsage(&bytes.Buffer{}, func(cloud.DeviceConfig) cloud.UsageEvent { built = true; return cloud.UsageEvent{} })()
	if built {
		t.Fatal("usage event built without a connection")
	}
	useCloudClient(t, nil, cloud.DeviceConfig{}, errors.New("broken keychain"))
	startCloudUsage(&bytes.Buffer{}, func(cloud.DeviceConfig) cloud.UsageEvent { built = true; return cloud.UsageEvent{} })()
	if built {
		t.Fatal("usage event built for a broken connection")
	}
}
