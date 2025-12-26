package commands

import (
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
        config.Headless = true
		_, _, _ = app.Run(config)
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)
}
