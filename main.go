package main

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/cnrancher/autok3s/cmd"
	"github.com/cnrancher/autok3s/pkg/cli/kubectl"
	"github.com/cnrancher/autok3s/pkg/common"

	"github.com/Jason-ZW/autok3s-geo/pkg/types"
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
		ReportMetrics()
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func ReportMetrics() {
	logger := common.NewLogger(common.Debug, nil)

	client := &http.Client{}

	b, err := json.Marshal(types.GeoIP{})
	if err != nil {
		logger.Debugf("failed to collected usage metrics: %s", err.Error())
		return
	}

	req, err := http.NewRequest("POST", metricsEndpoint, bytes.NewBuffer(b))
	if err != nil {
		logger.Debugf("failed to collected usage metrics: %s", err.Error())
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Req-Source", metricsSourceTag)

	resp, err := client.Do(req)
	if err != nil {
		logger.Debugf("failed to collected usage metrics: %s", err.Error())
		return
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		logger.Debugf("failed to collected usage metrics: %s", resp.Status)
	}
}
