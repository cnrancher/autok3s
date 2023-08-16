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
		Short: "Remove an addon.",
		Args:  cobra.ExactArgs(1),
	}
)

func RemoveCmd() *cobra.Command {
	removeCmd.Run = func(cmd *cobra.Command, args []string) {
		name := args[0]
		if utils.AskForConfirmation(fmt.Sprintf("are you going to remove the addon %s", name), false) {
			if err := common.DefaultDB.DeleteAddon(name); err != nil {
				logrus.Fatalln(err)
			}
		}
	}

	return removeCmd
}
