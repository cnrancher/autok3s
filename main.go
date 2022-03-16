package main

import (
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/cnrancher/autok3s/cmd"
	"github.com/cnrancher/autok3s/pkg/cli/kubectl"
	"github.com/cnrancher/autok3s/pkg/metrics"

	"github.com/docker/docker/pkg/reexec"
	"github.com/spf13/cobra"
)

var (
	gitVersion   string
	gitCommit    string
	gitTreeState string
	buildDate    string

	metricsEndpoint  = "http://metrics.cnrancher.com:8080/v1/geoIPs"
	metricsSourceTag = "AutoK3s"
)

func init() {
	reexec.Register("kubectl", kubectl.Main)
}

func main() {
	rand.Seed(time.Now().UTC().UnixNano())

	args := os.Args[0]
	os.Args[0] = filepath.Base(os.Args[0])
	if reexec.Init() {
		return
	}
	os.Args[0] = args

	rootCmd := cmd.Command()
	rootCmd.AddCommand(cmd.CompletionCommand(), cmd.VersionCommand(gitVersion, gitCommit, gitTreeState, buildDate),
		cmd.ListCommand(), cmd.CreateCommand(), cmd.JoinCommand(), cmd.KubectlCommand(), cmd.DeleteCommand(),
		cmd.SSHCommand(), cmd.DescribeCommand(), cmd.ServeCommand(), cmd.ExplorerCommand())

	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		metrics.ReportMetrics()
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
