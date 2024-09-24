package addon

import (
	"bytes"
	"errors"
	"reflect"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/rancher/apiserver/pkg/store/empty"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/v2/pkg/data/convert"
	"github.com/sirupsen/logrus"
)

type Store struct {
	empty.Store
}

func (a *Store) Create(apiOp *types.APIRequest, schema *types.APISchema, data types.APIObject) (types.APIObject, error) {
	input := &common.Addon{}
	err := convert.ToObj(data.Data(), input)
	if err != nil {
		return types.APIObject{}, err
	}

	// validate creation
	if err := common.ValidateName(input.Name); err != nil {
		return types.APIObject{}, err
	}
	if len(input.Manifest) == 0 {
		return types.APIObject{}, errors.New("manifest file content cannot be empty")
	}

	addon := &common.Addon{
		Name:        input.Name,
		Description: input.Description,
		Manifest:    input.Manifest,
		Values:      input.Values,
	}
	err = common.DefaultDB.SaveAddon(addon)
	if err != nil {
		return types.APIObject{}, err
	}
	return a.ByID(apiOp, schema, addon.Name)
}

func (a *Store) List(_ *types.APIRequest, schema *types.APISchema) (types.APIObjectList, error) {
	result := types.APIObjectList{}
	list, err := common.DefaultDB.ListAddon()
	if err != nil {
		return result, err
	}

	for _, addon := range list {
		result.Objects = append(result.Objects, types.APIObject{
			ID:     addon.Name,
			Type:   schema.ID,
			Object: addon,
		})
	}

	return result, nil
}

func (a *Store) ByID(_ *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	addon, err := common.DefaultDB.GetAddon(id)
	if err != nil {
		return types.APIObject{}, err
	}
	return types.APIObject{
		ID:     addon.Name,
		Type:   schema.ID,
		Object: addon,
	}, nil
}

func (a *Store) Update(apiOp *types.APIRequest, schema *types.APISchema, data types.APIObject, id string) (types.APIObject, error) {
	input := &common.Addon{}
	err := convert.ToObj(data.Data(), input)
	if err != nil {
		return types.APIObject{}, err
	}

	addon, err := common.DefaultDB.GetAddon(id)
	if err != nil {
		return types.APIObject{}, err
	}

	manifest := addon.Manifest
	if len(input.Manifest) != 0 {
		manifest = input.Manifest
	}

	isChanged := false
	if input.Description != "" && input.Description != addon.Description {
		addon.Description = input.Description
		isChanged = true
	}

	if !bytes.Equal(manifest, addon.Manifest) {
		addon.Manifest = manifest
		isChanged = true
	}

	if !reflect.DeepEqual(input.Values, addon.Values) {
		addon.Values = input.Values
		isChanged = true
	}

	if isChanged {
		if err := common.DefaultDB.SaveAddon(addon); err != nil {
			logrus.Fatalln(err)
		}
	}
	return a.ByID(apiOp, schema, id)
}

func (a *Store) Delete(_ *types.APIRequest, _ *types.APISchema, id string) (types.APIObject, error) {
	err := common.DefaultDB.DeleteAddon(id)
	return types.APIObject{}, err
}

func (a *Store) Watch(apiOp *types.APIRequest, schema *types.APISchema, _ types.WatchRequest) (chan types.APIEvent, error) {
	return common.DefaultDB.Watch(apiOp, schema), nil
}
