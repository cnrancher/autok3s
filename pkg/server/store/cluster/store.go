package cluster

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cnrancher/autok3s/pkg/cluster"
	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/providers"
	autok3stypes "github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/types/apis"

	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/apiserver/pkg/store/empty"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/v2/pkg/schemas/validation"
	"github.com/sirupsen/logrus"
)

// Store holds cluster's API state
type Store struct {
	empty.Store
}

// Create creates cluster based on the request data.
func (c *Store) Create(_ *types.APIRequest, schema *types.APISchema, data types.APIObject) (types.APIObject, error) {
	providerName := data.Data().String("provider")
	b, err := json.Marshal(data.Data())
	if err != nil {
		return types.APIObject{}, err
	}
	p, err := providers.GetProvider(providerName)
	if err != nil {
		return types.APIObject{}, apierror.NewAPIError(validation.NotFound, err.Error())
	}
	err = p.SetConfig(b)
	if err != nil {
		return types.APIObject{}, apierror.NewAPIError(validation.InvalidOption, err.Error())
	}
	id := p.GenerateClusterName()
	// save credential config.
	if err = p.BindCredential(); err != nil {
		return types.APIObject{}, err
	}

	if err := p.CreateCheck(); err != nil {
		return types.APIObject{}, apierror.NewAPIError(validation.InvalidOption, err.Error())
	}
	// register log callbacks
	p.RegisterCallbacks(id, "create", common.DefaultDB.BroadcastObject)
	go func() {
		err = p.CreateK3sCluster()
		if err != nil {
			logrus.Errorf("create cluster error: %v", err)
		}
	}()

	return types.APIObject{
		Type: schema.ID,
		ID:   id,
	}, err
}

// List returns clusters as list.
func (c *Store) List(apiOp *types.APIRequest, schema *types.APISchema) (types.APIObjectList, error) {
	list := types.APIObjectList{}
	queryParams := apiOp.Request.URL.Query()
	provider := queryParams.Get("provider")
	clusterList, err := cluster.ListClusters(provider)
	if err != nil {
		return list, err
	}
	for _, config := range clusterList {
		obj := types.APIObject{
			Type:   schema.ID,
			ID:     config.ID,
			Object: config,
		}
		list.Objects = append(list.Objects, obj)
	}
	return list, nil
}

// ByID returns cluster by ID.
func (c *Store) ByID(_ *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	state, err := common.DefaultDB.GetClusterByID(id)
	if err != nil || state == nil {
		return types.APIObject{}, apierror.NewAPIError(validation.NotFound, fmt.Sprintf("cluster %s is not found, got error: %v", id, err))
	}
	provider, err := providers.GetProvider(state.Provider)
	if err != nil {
		return types.APIObject{}, apierror.NewAPIError(validation.NotFound, err.Error())
	}
	opt, err := provider.GetProviderOptions(state.Options)
	if err != nil {
		return types.APIObject{}, err
	}
	obj := apis.Cluster{
		Metadata: state.Metadata,
		Options:  opt,
		SSH:      state.SSH,
	}
	if state.Cluster {
		obj.IsHAMode = true
		obj.DataStoreType = "Embedded DB(etcd)"
	} else if obj.DataStore != "" {
		obj.IsHAMode = true
		dataStoreArray := strings.Split(obj.DataStore, "://")
		if dataStoreArray[0] == "http" {
			obj.DataStoreType = "External DB(etcd)"
		} else {
			obj.DataStoreType = fmt.Sprintf("External DB(%s)", dataStoreArray[0])
		}
	}
	return types.APIObject{
		Type:   schema.ID,
		ID:     id,
		Object: obj,
	}, nil
}

// Delete deletes cluster by ID.
func (c *Store) Delete(_ *types.APIRequest, _ *types.APISchema, id string) (types.APIObject, error) {
	state, err := common.DefaultDB.GetClusterByID(id)
	if err != nil || state == nil {
		return types.APIObject{}, apierror.NewAPIError(validation.NotFound, fmt.Sprintf("cluster %s is not found, got error: %v", id, err))
	}
	provider, err := providers.GetProvider(state.Provider)
	if err != nil {
		return types.APIObject{}, apierror.NewAPIError(validation.NotFound, err.Error())
	}
	opt, err := provider.GetProviderOptions(state.Options)
	if err != nil {
		return types.APIObject{}, err
	}
	cluster := &autok3stypes.Cluster{
		Metadata: state.Metadata,
		Options:  opt,
	}
	b, err := json.Marshal(cluster)
	if err != nil {
		return types.APIObject{}, err
	}
	err = provider.SetConfig(b)
	if err != nil {
		return types.APIObject{}, err
	}
	err = provider.MergeClusterOptions()
	if err != nil {
		return types.APIObject{}, err
	}
	provider.GenerateClusterName()
	go func() { _ = provider.DeleteK3sCluster(true) }()
	return types.APIObject{}, nil
}

// Watch watches cluster change event.
func (c *Store) Watch(apiOp *types.APIRequest, schema *types.APISchema, _ types.WatchRequest) (chan types.APIEvent, error) {
	result := make(chan types.APIEvent)
	data := common.DefaultDB.Watch(apiOp, schema)
	go func() {
		for {
			select {
			case v, ok := <-data:
				if !ok {
					continue
				}
				obj := v.Object.Object.(autok3stypes.Cluster)
				cluster := &apis.Cluster{
					Metadata: obj.Metadata,
					SSH:      obj.SSH,
					Options:  obj.Options,
					Status:   obj.Status,
				}
				if obj.Cluster {
					cluster.IsHAMode = true
					cluster.DataStoreType = "Embedded DB(etcd)"
				} else if obj.DataStore != "" {
					cluster.IsHAMode = true
					dataStoreArray := strings.Split(obj.DataStore, "://")
					if dataStoreArray[0] == "http" {
						cluster.DataStoreType = "External DB(etcd)"
					} else {
						cluster.DataStoreType = fmt.Sprintf("External DB(%s)", dataStoreArray[0])
					}
				}
				e := types.APIEvent{
					Name:         v.Name,
					ResourceType: v.ResourceType,
					Object: types.APIObject{
						Type:   schema.ID,
						ID:     v.Object.ID,
						Object: cluster,
					},
				}
				result <- e
			case <-apiOp.Context().Done():
				close(result)
				return
			}
		}
	}()
	return result, nil
}
