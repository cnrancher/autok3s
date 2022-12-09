package sshkey

import (
	"github.com/spf13/cobra"
)

var (
	sshkey = &cobra.Command{
		Use:   "sshkey",
		Short: "The SSH key management.",
		Long:  "The sshkey command manages the SSH key pairs to access your to-install k3s node.",
	}
)

func Command() *cobra.Command {
	sshkey.AddCommand(
		listCmd,
		createCmd,
		removeCmd,
		exportCmd,
	)
	return sshkey
}
