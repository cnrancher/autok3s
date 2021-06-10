package utils

import (
	"encoding/json"
	"os"
	"reflect"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/providers"
	"github.com/cnrancher/autok3s/pkg/types"

	"github.com/rancher/wrangler/pkg/schemas"
	"github.com/sirupsen/logrus"
)

// GetCredentialFields returns credential fields.
func GetCredentialFields(p providers.Provider) map[string]schemas.Field {
	credFlags := p.GetCredentialFlags()
	result := make(map[string]schemas.Field, 0)
	for _, flag := range credFlags {
		result[flag.Name] = schemas.Field{
			Type:        "password",
			Description: flag.Usage,
			Required:    flag.Required,
		}
	}
	return result
}

// GetCredentialByProvider returns credential by provider.
func GetCredentialByProvider(p providers.Provider) (map[string]schemas.Field, error) {
	result := GetCredentialFields(p)
	credList, err := common.DefaultDB.GetCredentialByProvider(p.GetProviderName())
	if err != nil {
		logrus.Errorf("failed to get credential for provider %s: %v", p.GetProviderName(), err)
		return result, nil
	}
	if len(credList) > 0 {
		cred := credList[0]
		secrets := map[string]string{}
		err = json.Unmarshal(cred.Secrets, &secrets)
		if err != nil {
			logrus.Errorf("failed to get convert credential secrets for provider %s: %v", p.GetProviderName(), err)
			return result, nil
		}
		for name, field := range result {
			if value, ok := secrets[name]; ok {
				field.Default = value
				result[name] = field
			}
		}
	}
	return result, nil
}

// ConvertFlagsToFields convert flags to fields.
func ConvertFlagsToFields(flags []types.Flag) map[string]schemas.Field {
	result := make(map[string]schemas.Field, 0)
	for _, flag := range flags {
		var value interface{}
		if flag.EnvVar != "" && os.Getenv(flag.EnvVar) != "" {
			value = os.Getenv(flag.EnvVar)
		} else {
			value = flag.V
		}
		result[flag.Name] = schemas.Field{
			Type:        reflect.TypeOf(flag.V).Kind().String(),
			Description: flag.Usage,
			Required:    flag.Required,
			Default:     value,
		}
	}
	return result
}
