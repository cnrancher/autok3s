package kubectl

import (
	"fmt"
	"strings"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/types/apis"

	"github.com/rancher/apiserver/pkg/store/empty"
	"github.com/rancher/apiserver/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
)

// Store holds kubectl API state.
type Store struct {
	empty.Store
}

// List returns kubectl contexts.
func (k *Store) List(apiOp *types.APIRequest, schema *types.APISchema) (types.APIObjectList, error) {
	result := types.APIObjectList{}
	kubeCfg := fmt.Sprintf("%s/%s", common.CfgPath, common.KubeCfgFile)
	clientConfig, err := clientcmd.LoadFromFile(kubeCfg)
	if err != nil {
		return result, err
	}
	contexts := clientConfig.Contexts
	for name := range contexts {
		content := strings.Split(name, ".")
		result.Objects = append(result.Objects, types.APIObject{
			ID:   name,
			Type: schema.ID,
			Object: apis.Config{
				Context: content[0],
			},
		})
	}
	return result, nil
}
