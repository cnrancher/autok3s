package cluster

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/providers"

	"github.com/gorilla/mux"
	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"github.com/sirupsen/logrus"
)

const (
	actionJoin = "join"
	linkNodes  = "nodes"
)

func Formatter(request *types.APIRequest, resource *types.RawResource) {
	resource.Links[linkNodes] = request.URLBuilder.Link(resource.Schema, resource.ID, linkNodes)
	resource.AddAction(request, actionJoin)
}

func HandleCluster() map[string]http.Handler {
	return map[string]http.Handler{
		actionJoin: joinHandler(),
	}
}

func LinkCluster(request *types.APIRequest) (types.APIObject, error) {
	if request.Link != "" {
		return nodesHandler(request, request.Schema, request.Name)
	}

	return request.Schema.Store.ByID(request, request.Schema, request.Name)
}

func joinHandler() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		clusterID := vars["name"]
		if clusterID == "" {
			rw.WriteHeader(http.StatusUnprocessableEntity)
			rw.Write([]byte("clusterID cannot be empty"))
			return
		}
		state, err := common.DefaultDB.GetClusterByID(clusterID)
		if err != nil || state == nil {
			rw.WriteHeader(http.StatusNotFound)
			rw.Write([]byte(fmt.Sprintf("cluster %s is not found", clusterID)))
		}
		provider, err := providers.GetProvider(state.Provider)
		if err != nil {
			rw.WriteHeader(http.StatusNotFound)
			rw.Write([]byte(fmt.Sprintf("provider %s is not found", state.Provider)))
		}
		provider.SetMetadata(&state.Metadata)
		provider.SetOptions(state.Options)

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			rw.Write([]byte(err.Error()))
			return
		}
		err = provider.SetConfig(body)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			rw.Write([]byte(err.Error()))
			return
		}
		err = provider.MergeClusterOptions()
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			rw.Write([]byte(err.Error()))
			return
		}
		provider.GenerateClusterName()

		go func() {
			err := provider.JoinK3sNode()
			if err != nil {
				logrus.Errorf("join cluster error: %v", err)
				provider.Rollback()
			}
		}()

		rw.WriteHeader(http.StatusOK)
	})
}

func nodesHandler(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	state, err := common.DefaultDB.GetClusterByID(id)
	if err != nil || state == nil {
		// find from failed cluster
		return types.APIObject{}, apierror.NewAPIError(validation.NotFound, fmt.Sprintf("cluster %s is not found, got error: %v", id, err))
	}
	provider, err := providers.GetProvider(state.Provider)
	if err != nil {
		return types.APIObject{}, apierror.NewAPIError(validation.NotFound, err.Error())
	}
	provider.SetMetadata(&state.Metadata)
	provider.SetOptions(state.Options)
	kubeCfg := fmt.Sprintf("%s/%s", common.CfgPath, common.KubeCfgFile)
	if state.Status == common.StatusMissing {
		kubeCfg = ""
	}
	c := provider.DescribeCluster(kubeCfg)
	return types.APIObject{
		Type:   schema.ID,
		ID:     id,
		Object: c,
	}, nil
}
