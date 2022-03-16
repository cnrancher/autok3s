//go:build !race
// +build !race

package common

import (
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

func TestMergeConfig(t *testing.T) {
	path := "./test.lock"
	_ = os.Setenv(clientcmd.RecommendedConfigPathEnvVar, path)
	_, err := os.Create(path)
	assert.Nil(t, err)
	defer func() {
		c, err := clientcmd.LoadFromFile(path)
		assert.Nil(t, err)
		assert.Len(t, c.Clusters, 0)
		_ = os.Remove(path)
	}()

	config := api.Config{
		Clusters: map[string]*api.Cluster{},
	}
	config.Clusters["test1"] = &api.Cluster{}
	config.Clusters["test2"] = &api.Cluster{}
	_ = clientcmd.WriteToFile(config, path)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		cm := ConfigFileManager{}
		err = cm.OverwriteCfg(path, "test1", cm.RemoveCfg)
		assert.Nil(t, err)
	}()

	go func() {
		defer wg.Done()
		cm := ConfigFileManager{}
		err = cm.OverwriteCfg(path, "test2", cm.RemoveCfg)
		assert.Nil(t, err)
	}()
	wg.Wait()
}
