package gateway

import (
	"context"
	"testing"

	fluxengine "github.com/GrayCodeAI/flux/engine"
	"github.com/GrayCodeAI/rho/internal/types"
)

// stubProvider proves the Provider interface is swappable: a test can inject a
// deterministic fake and drive the ChatClient adapter without constructing any
// Flux engine. This is the swappable-Engine guarantee Phase 3 delivers.
type stubProvider struct {
	resp *fluxengine.GenerateResponse
	err  error
}

func (s *stubProvider) Resolve(context.Context, fluxengine.SelectionRequest) (fluxengine.Route, error) {
	return fluxengine.Route{}, nil
}

func (s *stubProvider) Generate(context.Context, fluxengine.GenerateRequest) (*fluxengine.GenerateResponse, error) {
	return s.resp, s.err
}

func (s *stubProvider) Stream(context.Context, fluxengine.GenerateRequest) (fluxengine.EventStreamer, error) {
	return nil, nil
}

func (s *stubProvider) ListModels(context.Context, string, bool) ([]fluxengine.Model, error) {
	return nil, nil
}

func (s *stubProvider) ListLiveModels(context.Context, string) ([]fluxengine.Model, error) {
	return nil, nil
}

func (s *stubProvider) ListPublicModels(context.Context, string) ([]fluxengine.Model, error) {
	return nil, nil
}

func (s *stubProvider) ModelInfo(context.Context, string) (fluxengine.Model, bool, error) {
	return fluxengine.Model{}, false, nil
}
func (s *stubProvider) ModelProviders(context.Context) ([]string, error)    { return nil, nil }
func (s *stubProvider) DefaultModel(context.Context, string, string) string { return "" }
func (s *stubProvider) PreferredModel(context.Context, string, fluxengine.ModelClass, string) string {
	return ""
}

func (s *stubProvider) PreferredModels(context.Context, string, fluxengine.ModelClass, int) []string {
	return nil
}

func (s *stubProvider) ModelClassOf(context.Context, string) fluxengine.ModelClass {
	return ModelClassEconomical
}
func (s *stubProvider) ProviderForModel(context.Context, string) string { return "" }
func (s *stubProvider) PrimaryModel(context.Context) string             { return "" }
func (s *stubProvider) ModelNames(context.Context) []string             { return nil }
func (s *stubProvider) StatePaths() fluxengine.StatePaths {
	return fluxengine.StatePaths{}
}
func (s *stubProvider) DefaultProviderFilter(context.Context) string { return "" }
func (s *stubProvider) Catalog(context.Context) (fluxengine.CatalogSnapshot, error) {
	return fluxengine.CatalogSnapshot{}, nil
}

func (s *stubProvider) RefreshCatalog(context.Context, string) (fluxengine.CatalogSnapshot, error) {
	return fluxengine.CatalogSnapshot{}, nil
}

func (s *stubProvider) ApplyCredentials(context.Context, string) (fluxengine.CatalogSnapshot, error) {
	return fluxengine.CatalogSnapshot{}, nil
}

func (s *stubProvider) SaveCredential(context.Context, string, string) (fluxengine.CredentialStatus, error) {
	return fluxengine.CredentialStatus{}, nil
}
func (s *stubProvider) RemoveCredential(context.Context, string) error { return nil }
func (s *stubProvider) CredentialStatus(context.Context, string) (fluxengine.CredentialStatus, error) {
	return fluxengine.CredentialStatus{}, nil
}
func (s *stubProvider) SaveCredentialEnv(context.Context, string, string) error { return nil }
func (s *stubProvider) HasCredentialEnv(context.Context, string) bool           { return false }
func (s *stubProvider) CredentialEnvKeys(string) []string                       { return nil }
func (s *stubProvider) ResolveCredential(context.Context, string) fluxengine.CredentialResolution {
	return fluxengine.CredentialResolution{}
}

func (s *stubProvider) CredentialProviders(context.Context) []fluxengine.CredentialProvider {
	return nil
}

func (s *stubProvider) GatewayDefinitions() []fluxengine.Gateway { return nil }

func (s *stubProvider) Gateways(context.Context) []fluxengine.Gateway          { return nil }
func (s *stubProvider) GatewayRegion(string) (string, bool)                    { return "", false }
func (s *stubProvider) SetGatewayRegion(context.Context, string, string) error { return nil }
func (s *stubProvider) GatewayForModel(context.Context, string) string         { return "" }
func (s *stubProvider) CanonicalModel(context.Context, string) string          { return "" }
func (s *stubProvider) ApplyGatewayEnvironment(context.Context, string)        {}
func (s *stubProvider) DeploymentRoutingEnabled(*bool) bool                    { return false }
func (s *stubProvider) DeploymentStatus(context.Context, string) (string, error) {
	return "", nil
}

func (s *stubProvider) DeploymentSummary(context.Context, string) (fluxengine.DeploymentSummary, error) {
	return fluxengine.DeploymentSummary{}, nil
}
func (s *stubProvider) RoutingPreview(context.Context, string) (string, error) { return "", nil }
func (s *stubProvider) CatalogHealth(context.Context) fluxengine.CatalogHealth {
	return fluxengine.CatalogHealth{}
}

func (s *stubProvider) Preflight(context.Context) fluxengine.PreflightReport {
	return fluxengine.PreflightReport{}
}

func (s *stubProvider) PreflightWithOptions(context.Context, fluxengine.PreflightOptions) fluxengine.PreflightReport {
	return fluxengine.PreflightReport{}
}

func (s *stubProvider) ActiveSelection(context.Context) fluxengine.Route {
	return fluxengine.Route{}
}

func (s *stubProvider) EffectiveSelection(context.Context, fluxengine.SelectionOptions) fluxengine.Selection {
	return fluxengine.Selection{}
}
func (s *stubProvider) SetActiveProvider(context.Context, string) error    { return nil }
func (s *stubProvider) SetActiveModel(context.Context, string) error       { return nil }
func (s *stubProvider) SetSelection(context.Context, string, string) error { return nil }
func (s *stubProvider) ClearSelection(context.Context) error               { return nil }
func (s *stubProvider) ProviderStateSecurityStatus() fluxengine.ProviderStateSecurity {
	return fluxengine.ProviderStateSecurity{}
}

func (s *stubProvider) SupportsNativeCompaction(context.Context, string, string) bool { return false }

func (s *stubProvider) CompactNative(context.Context, fluxengine.NativeCompactionRequest) (string, error) {
	return "", nil
}

func TestStubProviderDrivesChatClient(t *testing.T) {
	stub := &stubProvider{resp: &fluxengine.GenerateResponse{Content: "from stub", FinishReason: "end_turn"}}
	gw := &Gateway{Generator: stub}
	client := gw.ChatClient()

	got, err := client.Chat(context.Background(), []types.FluxMessage{{Role: "user", Content: "hi"}}, types.ChatOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Content != "from stub" {
		t.Fatalf("stub content not propagated: %+v", got)
	}
	if !client.ManagesResilience() {
		t.Fatal("expected resilience managed flag")
	}
}

var _ Provider = (*stubProvider)(nil)
