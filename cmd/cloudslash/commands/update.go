package commands

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/DrSkyle/cloudslash/internal/version"
	"github.com/spf13/cobra"
)

const VersionURL = "https://raw.githubusercontent.com/DrSkyle/CloudSlash/main/scripts/version.txt"

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update CloudSlash to the latest version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Checking for updates...")

		latest, err := fetchLatestVersion()
		if err != nil {
			fmt.Printf("Failed to check version: %v\n", err)
			return
		}

		if strings.TrimSpace(latest) == version.Current {
			fmt.Printf("You are already running the latest version (%s).\n", version.Current)
			return
		}

		// Basic semantic check to prevent "downgrade" notifications if local > remote
		// (Assuming format vX.Y.Z)
		if latest < version.Current {
			fmt.Printf("You are running a newer version (%s) than the latest release (%s).\n", version.Current, latest)
			return
		}

		fmt.Printf("Found new version: %s (Current: %s)\n", latest, version.Current)
		fmt.Println("Downloading update...")

		if err := doUpdate(); err != nil {
			fmt.Printf("Update failed: %v\n", err)
			return
		}

		fmt.Println("[SUCCESS] Update successful! Please restart your terminal.")
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

func fetchLatestVersion() (string, error) {
	resp, err := http.Get(VersionURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}

func doUpdate() error {
	// 1. Determine download URL based on OS specific command
	// The simplest "Auto-Update" is actually just re-running the install script!

	cmd := exec.Command("sh", "-c", "curl -sL https://raw.githubusercontent.com/DrSkyle/CloudSlash/main/scripts/install.sh | bash")
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell", "-Command", "irm https://raw.githubusercontent.com/DrSkyle/CloudSlash/main/scripts/install.ps1 | iex")
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
