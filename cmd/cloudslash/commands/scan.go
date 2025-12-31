package commands

import (
	"strings"

	"github.com/DrSkyle/cloudslash/internal/app"
	"github.com/spf13/cobra"
)

// scanCmd represents the scan command
var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Run a headless scan (no TUI)",
	Long: `Run CloudSlash in headless mode. Useful for CI/CD pipelines or cron jobs.
    
Example:
  cloudslash scan --region us-west-2`,
	Run: func(cmd *cobra.Command, args []string) {
		// Interactive Filtering (v1.2.6)
		if !cmd.Flags().Changed("region") {
			// Check if TTY? Cobra Run usually implies we can check inputs.
			// Just run the prompt.
			regions, err := PromptForRegions()
			if err == nil {
				config.Region = strings.Join(regions, ",")
			}
		}
		
		config.Headless = true
		_, _, _ = app.Run(config)
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)
}
