package cmd

import (
	"fmt"
	"os"

	"github.com/morikuni/aec"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
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

func Command() *cobra.Command {
	return &cobra.Command{
		Use:   "autok3s",
		Short: "autok3s is used to manage the lifecycle of K3s on multiple cloud providers",
		Long:  `autok3s is used to manage the lifecycle of K3s on multiple cloud providers.`,
		Run: func(cmd *cobra.Command, args []string) {
			printASCII()
			err := cmd.Help()
			if err != nil {
				logrus.Errorln(err)
				os.Exit(1)
			}
		},
	}
}

func printASCII() {
	fmt.Print(aec.Apply(ascIIStr))
}
