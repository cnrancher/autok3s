package kubectl

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	"k8s.io/kubernetes/pkg/kubectl/cmd"
)

// Borrowed from https://github.com/kubernetes/kubernetes/blob/master/cmd/kubectl/kubectl.go.
func Main() {
	rand.Seed(time.Now().UnixNano())

	pflag.CommandLine.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	logs.InitLogs()
	defer logs.FlushLogs()

	if err := EmbedCommand().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func EmbedCommand() *cobra.Command {
	c := cmd.NewDefaultKubectlCommand()
	c.Short = "Kubectl controls the Kubernetes cluster manager"
	return c
}
