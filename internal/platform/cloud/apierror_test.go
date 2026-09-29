package cloud

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"
)

func respondWith(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func TestDeviceLoginStartSurfacesJSONError(t *testing.T) {
	server := httptest.NewServer(respondWith(http.StatusBadRequest, `{"error":"Invalid device authorization request"}`))
	defer server.Close()
	_, err := New(Config{Endpoint: server.URL}).StartDeviceLogin(context.Background(), "laptop", "darwin", "0.3.0")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("error = %#v, want *APIError with status 400", err)
	}
	if !strings.Contains(err.Error(), "Invalid device authorization request") || !strings.Contains(err.Error(), "HTTP 400") {
		t.Fatalf("error = %q, want the server message and status", err)
	}
}

func TestAPIErrorFallsBackToStatusForNonJSONBody(t *testing.T) {
	server := httptest.NewServer(respondWith(http.StatusBadGateway, `<html>bad gateway</html>`))
	defer server.Close()
	_, err := New(Config{Endpoint: server.URL}).StartDeviceLogin(context.Background(), "laptop", "darwin", "0.3.0")
	if err == nil || !strings.Contains(err.Error(), "HTTP 502 Bad Gateway") || strings.Contains(err.Error(), "<html>") {
		t.Fatalf("error = %v, want the HTTP status without the HTML body", err)
	}
}

func TestAPIErrorReadIsBounded(t *testing.T) {
	huge := `{"error":"` + strings.Repeat("A", 4*maxErrorBodyBytes) + `"}`
	server := httptest.NewServer(respondWith(http.StatusBadRequest, huge))
	defer server.Close()
	_, err := New(Config{Endpoint: server.URL}).StartDeviceLogin(context.Background(), "laptop", "darwin", "0.3.0")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v, want *APIError", err)
	}
	// Only maxErrorBodyBytes are read, so the truncated JSON is not decoded.
	if apiErr.Message != "" || len(err.Error()) > 200 {
		t.Fatalf("unbounded error body leaked into the message (%d bytes)", len(err.Error()))
	}
}

func TestSanitizeServerTextStripsControlsAndCaps(t *testing.T) {
	got := sanitizeServerText("  \x1b[31mred\x1b[0m\n\tline​ two  ")
	if strings.ContainsAny(got, "\x1b\n\t​") || got != "[31mred[0m line two" {
		t.Fatalf("sanitized = %q", got)
	}
	long := sanitizeServerText(strings.Repeat("é", 2*maxServerTextRunes))
	if n := utf8.RuneCountInString(long); n != maxServerTextRunes+1 || !strings.HasSuffix(long, "…") {
		t.Fatalf("capped length = %d runes (%q...), want %d plus an ellipsis", n, long[:10], maxServerTextRunes)
	}
}

func TestPollDeviceLoginTerminalStates(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"consumed 409", http.StatusConflict, `{"status":"consumed"}`, ErrDeviceLoginConsumed},
		{"consumed 200", http.StatusOK, `{"status":"consumed"}`, ErrDeviceLoginConsumed},
		{"expired", http.StatusOK, `{"status":"expired"}`, ErrDeviceLoginExpired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(respondWith(tc.status, tc.body))
			defer server.Close()
			_, err := New(Config{Endpoint: server.URL}).PollDeviceLogin(context.Background(), "device-code-0123456789")
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
			if !strings.Contains(err.Error(), "rho cloud login") {
				t.Fatalf("error = %q, want a hint to rerun rho cloud login", err)
			}
		})
	}
}

func TestPollDeviceLoginNonTerminalAndUnknownStates(t *testing.T) {
	server := httptest.NewServer(respondWith(http.StatusOK, `{"status":"pending"}`))
	defer server.Close()
	poll, err := New(Config{Endpoint: server.URL}).PollDeviceLogin(context.Background(), "device-code-0123456789")
	if err != nil || poll.Status != DeviceLoginPending {
		t.Fatalf("pending poll = %+v, %v", poll, err)
	}

	unknown := httptest.NewServer(respondWith(http.StatusOK, "{\"status\":\"sl\\u001bow\"}"))
	defer unknown.Close()
	_, err = New(Config{Endpoint: unknown.URL}).PollDeviceLogin(context.Background(), "device-code-0123456789")
	if err == nil || strings.Contains(err.Error(), "\x1b") || !strings.Contains(err.Error(), "unknown device login status") {
		t.Fatalf("unknown status error = %q", err)
	}

	conflict := httptest.NewServer(respondWith(http.StatusConflict, `{"error":"Something else"}`))
	defer conflict.Close()
	_, err = New(Config{Endpoint: conflict.URL}).PollDeviceLogin(context.Background(), "device-code-0123456789")
	var apiErr *APIError
	if errors.Is(err, ErrDeviceLoginConsumed) || !errors.As(err, &apiErr) || apiErr.Message != "Something else" {
		t.Fatalf("non-consumed 409 error = %v, want a plain *APIError", err)
	}
}

func TestAPIErrorUnauthorizedHint(t *testing.T) {
	err := (&APIError{Operation: "usage upload", StatusCode: http.StatusUnauthorized, Message: "Unauthorized"}).Error()
	if !strings.Contains(err, "rho cloud login") {
		t.Fatalf("401 error = %q, want a reconnect hint", err)
	}
}
