package config

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GrayCodeAI/flux/catalog"
	fluxcfg "github.com/GrayCodeAI/flux/config"
	"github.com/GrayCodeAI/flux/credentials"
)

// isolateMilestoneTest uses a temp HOME and RHO_CONFIG_DIR so verification does not touch the user machine.
func isolateMilestoneTest(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	rhoDir := filepath.Join(home, ".rho")
	if err := os.MkdirAll(rhoDir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("RHO_CONFIG_DIR", rhoDir)
	t.Setenv("FLUX_CONFIG_DIR", rhoDir)
	return rhoDir
}

func TestVerify_ProviderJSONOnDiskHasNoSecrets(t *testing.T) {
	isolateMilestoneTest(t)
	compiled := CompiledCatalogV1()
	if compiled == nil {
		t.Fatal("compiled catalog required")
	}
	env := map[string]string{"ANTHROPIC_API_KEY": "sk-ant-verify-test-key-1234567890"}
	cfg := fluxcfg.SyncProviderConfigFromCatalog(compiled, env)
	path, err := fluxcfg.GetProviderConfigPath()
	if err != nil {
		t.Fatalf("GetProviderConfigPath: %v", err)
	}
	if err := fluxcfg.SaveProviderConfig(cfg, path); err != nil {
		t.Fatal(err)
	}
	assertProviderJSONFileHasNoSecrets(t, path)
}

func TestVerify_PersistAPIKeyDoesNotWriteProviderJSON(t *testing.T) {
	rhoDir := isolateMilestoneTest(t)
	credentials.SetDefaultStore(emptyCredentialStore{})
	t.Cleanup(func() { credentials.SetDefaultStore(nil) })

	secret := "sk-ant-persist-verify-key-1234567890"
	if err := PersistAPIKey(context.Background(), "ANTHROPIC_API_KEY", secret); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(rhoDir, "provider.json")
	if _, err := os.Stat(path); err == nil {
		data, _ := os.ReadFile(path)
		if strings.Contains(string(data), secret) {
			t.Fatal("PersistAPIKey must not write secrets to provider.json")
		}
	}
}

func TestVerify_EvaluateSetupFlow(t *testing.T) {
	InvalidateConfigUICache()
	isolateMilestoneTest(t)
	store := &credentials.MapStore{}
	credentials.SetDefaultStore(store)
	t.Cleanup(func() {
		credentials.SetDefaultStore(nil)
		InvalidateConfigUICache()
	})

	ctx := context.Background()
	compiled := CompiledCatalogV1()
	if compiled != nil {
		for _, k := range catalog.DiscoveryEnvKeysFromCatalog(compiled) {
			t.Setenv(k, "")
		}
	}

	st := EvaluateSetup(ctx)
	if !st.NeedsSetup || st.HasCredentials {
		t.Fatalf("expected setup needed without credentials, got %+v", st)
	}

	secret := "sk-ant-flow-verify-key-1234567890"
	if err := store.Set(ctx, credentials.AccountForEnv("ANTHROPIC_API_KEY"), secret); err != nil {
		t.Fatal(err)
	}
	InvalidateConfigUICache()
	st = EvaluateSetup(ctx)
	if !st.HasCredentials {
		t.Fatal("expected credentials after keychain key set")
	}
	if !st.NeedsSetup || st.HasModel {
		t.Fatal("expected setup still needed until model selected")
	}

	providerPath := filepath.Join(os.Getenv("HOME"), ".rho", "provider.json")
	cfg := &fluxcfg.ProviderConfig{
		ActiveProvider: "anthropic",
		ActiveModel:    "claude-sonnet-4-20250514",
		AnthropicModel: "claude-sonnet-4-20250514",
	}
	if err := fluxcfg.SaveProviderConfig(cfg, providerPath); err != nil {
		t.Fatal(err)
	}
	st = EvaluateSetup(ctx)
	if st.NeedsSetup {
		t.Fatalf("expected setup complete with key + model, got %+v", st)
	}
}

func assertProviderJSONFileHasNoSecrets(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, needle := range []string{`"api_key"`, `"secret_access_key"`, `"session_token"`} {
		if !strings.Contains(text, needle) {
			continue
		}
		// Empty values are OK: "api_key": ""
		if strings.Contains(text, needle+`": ""`) || strings.Contains(text, needle+`":""`) {
			continue
		}
		if strings.Contains(text, needle+`": "`) && !strings.Contains(text, needle+`": ""`) {
			t.Fatalf("provider.json at %s contains non-empty %s", path, needle)
		}
	}
	var cfg fluxcfg.ProviderConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	for id, dep := range cfg.Deployments {
		if deploymentHasSecrets(dep) {
			t.Fatalf("deployment %q still has secret fields in struct", id)
		}
	}
}
