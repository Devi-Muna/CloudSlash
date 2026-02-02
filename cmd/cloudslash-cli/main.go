// Package main is the entry point for the CloudSlash CLI.
package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/DrSkyle/cloudslash/v2/cmd/cloudslash-cli/commands"
)

// main initializes the application and executes the root command.
func main() {
	// Ensure POSIX compatibility (Windows requires WSL2).
	if runtime.GOOS == "windows" {
		fmt.Println("[ERROR] CloudSlash does not support native Windows.")
		fmt.Println("[INFO]  Solution: Please run CloudSlash inside WSL2 (Windows Subsystem for Linux).")
		fmt.Println("        Docs: https://learn.microsoft.com/en-us/windows/wsl/install")
		os.Exit(1)
	}

	// Delegate execution to the root command handler.
	commands.Execute()
}
