package commands

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/DrSkyle/cloudslash/v2/pkg/version"
	"github.com/spf13/cobra"
)

// updateCmd initiates a check for available updates.
var updateCmd = &cobra.Command{
	Use:   "check-update",
	Short: "Check for available updates",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print("Checking for updates... ")
		
		// Retrieve the latest version from GitHub API.
		latestTag, err := fetchLatestVersion()
		if err != nil {
			fmt.Printf("Failed.\n[WARN] Could not check for updates: %v\n", err)
			return
		}

		// Normalize versions (strip 'v' prefix if present).
		current := strings.TrimPrefix(version.Current, "v")
		latest := strings.TrimPrefix(latestTag, "v")

		if current != latest {
			fmt.Printf("\n📦 Update Available: v%s -> v%s\n", current, latest)
			fmt.Println("   Run the following to upgrade:")
			fmt.Println("\n   brew upgrade cloudslash")
		} else {
			fmt.Printf("Up to date (v%s).\n", current)
		}
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

type githubRelease struct {
	TagName string `json:"tag_name"`
}

func fetchLatestVersion() (string, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequest("GET", "https://api.github.com/repos/DrSkyle/CloudSlash/releases/latest", nil)
	if err != nil {
		return "", err
	}
	
	req.Header.Set("User-Agent", "CloudSlash-CLI")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status: %s", resp.Status)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	return release.TagName, nil
}
