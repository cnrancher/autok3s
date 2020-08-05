package cmd

import (
	"fmt"
	"os"

	"github.com/Jason-ZW/autok3s/pkg/utils"

	"github.com/morikuni/aec"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	cfgFile = "/var/lib/rancher/autok3s/config.yaml"

	cmd = &cobra.Command{
		Use:   "autok3s",
		Short: "autok3s is used to manage the lifecycle of K3s on multiple cloud providers",
		Long:  `autok3s is used to manage the lifecycle of K3s on multiple cloud providers.`,
	}
)

func init() {
	cobra.OnInitialize(initCfg)
	cmd.PersistentFlags().StringVarP(&cfgFile, "cfg", "c", cfgFile, "Path to the cfg file to use for CLI requests")
}

func Command() *cobra.Command {
	cmd.PreRun = func(cmd *cobra.Command, args []string) {
		if os.Getuid() != 0 {
			logrus.Errorf("%s: need to be root", os.Args[0])
			os.Exit(1)
		}
	}

	cmd.Run = func(cmd *cobra.Command, args []string) {
		printASCII()
		if err := cmd.Help(); err != nil {
			logrus.Errorln(err)
			os.Exit(1)
		}
	}
	return cmd
}

func initCfg() {
	viper.SetConfigType("yaml")
	viper.SetConfigFile(cfgFile)
	viper.AutomaticEnv()

	if err := utils.EnsureCfgFileExist(cfgFile); err != nil {
		logrus.Fatalln(err)
	}
}

func printASCII() {
	fmt.Print(aec.Apply(ascIIStr))
}
