package cmd

import (
	"github.com/cnrancher/autok3s/pkg/cli/kubectl"

	"github.com/spf13/cobra"
)

// KubectlCommand kubectl command.
func KubectlCommand() *cobra.Command {
	return kubectl.EmbedCommand()
}
