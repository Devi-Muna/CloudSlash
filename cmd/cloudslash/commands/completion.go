package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate completion script",
	Long: `To load completions:

Bash:
  $ source <(cloudslash completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ cloudslash completion bash > /etc/bash_completion.d/cloudslash
  # macOS:
  $ cloudslash completion bash > /usr/local/etc/bash_completion.d/cloudslash

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ cloudslash completion zsh > "${fpath[1]}/_cloudslash"

  # You will need to start a new shell for this setup to take effect.

Fish:
  $ cloudslash completion fish | source

  # To load completions for each session, execute once:
  $ cloudslash completion fish > ~/.config/fish/completions/cloudslash.fish
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.ExactValidArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		switch args[0] {
		case "bash":
			fmt.Print(humanBashCompletion)
		case "zsh":
			rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			rootCmd.GenPowerShellCompletion(os.Stdout)
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}

// humanBashCompletion is a handcrafted, minimal bash completion script
// that avoids the robotic verbosity of auto-generated ones.
const humanBashCompletion = `
# cloudslash bash completion

_cloudslash_completion() {
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    opts="scan cleanup export update completion help"

    case "${prev}" in
        scan)
            COMPREPLY=( $(compgen -W "--headless --region --no-metrics --help" -- ${cur}) )
            return 0
            ;;
        cleanup)
            COMPREPLY=( $(compgen -W "--help" -- ${cur}) )
            return 0
            ;;
        export)
             COMPREPLY=( $(compgen -W "--help" -- ${cur}) )
             return 0
             ;;
        update)
             COMPREPLY=( $(compgen -W "--help" -- ${cur}) )
             return 0
             ;;
        completion)
             COMPREPLY=( $(compgen -W "bash zsh fish powershell" -- ${cur}) )
             return 0
             ;;
        --region)
             # Common regions
             local regions="us-east-1 us-east-2 us-west-1 us-west-2 eu-central-1 eu-west-1 ap-southeast-1"
             COMPREPLY=( $(compgen -W "${regions}" -- ${cur}) )
             return 0
             ;;
        *)
            ;;
    esac

    # Global Flags
    if [[ ${cur} == -* ]] ; then
        COMPREPLY=( $(compgen -W "--help --version --region --tfstate --verbose" -- ${cur}) )
        return 0
    fi

    # Subcommands
    COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
}

complete -F _cloudslash_completion cloudslash
`
