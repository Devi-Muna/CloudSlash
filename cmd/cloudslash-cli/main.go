package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/DrSkyle/cloudslash/cmd/cloudslash-cli/commands"
)

func main() {
	// 1. THE BOUNCER: Block Native Windows
	if runtime.GOOS == "windows" {
		fmt.Println("❌  Error: CloudSlash does not support native Windows.")
		fmt.Println("💡  Solution: Please run CloudSlash inside WSL2 (Windows Subsystem for Linux).")
		fmt.Println("   Docs: https://learn.microsoft.com/en-us/windows/wsl/install")
		os.Exit(1)
	}

	// 2. Run the App
	commands.Execute()
}
