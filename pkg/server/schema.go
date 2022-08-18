package server

import (
	"net/http"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/server/store/cluster"
	"github.com/cnrancher/autok3s/pkg/server/store/credential"
	"github.com/cnrancher/autok3s/pkg/server/store/explorer"
	"github.com/cnrancher/autok3s/pkg/server/store/kubectl"
	"github.com/cnrancher/autok3s/pkg/server/store/pkg"
	"github.com/cnrancher/autok3s/pkg/server/store/provider"
	"github.com/cnrancher/autok3s/pkg/server/store/settings"
	"github.com/cnrancher/autok3s/pkg/server/store/template"
	"github.com/cnrancher/autok3s/pkg/server/store/websocket"
	wkube "github.com/cnrancher/autok3s/pkg/server/store/websocket/kubectl"
	"github.com/cnrancher/autok3s/pkg/server/store/websocket/ssh"
	autok3stypes "github.com/cnrancher/autok3s/pkg/types/apis"

	"github.com/rancher/apiserver/pkg/types"
	wranglertypes "github.com/rancher/wrangler/pkg/schemas"
)

func initProvider(s *types.APISchemas) {
	s.MustImportAndCustomize(autok3stypes.Provider{}, func(schema *types.APISchema) {
		schema.Store = &provider.Store{}
		schema.CollectionMethods = []string{http.MethodGet}
		schema.ResourceMethods = []string{http.MethodGet}
	})
}

func initCluster(s *types.APISchemas) {
	s.MustImportAndCustomize(autok3stypes.KubeconfigOutput{}, nil)
	s.MustImportAndCustomize(autok3stypes.EnableExplorerOutput{}, nil)
	s.MustImportAndCustomize(autok3stypes.UpgradeInput{}, nil)
	s.MustImportAndCustomize(autok3stypes.Cluster{}, func(schema *types.APISchema) {
		schema.Store = &cluster.Store{}
		common.DefaultDB.Register()
		schema.CollectionMethods = []string{http.MethodGet, http.MethodPost}
		schema.ResourceMethods = []string{http.MethodGet, http.MethodDelete}
		schema.ResourceActions["join"] = wranglertypes.Action{
			Input: "cluster",
		}
		schema.ResourceActions["enable-explorer"] = wranglertypes.Action{
			Output: "enableExplorerOutput",
		}
		schema.ResourceActions["disable-explorer"] = wranglertypes.Action{}
		schema.ResourceActions["download-kubeconfig"] = wranglertypes.Action{
			Output: "kubeconfigOutput",
		}
		schema.ResourceActions["upgrade"] = wranglertypes.Action{
			Input: "upgradeInput",
		}
		schema.Formatter = cluster.Formatter
		schema.ActionHandlers = cluster.HandleCluster()
		schema.ByIDHandler = cluster.LinkCluster
	})
}

func initCredential(s *types.APISchemas) {
	s.MustImportAndCustomize(autok3stypes.Credential{}, func(schema *types.APISchema) {
		schema.Store = &credential.Store{}
		schema.CollectionMethods = []string{http.MethodGet, http.MethodPost}
		schema.ResourceMethods = []string{http.MethodGet, http.MethodPut, http.MethodDelete}
	})
}

func initMutual(s *types.APISchemas) {
	s.MustImportAndCustomize(autok3stypes.Mutual{}, func(schema *types.APISchema) {
		schema.CollectionMethods = []string{http.MethodGet}
		schema.ResourceMethods = []string{}
		schema.ListHandler = ssh.Handler
	})
}

func initKubeconfig(s *types.APISchemas) {
	s.MustImportAndCustomize(autok3stypes.Config{}, func(schema *types.APISchema) {
		schema.Store = &kubectl.Store{}
		schema.CollectionMethods = []string{http.MethodGet}
		schema.ResourceMethods = []string{http.MethodGet}
		schema.ByIDHandler = wkube.KubeHandler
	})
}

func initLogs(s *types.APISchemas) {
	s.MustImportAndCustomize(autok3stypes.Logs{}, func(schema *types.APISchema) {
		schema.CollectionMethods = []string{http.MethodGet}
		schema.ResourceMethods = []string{}
		schema.ListHandler = websocket.LogHandler
	})
}

func initTemplates(s *types.APISchemas) {
	s.MustImportAndCustomize(autok3stypes.ClusterTemplate{}, func(schema *types.APISchema) {
		schema.Store = &template.Store{}
		schema.CollectionMethods = []string{http.MethodGet, http.MethodPost}
		schema.ResourceMethods = []string{http.MethodGet, http.MethodDelete, http.MethodPut}

	})
}

func initExplorer(s *types.APISchemas) {
	s.MustImportAndCustomize(common.Explorer{}, func(schema *types.APISchema) {
		schema.Store = &explorer.Store{}
		formatter := explorer.NewFormatter()
		schema.Formatter = formatter.Formatter
		schema.CollectionMethods = []string{http.MethodGet}
		schema.ResourceMethods = []string{http.MethodGet}
	})
}

func initSettings(s *types.APISchemas) {
	s.MustImportAndCustomize(common.Setting{}, func(schema *types.APISchema) {
		schema.Store = &settings.Store{}
		schema.CollectionMethods = []string{http.MethodGet}
		schema.ResourceMethods = []string{http.MethodPut, http.MethodGet}
	})
}

func initPackage(s *types.APISchemas) {
	s.MustImportAndCustomize(common.Package{}, func(schema *types.APISchema) {
		schema.Store = &pkg.Store{}
		schema.CollectionMethods = []string{http.MethodGet, http.MethodPost}
		schema.ResourceMethods = []string{http.MethodGet, http.MethodDelete, http.MethodPut}
		schema.CollectionActions["import"] = wranglertypes.Action{
			Output: "package",
		}
		schema.CollectionActions["update-install-script"] = wranglertypes.Action{}
		schema.Formatter = pkg.Format
		schema.CollectionFormatter = pkg.CollectionFormat
		schema.ActionHandlers = pkg.ActionHandlers()
		schema.LinkHandlers = pkg.LinkHandlers()
	})
}
