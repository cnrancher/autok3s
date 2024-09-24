package addon

import (
	"fmt"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

var (
	getCmd = &cobra.Command{
		Use:   "get <name>",
		Short: "Get an add-on information.",
		Args:  cobra.ExactArgs(1),
	}
)

func GetCmd() *cobra.Command {
	getCmd.Run = func(_ *cobra.Command, args []string) {
		name := args[0]
		addon, err := common.DefaultDB.GetAddon(name)
		if err != nil {
			logrus.Fatalln(err)
		}
		addonMap := map[string]interface{}{
			"Name":        addon.Name,
			"Description": addon.Description,
			"Manifest":    string(addon.Manifest),
			"Values":      addon.Values,
		}
		data, _ := yaml.Marshal(addonMap)
		fmt.Println(string(data))
	}
	return getCmd
}
