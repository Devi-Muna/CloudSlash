package commands

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/DrSkyle/cloudslash/pkg/version"
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

		// Prevent downgrades to older versions.
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
	// We only support Bash (Linux/Mac/WSL)
	// The Bouncer in main.go guarantees we are not on native Windows.
	cmd := exec.Command("sh", "-c", "curl -sL https://raw.githubusercontent.com/DrSkyle/CloudSlash/main/scripts/install.sh | bash")

	// CRITICAL: Allow the script to ask for sudo password if needed
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
