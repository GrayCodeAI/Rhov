package cmd

import (
	"strings"
	"testing"

	rhoconfig "github.com/GrayCodeAI/rho/internal/config"
)

func TestEcosystemCmdRuns(t *testing.T) {
	settings := rhoconfig.Settings{}
	model, provider := effectiveModelAndProvider(settings)
	if provider == "" {
		provider = "auto"
	}
	out := rhoconfig.FormatEcosystemPanel(t.Context(), provider, model)
	if !strings.Contains(out, "Ecosystem (flux · token engine)") {
		t.Fatalf("unexpected panel: %q", out)
	}
	if err := ecosystemCmd.RunE(ecosystemCmd, nil); err != nil {
		t.Fatalf("ecosystem command: %v", err)
	}
}
