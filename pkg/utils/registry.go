package utils

import (
	"fmt"
	"io/ioutil"

	"github.com/k3d-io/k3d/v5/pkg/types/k3s"
	"sigs.k8s.io/yaml"
)

func VerifyRegistryFileContent(path, content string) (*k3s.Registry, error) {
	var registry k3s.Registry
	var err error
	contentBytes := []byte(content)
	if path != "" && content == "" {
		contentBytes, err = ioutil.ReadFile(path)
		if err != nil {
			return nil, err
		}

		if len(contentBytes) == 0 {
			return nil, fmt.Errorf("registry file %s is empty", path)
		}
	}

	if err := yaml.Unmarshal(contentBytes, &registry); err != nil {
		return nil, err
	}
	return &registry, nil
}

func RegistryToString(registry *k3s.Registry) (string, error) {
	if registry == nil {
		return "", fmt.Errorf("can't save registry file: registry is nil")
	}
	b, err := yaml.Marshal(registry)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
