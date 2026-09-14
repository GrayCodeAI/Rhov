package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	hawkconfig "github.com/GrayCodeAI/hawk/internal/config"
	"github.com/spf13/cobra"
)

var (
	pathStrict bool
	pathJSON   bool
)

var pathCmd = &cobra.Command{
	Use:   "path",
	Short: "Developer path readiness (setup, security, ecosystem)",
	Long: `Check whether hawk is configured on the developer path:
API keys in OS secret store, model selected, no secrets on disk,
and eyrie integration.

Built for individual developers first — teams and enterprise later.

See docs/DEVELOPER-PATH.md and docs/SECURITY-DEVELOPER.md.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		report := hawkconfig.EvaluateDeveloperPath(ctx)

		if pathJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(report)
		}

		cmd.Println(hawkconfig.FormatDeveloperPathReport(ctx))

		if !report.Ready {
			return fmt.Errorf("developer path not ready — %s", report.NextStep)
		}
		return nil
	},
}

func init() {
	pathCmd.Flags().BoolVar(&pathStrict, "strict", false, "compatibility flag; retained for scripts")
	pathCmd.Flags().BoolVar(&pathJSON, "json", false, "output readiness report as JSON")
	rootCmd.AddCommand(pathCmd)
}
