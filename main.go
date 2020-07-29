package main

import (
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/Jason-ZW/autok3s/cmd"
	"github.com/Jason-ZW/autok3s/pkg/cli/kubectl"

	"github.com/docker/docker/pkg/reexec"
)

var (
	gitVersion   string
	gitCommit    string
	gitTreeState string
	buildDate    string
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
		cmd.GetCommand(), cmd.CreateCommand(), cmd.KubectlCommand())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
