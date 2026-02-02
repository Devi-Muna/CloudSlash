package commands

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/DrSkyle/cloudslash/v2/pkg/version"
	"github.com/spf13/cobra"
)

// VersionURL defines the remote endpoint to query for the latest version.

// updateCmd initiates a check for available updates.
var updateCmd = &cobra.Command{
	Use:   "check-update",
	Short: "Check for available updates",
	Run: func(cmd *cobra.Command, args []string) {
		// Retrieve the latest version from the remote source.
		latest, err := fetchLatestVersion()
		if err != nil {
			// Fail silently on update check errors.
			return
		}

		if latest != version.Current {
			fmt.Printf("\n📦 Update Available: %s -> %s\n", version.Current, latest)
			fmt.Println("   Run the following to upgrade:")
			fmt.Println("\n   brew upgrade cloudslash")
		} else {
			fmt.Println("You are up to date.")
		}
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

func fetchLatestVersion() (string, error) {
	resp, err := http.Get(version.VersionURL)
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
