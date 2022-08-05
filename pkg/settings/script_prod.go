//go:build prod
// +build prod

package settings

import (
	"embed"
)

//go:embed install.sh
var asserts embed.FS

func init() {
	data, err := asserts.ReadFile("install.sh")
	if err != nil {
		panic("install.sh should be included when compiling autok3s")
	}
	set := settings[InstallScript.Name]
	set.Default = string(data)
	settings[InstallScript.Name] = set
}
