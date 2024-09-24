package airgap

import (
	"fmt"
	"sort"
	"strings"

	pkgairgap "github.com/cnrancher/autok3s/pkg/airgap"
	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/AlecAivazis/survey/v2"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

var updateCmd = &cobra.Command{
	Use:   "update <name>",
	Short: "Update a stored package with new selected archs.",
	Args:  cobra.ExactArgs(1),
	PreRun: func(_ *cobra.Command, _ []string) {
		sort.Strings(airgapFlags.Archs)
	},
	RunE: update,
}

func init() {
	updateCmd.Flags().StringVarP(&airgapFlags.K3sVersion, "k3s-version", "v", airgapFlags.K3sVersion, "The version of k3s to store airgap resources.")
	updateCmd.Flags().StringArrayVar(&airgapFlags.Archs, "arch", airgapFlags.Archs, "The archs of the k3s version. Following archs are support: "+strings.Join(pkgairgap.GetValidatedArchs(), ",")+".")
	updateCmd.Flags().BoolVarP(&airgapFlags.isForce, "force", "f", false, "Force update without comfirm and skip state check")
}

func update(cmd *cobra.Command, args []string) error {
	name := args[0]
	pkgs, err := common.DefaultDB.ListPackages(&name)
	if err == gorm.ErrRecordNotFound {
		return fmt.Errorf("package %s not found", name)
	}
	if err != nil {
		return err
	}
	toUpdate := pkgs[0]

	versionChanged := airgapFlags.K3sVersion != "" && airgapFlags.K3sVersion != toUpdate.K3sVersion

	if len(airgapFlags.Archs) == 0 {
		if !utils.IsTerm() {
			return errors.New("at least one arch is required for updating airgap package")
		}
		if err := survey.AskOne(getArchSelect(toUpdate.Archs), &airgapFlags.Archs); err != nil {
			return err
		}
	}
	add, del := pkgairgap.GetArchDiff(toUpdate.Archs, airgapFlags.Archs)

	if !airgapFlags.isForce && !utils.IsTerm() {
		if versionChanged {
			return errors.New("k3s version is changed, you must use -f flag to force update")
		}
		if len(del) != 0 {
			return fmt.Errorf("going to delete arch(s) %s, you must use -f flag to force update", strings.Join(del, ","))
		}
	}

	if !airgapFlags.isForce && utils.IsTerm() {
		if versionChanged &&
			!utils.AskForConfirmation(fmt.Sprintf("New k3s version %s is summitted, old version package will be removed.", airgapFlags.K3sVersion), false) {
			return nil
		}
		if len(del) != 0 &&
			!utils.AskForConfirmation(fmt.Sprintf("Are you going to delete arch(s) %s", strings.Join(del, ",")), false) {
			return nil
		}
	}
	if !versionChanged && len(add) == 0 && len(del) == 0 {
		if toUpdate.State == common.PackageActive && !airgapFlags.isForce {
			cmd.Println("package not changed")
			return nil
		}
	} else {
		toUpdate.Archs = airgapFlags.Archs
		toUpdate.State = common.PackageOutOfSync
		if versionChanged {
			toUpdate.K3sVersion = airgapFlags.K3sVersion
		}
		if err := common.DefaultDB.SavePackage(toUpdate); err != nil {
			return err
		}
		cmd.Printf("package %s of k3s version %s updated with arch(s) %s\n", name, toUpdate.K3sVersion, strings.Join(toUpdate.Archs, ","))
	}

	if err := pkgairgap.DownloadPackage(toUpdate, nil); err != nil {
		return errors.Wrapf(err, "failed to download package %s", toUpdate.Name)
	}

	cmd.Println("package updated")
	return nil
}
