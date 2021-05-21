package template

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/providers"
	"github.com/cnrancher/autok3s/pkg/types/apis"

	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/apiserver/pkg/store/empty"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/pkg/data/convert"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"github.com/sirupsen/logrus"
)

type Store struct {
	empty.Store
}

func (t *Store) Create(apiOp *types.APIRequest, schema *types.APISchema, data types.APIObject) (types.APIObject, error) {
	template := &apis.ClusterTemplate{}
	err := convert.ToObj(data.Data(), template)
	if err != nil {
		return types.APIObject{}, err
	}
	temp, err := common.DefaultDB.GetTemplate(template.Name, template.Provider)
	if err != nil {
		return types.APIObject{}, err
	}
	if temp != nil {
		return types.APIObject{}, apierror.NewAPIError(validation.Conflict, fmt.Sprintf("template %s for provider %s is already exist", template.Name, template.Provider))
	}
	template.ContextName = fmt.Sprintf("%s.%s", template.Name, template.Provider)
	opt, err := json.Marshal(template.Options)
	if err != nil {
		return types.APIObject{}, err
	}
	temp = &common.Template{
		Metadata:  template.Metadata,
		SSH:       template.SSH,
		Options:   opt,
		IsDefault: template.IsDefault,
	}
	err = common.DefaultDB.CreateTemplate(temp)
	if err != nil {
		return types.APIObject{}, err
	}
	return t.ByID(apiOp, schema, template.ContextName)
}

func (t *Store) List(apiOp *types.APIRequest, schema *types.APISchema) (types.APIObjectList, error) {
	result := types.APIObjectList{}
	templates, err := common.DefaultDB.ListTemplates()
	if err != nil {
		return result, err
	}

	for _, template := range templates {
		temp := &apis.ClusterTemplate{
			Metadata:  template.Metadata,
			SSH:       template.SSH,
			IsDefault: template.IsDefault,
		}
		provider, err := providers.GetProvider(template.Provider)
		if err != nil {
			logrus.Errorf("failed to get provider by name %s: %v", template.Provider, err)
			continue
		}
		opt, err := provider.GetProviderOptions(template.Options)
		if err != nil {
			var status string
			if e, ok := err.(*json.UnmarshalTypeError); ok {
				status = fmt.Sprintf("field %s.%s is no longer %s but is %s type, please change the config", e.Struct, e.Field, e.Value, e.Type.String())
			} else {
				status = fmt.Sprintf("convert %s.Options error: %v", template.Name, err)
			}
			logrus.Errorf("failed to get convert template %s options by provider %s: %v", template.Name, template.Provider, err)
			temp.Status = status
		}
		temp.Options = opt
		result.Objects = append(result.Objects, types.APIObject{
			ID:     template.ContextName,
			Type:   schema.ID,
			Object: temp,
		})
	}
	return result, nil
}

func (t *Store) ByID(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	context := strings.Split(id, ".")
	if len(context) != 2 {
		return types.APIObject{}, apierror.NewAPIError(validation.InvalidOption, fmt.Sprintf("invalid template id %s", id))
	}
	template, err := common.DefaultDB.GetTemplate(context[0], context[1])
	if err != nil {
		return types.APIObject{}, err
	}
	if template == nil {
		return types.APIObject{}, apierror.NewAPIError(validation.NotFound, fmt.Sprintf("template %s of provider %s is not exist", context[0], context[1]))
	}
	temp := &apis.ClusterTemplate{
		Metadata:  template.Metadata,
		SSH:       template.SSH,
		IsDefault: template.IsDefault,
	}
	provider, err := providers.GetProvider(template.Provider)
	if err != nil {
		return types.APIObject{}, apierror.NewAPIError(validation.NotFound, err.Error())
	}
	opt, err := provider.GetProviderOptions(template.Options)
	if err != nil {
		return types.APIObject{}, err
	}
	temp.Options = opt
	return types.APIObject{
		ID:     template.ContextName,
		Type:   schema.ID,
		Object: temp,
	}, nil
}

func (t *Store) Update(apiOp *types.APIRequest, schema *types.APISchema, data types.APIObject, id string) (types.APIObject, error) {
	template := &apis.ClusterTemplate{}
	err := convert.ToObj(data.Data(), template)
	if err != nil {
		return types.APIObject{}, err
	}
	temp := &common.Template{
		Metadata:  template.Metadata,
		SSH:       template.SSH,
		IsDefault: template.IsDefault,
	}
	temp.ContextName = fmt.Sprintf("%s.%s", template.Name, template.Provider)
	opt, err := json.Marshal(template.Options)
	if err != nil {
		return types.APIObject{}, err
	}
	temp.Options = opt
	err = common.DefaultDB.UpdateTemplate(temp)
	if err != nil {
		return types.APIObject{}, err
	}
	return t.ByID(apiOp, schema, id)
}

func (t *Store) Delete(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	context := strings.Split(id, ".")
	if len(context) != 2 {
		return types.APIObject{}, apierror.NewAPIError(validation.InvalidOption, fmt.Sprintf("invalid template id %s", id))
	}
	err := common.DefaultDB.DeleteTemplate(context[0], context[1])
	return types.APIObject{}, err
}

func (t *Store) Watch(apiOp *types.APIRequest, schema *types.APISchema, w types.WatchRequest) (chan types.APIEvent, error) {
	var (
		result = make(chan types.APIEvent)
	)

	go common.DefaultDB.WatchTemplate(apiOp, schema, result)

	go func() {
		<-apiOp.Context().Done()
		close(result)
	}()

	return result, nil
}
