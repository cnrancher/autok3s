package provider

import (
	"github.com/cnrancher/autok3s/pkg/providers"
	"github.com/cnrancher/autok3s/pkg/server/store/utils"
	autok3stypes "github.com/cnrancher/autok3s/pkg/types/apis"

	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/apiserver/pkg/store/empty"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"github.com/sirupsen/logrus"
)

type Store struct {
	empty.Store
}

func (p *Store) ByID(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	provider, err := providers.GetProvider(id)
	if err != nil {
		return types.APIObject{}, apierror.NewAPIError(validation.NotFound, err.Error())
	}
	return toProviderObject(provider, schema, id)
}

func (p *Store) List(apiOp *types.APIRequest, schema *types.APISchema) (types.APIObjectList, error) {
	list := providers.ListProviders()
	result := types.APIObjectList{}
	for _, object := range list {
		provider, err := providers.GetProvider(object.Name)
		if err != nil {
			logrus.Errorf("provider %s is not exist: %v", object.Name, err)
			continue
		}
		obj, err := toProviderObject(provider, schema, object.Name)
		if err != nil {
			logrus.Errorf("get provider config error: %v", err)
			continue
		}
		result.Objects = append(result.Objects, obj)
	}
	return result, nil
}

func toProviderObject(provider providers.Provider, schema *types.APISchema, id string) (types.APIObject, error) {
	// get provider options
	options, err := provider.GetProviderOption()
	if err != nil {
		return types.APIObject{}, err
	}

	// get credential flag and value
	opt, err := utils.GetCredentialByProvider(provider)
	if err != nil {
		return types.APIObject{}, err
	}
	for k, v := range opt {
		options[k] = v
	}
	// get cluster config
	config, err := provider.GetClusterConfig()
	if err != nil {
		return types.APIObject{}, err
	}
	obj := types.APIObject{
		Type: schema.ID,
		ID:   id,
		Object: autok3stypes.Provider{
			Name:    id,
			Options: options,
			Config:  config,
		},
	}
	return obj, nil
}
