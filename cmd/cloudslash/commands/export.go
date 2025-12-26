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
        fmt.Println("🚀 Initializing Forensic Export...")
        config.Headless = true
        // In the future, we can inject exportPath into Config if we refactor app.Run
        // For now, this is essentially an alias for 'scan' but semantically focused on data extraction.
		isPro, g, err := app.Run(config)
        if err != nil {
             fmt.Printf("\n❌ Export Failed: %v\n", err)
             return
        }
        
        if isPro {
            fmt.Println("\n✅ Export Complete.")
            fmt.Println("   📂 CSV:  ./cloudslash-out/waste_report.csv")
            fmt.Println("   xxxxx JSON: ./cloudslash-out/waste_report.json")
            fmt.Println("   📊 HTML: ./cloudslash-out/dashboard.html")
        } else {
            // Calculate Potential Cost Savings
            var monthlyWaste float64
            g.Mu.RLock()
            wasteCount := 0
            for _, node := range g.Nodes {
                if node.IsWaste {
                    monthlyWaste += node.Cost
                    wasteCount++
                }
            }
            g.Mu.RUnlock()

            fmt.Printf("\n⚠️  Export Skipped [Community Edition]\n")
            fmt.Printf("\n   🔥 YOU ARE BURNING $%.2f / MONTH\n", monthlyWaste)
            fmt.Printf("   We successfully identified %d wasted resources.\n\n", wasteCount)
            
            fmt.Println("   Data export and detailed reports are locked.")
            fmt.Printf("   Unlock full visibility to save $%.2f every year.\n\n", monthlyWaste * 12)
            fmt.Println("   > cloudslash --license [KEY]")
        }
	},
}

func init() {
    // Future expansion: --format
    // ExportCmd.Flags().StringVar(&exportFormat, "format", "csv", "Export format (csv, json)")
}
