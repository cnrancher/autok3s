package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/cnrancher/autok3s/pkg/common"

	// import custom provider
	_ "github.com/cnrancher/autok3s/pkg/providers/alibaba"
	_ "github.com/cnrancher/autok3s/pkg/providers/aws"
	_ "github.com/cnrancher/autok3s/pkg/providers/google"
	_ "github.com/cnrancher/autok3s/pkg/providers/k3d"
	_ "github.com/cnrancher/autok3s/pkg/providers/native"
	_ "github.com/cnrancher/autok3s/pkg/providers/tencent"

	"github.com/morikuni/aec"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

const ascIIStr = `
               ,        , 
  ,------------|'------'|             _        _    _____ 
 / .           '-'    |-             | |      | |  |____ | 
 \\/|             |    |   __ _ _   _| |_ ___ | | __   / / ___
   |   .________.'----'   / _  | | | | __/ _ \| |/ /   \ \/ __|
   |   |        |   |    | (_| | |_| | || (_) |   <.___/ /\__ \
   \\___/        \\___/   \__,_|\__,_|\__\___/|_|\_\____/ |___/

`

var (
	cmd = &cobra.Command{
		Use:              "autok3s",
		Short:            "autok3s is used to manage the lifecycle of K3s on multiple cloud providers",
		Long:             `autok3s is used to manage the lifecycle of K3s on multiple cloud providers.`,
		TraverseChildren: true,
	}
)

func init() {
	cobra.OnInitialize(initCfg)
	setHelpTemplate(cmd)
	setEnvVars()
	cmd.PersistentFlags().BoolVarP(&common.Debug, "debug", "d", common.Debug, "Enable log debug level")
}

// Command root command.
func Command() *cobra.Command {
	cmd.Run = func(cmd *cobra.Command, _ []string) {
		printASCII()

		if err := cmd.Help(); err != nil {
			logrus.Errorln(err)
			os.Exit(1)
		}
	}
	return cmd
}

func initCfg() {
	if err := common.MoveLogs(); err != nil {
		logrus.Errorf("failed to relocate cluster logs, %v", err)
	}

	kubeCfg := filepath.Join(common.CfgPath, common.KubeCfgFile)
	if err := os.Setenv(clientcmd.RecommendedConfigPathEnvVar, kubeCfg); err != nil {
		logrus.Errorf("[kubectl] failed to set %s=%s env", clientcmd.RecommendedConfigPathEnvVar, kubeCfg)
	}

	if err := common.InitStorage(cmd.Context()); err != nil {
		logrus.Fatalln(err)
	}

	common.ExplorerWatchers = map[string]context.CancelFunc{}
	common.FileManager = &common.ConfigFileManager{}

	if err := common.SetupNewInstall(); err != nil {
		logrus.Fatalln(err)
	}
}

func printASCII() {
	fmt.Print(aec.Apply(ascIIStr))
}

/*
 * setEnvVars In order to avoid autok3s kubectl pre-check problem, we have to using environment variables to set the
 * global parameters(https://github.com/kubernetes/kubernetes/pull/92343).
 */
func setEnvVars() {
	cfgEnv := os.Getenv("AUTOK3S_CONFIG")
	retryEnv := os.Getenv("AUTOK3S_RETRY")

	if cfgEnv != "" {
		common.CfgPath = os.Getenv("AUTOK3S_CONFIG")
	}

	if retryEnv != "" {
		retryInt, err := strconv.Atoi(retryEnv)
		if err != nil {
			logrus.Errorln(err)
			os.Exit(1)
		}
		common.Backoff.Steps = retryInt
	}
}

func setHelpTemplate(cmd *cobra.Command) {
	t := `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Global Environments:
  AUTOK3S_CONFIG  Path to the cfg file to use for CLI requests (default ~/.autok3s)
  AUTOK3S_RETRY   The number of retries waiting for the desired state (default 20)

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`
	cmd.SetHelpTemplate(t)
}
