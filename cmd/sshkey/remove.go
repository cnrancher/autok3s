package sshkey

import (
	"errors"
	"fmt"
	"strings"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:     "remove <name> [name...]",
	Aliases: []string{"rm"},
	Short:   "Remove a stored ssh key pair.",
	Args:    cobra.MinimumNArgs(1),
	Run:     utils.CommandExitWithoutHelpInfo(remove),
}

func init() {
	removeCmd.Flags().BoolVarP(&sshKeyFlags.isForce, "force", "f", false, "Force to delete a package.")
}

func remove(cmd *cobra.Command, args []string) error {
	if !sshKeyFlags.isForce {
		if !utils.IsTerm() {
			return errors.New("please using --force to delete a ssh key pair")
		}
		if !utils.AskForConfirmation(fmt.Sprintf("are you going to remove ssh key pair(s) %s", strings.Join(args, ",")), false) {
			return nil
		}
	}
	for _, name := range args {
		if err := common.DefaultDB.DeleteSSHKey(name); err != nil {
			return err
		}

		cmd.Printf("ssh key %s removed\n", name)
	}

	return nil
}
