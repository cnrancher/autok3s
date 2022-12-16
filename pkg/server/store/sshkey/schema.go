package sshkey

import (
	"net/http"

	"github.com/cnrancher/autok3s/pkg/common"

	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/pkg/schemas/validation"
)

func ActionHandlers() map[string]http.Handler {
	return map[string]http.Handler{
		"export": http.HandlerFunc(exportHandler),
	}
}

func exportHandler(w http.ResponseWriter, r *http.Request) {
	apiContext := types.GetAPIContext(r.Context())
	defer r.Body.Close()
	name := apiContext.Name
	sshkeys, err := common.DefaultDB.ListSSHKey(&name)
	if err != nil {
		apiContext.WriteError(apierror.WrapAPIError(err, validation.ServerError, "failed to get sshkey"))
		return
	}
	obj := common.GetAPIObject(*sshkeys[0])
	apiContext.WriteResponse(200, *obj)
	return
}

func Format(request *types.APIRequest, resource *types.RawResource) {
	resource.AddAction(request, "export")
}
