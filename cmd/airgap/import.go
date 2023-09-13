package airgap

import (
	"errors"
	"os"

	pkgairgap "github.com/cnrancher/autok3s/pkg/airgap"
	"github.com/cnrancher/autok3s/pkg/common"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

var (
	importCmd = &cobra.Command{
		Use:   "import <path> [name]",
		Short: "Import an existing tar.gz file of airgap package. Please refer to export command",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.MaximumNArgs(2)(cmd, args); err != nil {
				return err
			}

			return cobra.MinimumNArgs(1)(cmd, args)
		},
		RunE: importFunc,
	}
	errNameRequire = errors.New("name is required for importing a airgap package")
)

func importFunc(cmd *cobra.Command, args []string) error {
	path := args[0]
	var name string
	var err error
	if len(args) < 2 {
		name, err = askForName()
		if err != nil {
			return err
		}
	} else {
		name = args[1]
	}

	if name == "" {
		return errNameRequire
	}
	tmpPath, err := pkgairgap.SaveToTmp(path, name)
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpPath)

	toSave, err := pkgairgap.VerifyFiles(tmpPath)
	if err != nil {
		return err
	}

	toSave.Name = name
	toSave.FilePath = pkgairgap.PackagePath(name)
	if err := os.Rename(tmpPath, toSave.FilePath); err != nil {
		return err
	}
	toSave.State = common.PackageActive
	if err := common.DefaultDB.SavePackage(*toSave); err != nil {
		_ = os.RemoveAll(toSave.FilePath)
		return err
	}
	cmd.Printf("package %s imported from %s\n", name, path)
	return nil
}

func askForName() (string, error) {
	rtn := ""
	err := survey.AskOne(&survey.Input{
		Message: "Please input the package name",
	}, &rtn, survey.WithValidator(func(ans interface{}) error {
		name, _ := ans.(string)
		return validateName(name)
	}))
	return rtn, err
}
