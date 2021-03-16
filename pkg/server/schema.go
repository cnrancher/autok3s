package server

import (
	"net/http"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/server/store/cluster"
	"github.com/cnrancher/autok3s/pkg/server/store/credential"
	"github.com/cnrancher/autok3s/pkg/server/store/kubectl"
	"github.com/cnrancher/autok3s/pkg/server/store/provider"
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
	s.MustImportAndCustomize(autok3stypes.Cluster{}, func(schema *types.APISchema) {
		schema.Store = &cluster.Store{}
		common.DefaultDB.Register()
		schema.CollectionMethods = []string{http.MethodGet, http.MethodPost}
		schema.ResourceMethods = []string{http.MethodGet, http.MethodDelete}
		schema.ResourceActions["join"] = wranglertypes.Action{
			Input: "cluster",
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
