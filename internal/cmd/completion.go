package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for f5xcctl.

To load completions:

Bash:
  $ source <(f5xcctl completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ f5xcctl completion bash > /etc/bash_completion.d/f5xcctl
  # macOS:
  $ f5xcctl completion bash > $(brew --prefix)/etc/bash_completion.d/f5xcctl

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ f5xcctl completion zsh > "${fpath[1]}/_f5xcctl"

  # You will need to start a new shell for this setup to take effect.

Fish:
  $ f5xcctl completion fish | source

  # To load completions for each session, execute once:
  $ f5xcctl completion fish > ~/.config/fish/completions/f5xcctl.fish

PowerShell:
  PS> f5xcctl completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> f5xcctl completion powershell > f5xcctl.ps1
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

func init() {
	rootCmd.AddCommand(completionCmd)
}
