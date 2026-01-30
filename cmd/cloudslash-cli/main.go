package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/DrSkyle/cloudslash/cmd/cloudslash-cli/commands"
)

func main() {
	// Validate operating system compatibility.
	// CloudSlash relies on POSIX standards unavailable in native Windows.
	if runtime.GOOS == "windows" {
		fmt.Println("[ERROR] CloudSlash does not support native Windows.")
		fmt.Println("[INFO]  Solution: Please run CloudSlash inside WSL2 (Windows Subsystem for Linux).")
		fmt.Println("        Docs: https://learn.microsoft.com/en-us/windows/wsl/install")
		os.Exit(1)
	}

	// Hand off control to the CLI command processor.
	commands.Execute()
}
