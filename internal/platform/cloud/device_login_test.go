package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf16"
)

const validStartResponse = `{"deviceCode":"dc_0123456789abcdefghij","userCode":"ABCD-EFGH","verificationUri":"https://graycodeai.com/cli/approve","expiresIn":600,"interval":5}`

// captureStart returns a server that records the raw device-start body.
func captureStart(t *testing.T, response string, body *[]byte) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/auth/device/start" || r.Method != http.MethodPost {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("device start must not carry a device token")
		}
		raw, _ := io.ReadAll(r.Body)
		*body = raw
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(server.Close)
	return server
}

func TestStartDeviceLoginSendsGraycodeVersion(t *testing.T) {
	var raw []byte
	server := captureStart(t, validStartResponse, &raw)
	start, err := New(Config{Endpoint: server.URL}).StartDeviceLogin(context.Background(), "build-host", "linux", "0.3.0")
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		Label           string `json:"label"`
		Platform        string `json:"platform"`
		GraycodeVersion string `json:"graycodeVersion"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		t.Fatalf("device start body %s violates the strict schema: %v", raw, err)
	}
	if body.Label != "build-host" || body.Platform != "linux" || body.GraycodeVersion != "0.3.0" {
		t.Fatalf("body = %+v", body)
	}
	if bytes.Contains(raw, []byte("rhoVersion")) {
		t.Fatalf("body still sends rhoVersion: %s", raw)
	}
	if start.UserCode != "ABCD-EFGH" || start.Interval != 5 {
		t.Fatalf("start = %+v", start)
	}
}

func TestStartDeviceLoginBoundsFields(t *testing.T) {
	var raw []byte
	server := captureStart(t, validStartResponse, &raw)
	longLabel := strings.Repeat("\U0001F600", 60) // 120 UTF-16 code units
	if _, err := New(Config{Endpoint: server.URL}).StartDeviceLogin(context.Background(), longLabel, "", strings.Repeat("9", 80)); err != nil {
		t.Fatal(err)
	}
	var body map[string]string
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if n := len(utf16.Encode([]rune(body["label"]))); n == 0 || n > maxDeviceLabel {
		t.Fatalf("label is %d UTF-16 units, want 1..%d", n, maxDeviceLabel)
	}
	if body["platform"] != "unknown" {
		t.Fatalf("empty platform = %q, want fallback", body["platform"])
	}
	if n := len(body["graycodeVersion"]); n != maxGraycodeVersion {
		t.Fatalf("graycodeVersion length = %d, want %d", n, maxGraycodeVersion)
	}

	if _, err := New(Config{Endpoint: server.URL}).StartDeviceLogin(context.Background(), " \t\x00 ", "darwin", ""); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if body["label"] != "rho" || body["graycodeVersion"] != "dev" {
		t.Fatalf("blank fields = %+v, want fallbacks", body)
	}
}

func TestStartDeviceLoginRejectsUnsafeResponses(t *testing.T) {
	cases := map[string]string{
		"plaintext remote uri": `{"deviceCode":"dc_0123456789abcdefghij","userCode":"ABCD-EFGH","verificationUri":"http://graycodeai.com/cli/approve","expiresIn":600,"interval":5}`,
		"non-web uri":          `{"deviceCode":"dc_0123456789abcdefghij","userCode":"ABCD-EFGH","verificationUri":"file:///etc/passwd","expiresIn":600,"interval":5}`,
		"user code injection":  `{"deviceCode":"dc_0123456789abcdefghij","userCode":"AB&x=1","verificationUri":"https://graycodeai.com/cli/approve","expiresIn":600,"interval":5}`,
		"missing device code":  `{"userCode":"ABCD-EFGH","verificationUri":"https://graycodeai.com/cli/approve","expiresIn":600,"interval":5}`,
	}
	for name, response := range cases {
		t.Run(name, func(t *testing.T) {
			var raw []byte
			server := captureStart(t, response, &raw)
			if _, err := New(Config{Endpoint: server.URL}).StartDeviceLogin(context.Background(), "host", "linux", "0.3.0"); err == nil {
				t.Fatal("unsafe device start response was accepted")
			}
		})
	}
}

func TestApprovalURLEscapesCodeAndKeepsPath(t *testing.T) {
	got, err := DeviceLoginStart{UserCode: "ABCD-EFGH", VerificationURI: "https://graycodeai.com/cli/approve"}.ApprovalURL()
	if err != nil || got != "https://graycodeai.com/cli/approve?code=ABCD-EFGH" {
		t.Fatalf("ApprovalURL = %q, %v", got, err)
	}
	got, err = DeviceLoginStart{UserCode: "WXYZ-2345", VerificationURI: "http://localhost:3000/cli/approve?ref=cli#x"}.ApprovalURL()
	if err != nil || got != "http://localhost:3000/cli/approve?code=WXYZ-2345&ref=cli" {
		t.Fatalf("loopback ApprovalURL = %q, %v", got, err)
	}
	if _, err := (DeviceLoginStart{UserCode: "ABCD-EFGH", VerificationURI: "http://graycodeai.com/cli/approve"}).ApprovalURL(); !errors.Is(err, ErrInsecureEndpoint) {
		t.Fatalf("plaintext ApprovalURL error = %v", err)
	}
}

func TestPollDeviceLoginRejectsIncompleteApproval(t *testing.T) {
	for name, body := range map[string]string{
		"no token":         `{"status":"approved","deviceId":"d_0123456789abcdef","projectId":"p_0123456789abcdef","principalId":"u1"}`,
		"no device":        `{"status":"approved","token":"hwc_abc","projectId":"p_0123456789abcdef","principalId":"u1"}`,
		"token whitespace": `{"status":"approved","token":"hwc_abc\n","deviceId":"d_0123456789abcdef","projectId":"p_0123456789abcdef","principalId":"u1"}`,
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(respondWith(http.StatusOK, body))
			defer server.Close()
			if _, err := New(Config{Endpoint: server.URL}).PollDeviceLogin(context.Background(), "dc_0123456789abcdefghij"); err == nil {
				t.Fatal("incomplete approval accepted")
			}
		})
	}
	server := httptest.NewServer(respondWith(http.StatusOK, `{"status":"approved","token":"hwc_abc","deviceId":"d_0123456789abcdef","projectId":"p_0123456789abcdef","principalId":"u1"}`))
	defer server.Close()
	poll, err := New(Config{Endpoint: server.URL}).PollDeviceLogin(context.Background(), "dc_0123456789abcdefghij")
	if err != nil || poll.Token != "hwc_abc" || poll.ProjectID != "p_0123456789abcdef" {
		t.Fatalf("approved poll = %+v, %v", poll, err)
	}
}
