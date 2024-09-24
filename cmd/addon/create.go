package addon

import (
	"errors"
	"fmt"
	"os"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/utils"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	createCmd = &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new add-on",
		Args:  cobra.ExactArgs(1),
	}
)

func init() {
	createCmd.Flags().StringVar(&addonFlags.Description, "description", addonFlags.Description, "The description of add-on")
	createCmd.Flags().StringVarP(&addonFlags.FromFile, "from", "f", addonFlags.FromFile, "The manifest file path of add-on")
	createCmd.Flags().StringToStringVar(&addonFlags.Values, "set", addonFlags.Values, "Set value to replace parameters defined in manifest")
}

func CreateCmd() *cobra.Command {
	createCmd.PreRunE = func(_ *cobra.Command, args []string) error {
		name := args[0]
		if err := common.ValidateName(name); err != nil {
			return err
		}
		if addonFlags.FromFile == "" {
			return errors.New("must set addon manifest by -f <file-path>")
		}
		if addonFlags.FromFile != "" && !utils.IsFileExists(addonFlags.FromFile) {
			return fmt.Errorf("manifest file %s is not exist", addonFlags.FromFile)
		}
		return nil
	}

	createCmd.Run = func(_ *cobra.Command, args []string) {
		name := args[0]
		manifest, err := os.ReadFile(addonFlags.FromFile)
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
