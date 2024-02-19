package explorer

import (
	"fmt"
	"strings"

	"github.com/cnrancher/autok3s/pkg/common"

	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/apiserver/pkg/store/empty"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/v2/pkg/schemas/validation"
)

// Store holds explorer's API state.
type Store struct {
	empty.Store
}

// ByID returns explorer setting by ID.
func (s *Store) ByID(_ *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	exp, err := common.DefaultDB.GetExplorer(id)
	if err != nil {
		return types.APIObject{}, err
	}
	if exp == nil {
		return types.APIObject{}, apierror.NewAPIError(validation.NotFound, fmt.Sprintf("cluster %s is not enable kube-explorer", id))
	}
	return types.APIObject{
		Type:   schema.ID,
		ID:     id,
		Object: exp,
	}, nil
}

// List returns all K3s explorer settings
func (s *Store) List(_ *types.APIRequest, schema *types.APISchema) (types.APIObjectList, error) {
	result := types.APIObjectList{}
	expList, err := common.DefaultDB.ListExplorer()
	if err != nil {
		return result, err
	}
	for _, exp := range expList {
		result.Objects = append(result.Objects, types.APIObject{
			Type:   schema.ID,
			ID:     exp.ContextName,
			Object: exp,
		})
	}
	return result, nil
}

// Watch explorer settings change
func (s *Store) Watch(apiOp *types.APIRequest, schema *types.APISchema, _ types.WatchRequest) (chan types.APIEvent, error) {
	return common.DefaultDB.Watch(apiOp, schema), nil
}

type Formatter struct {
}

func NewFormatter() *Formatter {
	return &Formatter{}
}

func (f *Formatter) Formatter(request *types.APIRequest, resource *types.RawResource) {
	if resource.APIObject.Data().Bool("enabled") {
		proxyLink := request.URLBuilder.Link(resource.Schema, resource.APIObject.ID, "explorer")
		proxyLink = strings.Replace(proxyLink, "/v1/explorers", "/proxy/explorer", 1)
		proxyLink = strings.Replace(proxyLink, "?link=explorer", "", 1)
		resource.Links["explorer"] = proxyLink
	}
}
