package config

import (
	"context"
	"fmt"
	"time"

	"github.com/GrayCodeAI/rho/internal/provider/gateway"
)

// ApplyCredentialsResult is Rho's UI-safe view of an Flux catalog/routing
// application. It intentionally excludes Flux setup/config implementation
// types from the product boundary.
type ApplyCredentialsResult struct {
	Catalog gateway.CatalogSnapshot
}

// ApplyFluxCredentialsForProvider refreshes live models and writes sanitized
// deployment routing after /config saves a key.
func ApplyFluxCredentialsForProvider(ctx context.Context, providerID string) (*ApplyCredentialsResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	engine, err := newFluxEngine()
	if err != nil {
		return nil, err
	}
	snapshot, err := engine.ApplyCredentials(ctx, providerID)
	if err != nil {
		return nil, err
	}
	_ = SaveProjectOrGlobalDeploymentRouting(true)
	return &ApplyCredentialsResult{Catalog: snapshot}, nil
}

// ApplyFluxCredentials refreshes all configured providers and writes
// sanitized deployment routing.
func ApplyFluxCredentials(ctx context.Context) (*ApplyCredentialsResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	engine, err := newFluxEngine()
	if err != nil {
		return nil, err
	}
	snapshot, err := engine.ApplyCredentials(ctx, "")
	if err != nil {
		return nil, err
	}
	_ = SaveProjectOrGlobalDeploymentRouting(true)
	return &ApplyCredentialsResult{Catalog: snapshot}, nil
}

// RefreshGatewayCatalog refreshes catalog state through the facade.
func RefreshGatewayCatalog(ctx context.Context, providerID string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	engine, err := newFluxEngine()
	if err != nil {
		return "", err
	}
	snapshot, err := engine.RefreshCatalog(ctx, providerID)
	if err != nil {
		return "", err
	}
	return formatCatalogSnapshot(snapshot), nil
}

func FormatApplyCredentialsSummary(result *ApplyCredentialsResult) string {
	if result == nil {
		return ""
	}
	return formatCatalogSnapshot(result.Catalog)
}

func formatCatalogSnapshot(snapshot gateway.CatalogSnapshot) string {
	if snapshot.CachePath == "" {
		return fmt.Sprintf("Catalog ready: %d models", len(snapshot.Models))
	}
	return fmt.Sprintf("Catalog ready: %d models → %s", len(snapshot.Models), snapshot.CachePath)
}
