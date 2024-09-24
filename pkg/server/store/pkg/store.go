package pkg

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/cnrancher/autok3s/pkg/airgap"
	"github.com/cnrancher/autok3s/pkg/common"

	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/apiserver/pkg/store/empty"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/v2/pkg/data/convert"
	"github.com/rancher/wrangler/v2/pkg/schemas/validation"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type Store struct {
	empty.Store
}

func (e *Store) Delete(_ *types.APIRequest, _ *types.APISchema, id string) (types.APIObject, error) {
	defer func() { _ = airgap.RemovePackage(id) }()
	return types.APIObject{}, common.DefaultDB.DeletePackage(id)
}

func (e *Store) ByID(_ *types.APIRequest, _ *types.APISchema, id string) (types.APIObject, error) {
	rtn, err := common.DefaultDB.ListPackages(&id)
	if err != nil {
		return types.APIObject{}, err
	}
	obj := common.GetAPIObject(rtn[0])
	return *obj, nil
}

func (e *Store) List(_ *types.APIRequest, _ *types.APISchema) (types.APIObjectList, error) {
	var rtn types.APIObjectList
	pkgs, err := common.DefaultDB.ListPackages(nil)
	if err != nil {
		return rtn, err
	}
	for _, pkg := range pkgs {
		obj := common.GetAPIObject(pkg)
		rtn.Objects = append(rtn.Objects, *obj)
	}
	return rtn, nil
}

func (e *Store) Create(_ *types.APIRequest, _ *types.APISchema, data types.APIObject) (types.APIObject, error) {
	rtn := common.Package{}
	if err := convert.ToObj(data.Object, &rtn); err != nil {
		return types.APIObject{}, err
	}

	if err := common.DefaultDB.PackageExists(rtn.Name); err == nil {
		return types.APIObject{}, apierror.NewAPIError(validation.Conflict, fmt.Sprintf("package %s already exists", rtn.Name))
	}

	if len(rtn.Archs) == 0 {
		return types.APIObject{}, apierror.NewFieldAPIError(validation.MissingRequired, "archs", "at lease one arch should be selected")
	}
	if err := airgap.ValidateArchs(rtn.Archs); err != nil {
		return types.APIObject{}, apierror.WrapFieldAPIError(err, validation.InvalidBodyContent, "archs", "arch(s) are invalid")
	}

	return saveAndDownload(rtn)
}

func (e *Store) Update(_ *types.APIRequest, _ *types.APISchema, data types.APIObject, id string) (types.APIObject, error) {
	currents, err := common.DefaultDB.ListPackages(&id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return types.APIObject{}, apierror.NewAPIError(validation.NotFound, "package not found")
		}
		return types.APIObject{}, apierror.WrapAPIError(err, validation.ServerError, "failed to get package")
	}

	current := currents[0]
	updateData := data.Data()
	k3sVersion := updateData.String("k3sVersion")
	archs := updateData.StringSlice("archs")
	sort.Strings(archs)

	changed := false
	if k3sVersion != "" && current.K3sVersion != k3sVersion {
		current.K3sVersion = k3sVersion
		changed = true
	}
	if len(archs) != 0 && !reflect.DeepEqual(archs, []string(current.Archs)) {
		if err := airgap.ValidateArchs(archs); err != nil {
			return types.APIObject{}, apierror.WrapFieldAPIError(err, validation.InvalidBodyContent, "archs", "arch(s) are invalid")
		}
		current.Archs = archs
		changed = true
	}
	if !changed {
		apiObj := common.GetAPIObject(current)
		return *apiObj, nil
	}

	return saveAndDownload(current)
}

func (e *Store) Watch(apiOp *types.APIRequest, schema *types.APISchema, _ types.WatchRequest) (chan types.APIEvent, error) {
	return common.DefaultDB.Watch(apiOp, schema), nil
}

func downloadAndUpdatepackage(pkg common.Package) {
	var logger *logrus.Logger
	if file, err := airgap.GetLogFile(pkg.Name); err != nil {
		logrus.Warnf("failed to create log file for package %s download, %v", pkg.Name, err)
	} else {
		logger = common.NewLogger(file)
	}

	if err := airgap.DownloadPackage(pkg, logger); err != nil {
		logger.Errorf("failed to download resource for package %s, %v", pkg.Name, err)
		sendEvent(pkg.Name, "Cancel")
		return
	}
	sendEvent(pkg.Name, "Downloaded")
}

func saveAndDownload(current common.Package) (types.APIObject, error) {
	current.State = common.PackageOutOfSync
	if err := common.DefaultDB.SavePackage(current); err != nil {
		return types.APIObject{}, apierror.WrapAPIError(err, validation.ServerError, "failed to save package")
	}
	apiObj := common.GetAPIObject(current)
	go downloadAndUpdatepackage(current)
	return *apiObj, nil
}
