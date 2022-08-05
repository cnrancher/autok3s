package airgap

import (
	"errors"
	"fmt"

	pkgairgap "github.com/cnrancher/autok3s/pkg/airgap"
	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a stored airgap package.",
	Args:  cobra.ExactArgs(1),
	RunE:  remove,
}

func init() {
	removeCmd.Flags().BoolVarP(&airgapFlags.isForce, "force", "f", false, "Force to delete a package.")
}

func remove(cmd *cobra.Command, args []string) error {
	name := args[0]
	if !airgapFlags.isForce {
		if !utils.IsTerm() {
			return errors.New("please using --force to delete a package")
		}
		if !utils.AskForConfirmation(fmt.Sprintf("are you going to remove package %s", name), false) {
			return nil
		}
	}

	if err := pkgairgap.RemovePackage(name); err != nil {
		return err
	}

	if err := common.DefaultDB.DeletePackage(name); err != nil {
		return err
	}

	cmd.Printf("package %s removed\n", name)
	return nil
}
