package kubectl

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/sirupsen/logrus"
	"k8s.io/kubectl/pkg/cmd"
)

func Main() {
	if runtime.GOOS == "windows" {
		os.Args = os.Args[1:]
	}
	kubenv := os.Getenv("KUBECONFIG")
	for i, arg := range os.Args {
		if strings.HasPrefix(arg, "--kubeconfig=") {
			kubenv = strings.Split(arg, "=")[1]
		} else if strings.HasPrefix(arg, "--kubeconfig") && i+1 < len(os.Args) {
			kubenv = os.Args[i+1]
		}
	}
	if kubenv == "" {
		// get default kubeconfig
		config := filepath.Join(common.CfgPath, common.KubeCfgFile)
		if _, serr := os.Stat(config); serr == nil {
			os.Setenv("KUBECONFIG", config)
		}
		if err := checkReadConfigPermissions(config); err != nil {
			logrus.Warn(err)
		}
	}

	if err := EmbedCommand().Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func EmbedCommand() *cobra.Command {
	return cmd.NewDefaultKubectlCommand()
}

func checkReadConfigPermissions(configFile string) error {
	file, err := os.OpenFile(configFile, os.O_RDONLY, 0600)
	if err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("Unable to read %s, please start server "+
				"with --write-kubeconfig-mode to modify kube config permissions", configFile)
		}
	}
	file.Close()
	return nil
}
