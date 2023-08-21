package addon

import (
	"fmt"
	"strings"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	getCmd = &cobra.Command{
		Use:   "get <name>",
		Short: "Get an add-on information.",
		Args:  cobra.ExactArgs(1),
	}
)

func GetCmd() *cobra.Command {
	getCmd.Run = func(cmd *cobra.Command, args []string) {
		name := args[0]
		addon, err := common.DefaultDB.GetAddon(name)
		if err != nil {
			logrus.Fatalln(err)
		}
		fmt.Println("Name: " + addon.Name)
		fmt.Println("Description: " + addon.Description)
		fmt.Println("Manifest: " + string(addon.Manifest))
		if len(addon.Values) > 0 {
			values := []string{}
			for key, value := range addon.Values {
				values = append(values, fmt.Sprintf("%s=%s", key, value))
			}
			fmt.Println("Values: " + strings.Join(values, ","))
		}
	}
	return getCmd
}
