package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rhoconfig "github.com/GrayCodeAI/rho/internal/config"
	"github.com/GrayCodeAI/rho/internal/provider/gateway"
)

func isolateCredentialHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	rhoDir := filepath.Join(home, ".rho")
	_ = os.MkdirAll(rhoDir, 0o700)
	t.Setenv("HOME", home)
	t.Setenv("RHO_CONFIG_DIR", rhoDir)
	t.Setenv("EYRIE_CONFIG_DIR", filepath.Join(home, "eyrie"))
}

func TestEffectiveModelAndProvider_ClearsWithoutCredentials(t *testing.T) {
	rhoconfig.InvalidateConfigUICache()
	isolateCredentialHome(t)
	store := &gateway.MapStore{}
	gateway.SetDefaultStore(store)
	t.Cleanup(func() {
		gateway.SetDefaultStore(nil)
		rhoconfig.InvalidateConfigUICache()
	})

	ctx := context.Background()
	if err := rhoconfig.SetActiveProvider(ctx, "openrouter"); err != nil {
		t.Fatal(err)
	}
	if err := rhoconfig.SetActiveModel(ctx, "gpt-4o"); err != nil {
		t.Fatal(err)
	}

	model, provider := effectiveModelAndProvider(rhoconfig.Settings{})
	if model != "" || provider != "" {
		t.Fatalf("expected empty selection without credentials, got model=%q provider=%q", model, provider)
	}
}

func TestEffectiveModelAndProvider_KeepsWithCredentials(t *testing.T) {
	rhoconfig.InvalidateConfigUICache()
	isolateCredentialHome(t)
	store := &gateway.MapStore{}
	gateway.SetDefaultStore(store)
	t.Cleanup(func() {
		gateway.SetDefaultStore(nil)
		rhoconfig.InvalidateConfigUICache()
	})

	ctx := context.Background()
	_ = store.Set(ctx, gateway.AccountForEnv("OPENROUTER_API_KEY"), "sk-or-test-key-1234567890")
	rhoconfig.InvalidateConfigUICache()
	if err := rhoconfig.SetActiveProvider(ctx, "openrouter"); err != nil {
		t.Fatal(err)
	}
	if err := rhoconfig.SetActiveModel(ctx, "gpt-4o"); err != nil {
		t.Fatal(err)
	}

	model, provider := effectiveModelAndProvider(rhoconfig.Settings{})
	if provider == "" {
		t.Fatalf("expected provider with credentials, got model=%q provider=%q", model, provider)
	}
	if strings.TrimSpace(model) == "" {
		t.Fatalf("expected model preserved, got model=%q provider=%q", model, provider)
	}
}
