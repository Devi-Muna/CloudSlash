package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"math/rand"
	"time"

	"github.com/DrSkyle/cloudslash/internal/app"
	"github.com/DrSkyle/cloudslash/internal/license"
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
		// 1. Env Var Override
		if config.LicenseKey == "" {
			config.LicenseKey = os.Getenv("CLOUDSLASH_LICENSE")
		}

        // 2. Machine ID Generation (Persistent Identity)
        machineID := viper.GetString("machine_id")
        // fmt.Printf("DEBUG: Current Machine ID in Viper: '%s'\n", machineID)
        
        if machineID == "" {
            // Generate simple random hex ID (16 chars)
            b := make([]byte, 8)
            rand.Seed(time.Now().UnixNano())
            rand.Read(b)
            machineID = fmt.Sprintf("%x", b)
            
            // fmt.Printf("DEBUG: Generated New ID: %s. Saving...\n", machineID)

            viper.Set("machine_id", machineID)
            if err := viper.WriteConfig(); err != nil {
                // fmt.Printf("DEBUG: WriteConfig failed: %v. Trying SafeWrite...\n", err)
                safeWriteConfig()
            } else {
                // fmt.Println("DEBUG: WriteConfig Success.")
            }
        }
        config.MachineID = machineID

		// 3. Logic: If user provided a key (Flag or Env), try to SAVE it.
		//    If not, try to LOAD it.
		savedLicense := viper.GetString("license")

		if config.LicenseKey != "" {
			// User is providing a key based on input.
			// Only save if it's different or we just want to ensure it's saved.
			// Check validity first to avoid saving garbage.
			if err := license.Check(config.LicenseKey, machineID); err == nil {
                // It is valid. Persist it.
				viper.Set("license", config.LicenseKey)
                // Use safe write
                if err := viper.WriteConfig(); err != nil {
                     // If file doesn't exist, Create it
                     safeWriteConfig()
                }
                // Feedback only if it was a manual flag input (contextual guess)
                if cmd.Flags().Changed("license") {
                    fmt.Println("✅ License Verified & Saved to ~/.cloudslash.yaml")
                }
			} else {
                 // Warn but don't stop? Or stop? 
                 // If the user explicitly provided a flag, we should probably warn if it's invalid.
                 // But let's leave the strict check to the actual feature usage points to avoid breaking help/completion.
            }
		} else {
            // No input provided, load from disk
            config.LicenseKey = savedLicense
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
