package cluster

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/providers"
	autok3stypes "github.com/cnrancher/autok3s/pkg/types/apis"

	"github.com/gorilla/mux"
	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

const (
	actionJoin               = "join"
	linkNodes                = "nodes"
	actionEnableExplorer     = "enable-explorer"
	actionDisableExplorer    = "disable-explorer"
	actionDownloadKubeconfig = "download-kubeconfig"
)

// Formatter cluster's formatter.
func Formatter(request *types.APIRequest, resource *types.RawResource) {
	resource.Links[linkNodes] = request.URLBuilder.Link(resource.Schema, resource.ID, linkNodes)
	resource.AddAction(request, actionJoin)
}

// HandleCluster cluster's action handler.
func HandleCluster() map[string]http.Handler {
	kubeconfigAction := downloadKubeconfig{}
	explorerAction := explorer{}
	joinAction := join{}
	return map[string]http.Handler{
		actionJoin:               joinAction,
		actionEnableExplorer:     explorerAction,
		actionDisableExplorer:    explorerAction,
		actionDownloadKubeconfig: kubeconfigAction,
	}
}

// LinkCluster cluster's link handler.
func LinkCluster(request *types.APIRequest) (types.APIObject, error) {
	if request.Link == linkNodes {
		return nodesHandler(request, request.Schema, request.Name)
	}

	return request.Schema.Store.ByID(request, request.Schema, request.Name)
}

type join struct{}

func (j join) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	apiRequest := types.GetAPIContext(req.Context())
	clusterID := apiRequest.Name
	if clusterID == "" {
		apiRequest.WriteError(apierror.NewAPIError(validation.InvalidOption, "clusterID cannot be empty"))
		return
	}
	state, err := common.DefaultDB.GetClusterByID(clusterID)
	if err != nil || state == nil {
		apiRequest.WriteError(apierror.NewAPIError(validation.NotFound, fmt.Sprintf("cluster %s is not found", clusterID)))
		return
	}
	provider, err := providers.GetProvider(state.Provider)
	if err != nil {
		apiRequest.WriteError(apierror.NewAPIError(validation.NotFound, fmt.Sprintf("provider %s is not found", state.Provider)))
		return
	}
	provider.SetMetadata(&state.Metadata)
	_ = provider.SetOptions(state.Options)
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		apiRequest.WriteError(apierror.NewAPIError(validation.ServerError, err.Error()))
		return
	}
	err = provider.SetConfig(body)
	if err != nil {
		apiRequest.WriteError(apierror.NewAPIError(validation.ServerError, err.Error()))
		return
	}
	err = provider.MergeClusterOptions()
	if err != nil {
		apiRequest.WriteError(apierror.NewAPIError(validation.ServerError, err.Error()))
		return
	}
	id := provider.GenerateClusterName()
	if err = provider.JoinCheck(); err != nil {
		apiRequest.WriteError(apierror.NewAPIError(validation.InvalidOption, err.Error()))
		return
	}

	provider.RegisterCallbacks(id, "update", common.DefaultDB.BroadcastObject)
	go func() {
		err := provider.JoinK3sNode()
		if err != nil {
			logrus.Errorf("join cluster error: %v", err)
		}
	}()
	apiRequest.WriteResponse(http.StatusOK, types.APIObject{})
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
	kubeCfg := filepath.Join(common.CfgPath, common.KubeCfgFile)
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

type explorer struct{}

func (e explorer) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	apiRequest := types.GetAPIContext(req.Context())
	clusterID := apiRequest.Name
	if clusterID == "" {
		apiRequest.WriteError(apierror.NewAPIError(validation.InvalidOption, "clusterID cannot be empty"))
		return
	}
	action := apiRequest.Action
	switch action {
	case actionEnableExplorer:
		port, err := common.EnableExplorer(context.Background(), clusterID)
		if err != nil {
			apiRequest.WriteError(apierror.NewAPIError(validation.ServerError, err.Error()))
			return
		}
		apiRequest.WriteResponse(http.StatusOK, types.APIObject{
			Type: "enableExplorerOutput",
			Object: &autok3stypes.EnableExplorerOutput{
				Data: fmt.Sprintf("kube-explorer for cluster %s will listen on 127.0.0.1:%d...", clusterID, port),
			},
		})
	case actionDisableExplorer:
		err := common.DisableExplorer(clusterID)
		if err != nil {
			apiRequest.WriteError(apierror.NewAPIError(validation.ServerError, err.Error()))
			return
		}
		apiRequest.WriteResponse(http.StatusOK, types.APIObject{})
	default:
		apiRequest.WriteError(apierror.NewAPIError(validation.ActionNotAvailable, fmt.Sprintf("invalid action %s", action)))
	}
}

type downloadKubeconfig struct{}

func (d downloadKubeconfig) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	apiRequest := types.GetAPIContext(req.Context())
	vars := mux.Vars(req)
	clusterID := vars["name"]
	if clusterID == "" {
		apiRequest.WriteError(apierror.NewAPIError(validation.InvalidOption, "clusterID cannot be empty"))
		return
	}
	kubeconfigPath := filepath.Join(common.CfgPath, common.KubeCfgFile)
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{
			CurrentContext: clusterID,
		}).RawConfig()
	if err != nil {
		apiRequest.WriteError(apierror.NewAPIError(validation.ServerError, err.Error()))
		return
	}
	// generate current config for cluster
	currentCfg := api.Config{
		Kind:           cfg.Kind,
		APIVersion:     cfg.APIVersion,
		Preferences:    cfg.Preferences,
		CurrentContext: clusterID,
	}
	if clusterCfg, ok := cfg.Clusters[clusterID]; ok {
		currentCfg.Clusters = map[string]*api.Cluster{
			clusterID: clusterCfg,
		}
	}
	if authCfg, ok := cfg.AuthInfos[clusterID]; ok {
		currentCfg.AuthInfos = map[string]*api.AuthInfo{
			clusterID: authCfg,
		}
	}
	if contextCfg, ok := cfg.Contexts[clusterID]; ok {
		currentCfg.Contexts = map[string]*api.Context{
			clusterID: contextCfg,
		}
	}
	if extensionCfg, ok := cfg.Extensions[clusterID]; ok {
		currentCfg.Extensions = map[string]runtime.Object{
			clusterID: extensionCfg,
		}
	}

	result, err := clientcmd.Write(currentCfg)
	if err != nil {
		apiRequest.WriteError(apierror.NewAPIError(validation.ServerError, err.Error()))
		return
	}

	apiRequest.WriteResponse(http.StatusOK, types.APIObject{
		Type: "kubeconfigOutput",
		Object: &autok3stypes.KubeconfigOutput{
			Config: string(result),
		},
	})
}
