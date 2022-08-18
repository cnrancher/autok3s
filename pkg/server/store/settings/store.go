package settings

import (
	"fmt"

	"github.com/cnrancher/autok3s/pkg/common"

	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/apiserver/pkg/store/empty"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/pkg/data/convert"
	"github.com/rancher/wrangler/pkg/schemas/validation"
)

// Store holds settings's API state.
type Store struct {
	empty.Store
}

// Update settings value
func (s *Store) Update(apiOp *types.APIRequest, schema *types.APISchema, data types.APIObject, id string) (types.APIObject, error) {
	setting := &common.Setting{}
	err := convert.ToObj(data.Data(), setting)
	if err != nil {
		return types.APIObject{}, err
	}
	err = common.DefaultDB.SaveSetting(setting)
	if err != nil {
		return types.APIObject{}, err
	}
	return s.ByID(apiOp, schema, setting.Name)
}

// ByID get setting information by name
func (s *Store) ByID(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	setting, err := common.DefaultDB.GetSetting(id)
	if err != nil {
		return types.APIObject{}, err
	}
	if setting == nil {
		return types.APIObject{}, apierror.NewAPIError(validation.NotFound, fmt.Sprintf("setting %s is not found", id))
	}
	return types.APIObject{
		Type:   schema.ID,
		ID:     id,
		Object: setting,
	}, nil
}

// List all settings
func (s *Store) List(apiOp *types.APIRequest, schema *types.APISchema) (types.APIObjectList, error) {
	result := types.APIObjectList{}
	settings, err := common.DefaultDB.ListSettings()
	if err != nil {
		return result, err
	}
	for _, setting := range settings {
		result.Objects = append(result.Objects, types.APIObject{
			Type:   schema.ID,
			ID:     setting.Name,
			Object: setting,
		})
	}
	return result, nil
}

// Watch watches Settings.
func (s *Store) Watch(apiOp *types.APIRequest, schema *types.APISchema, w types.WatchRequest) (chan types.APIEvent, error) {
	return common.DefaultDB.Watch(apiOp, schema), nil
}
