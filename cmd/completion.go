package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

const exampleStr = `
  To load completions:

  Bash:

  $ source <(autok3s completion bash)

  # To load completions for each session, execute once:
  Linux:
    $ autok3s completion bash > /etc/bash_completion.d/autok3s
  MacOS:
    $ autok3s completion bash > /usr/local/etc/bash_completion.d/autok3s

  Zsh:

  # If shell completion is not already enabled in your environment you will need
  # to enable it.  You can execute the following once:

  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ autok3s completion zsh > "${fpath[1]}/_autok3s"

  # You will need to start a new shell for this setup to take effect.

  Fish:

  $ autok3s completion fish | source

  # To load completions for each session, execute once:
  $ autok3s completion fish > ~/.config/fish/completions/autok3s.fish
`

// CompletionCommand used for command completion.
func CompletionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "completion [bash|zsh|fish|powershell]",
		Short:                 "Generate completion script",
		Example:               exampleStr,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		Run: func(cmd *cobra.Command, args []string) {
			switch args[0] {
			case "bash":
				_ = cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				_ = cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				_ = cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				_ = cmd.Root().GenPowerShellCompletion(os.Stdout)
			}
		},
	}

	return cmd
}
