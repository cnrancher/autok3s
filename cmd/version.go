package cmd

import (
	"fmt"
	"runtime"

	"github.com/Jason-ZW/autok3s/pkg/types"

	"github.com/spf13/cobra"
)

var (
	versionCmd = &cobra.Command{
		Use:     "version",
		Short:   "Show the version",
		Example: `  autok3s version`,
	}

	short = false
)

func init() {
	versionCmd.Flags().BoolVarP(&short, "short", "s", short, "Print just the version number")
}

func VersionCommand(gitVersion, gitCommit, gitTreeState, buildDate string) *cobra.Command {
	version := types.VersionInfo{
		GitVersion:   gitVersion,
		GitCommit:    gitCommit,
		GitTreeState: gitTreeState,
		BuildDate:    buildDate,
		GoVersion:    runtime.Version(),
		Compiler:     runtime.Compiler,
		Platform:     fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}

	versionCmd.Run = func(cmd *cobra.Command, args []string) {
		if short {
			fmt.Printf("Version: %s\n", version.Short())
		} else {
			fmt.Printf("Version: %s\n", version.String())
		}
	}

	return versionCmd
}
