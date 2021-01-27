package utils

import (
	"fmt"
	"os"
	"reflect"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/providers"
	"github.com/cnrancher/autok3s/pkg/types"

	"github.com/rancher/wrangler/pkg/schemas"
	"github.com/spf13/viper"
)

func GetCredentialByProvider(p providers.Provider) (map[string]schemas.Field, error) {
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}
	credFlags := p.GetCredentialFlags()
	result := make(map[string]schemas.Field, 0)
	for _, flag := range credFlags {
		value := ""
		if flag.EnvVar != "" && os.Getenv(flag.EnvVar) != "" {
			value = os.Getenv(flag.EnvVar)
		} else {
			value = viper.GetString(fmt.Sprintf(common.BindPrefix, p.GetProviderName(), flag.Name))
		}
		result[flag.Name] = schemas.Field{
			Type:        "password",
			Description: flag.Usage,
			Required:    flag.Required,
			Default:     value,
		}
	}
	return result, nil
}

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
