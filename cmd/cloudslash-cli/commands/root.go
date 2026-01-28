package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DrSkyle/cloudslash/pkg/engine"
	"github.com/DrSkyle/cloudslash/pkg/version"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	config  engine.Config
)

var rootCmd = &cobra.Command{
	Use:   "cloudslash",
	Short: "Infrastructure Optimization Tool",
	Long: `CloudSlash - Infrastructure Analysis Platform
    
Identify. Audit. Optimize.`,
	Version: version.Current,
	// Run: nil (Forces help output).
	Run: nil,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Persistent Flags
	rootCmd.PersistentFlags().StringVar(&config.Region, "region", "us-east-1", "AWS Region")
	rootCmd.PersistentFlags().StringVar(&config.TFStatePath, "tfstate", "terraform.tfstate", "Path to web.tfstate")
	rootCmd.PersistentFlags().BoolVar(&config.AllProfiles, "all-profiles", false, "Scan all AWS profiles")
	rootCmd.PersistentFlags().StringVar(&config.RequiredTags, "required-tags", "", "Required tags (comma-separated)")
	rootCmd.PersistentFlags().StringVar(&config.SlackWebhook, "slack-webhook", "", "Slack Webhook URL")
	rootCmd.PersistentFlags().BoolVarP(&config.Verbose, "verbose", "v", false, "Enable Matrix Mode (Visual API Logging)")

	// Hidden Flags
	rootCmd.PersistentFlags().BoolVar(&config.MockMode, "mock", false, "Run in Mock Mode")
	rootCmd.PersistentFlags().MarkHidden("mock")

	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		renderFutureGlassHelp(cmd)
	})

	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		// Just version check
		if cmd.Name() == "help" || cmd.Name() == "scan" || cmd.Name() == "update" {
			checkUpdate()
		}
	}

	rootCmd.AddCommand(CleanupCmd)
	rootCmd.AddCommand(ExportCmd)
}

func checkUpdate() {
	latest, err := fetchLatestVersion()
	if err == nil && strings.TrimSpace(latest) > version.Current {
		fmt.Printf("\n[UPDATE] Available: %s -> %s\nRun 'cloudslash update' to upgrade.\n\n", version.Current, latest)
	}
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err == nil {
			viper.SetConfigFile(filepath.Join(home, ".cloudslash.yaml"))
			viper.SetConfigType("yaml")
		}
	}
	viper.AutomaticEnv()
	viper.ReadInConfig()
}

func renderFutureGlassHelp(cmd *cobra.Command) {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00FF99")).
		MarginBottom(1)

	flagStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#AAAAAA"))

	fmt.Println(titleStyle.Render(fmt.Sprintf("CLOUDSLASH %s", version.Current)))
	fmt.Println("Infrastructure Optimization and Analysis Tool.")

	fmt.Println(titleStyle.Render("USAGE"))
	fmt.Printf("  %s\n\n", cmd.UseLine())

	fmt.Println(titleStyle.Render("COMMANDS"))
	for _, c := range cmd.Commands() {
		if c.IsAvailableCommand() {
			fmt.Printf("  %-12s %s\n", c.Name(), c.Short)
		}
	}
	fmt.Println("")
	
	fmt.Println(titleStyle.Render("EXAMPLES"))
	fmt.Println("  cloudslash scan                          # Interactive Mode (TUI)")
	fmt.Println("  cloudslash scan --headless --region ...  # CI/CD Mode (No TUI)")
	fmt.Println("")

	fmt.Println(titleStyle.Render("FLAGS"))
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		output := fmt.Sprintf("  --%-15s %s", f.Name, f.Usage)
		if f.DefValue != "" && f.DefValue != "false" && f.DefValue != "0" {
			output += fmt.Sprintf(" (default %s)", f.DefValue)
		}
		fmt.Println(flagStyle.Render(output))
	})
	fmt.Println("")

}

func safeWriteConfig() {
	// 1. Try SafeWrite (creates if creates if missing, fails if exists)
	if err := viper.SafeWriteConfig(); err != nil {
		// 2. If already exists, try Overwrite
		if err2 := viper.WriteConfig(); err2 != nil {
			// 3. Fallback: Force create file at explicit path
			path := viper.ConfigFileUsed()
			if path != "" {
				f, createErr := os.Create(path)
				if createErr == nil {
					f.Close()
					viper.WriteConfig()
				} else {
					fmt.Printf("Error creating config file: %v\n", createErr)
				}
			} else {
				// If path is empty, try manual home construction
				home, _ := os.UserHomeDir()
				path = filepath.Join(home, ".cloudslash.yaml")
				f, _ := os.Create(path)
				f.Close()
				viper.SetConfigFile(path)
				viper.WriteConfig()
			}
		}
	}
}
