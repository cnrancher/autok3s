package cluster

import (
	"encoding/json"
	"fmt"

	"github.com/cnrancher/autok3s/pkg/cluster"
	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/providers"
	autok3stypes "github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/types/apis"

	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/apiserver/pkg/store/empty"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"github.com/sirupsen/logrus"
)

// Store holds cluster's API state
type Store struct {
	empty.Store
}

// Create creates cluster based on the request data.
func (c *Store) Create(apiOp *types.APIRequest, schema *types.APISchema, data types.APIObject) (types.APIObject, error) {
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
	clusterList, err := cluster.ListClusters()
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
func (c *Store) ByID(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
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
	return types.APIObject{
		Type:   schema.ID,
		ID:     id,
		Object: obj,
	}, nil
}

// Delete deletes cluster by ID.
func (c *Store) Delete(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
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
	go provider.DeleteK3sCluster(true)
	return types.APIObject{}, nil
}

// Watch watches template.
func (c *Store) Watch(apiOp *types.APIRequest, schema *types.APISchema, w types.WatchRequest) (chan types.APIEvent, error) {
	var (
		result = make(chan types.APIEvent)
	)

	go common.DefaultDB.WatchCluster(apiOp, schema, result)

	go func() {
		<-apiOp.Context().Done()
		close(result)
	}()

	return result, nil
}
