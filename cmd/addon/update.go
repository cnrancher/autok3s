package addon

import (
	"bytes"
	"reflect"

	"github.com/cnrancher/autok3s/pkg/common"
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
	updateCmd.Flags().StringVar(&addonFlags.FromFile, "from", addonFlags.FromFile, "The manifest file path of add-on")
	updateCmd.Flags().StringVar(&addonFlags.Manifest, "manifest", addonFlags.Manifest, "The manifest file content of add-on, need to be base64 encode")
	updateCmd.Flags().StringToStringVar(&addonFlags.Values, "set", addonFlags.Values, "Set value to replace parameters defined in manifest")
}

func UpdateCmd() *cobra.Command {
	updateCmd.Run = func(cmd *cobra.Command, args []string) {
		name := args[0]
		addon, err := common.DefaultDB.GetAddon(name)
		if err != nil {
			logrus.Fatalln(err)
		}
		var manifestFile string
		manifestContent := string(addon.Manifest)
		if addonFlags.FromFile != "" {
			manifestFile = addonFlags.FromFile
		} else if addonFlags.Manifest != "" {
			manifestContent = addonFlags.Manifest
		}

		manifestString, err := common.GetManifest(manifestFile, manifestContent)
		if err != nil {
			logrus.Fatalln(err)
		}
		manifest := []byte(manifestString)

		isChanged := false
		if addonFlags.Description != "" && addonFlags.Description != addon.Description {
			addon.Description = addonFlags.Description
			isChanged = true
		}

		if !bytes.Equal(manifest, addon.Manifest) {
			addon.Manifest = manifest
			isChanged = true
		}

		if !reflect.DeepEqual(addonFlags.Values, addon.Values) {
			addon.Values = addonFlags.Values
			isChanged = true
		}

		if isChanged {
			if err := common.DefaultDB.SaveAddon(addon); err != nil {
				logrus.Fatalln(err)
			}
		}
	}

	return updateCmd
}
