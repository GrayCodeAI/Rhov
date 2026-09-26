package cloud

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/GrayCodeAI/rho/internal/auth"
	"github.com/GrayCodeAI/rho/internal/storage"
)

const (
	tokenService = "graycode-cloud"
	tokenAccount = "device-token"
)

// ErrNotConnected means this device has no saved GrayCode Cloud connection.
// Any other LoadClient error means a connection was saved but cannot be used.
var ErrNotConnected = errors.New("GrayCode Cloud is not connected; run `rho cloud login`")

// secretStore is the part of auth.SecureStorage the device token needs.
type secretStore interface {
	Get(account string) (string, error)
	Set(account, token string) error
}

// newTokenStore returns the device-token store; tests replace it so they
// never touch the real OS keychain.
var newTokenStore = func() secretStore { return auth.NewSecureStorage(tokenService) }

type DeviceConfig struct {
	Endpoint  string `json:"endpoint"`
	DeviceID  string `json:"device_id"`
	ProjectID string `json:"project_id"`
}

func configPath() string { return filepath.Join(storage.ConfigDir(), "cloud.json") }

func LoadDeviceConfig() (DeviceConfig, error) {
	var cfg DeviceConfig
	b, err := os.ReadFile(configPath()) // #nosec G304 -- fixed Rho config path
	if err != nil {
		return cfg, err
	}
	return cfg, json.Unmarshal(b, &cfg)
}

func SaveDeviceConfig(cfg DeviceConfig, token string) error {
	endpoint, err := NormalizeEndpoint(cfg.Endpoint)
	if err != nil {
		return err
	}
	cfg.Endpoint = endpoint
	if err := newTokenStore().Set(tokenAccount, token); err != nil {
		return err
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(configPath()), 0o700); err != nil {
		return err
	}
	return os.WriteFile(configPath(), b, 0o600)
}

// LoadClient returns a client for the saved connection. It returns
// ErrNotConnected when nothing was saved, and a descriptive error when the
// saved configuration or device token cannot be used, so callers can tell
// "never connected" apart from a broken connection.
func LoadClient() (*Client, DeviceConfig, error) {
	cfg, err := LoadDeviceConfig()
	if errors.Is(err, os.ErrNotExist) {
		return nil, cfg, ErrNotConnected
	}
	if err != nil {
		return nil, cfg, fmt.Errorf("read GrayCode Cloud configuration %s: %w", configPath(), err)
	}
	if cfg.Endpoint == "" || cfg.DeviceID == "" || cfg.ProjectID == "" {
		return nil, cfg, fmt.Errorf("GrayCode Cloud configuration %s is incomplete (endpoint, device ID and project ID are required); run `rho cloud login` again", configPath())
	}
	// Refuse a saved plaintext endpoint before the token is even read, so a
	// tampered or pre-TLS-check cloud.json cannot route the token over HTTP.
	if _, err := NormalizeEndpoint(cfg.Endpoint); err != nil {
		return nil, cfg, fmt.Errorf("saved GrayCode Cloud endpoint in %s is not allowed: %w; run `rho cloud login` again", configPath(), err)
	}
	token, err := newTokenStore().Get(tokenAccount)
	// Credential helpers can return a trailing newline (the Windows
	// PowerShell reader does), which would make an invalid header value.
	token = strings.TrimSpace(token)
	if errors.Is(err, os.ErrNotExist) || (err == nil && token == "") {
		return nil, cfg, errors.New("GrayCode Cloud device token is missing from the credential store; run `rho cloud login` again")
	}
	if err != nil {
		return nil, cfg, fmt.Errorf("read GrayCode Cloud device token from the credential store: %w", err)
	}
	return New(Config{Endpoint: cfg.Endpoint, DeviceToken: token}), cfg, nil
}
