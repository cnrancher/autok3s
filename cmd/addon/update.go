package addon

import (
	"fmt"
	"os"
	"reflect"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/utils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	updateCmd = &cobra.Command{
		Use:   "update <name>",
		Short: "Update manifest for an add-on",
		Args:  cobra.ExactArgs(1),
	}
)

func init() {
	updateCmd.Flags().StringVar(&addonFlags.Description, "description", addonFlags.Description, "The description of add-on")
	updateCmd.Flags().StringVarP(&addonFlags.FromFile, "from", "f", addonFlags.FromFile, "The manifest file path of add-on")
	updateCmd.Flags().StringToStringVar(&addonFlags.Values, "set", addonFlags.Values, "Set value to replace parameters defined in manifest")
	updateCmd.Flags().StringArrayVar(&addonFlags.RemoveValues, "unset", addonFlags.RemoveValues, "The values of the add-on to unset, will ignore if the value is not present in the list")
}

func UpdateCmd() *cobra.Command {
	updateCmd.PreRunE = func(_ *cobra.Command, _ []string) error {
		if addonFlags.FromFile != "" && !utils.IsFileExists(addonFlags.FromFile) {
			return fmt.Errorf("manifest file %s is not exist", addonFlags.FromFile)
		}
		return nil
	}
	updateCmd.Run = func(cmd *cobra.Command, args []string) {
		name := args[0]
		addon, err := common.DefaultDB.GetAddon(name)
		if err != nil {
			logrus.Fatalln(err)
		}
		if addon.Values == nil {
			addon.Values = make(types.StringMap)
		}

		newAddon := &common.Addon{
			Name:        addon.Name,
			Description: addon.Description,
			Manifest:    addon.Manifest,
			Values:      map[string]string{},
		}
		for k, v := range addon.Values {
			newAddon.Values[k] = v
		}

		if addonFlags.FromFile != "" {
			manifestString, err := os.ReadFile(addonFlags.FromFile)
			if err != nil {
				logrus.Error(err)
				return
			}
			newAddon.Manifest = []byte(manifestString)
		}

		if addonFlags.Description != "" {
			newAddon.Description = addonFlags.Description
		}

		// unset values
		for _, v := range addonFlags.RemoveValues {
			delete(newAddon.Values, v)
		}
		// re-set values
		for k, v := range addonFlags.Values {
			newAddon.Values[k] = v
		}

		if !reflect.DeepEqual(addon, newAddon) {
			if err := common.DefaultDB.SaveAddon(newAddon); err != nil {
				logrus.Error(err)
				return
			}
			cmd.Println("add-on updated")
		} else {
			cmd.Println("add-on is not changed")
		}
	}

	return updateCmd
}
