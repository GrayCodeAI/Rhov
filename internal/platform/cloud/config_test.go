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
	"testing"
)

// fakeTokenStore is an in-memory secretStore.
type fakeTokenStore struct {
	tokens map[string]string
	getErr error
}

func (f *fakeTokenStore) Get(account string) (string, error) {
	if f.getErr != nil {
		return "", f.getErr
	}
	token, ok := f.tokens[account]
	if !ok {
		return "", os.ErrNotExist
	}
	return token, nil
}

func (f *fakeTokenStore) Set(account, token string) error {
	f.tokens[account] = token
	return nil
}

// useFakeTokenStore isolates the config dir and token store for one test.
func useFakeTokenStore(t *testing.T) (*fakeTokenStore, string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("RHO_CONFIG_DIR", dir)
	store := &fakeTokenStore{tokens: map[string]string{}}
	original := newTokenStore
	t.Cleanup(func() { newTokenStore = original })
	newTokenStore = func() secretStore { return store }
	return store, dir
}

func writeCloudJSON(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "cloud.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

const validCloudJSON = `{"endpoint":"https://cloud.graycodeai.com","device_id":"device_0123456789","project_id":"project_0123456789"}`

func TestLoadClientNotConnected(t *testing.T) {
	useFakeTokenStore(t)
	if _, _, err := LoadClient(); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("error = %v, want ErrNotConnected", err)
	}
}

func TestLoadClientReportsBrokenConnections(t *testing.T) {
	cases := map[string]struct {
		cloudJSON string
		token     string
		getErr    error
		want      string
	}{
		"corrupt config":    {cloudJSON: `{`, want: "read GrayCode Cloud configuration"},
		"incomplete config": {cloudJSON: `{"endpoint":"https://cloud.graycodeai.com"}`, want: "incomplete"},
		"missing token":     {cloudJSON: validCloudJSON, want: "device token is missing"},
		"blank token":       {cloudJSON: validCloudJSON, token: " \n", want: "device token is missing"},
		"keychain failure":  {cloudJSON: validCloudJSON, getErr: errors.New("security: exit status 51: user interaction is not allowed"), want: "user interaction is not allowed"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			store, dir := useFakeTokenStore(t)
			writeCloudJSON(t, dir, tc.cloudJSON)
			if tc.token != "" {
				store.tokens[tokenAccount] = tc.token
			}
			store.getErr = tc.getErr
			_, _, err := LoadClient()
			if err == nil || errors.Is(err, ErrNotConnected) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want a non-ErrNotConnected error containing %q", err, tc.want)
			}
		})
	}
}

func TestSaveAndLoadClientRoundTripTrimsToken(t *testing.T) {
	store, _ := useFakeTokenStore(t)
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	if err := SaveDeviceConfig(DeviceConfig{Endpoint: server.URL + "/", DeviceID: "device_0123456789", ProjectID: "project_0123456789"}, "hwc_saved"); err != nil {
		t.Fatal(err)
	}
	// A credential helper that appends CRLF (the Windows reader does) must
	// not produce an invalid Authorization header.
	store.tokens[tokenAccount] = "hwc_saved\r\n"
	client, cfg, err := LoadClient()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Endpoint != server.URL {
		t.Fatalf("saved endpoint = %q, want normalized %q", cfg.Endpoint, server.URL)
	}
	if err := client.SendDeliveryContext(context.Background(), validDeliveryContext()); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer hwc_saved" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
}

func TestSavedConfigFileIsPrivate(t *testing.T) {
	_, dir := useFakeTokenStore(t)
	if err := SaveDeviceConfig(DeviceConfig{Endpoint: DefaultEndpoint, DeviceID: "device_0123456789", ProjectID: "project_0123456789"}, "hwc_saved"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "cloud.json"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("cloud.json mode = %o, want 600", perm)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "cloud.json"))
	var saved map[string]string
	if err := json.Unmarshal(raw, &saved); err != nil || strings.Contains(string(raw), "hwc_saved") {
		t.Fatalf("cloud.json = %s (err %v); the token must live only in the credential store", raw, err)
	}
}
