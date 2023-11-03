package utils

import (
	"fmt"
	"io/ioutil"

	"github.com/rancher/wharfie/pkg/registries"
	"sigs.k8s.io/yaml"
)

func VerifyRegistryFileContent(path, content string) (*registries.Registry, error) {
	var registry registries.Registry
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

func RegistryToString(registry *registries.Registry) (string, error) {
	if registry == nil {
		return "", fmt.Errorf("can't save registry file: registry is nil")
	}
	b, err := yaml.Marshal(registry)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
