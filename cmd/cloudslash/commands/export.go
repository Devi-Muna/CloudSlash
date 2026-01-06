package commands

import (
	"fmt"

	"github.com/DrSkyle/cloudslash/internal/app"
	"github.com/spf13/cobra"
)

var exportFormat string
var exportPath string

var ExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export forensic data (CSV, JSON)",
	Long: `Run a scan and export the results to a specified format.
    
Default output directory: ./cloudslash-out/`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Initializing Forensic Export...")
		config.Headless = true
		// In the future, we can inject exportPath into Config if we refactor app.Run
		// For now, this is essentially an alias for 'scan' but semantically focused on data extraction.
		_, _, err := app.Run(config)
		if err != nil {
			fmt.Printf("\n[ERROR] Export Failed: %v\n", err)
			return
		}

		fmt.Println("\n[SUCCESS] Export Complete.")
		fmt.Println("   CSV:  ./cloudslash-out/waste_report.csv")
		fmt.Println("   xxxxx JSON: ./cloudslash-out/waste_report.json")
		fmt.Println("   HTML: ./cloudslash-out/dashboard.html")
	},
}

func init() {
	// Future expansion: --format
	// ExportCmd.Flags().StringVar(&exportFormat, "format", "csv", "Export format (csv, json)")
}
