package addon

import (
	"fmt"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/utils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	removeCmd = &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove an add-on.",
		Args:  cobra.ExactArgs(1),
	}
)

func RemoveCmd() *cobra.Command {
	removeCmd.Run = func(_ *cobra.Command, args []string) {
		name := args[0]
		if utils.AskForConfirmation(fmt.Sprintf("are you going to remove the addon %s", name), false) {
			if err := common.DefaultDB.DeleteAddon(name); err != nil {
				logrus.Fatalln(err)
			}
		}
	}

	return removeCmd
}
