package config

import (
	"context"
	"fmt"
	"strings"

	"github.com/GrayCodeAI/rho/internal/theme"
	"github.com/GrayCodeAI/rho/internal/token"
)

// EcosystemReport is the structured view of the ecosystem panel.
type EcosystemReport struct {
	Flux  EcosystemFlux `json:"flux"`
	Token EngineToken   `json:"token"`
}

type EcosystemFlux struct {
	CatalogExists bool   `json:"catalog_exists"`
	ModelCount    int    `json:"model_count,omitempty"`
	Ready         bool   `json:"ready"`
	Provider      string `json:"provider,omitempty"`
	RoutingSource string `json:"routing_source,omitempty"`
	RoutingStages int    `json:"routing_stages,omitempty"`
}

type EngineToken struct {
	Embedded     bool `json:"embedded"`
	SampleTokens int  `json:"sample_tokens"`
}

// BuildEcosystemReport returns a structured ecosystem report.
func BuildEcosystemReport(ctx context.Context, provider, model string) EcosystemReport {
	var r EcosystemReport

	// flux
	cat := CatalogHealthReport(ctx)
	r.Flux.CatalogExists = cat.Exists
	r.Flux.ModelCount = cat.Models
	pre := EnginePreflightReport(ctx)
	r.Flux.Ready = pre.Ready
	if strings.TrimSpace(provider) != "" && provider != "auto" {
		r.Flux.Provider = provider
	}
	if dep, err := EngineDeploymentSummary(ctx, model); err == nil {
		r.Flux.RoutingSource = dep.RoutingSource
		r.Flux.RoutingStages = dep.RoutingStages
	}

	// embedded token engine
	r.Token.Embedded = token.ShrikeAvailable()
	r.Token.SampleTokens = token.CountTokensFast("rho context compression pipeline")

	return r
}

// FormatEcosystemPanel summarizes flux and token-engine integration for doctor and status output.
func FormatEcosystemPanel(ctx context.Context, provider, model string) string {
	var b strings.Builder
	b.WriteString(theme.Tint("Ecosystem (flux · token engine):", theme.ReportInfo) + "\n")

	// flux — LLM provider layer
	cat := CatalogHealthReport(ctx)
	fluxLine := "  " + theme.Tint("flux:", theme.ReportMuted) + " "
	if cat.Exists {
		fluxLine += theme.Tint(fmt.Sprintf("catalog %d models", cat.Models), theme.ReportInfo)
	} else {
		fluxLine += theme.Tint("catalog missing (run rho models refresh)", theme.ReportWarn)
	}
	pre := EnginePreflightReport(ctx)
	if pre.Ready {
		fluxLine += " · " + theme.Tint("locally ready", theme.ReportSuccess)
	} else {
		fluxLine += " · " + theme.Tint("setup incomplete", theme.ReportWarn)
	}
	if strings.TrimSpace(provider) != "" && provider != "auto" {
		fluxLine += " · " + theme.Tint("provider "+provider, theme.ReportInfo)
	}
	if dep, err := EngineDeploymentSummary(ctx, model); err == nil {
		if dep.RoutingStages > 0 {
			fluxLine += " · " + theme.Tint(fmt.Sprintf("routing %s (%d stages)", dep.RoutingSource, dep.RoutingStages), theme.ReportInfo)
		} else {
			fluxLine += " · " + theme.Tint("routing "+dep.RoutingSource, theme.ReportInfo)
		}
	}
	b.WriteString(fluxLine + "\n")

	// Embedded token engine — token counting and context compression.
	sample := token.CountTokensFast("rho context compression pipeline")
	if token.ShrikeAvailable() {
		b.WriteString("  " + theme.Tint("token:", theme.ReportMuted) + " " + theme.Tint("embedded", theme.ReportInfo) + " · " + theme.Tint("token/compress pipeline OK", theme.ReportSuccess) + fmt.Sprintf(" (sample=%d tokens)", sample) + "\n")
	} else {
		b.WriteString("  " + theme.Tint("token:", theme.ReportMuted) + " " + theme.Tint("unavailable", theme.ReportWarn) + " · " + theme.Tint("token/compress pipeline not linked", theme.ReportWarn) + fmt.Sprintf(" (sample=%d tokens)", sample) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
