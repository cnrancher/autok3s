package common

import (
	"fmt"
	"sync"
	"time"

	"github.com/gofrs/flock"
	"github.com/sirupsen/logrus"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

var (
	pollInterval = 250 * time.Millisecond
	maxWait      = 10 * time.Second
)

type ConfigFileManager struct {
	mutex sync.RWMutex
}

func (c *ConfigFileManager) OverwriteCfg(path string, context string, cfg func(string, clientcmd.ConfigAccess) (*api.Config, error)) error {
	paOpt := clientcmd.NewDefaultPathOptions()

	c.mutex.Lock()
	fileLock := flock.New(path)
	locked, err := fileLock.TryLock()
	if err != nil {
		return err
	}
	c.mutex.Unlock()
	defer fileLock.Unlock()
	for i := time.Duration(0); i < maxWait; i += pollInterval {
		if locked {
			config, err := cfg(context, paOpt)
			if err != nil {
				return err
			}
			return clientcmd.WriteToFile(*config, path)
		}
		logrus.Infof("file %v is locking, wait until unlock", path)
		time.Sleep(pollInterval)
		locked, err = fileLock.TryLock()
		if err != nil {
			return err
		}
		continue

	}
	return fmt.Errorf("timeout for wait config file %v unlock", path)
}

func (c *ConfigFileManager) RemoveCfg(context string, configAccess clientcmd.ConfigAccess) (*api.Config, error) {
	config, err := configAccess.GetStartingConfig()
	if err != nil {
		return nil, err
	}

	if config.CurrentContext == context {
		config.CurrentContext = ""
	}

	delete(config.Contexts, context)
	delete(config.Clusters, context)
	delete(config.AuthInfos, context)
	return config, nil
}

func (c *ConfigFileManager) MergeCfg(context string, configAccess clientcmd.ConfigAccess) (*api.Config, error) {
	// check context exists
	oldConfig, err := clientcmd.LoadFromFile(fmt.Sprintf("%s/%s", CfgPath, KubeCfgFile))
	if err != nil {
		return nil, err
	}
	if _, ok := oldConfig.Contexts[context]; ok {
		return nil, fmt.Errorf("context %s is already exist in kubeconfig file, please remove the exist one or change the context name of current cluster", context)
	}
	// get merged config
	config, err := configAccess.GetStartingConfig()
	return config, err
}
