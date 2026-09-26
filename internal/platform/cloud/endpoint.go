package cloud

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
)

const (
	// DefaultEndpoint is the hosted GrayCode Cloud API. `rho cloud login` and
	// `rho cloud connect` use it when neither --endpoint nor EndpointEnv is
	// set. It is never applied to an existing connection: a saved device
	// token stays bound to the endpoint that issued it.
	DefaultEndpoint = "https://cloud.graycodeai.com"
	// EndpointEnv overrides DefaultEndpoint for new connections.
	EndpointEnv = "RHO_CLOUD_URL"
)

// ErrInsecureEndpoint reports a GrayCode Cloud URL that would send the device
// token or telemetry over plaintext HTTP to a non-loopback host.
var ErrInsecureEndpoint = errors.New("GrayCode Cloud endpoints must use https:// (plain http:// is allowed only for localhost, 127.0.0.1 and [::1])")

// ResolveEndpoint returns the validated endpoint for a new connection: the
// --endpoint flag value when set, else $RHO_CLOUD_URL, else DefaultEndpoint.
func ResolveEndpoint(flagValue string) (string, error) {
	raw, source := strings.TrimSpace(flagValue), "--endpoint"
	if raw == "" {
		raw, source = strings.TrimSpace(os.Getenv(EndpointEnv)), EndpointEnv
	}
	if raw == "" {
		return DefaultEndpoint, nil
	}
	endpoint, err := NormalizeEndpoint(raw)
	if err != nil {
		return "", fmt.Errorf("%s: %w", source, err)
	}
	return endpoint, nil
}

// NormalizeEndpoint validates a GrayCode Cloud API base URL and returns it
// without a trailing slash. It accepts https:// URLs, and http:// URLs only
// for the loopback hosts localhost, 127.0.0.1 and [::1] (any port) so a local
// development Worker still works. URLs with credentials, a query or a fragment
// are rejected because the client appends API paths to the base URL.
func NormalizeEndpoint(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("GrayCode Cloud endpoint is empty")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Opaque != "" || u.Hostname() == "" {
		return "", fmt.Errorf("GrayCode Cloud endpoint must be an absolute URL such as %s", DefaultEndpoint)
	}
	if u.User != nil {
		return "", errors.New("GrayCode Cloud endpoint must not contain credentials")
	}
	if u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", errors.New("GrayCode Cloud endpoint must not contain a query or fragment")
	}
	switch u.Scheme {
	case "https":
	case "http":
		if !isLoopbackHost(u.Hostname()) {
			return "", fmt.Errorf("%w: got http://%s", ErrInsecureEndpoint, u.Host)
		}
	default:
		return "", fmt.Errorf("%w: got scheme %q", ErrInsecureEndpoint, u.Scheme)
	}
	return strings.TrimRight(u.String(), "/"), nil
}

// isLoopbackHost reports whether host is one of the loopback names for which
// plaintext HTTP is acceptable. The list is deliberately explicit rather than
// resolving names, so a DNS entry cannot make a remote host look local.
func isLoopbackHost(host string) bool {
	return strings.EqualFold(host, "localhost") || host == "127.0.0.1" || host == "::1"
}
