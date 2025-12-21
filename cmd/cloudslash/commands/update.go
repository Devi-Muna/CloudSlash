package commands

import (
    "fmt"
    "net/http"
    "io"
    "os"
    "os/exec"
    "runtime"
    "strings"
    
    "github.com/spf13/cobra"
)

// This usually comes from -ldflags "-X ...CurrentVersion=..."
// But for this simple implementation, we can hardcode or use the version.txt strategy.
var CurrentVersion = "v1.2.1"
const VersionURL = "https://raw.githubusercontent.com/DrSkyle/CloudSlash/main/dist/version.txt"

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
        
        if strings.TrimSpace(latest) == CurrentVersion {
            fmt.Printf("You are already running the latest version (%s). ✨\n", CurrentVersion)
            return
        }
        
        fmt.Printf("Found new version: %s (Current: %s)\n", latest, CurrentVersion)
        fmt.Println("Downloading update...")
        
        if err := doUpdate(); err != nil {
            fmt.Printf("Update failed: %v\n", err)
            return
        }
        
        fmt.Println("🚀 Update successful! Please restart your terminal.")
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
    
    cmd := exec.Command("sh", "-c", "curl -sL https://raw.githubusercontent.com/DrSkyle/CloudSlash/main/dist/install.sh | bash")
    if runtime.GOOS == "windows" {
         cmd = exec.Command("powershell", "-Command", "irm https://raw.githubusercontent.com/DrSkyle/CloudSlash/main/dist/install.ps1 | iex")
    }
    
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    return cmd.Run()
}
