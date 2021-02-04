package credential

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/providers"
	"github.com/cnrancher/autok3s/pkg/server/store/utils"
	"github.com/cnrancher/autok3s/pkg/types/apis"

	"github.com/rancher/apiserver/pkg/apierror"
	"github.com/rancher/apiserver/pkg/store/empty"
	"github.com/rancher/apiserver/pkg/types"
	"github.com/rancher/wrangler/pkg/schemas/validation"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type Store struct {
	empty.Store
}

func (cred *Store) ByID(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	provider, err := providers.GetProvider(id)
	if err != nil {
		logrus.Errorf("get provider %s error: %v", id, err)
		return types.APIObject{}, apierror.NewAPIError(validation.NotFound, err.Error())
	}
	fields, err := utils.GetCredentialByProvider(provider)
	if err != nil {
		return types.APIObject{}, err
	}
	secrets := make(map[string]string, 0)
	for k, field := range fields {
		value, ok := field.Default.(string)
		if ok && value != "" {
			secrets[k] = value
		}
	}
	credential := apis.Credential{
		Provider:     id,
		SecretFields: fields,
		Secrets:      secrets,
	}
	return types.APIObject{
		Type:   schema.ID,
		ID:     id,
		Object: credential,
	}, nil
}

func (cred *Store) List(apiOp *types.APIRequest, schema *types.APISchema) (types.APIObjectList, error) {
	list := providers.ListProviders()
	result := types.APIObjectList{}
	for _, p := range list {
		provider, err := providers.GetProvider(p.Name)
		if err != nil {
			logrus.Errorf("get provider %s error: %v", p.Name, err)
			continue
		}
		fields, err := utils.GetCredentialByProvider(provider)
		if err != nil {
			return result, err
		}
		credential := apis.Credential{
			Provider:     p.Name,
			SecretFields: fields,
		}
		result.Objects = append(result.Objects, types.APIObject{
			Type:   schema.ID,
			ID:     p.Name,
			Object: credential,
		})
	}
	return result, nil
}

func (cred *Store) Create(apiOp *types.APIRequest, schema *types.APISchema, data types.APIObject) (types.APIObject, error) {
	secrets := data.Data().Map("secrets")
	if err := viper.ReadInConfig(); err != nil {
		return types.APIObject{}, err
	}
	id := data.Data().String("provider")
	for key, secret := range secrets {
		if viper.IsSet(fmt.Sprintf(common.BindPrefix, id, key)) {
			return types.APIObject{}, apierror.NewAPIError(validation.Conflict, fmt.Sprintf("you have already set credential settings for provider %s", id))
		}
		viper.Set(fmt.Sprintf(common.BindPrefix, id, key), secret)
	}
	if err := viper.WriteConfig(); err != nil {
		return types.APIObject{}, err
	}
	if err := viper.MergeInConfig(); err != nil {
		return types.APIObject{}, err
	}
	return cred.ByID(apiOp, schema, id)
}

func (cred *Store) Update(apiOp *types.APIRequest, schema *types.APISchema, data types.APIObject, id string) (types.APIObject, error) {
	if err := viper.ReadInConfig(); err != nil {
		return types.APIObject{}, err
	}
	secrets := data.Data().Map("secrets")
	for key, secret := range secrets {
		if !viper.IsSet(fmt.Sprintf(common.BindPrefix, id, key)) {
			return types.APIObject{}, apierror.NewAPIError(validation.NotFound, fmt.Sprintf("please set credential settings for provider %s before update", id))
		}
		viper.Set(fmt.Sprintf(common.BindPrefix, id, key), secret)
	}
	if err := viper.WriteConfig(); err != nil {
		return types.APIObject{}, err
	}
	return cred.ByID(apiOp, schema, id)
}

func (cred *Store) Delete(apiOp *types.APIRequest, schema *types.APISchema, id string) (types.APIObject, error) {
	provider, err := providers.GetProvider(id)
	if err != nil {
		logrus.Errorf("get provider %s error: %v", id, err)
		return types.APIObject{}, err
	}
	flags := provider.GetCredentialFlags()
	if err := viper.ReadInConfig(); err != nil {
		return types.APIObject{}, err
	}
	// remove env vars and viper config for credential
	for _, flag := range flags {
		if flag.EnvVar != "" && os.Getenv(flag.EnvVar) != "" {
			os.Setenv(flag.EnvVar, "")
		}
	}

	settings := viper.AllSettings()
	providerConfigs, ok := settings["autok3s"].(map[string]interface{})
	if !ok {
		return types.APIObject{}, apierror.NewAPIError(validation.NotFound, fmt.Sprintf("credential settings for provider %s is not exist", id))
	}
	config, ok := providerConfigs["providers"].(map[string]interface{})
	if !ok {
		return types.APIObject{}, apierror.NewAPIError(validation.NotFound, fmt.Sprintf("credential settings for provider %s is not exist", id))
	}
	if _, ok := config[id]; !ok {
		return types.APIObject{}, apierror.NewAPIError(validation.NotFound, fmt.Sprintf("credential settings for provider %s is not exist", id))
	}
	delete(config, id)
	viper.MergeConfigMap(settings)
	encodedConfig, _ := json.MarshalIndent(settings, "", " ")
	err = viper.ReadConfig(bytes.NewReader(encodedConfig))
	if err != nil {
		return types.APIObject{}, err
	}
	err = viper.WriteConfig()
	return types.APIObject{}, err
}
