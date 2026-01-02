package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/DrSkyle/cloudslash/internal/app"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	config  app.Config
)

var rootCmd = &cobra.Command{
	Use:   "cloudslash",
	Short: "The Forensic Cloud Accountant",
	Long: `CloudSlash - Zero Trust Infrastructure Analysis
    
Identify. Audit. Slash.`,
	Version: CurrentVersion,
	Run: func(cmd *cobra.Command, args []string) {
		if !cmd.Flags().Changed("region") {
			regions, err := PromptForRegions()
			if err == nil {
				config.Region = strings.Join(regions, ",")
			}
		}

		config.Headless = false
		_, _, _ = app.Run(config)
	},
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
	rootCmd.PersistentFlags().StringVar(&config.LicenseKey, "license", "", "License Key")
	rootCmd.PersistentFlags().StringVar(&config.Region, "region", "us-east-1", "AWS Region")
	rootCmd.PersistentFlags().StringVar(&config.TFStatePath, "tfstate", "terraform.tfstate", "Path to web.tfstate")
	rootCmd.PersistentFlags().BoolVar(&config.AllProfiles, "all-profiles", false, "Scan all AWS profiles")
	rootCmd.PersistentFlags().StringVar(&config.RequiredTags, "required-tags", "", "Required tags (comma-separated)")
	rootCmd.PersistentFlags().StringVar(&config.SlackWebhook, "slack-webhook", "", "Slack Webhook URL")

	// Hidden Flags
	rootCmd.PersistentFlags().BoolVar(&config.MockMode, "mock", false, "Run in Mock Mode")
	rootCmd.PersistentFlags().MarkHidden("mock")

	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		renderFutureGlassHelp(cmd)
	})

	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		// Support Environment Variable for License Key (CI/CD Safety)
		if config.LicenseKey == "" {
			config.LicenseKey = os.Getenv("CLOUDSLASH_LICENSE")
		}

		if cmd.Name() == "help" || cmd.Name() == "scan" || cmd.Name() == "update" {
			checkUpdate()
		}
	}

	rootCmd.AddCommand(NukeCmd)
	rootCmd.AddCommand(ExportCmd)
}

func checkUpdate() {
	latest, err := fetchLatestVersion()
	if err == nil && strings.TrimSpace(latest) > CurrentVersion {
		fmt.Printf("\n[UPDATE] Available: %s -> %s\nRun 'cloudslash update' to upgrade.\n\n", CurrentVersion, latest)
	}
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err == nil {
			viper.AddConfigPath(home)
			viper.SetConfigName(".cloudslash")
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

	fmt.Println(titleStyle.Render(fmt.Sprintf("CLOUDSLASH %s [Future-Glass]", CurrentVersion)))
	fmt.Println("The Forensic Cloud Accountant for AWS.")

	fmt.Println(titleStyle.Render("USAGE"))
	fmt.Printf("  %s\n\n", cmd.UseLine())

	fmt.Println(titleStyle.Render("COMMANDS"))
	for _, c := range cmd.Commands() {
		if c.IsAvailableCommand() {
			fmt.Printf("  %-12s %s\n", c.Name(), c.Short)
		}
	}
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

	fmt.Println(titleStyle.Render("NEW FEATURES (v1.2.6)"))
	fmt.Println(flagStyle.Render("  • ECS Idle Clusters:   Detects empty clusters wasting EC2 money"))
	fmt.Println(flagStyle.Render("  • Crash Loop Detector: Identifies broken services & missing images"))
	fmt.Println(flagStyle.Render("  • Interactive Mode:    Region selection & Deep Links in output"))
	fmt.Println("")
}
