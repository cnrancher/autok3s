//go:build !prod

package k3d

import (
	"fmt"

	k3d "github.com/k3d-io/k3d/v5/pkg/types"
	k3dversion "github.com/k3d-io/k3d/v5/version"
)

func init() {
	k3dversion.Version = "v5.4.4"
	k3dversion.K3sVersion = "v1.23.8-k3s1"
	k3dImage = fmt.Sprintf("%s:%s", k3d.DefaultK3sImageRepo, k3dversion.K3sVersion)
}
