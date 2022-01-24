package common

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"

	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// ConfigFileManager struct for config file manager.
type ConfigFileManager struct {
	mutex sync.RWMutex
}

// ClearCfgByContext clear kube config by specified context.
func (c *ConfigFileManager) ClearCfgByContext(context string) error {
	path := filepath.Join(CfgPath, KubeCfgFile)
	_ = os.Setenv(clientcmd.RecommendedConfigPathEnvVar, path)
	return c.OverwriteCfg(path, context, c.RemoveCfg)
}

// SaveCfg save kube config file.
func (c *ConfigFileManager) SaveCfg(context, tempFile string) error {
	defer func() {
		_ = os.Setenv(clientcmd.RecommendedConfigPathEnvVar, filepath.Join(CfgPath, KubeCfgFile))
	}()
	kubeConfigPath := filepath.Join(CfgPath, KubeCfgFile)
	_ = os.Setenv(clientcmd.RecommendedConfigPathEnvVar, kubeConfigPath)
	err := c.OverwriteCfg(kubeConfigPath, context, c.RemoveCfg)
	if err != nil {
		return err
	}
	mergeKubeConfigENV := fmt.Sprintf("%s:%s", kubeConfigPath, tempFile)
	if runtime.GOOS == "windows" {
		mergeKubeConfigENV = fmt.Sprintf("%s;%s", kubeConfigPath, tempFile)
	}
	logrus.Debugf("merge kubeconfig with KUBECONFIG=%s", mergeKubeConfigENV)
	_ = os.Setenv(clientcmd.RecommendedConfigPathEnvVar, mergeKubeConfigENV)
	return c.OverwriteCfg(filepath.Join(CfgPath, KubeCfgFile), context, c.MergeCfg)
}

// OverwriteCfg overwrites kubectl config file.
func (c *ConfigFileManager) OverwriteCfg(path string, context string, cfg func(string, clientcmd.ConfigAccess) (*api.Config, error)) error {
	c.mutex.Lock()
	paOpt := clientcmd.NewDefaultPathOptions()
	defer func() {
		c.mutex.Unlock()
	}()

	config, err := cfg(context, paOpt)
	if err != nil {
		return err
	}
	return clientcmd.WriteToFile(*config, path)
}

// RemoveCfg removes kubectl config file.
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
	// the auth info entry associated with the context needs to be deleted.
	for key := range config.AuthInfos {
		if strings.Contains(key, fmt.Sprintf("@%s", context)) {
			delete(config.AuthInfos, key)
		}
	}
	return config, nil
}

// MergeCfg merges kubectl config file.
func (c *ConfigFileManager) MergeCfg(context string, configAccess clientcmd.ConfigAccess) (*api.Config, error) {
	// check context exists
	oldConfig, err := clientcmd.LoadFromFile(filepath.Join(CfgPath, KubeCfgFile))
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
