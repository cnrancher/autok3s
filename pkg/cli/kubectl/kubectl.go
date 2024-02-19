package kubectl

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/cmd"
)

// Main kubectl main function.
// Borrowed from https://github.com/kubernetes/kubernetes/blob/master/cmd/kubectl/kubectl.go.
func Main() {
	if err := EmbedCommand().Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

// EmbedCommand Used to embed the kubectl command.
func EmbedCommand() *cobra.Command {
	c := cmd.NewDefaultKubectlCommand()
	c.Short = "Kubectl controls the Kubernetes cluster manager"

	return c
}
