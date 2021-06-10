package types

import (
	"encoding/json"

	"github.com/sirupsen/logrus"
)

// VersionInfo contains version information.
// Borrowed from https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/apimachinery/pkg/version/types.go.
type VersionInfo struct {
	GitVersion   string `json:"gitVersion"`
	GitCommit    string `json:"gitCommit"`
	GitTreeState string `json:"gitTreeState"`
	BuildDate    string `json:"buildDate"`
	GoVersion    string `json:"goVersion"`
	Compiler     string `json:"compiler"`
	Platform     string `json:"platform"`
}

// String returns info as a full version string.
func (versionInfo VersionInfo) String() string {
	bytes, err := json.Marshal(versionInfo)
	if err != nil {
		logrus.Fatalln(err)
	}
	return string(bytes)
}

// Short returns info as a human-friendly version string.
func (versionInfo VersionInfo) Short() string {
	return versionInfo.GitVersion
}
