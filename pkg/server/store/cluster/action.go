package cluster

import (
	"context"
	"encoding/json"
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
	actionJoin            = "join"
	linkNodes             = "nodes"
	actionEnableExplorer  = "enable-explorer"
	actionDisableExplorer = "disable-explorer"
)

// Formatter cluster's formatter.
func Formatter(request *types.APIRequest, resource *types.RawResource) {
	resource.Links[linkNodes] = request.URLBuilder.Link(resource.Schema, resource.ID, linkNodes)
	resource.AddAction(request, actionJoin)
}

// HandleCluster cluster's action handler.
func HandleCluster() map[string]http.Handler {
	return map[string]http.Handler{
		actionJoin:            joinHandler(),
		actionEnableExplorer:  enableExplorer(),
		actionDisableExplorer: disableExplorer(),
	}
}

// LinkCluster cluster's link handler.
func LinkCluster(request *types.APIRequest) (types.APIObject, error) {
	if request.Link == linkNodes {
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
			_, _ = rw.Write([]byte("clusterID cannot be empty"))
			return
		}
		state, err := common.DefaultDB.GetClusterByID(clusterID)
		if err != nil || state == nil {
			rw.WriteHeader(http.StatusNotFound)
			_, _ = rw.Write([]byte(fmt.Sprintf("cluster %s is not found", clusterID)))
			return
		}
		provider, err := providers.GetProvider(state.Provider)
		if err != nil {
			rw.WriteHeader(http.StatusNotFound)
			_, _ = rw.Write([]byte(fmt.Sprintf("provider %s is not found", state.Provider)))
			return
		}
		provider.SetMetadata(&state.Metadata)
		_ = provider.SetOptions(state.Options)

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			_, _ = rw.Write([]byte(err.Error()))
			return
		}
		err = provider.SetConfig(body)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			_, _ = rw.Write([]byte(err.Error()))
			return
		}
		err = provider.MergeClusterOptions()
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			_, _ = rw.Write([]byte(err.Error()))
			return
		}
		id := provider.GenerateClusterName()
		if err = provider.JoinCheck(); err != nil {
			rw.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = rw.Write([]byte(err.Error()))
			return
		}

		provider.RegisterCallbacks(id, "update", common.DefaultDB.BroadcastObject)
		go func() {
			err := provider.JoinK3sNode()
			if err != nil {
				logrus.Errorf("join cluster error: %v", err)
			}
		}()

		rw.WriteHeader(http.StatusOK)
	})
}

func nodesHandler(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	state, err := common.DefaultDB.GetClusterByID(id)
	if err != nil || state == nil {
		// find from failed cluster.
		return types.APIObject{}, apierror.NewAPIError(validation.NotFound, fmt.Sprintf("cluster %s is not found, got error: %v", id, err))
	}
	provider, err := providers.GetProvider(state.Provider)
	if err != nil {
		return types.APIObject{}, apierror.NewAPIError(validation.NotFound, err.Error())
	}
	provider.SetMetadata(&state.Metadata)
	_ = provider.SetOptions(state.Options)
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

func enableExplorer() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		clusterID := vars["name"]
		if clusterID == "" {
			e := map[string]interface{}{
				"type":    "error",
				"message": "clusterID cannot be empty",
			}
			eb, _ := json.Marshal(e)
			rw.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = rw.Write(eb)
			return
		}
		port, err := common.EnableExplorer(context.Background(), clusterID)
		if err != nil {
			e := map[string]interface{}{
				"type":    "error",
				"message": err.Error(),
			}
			eb, _ := json.Marshal(e)
			rw.WriteHeader(http.StatusInternalServerError)
			_, _ = rw.Write(eb)
			return
		}
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte(fmt.Sprintf("{\"data\": \"%s\"}", fmt.Sprintf("kube-explorer for cluster %s will listen on 127.0.0.1:%d...", clusterID, port))))
	})
}

func disableExplorer() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		clusterID := vars["name"]
		if clusterID == "" {
			rw.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = rw.Write([]byte("clusterID cannot be empty"))
			return
		}
		if err := common.DisableExplorer(clusterID); err != nil {
			rw.WriteHeader(http.StatusNotFound)
			_, _ = rw.Write([]byte(err.Error()))
			return
		}
		rw.WriteHeader(http.StatusOK)
	})
}
