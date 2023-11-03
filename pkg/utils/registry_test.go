package utils

import (
	"testing"

	"github.com/rancher/wharfie/pkg/registries"
	"github.com/stretchr/testify/assert"
)

func TestRegistryValidating(t *testing.T) {
	content := `
mirrors:
  docker.io:
    endpoint:
      - "https://docker.nju.edu.cn"
`
	expected := &registries.Registry{
		Mirrors: map[string]registries.Mirror{
			"docker.io": {
				Endpoints: []string{"https://docker.nju.edu.cn"},
			},
		},
	}
	reg, err := VerifyRegistryFileContent("", content)
	if !assert.Nil(t, err, "should not return error for testing") {
		return
	}
	assert.Equal(t, expected, reg)
}
