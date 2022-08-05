package airgap

import (
	"fmt"

	pkgairgap "github.com/cnrancher/autok3s/pkg/airgap"
	"github.com/cnrancher/autok3s/pkg/common"

	"github.com/AlecAivazis/survey/v2"
	"k8s.io/apimachinery/pkg/util/validation"
)

var (
	airgapFlags = flags{}
)

type flags struct {
	isForce    bool
	isJSON     bool
	K3sVersion string
	Archs      []string
}

func getArchSelect(def []string) *survey.MultiSelect {
	return &survey.MultiSelect{
		Default: def,
		Message: "What arch do you prefer?",
		Options: pkgairgap.GetValidatedArchs(),
	}
}

func validateName(name string) error {
	if name == "" {
		return errNameRequire
	}
	pkgs, _ := common.DefaultDB.ListPackages(&name)
	if len(pkgs) > 0 {
		return fmt.Errorf("package name %s already exists", name)
	}
	if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
		return fmt.Errorf("name is not validated %s, %v", name, errs)
	}
	return nil
}
