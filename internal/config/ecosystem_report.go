package config

import (
	"context"
	"fmt"
	"strings"

	"github.com/GrayCodeAI/hawk/internal/theme"
	"github.com/GrayCodeAI/hawk/internal/token"
)

// EcosystemReport is the structured view of the ecosystem panel.
type EcosystemReport struct {
	Eyrie EcosystemEyrie `json:"eyrie"`
	Token EngineToken    `json:"token"`
}

type EcosystemEyrie struct {
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

	// eyrie
	cat := CatalogHealthReport(ctx)
	r.Eyrie.CatalogExists = cat.Exists
	r.Eyrie.ModelCount = cat.Models
	pre := EnginePreflightReport(ctx)
	r.Eyrie.Ready = pre.Ready
	if strings.TrimSpace(provider) != "" && provider != "auto" {
		r.Eyrie.Provider = provider
	}
	if dep, err := EngineDeploymentSummary(ctx, model); err == nil {
		r.Eyrie.RoutingSource = dep.RoutingSource
		r.Eyrie.RoutingStages = dep.RoutingStages
	}

	// embedded token engine
	r.Token.Embedded = token.ShrikeAvailable()
	r.Token.SampleTokens = token.CountTokensFast("hawk context compression pipeline")

	return r
}

// FormatEcosystemPanel summarizes eyrie and token-engine integration for doctor and status output.
func FormatEcosystemPanel(ctx context.Context, provider, model string) string {
	var b strings.Builder
	b.WriteString(theme.Tint("Ecosystem (eyrie · token engine):", theme.ReportInfo) + "\n")

	// eyrie — LLM provider layer
	cat := CatalogHealthReport(ctx)
	eyrieLine := "  " + theme.Tint("eyrie:", theme.ReportMuted) + " "
	if cat.Exists {
		eyrieLine += theme.Tint(fmt.Sprintf("catalog %d models", cat.Models), theme.ReportInfo)
	} else {
		eyrieLine += theme.Tint("catalog missing (run hawk models refresh)", theme.ReportWarn)
	}
	pre := EnginePreflightReport(ctx)
	if pre.Ready {
		eyrieLine += " · " + theme.Tint("locally ready", theme.ReportSuccess)
	} else {
		eyrieLine += " · " + theme.Tint("setup incomplete", theme.ReportWarn)
	}
	if strings.TrimSpace(provider) != "" && provider != "auto" {
		eyrieLine += " · " + theme.Tint("provider "+provider, theme.ReportInfo)
	}
	if dep, err := EngineDeploymentSummary(ctx, model); err == nil {
		if dep.RoutingStages > 0 {
			eyrieLine += " · " + theme.Tint(fmt.Sprintf("routing %s (%d stages)", dep.RoutingSource, dep.RoutingStages), theme.ReportInfo)
		} else {
			eyrieLine += " · " + theme.Tint("routing "+dep.RoutingSource, theme.ReportInfo)
		}
	}
	b.WriteString(eyrieLine + "\n")

	// Embedded token engine — token counting and context compression.
	sample := token.CountTokensFast("hawk context compression pipeline")
	if token.ShrikeAvailable() {
		b.WriteString("  " + theme.Tint("token:", theme.ReportMuted) + " " + theme.Tint("embedded", theme.ReportInfo) + " · " + theme.Tint("token/compress pipeline OK", theme.ReportSuccess) + fmt.Sprintf(" (sample=%d tokens)", sample) + "\n")
	} else {
		b.WriteString("  " + theme.Tint("token:", theme.ReportMuted) + " " + theme.Tint("unavailable", theme.ReportWarn) + " · " + theme.Tint("token/compress pipeline not linked", theme.ReportWarn) + fmt.Sprintf(" (sample=%d tokens)", sample) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
