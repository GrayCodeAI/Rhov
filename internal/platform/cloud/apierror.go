package cloud

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode"
)

const (
	// maxErrorBodyBytes bounds how much of a non-2xx response body is read.
	maxErrorBodyBytes = 16 << 10
	// maxResponseBytes bounds how much of a successful response is decoded.
	maxResponseBytes = 64 << 10
	// maxServerTextRunes bounds server-provided text shown to the user.
	maxServerTextRunes = 300
)

// APIError is a non-2xx response from GrayCode Cloud. Message, Code and State
// carry the JSON "error", "code" and "status" fields of the response body
// when the server sent them, already sanitized for terminal output.
type APIError struct {
	Operation  string
	StatusCode int
	Message    string
	Code       string
	State      string
}

func (e *APIError) Error() string {
	status := fmt.Sprintf("HTTP %d %s", e.StatusCode, http.StatusText(e.StatusCode))
	var msg string
	if e.Message != "" {
		msg = fmt.Sprintf("GrayCode Cloud %s failed: %s (%s)", e.Operation, e.Message, status)
	} else {
		msg = fmt.Sprintf("GrayCode Cloud %s failed: %s", e.Operation, status)
	}
	if e.StatusCode == http.StatusUnauthorized {
		msg += "; the device token was rejected or revoked, run `rho cloud login` to reconnect"
	}
	return msg
}

// readAPIError reads at most maxErrorBodyBytes of a non-2xx response and
// returns it as an *APIError. A body that is not JSON (for example an HTML
// error page from a proxy) leaves the message empty so the HTTP status is
// reported instead.
func readAPIError(operation string, resp *http.Response) *APIError {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
	var payload struct {
		Error  string `json:"error"`
		Code   string `json:"code"`
		Status string `json:"status"`
	}
	_ = json.Unmarshal(body, &payload)
	return &APIError{
		Operation:  operation,
		StatusCode: resp.StatusCode,
		Message:    sanitizeServerText(payload.Error),
		Code:       sanitizeServerText(payload.Code),
		State:      sanitizeServerText(payload.Status),
	}
}

// decodeResponse decodes at most maxResponseBytes of a successful response.
func decodeResponse(operation string, resp *http.Response, dst any) error {
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(dst); err != nil {
		return fmt.Errorf("decode GrayCode Cloud %s response: %w", operation, err)
	}
	return nil
}

// sanitizeServerText makes server-controlled text safe to print: control and
// other non-printable characters (including terminal escape sequences) are
// dropped, whitespace is collapsed, and the result is capped.
func sanitizeServerText(text string) string {
	var b strings.Builder
	runes := 0
	space := false
	for _, r := range text {
		if unicode.IsSpace(r) {
			space = b.Len() > 0
			continue
		}
		if !unicode.IsPrint(r) {
			continue
		}
		width := 1
		if space {
			width = 2
		}
		if runes+width > maxServerTextRunes {
			b.WriteString("…")
			break
		}
		if space {
			b.WriteByte(' ')
			space = false
		}
		b.WriteRune(r)
		runes += width
	}
	return b.String()
}
