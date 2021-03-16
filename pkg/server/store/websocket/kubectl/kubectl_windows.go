// +build windows

package kubectl

import (
	"fmt"

	"github.com/rancher/apiserver/pkg/types"
)

func KubeHandler(apiOp *types.APIRequest) (types.APIObject, error) {
	return types.APIObject{}, fmt.Errorf("not support windows")
}
