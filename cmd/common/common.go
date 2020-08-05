package common

import (
	"fmt"
	"strings"

	"github.com/Jason-ZW/autok3s/pkg/providers"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var bindPrefix = "autok3s.providers.%s.%s"

func BindPFlags(cmd *cobra.Command, p providers.Provider) {
	name, err := cmd.Flags().GetString("provider")
	if err != nil {
		logrus.Fatalln(err)
	}

	cmd.Flags().Visit(func(f *pflag.Flag) {
		if IsAccessFlag(f.Name, p.GetCredentialFlags()) {
			if err := viper.BindPFlag(fmt.Sprintf(bindPrefix, name, f.Name), f); err != nil {
				logrus.Fatalln(err)
			}
		}
	})
}

func IsAccessFlag(s string, nfs *pflag.FlagSet) bool {
	found := false
	nfs.VisitAll(func(f *pflag.Flag) {
		if strings.EqualFold(s, f.Name) {
			found = true
		}
	})
	return found
}
