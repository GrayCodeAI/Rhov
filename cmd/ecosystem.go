package cmd

import (
	"context"
	"encoding/json"

	rhoconfig "github.com/GrayCodeAI/rho/internal/config"
	"github.com/spf13/cobra"
)

var ecosystemJSON bool

var ecosystemCmd = &cobra.Command{
	Use:   "ecosystem",
	Short: "Show flux and token-pipeline integration status",
	Long:  "Print the ecosystem panel summarizing the LLM provider runtime (flux) and the local token/compression pipeline. Same block as the top of rho doctor.",
	RunE: func(cmd *cobra.Command, args []string) error {
		settings, err := loadEffectiveSettings()
		if err != nil {
			return err
		}
		modelName, providerName := effectiveModelAndProvider(settings)
		if providerName == "" {
			providerName = "auto"
		}
		if ecosystemJSON {
			report := rhoconfig.BuildEcosystemReport(context.Background(), providerName, modelName)
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(report)
		}
		cmd.Println(rhoconfig.FormatEcosystemPanel(context.Background(), providerName, modelName))
		return nil
	},
}

func init() {
	ecosystemCmd.Flags().BoolVar(&ecosystemJSON, "json", false, "output ecosystem report as JSON")
	rootCmd.AddCommand(ecosystemCmd)
}
