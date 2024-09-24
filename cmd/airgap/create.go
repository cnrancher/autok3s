package airgap

import (
	"sort"
	"strings"

	pkgairgap "github.com/cnrancher/autok3s/pkg/airgap"
	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/AlecAivazis/survey/v2"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new airgap package and will download related resources from internet.",
	Args:  cobra.ExactArgs(1),
	PreRun: func(_ *cobra.Command, _ []string) {
		sort.Strings(airgapFlags.Archs)
	},
	RunE: create,
}

func init() {
	createCmd.Flags().StringVarP(&airgapFlags.K3sVersion, "k3s-version", "v", airgapFlags.K3sVersion, "The version of k3s to store airgap resources.")
	createCmd.Flags().StringArrayVar(&airgapFlags.Archs, "arch", airgapFlags.Archs, "The archs of the k3s version. Following archs are support: "+strings.Join(pkgairgap.GetValidatedArchs(), ",")+".")
}

func create(cmd *cobra.Command, args []string) error {
	name := args[0]
	if err := validateName(name); err != nil {
		return err
	}

	var qs []*survey.Question

	if airgapFlags.K3sVersion == "" {
		if !utils.IsTerm() {
			return errors.New("k3s-version flags is required")
		}
		qs = append(qs, &survey.Question{
			Name:     "k3sVersion",
			Prompt:   &survey.Input{Message: "K3s Version?"},
			Validate: survey.Required,
		})
	}

	if len(airgapFlags.Archs) == 0 {
		if !utils.IsTerm() {
			return errors.New("at least one arch should be specified")
		}
		qs = append(qs, &survey.Question{
			Name:     "archs",
			Prompt:   getArchSelect([]string{}),
			Validate: survey.Required,
		})
	}

	if err := survey.Ask(qs, &airgapFlags); err != nil {
		return err
	}

	if err := pkgairgap.ValidateArchs(airgapFlags.Archs); err != nil {
		return err
	}

	pkg := common.Package{
		Name:       name,
		K3sVersion: airgapFlags.K3sVersion,
		Archs:      airgapFlags.Archs,
		State:      common.PackageOutOfSync,
	}

	if err := common.DefaultDB.SavePackage(pkg); err != nil {
		return err
	}
	cmd.Printf("airgap package %s record created, prepare to download\n", pkg.Name)
	if err := pkgairgap.DownloadPackage(pkg, nil); err != nil {
		return errors.Wrapf(err, "failed to download package %s", pkg.Name)
	}

	cmd.Printf("airgap package %s created and stored in path %s\n", name, pkgairgap.PackagePath(pkg.Name))
	return nil
}
