package pkg

import (
	"bytes"
	"net/http"
	"os"

	"github.com/cnrancher/autok3s/pkg/airgap"
	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/settings"

	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"github.com/sirupsen/logrus"
)

func ActionHandlers() map[string]http.Handler {
	return map[string]http.Handler{
		"import":                http.HandlerFunc(importHandler),
		"update-install-script": http.HandlerFunc(updateInstallScript),
	}
}

func LinkHandlers() map[string]http.Handler {
	return map[string]http.Handler{
		"export": http.HandlerFunc(exportHandler),
	}
}

func Format(request *types.APIRequest, resource *types.RawResource) {
	state := resource.APIObject.Data().String("state")
	if state == string(common.PackageActive) {
		resource.Links["export"] = request.URLBuilder.Link(request.Schema, request.Name, "export")
	}
}

func CollectionFormat(request *types.APIRequest, collection *types.GenericCollection) {
	collection.AddAction(request, "import")
	collection.AddAction(request, "update-install-script")
}

func importHandler(w http.ResponseWriter, r *http.Request) {
	apiContext := types.GetAPIContext(r.Context())
	defer r.Body.Close()
	name := apiContext.Query.Get("name")
	if name == "" {
		apiContext.WriteError(apierror.NewFieldAPIError(validation.MissingRequired, "name", ""))
		return
	}
	if err := common.DefaultDB.PackageExists(name); err == nil {
		apiContext.WriteError(validation.Conflict)
	}
	r.ParseMultipartForm(32 << 20)
	file, _, err := r.FormFile("package")
	if err != nil {
		apiContext.WriteError(apierror.WrapAPIError(err, validation.InvalidBodyContent, ""))
		return
	}
	defer file.Close()

	path, err := airgap.ReadToTmp(file, name)
	if err != nil {
		apiContext.WriteError(apierror.WrapAPIError(err, validation.InvalidFormat, "failed to decode tar.gz format package"))
		return
	}
	defer os.RemoveAll(path)
	targetPackage, err := airgap.VerifyFiles(path)
	if err != nil {
		apiContext.WriteError(apierror.WrapAPIError(err, validation.InvalidFormat, "file content is not valid"))
		return
	}
	targetPackage.Name = name
	newPath := airgap.PackagePath(name)
	targetPackage.FilePath = newPath
	targetPackage.State = common.PackageActive
	defer func() {
		if err != nil {
			os.RemoveAll(newPath)
		}
	}()
	if err = os.Rename(path, newPath); err != nil {
		apiContext.WriteError(apierror.WrapAPIError(err, validation.ServerError, "failed to move tmp package to static package"))
		return
	}
	if err = common.DefaultDB.SavePackage(*targetPackage); err != nil {
		apiContext.WriteError(apierror.WrapAPIError(err, validation.ServerError, "failed to save package record"))
		return
	}
	rtnObj := common.GetAPIObject(targetPackage)
	apiContext.WriteResponse(http.StatusOK, *rtnObj)
}

func exportHandler(w http.ResponseWriter, r *http.Request) {
	apiContext := types.GetAPIContext(r.Context())
	defer r.Body.Close()
	name := apiContext.Name
	pkgs, err := common.DefaultDB.ListPackages(&name)
	if err != nil {
		apiContext.WriteError(apierror.WrapAPIError(err, validation.ServerError, "failed to get package"))
		return
	}
	current := pkgs[0]
	if current.State != common.PackageActive {
		apiContext.WriteError(apierror.NewAPIError(validation.InvalidAction, "package is not in active state"))
		return
	}
	from := current.FilePath
	w.Header().Set("Content-Type", r.Header.Get("Content-Type"))
	w.Header().Set("Content-Disposition", "attachment; filename="+current.Name+".tar.gz")
	if err := airgap.TarAndGzipToWriter(from, w); err != nil {
		logrus.Warnf("failed to write response with tar.gz file %v", err)
	}
}

func updateInstallScript(w http.ResponseWriter, r *http.Request) {
	apiContext := types.GetAPIContext(r.Context())
	defer r.Body.Close()
	buff := bytes.NewBuffer([]byte{})
	if err := settings.GetScriptFromSource(buff); err != nil {
		apiContext.WriteError(apierror.WrapAPIError(err, validation.ServerError, "failed to get script from source"))
		return
	}
	if err := settings.InstallScript.Set(buff.String()); err != nil {
		apiContext.WriteError(apierror.WrapAPIError(err, validation.ServerError, "failed to update stored install script"))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("{}"))
}
