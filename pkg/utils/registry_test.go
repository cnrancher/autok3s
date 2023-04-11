package utils

import (
	"testing"

	"github.com/k3d-io/k3d/v5/pkg/types/k3s"
	"github.com/stretchr/testify/assert"
)

func TestRegistryValidating(t *testing.T) {
	content := `
mirrors:
  docker.io:
    endpoint:
      - "https://docker.nju.edu.cn"
`
	expected := &k3s.Registry{
		Mirrors: map[string]k3s.Mirror{
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
