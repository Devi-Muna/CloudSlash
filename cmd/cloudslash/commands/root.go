package commands

import (
	"fmt"
	"os"
	"path/filepath"
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

	rootCmd.AddCommand(NukeCmd)
	rootCmd.AddCommand(ExportCmd)
	rootCmd.AddCommand(coffeeCmd)
}

var coffeeCmd = &cobra.Command{
	Use:   "coffee",
	Short: "418 I'm a teapot",
	Hidden: true, // Easter egg
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(`
    (  )   (   )  )
     ) (   )  (  (
     ( )  (    ) )
     _____________
    <_____________> ___
    |             |/ _ \
    |               | | |
    |               |_| |
    |             |\___/
    \_____________/
`)
		fmt.Println("418 I'm a teapot.")
		fmt.Println("But seriously, buy me a coffee: \u001b]8;;https://buymeacoffee.com/drskyle\u001b\\Click Here\u001b]8;;\u001b\\") 
	},
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
