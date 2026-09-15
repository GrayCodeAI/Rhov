package config

import (
	"context"

	"github.com/GrayCodeAI/rho/internal/provider/gateway"
)

type (
	EngineModel            = gateway.Model
	EnginePreflight        = gateway.PreflightReport
	EnginePreflightOptions = gateway.PreflightOptions
)

func newFluxEngine() (*gateway.Gateway, error) {
	return gateway.New(context.Background(), globalCustomProviders())
}

func NewFluxEngine() (*gateway.Gateway, error) { return newFluxEngine() }

// NewFluxEngineForSettings composes a fresh gateway for one effective
// Rho settings snapshot. It performs no package-global registration and does
// not mutate provider environment variables.
func NewFluxEngineForSettings(settings Settings) (*gateway.Gateway, error) {
	return gateway.New(context.Background(), gatewayCustomGateways(settings.CustomProviders))
}

func globalCustomProviders() []gateway.CustomProviderConfig {
	return gatewayCustomGateways(LoadGlobalSettings().CustomProviders)
}

func customGatewayProviders(providers []CustomProviderConfig) []gateway.CustomProviderConfig {
	out := make([]gateway.CustomProviderConfig, 0, len(providers))
	for _, provider := range providers {
		if provider.Name == "" && provider.BaseURL == "" {
			continue
		}
		out = append(out, gateway.CustomProviderConfig{
			Name: provider.Name, BaseURL: provider.BaseURL,
			APIKeyEnv: provider.APIKeyEnv, Model: provider.Model,
		})
	}
	return out
}

// gatewayCustomGateways converts config providers to gateway specs, reusing
// the shared conversion loop. A new CustomProviderConfig field only needs
// wiring in customGatewayProviders.
func gatewayCustomGateways(providers []CustomProviderConfig) []gateway.CustomProviderConfig {
	return customGatewayProviders(providers)
}

func CredentialStoreName() string { return gateway.SecretStoreName() }

func CredentialStorageStatus(ctx context.Context) gateway.CredentialStorageReport {
	return gateway.CredentialStorage(ctx)
}

// EnginePreflightReport runs preflight against the default gateway.
func EnginePreflightReport(ctx context.Context) EnginePreflight {
	return gateway.PreflightWithProviders(ctx, nil, EnginePreflightOptions{})
}

// EnginePreflightReportWithOptions runs preflight against the default
// gateway with explicit options. Kept for callers that need VerifyLive
// or other non-default preflight checks.
func EnginePreflightReportWithOptions(ctx context.Context, opts EnginePreflightOptions) EnginePreflight {
	return gateway.PreflightWithProviders(ctx, nil, opts)
}

// EnginePreflightReportWithSettings runs preflight against one invocation's
// effective settings (including its custom gateways). The settings' provider
// list is converted to the gateway spec at this boundary.
func EnginePreflightReportWithSettings(ctx context.Context, settings Settings, opts EnginePreflightOptions) EnginePreflight {
	return gateway.PreflightWithProviders(ctx, gatewayCustomGateways(settings.CustomProviders), opts)
}

func FormatEnginePreflight(report EnginePreflight) string {
	return gateway.FormatPreflight(report)
}

func EngineGatewayRegion(ctx context.Context, providerID string) (string, bool) {
	gw, err := newFluxEngine()
	if err != nil {
		return "", false
	}
	return gw.GatewayRegion(providerID)
}

func SetEngineGatewayRegion(ctx context.Context, providerID, region string) error {
	gw, err := newFluxEngine()
	if err != nil {
		return err
	}
	return gw.SetGatewayRegion(ctx, providerID, region)
}

func CanonicalModelID(ctx context.Context, modelID string) string {
	gw, err := newFluxEngine()
	if err != nil {
		return modelID
	}
	return gw.CanonicalModel(ctx, modelID)
}

func HasCredentialEnv(ctx context.Context, envVar string) bool {
	gw, err := newFluxEngine()
	return err == nil && gw.HasCredentialEnv(ctx, envVar)
}

func CredentialGuidance(providerID, secret string) string {
	return gateway.CredentialGuidance(providerID, secret)
}

func ProviderStateSecurityStatus() gateway.ProviderStateSecurity {
	gw, err := newFluxEngine()
	if err != nil {
		return gateway.ProviderStateSecurity{Error: err.Error(), Detail: "Flux engine initialization failed"}
	}
	return gw.ProviderStateSecurityStatus()
}

func EngineDeploymentSummary(ctx context.Context, model string) (gateway.DeploymentSummary, error) {
	gw, err := newFluxEngine()
	if err != nil {
		return gateway.DeploymentSummary{}, err
	}
	return gw.DeploymentSummary(ctx, model)
}

// newFluxEngine is Rho's default composition root for Flux's stable host
// facade. Command paths that support --settings must use
// NewFluxEngineForSettings instead of relying on this global-settings default.

// ListEngineModels returns model-picker rows through Flux's stable facade.
// EngineModel is an alias of gateway.ModelInfo, so model lists returned by the
// gateway pass through without conversion.
func ListEngineModels(ctx context.Context, providerID string, refresh bool) ([]EngineModel, error) {
	gw, err := newFluxEngine()
	if err != nil {
		return nil, err
	}
	return gw.ListModels(ctx, providerID, refresh)
}

func ListEngineModelsWithSettings(ctx context.Context, settings Settings, providerID string, refresh bool) ([]EngineModel, error) {
	gw, err := NewFluxEngineForSettings(settings)
	if err != nil {
		return nil, err
	}
	return gw.ListModels(ctx, providerID, refresh)
}

func ListLiveEngineModelsWithSettings(ctx context.Context, settings Settings, providerID string) ([]EngineModel, error) {
	gw, err := NewFluxEngineForSettings(settings)
	if err != nil {
		return nil, err
	}
	return gw.ListLiveModels(ctx, providerID)
}

func ListPublicEngineModels(ctx context.Context, providerID string) ([]EngineModel, error) {
	gw, err := newFluxEngine()
	if err != nil {
		return nil, err
	}
	return gw.ListPublicModels(ctx, providerID)
}
