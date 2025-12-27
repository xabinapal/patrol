package cli

import (
	"os"

	"github.com/spf13/cobra"
)

// newCompletionCmd creates the completion command.
func (cli *CLI) newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for Patrol.

To load completions:

Bash:
  $ source <(patrol completion bash)
  # To load completions for each session, execute once:
  # Linux:
  $ patrol completion bash > /etc/bash_completion.d/patrol
  # macOS:
  $ patrol completion bash > $(brew --prefix)/etc/bash_completion.d/patrol

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ patrol completion zsh > "${fpath[1]}/_patrol"
  # You may need to start a new shell for this to take effect.

Fish:
  $ patrol completion fish | source
  # To load completions for each session, execute once:
  $ patrol completion fish > ~/.config/fish/completions/patrol.fish

PowerShell:
  PS> patrol completion powershell | Out-String | Invoke-Expression
  # To load completions for every new session, run:
  PS> patrol completion powershell > patrol.ps1
  # and source this file from your PowerShell profile.
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				return cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				return cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
			}
			return nil
		},
	}
	return cmd
}
