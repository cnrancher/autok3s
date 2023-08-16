package addon

import (
	"errors"
	"fmt"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/utils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	createCmd = &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new addon",
		Args:  cobra.ExactArgs(1),
	}
)

func init() {
	createCmd.Flags().StringVar(&addonFlags.Description, "description", addonFlags.Description, "The description of addon")
	createCmd.Flags().StringVar(&addonFlags.FromFile, "from", addonFlags.FromFile, "The manifest file path of addon")
	createCmd.Flags().StringVar(&addonFlags.Manifest, "manifest", addonFlags.Manifest, "The manifest file content of addon, need to be base64 encode")
	createCmd.Flags().StringToStringVar(&addonFlags.Values, "set", addonFlags.Values, "Set value to replace parameters defined in manifest")
}

func CreateCmd() *cobra.Command {
	createCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := common.ValidateName(name); err != nil {
			return err
		}
		if addonFlags.FromFile != "" && !utils.IsFileExists(addonFlags.FromFile) {
			return fmt.Errorf("manifest file %s is not exist", addonFlags.FromFile)
		}
		if addonFlags.FromFile == "" && addonFlags.Manifest == "" {
			return errors.New("must set addon manifest by --from <file-path> or --manifest <base64-encode-manifest>")
		}
		return nil
	}

	createCmd.Run = func(cmd *cobra.Command, args []string) {
		name := args[0]
		manifest, err := common.GetManifest(addonFlags.FromFile, addonFlags.Manifest)
		if err != nil {
			logrus.Fatalln(err)
		}
		addon := &common.Addon{
			Name:        name,
			Description: addonFlags.Description,
			Manifest:    []byte(manifest),
			Values:      addonFlags.Values,
		}
		if err := common.DefaultDB.SaveAddon(addon); err != nil {
			logrus.Fatalln(err)
		}

	}

	return createCmd
}
