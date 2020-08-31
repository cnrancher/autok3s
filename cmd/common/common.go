package common

import (
	"fmt"
	"os"
	"strings"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/providers"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func BindPFlags(cmd *cobra.Command, p providers.Provider) {
	name, err := cmd.Flags().GetString("provider")
	if err != nil {
		logrus.Fatalln(err)
	}

	cmd.Flags().Visit(func(f *pflag.Flag) {
		if IsCredentialFlag(f.Name, p.BindCredentialFlags()) {
			if err := viper.BindPFlag(fmt.Sprintf(common.BindPrefix, name, f.Name), f); err != nil {
				logrus.Fatalln(err)
			}
		}
	})
}

// Borrowed from https://github.com/docker/machine/blob/master/commands/create.go#L267.
func FlagHackLookup(flagName string) string {
	// e.g. "-d" for "--driver"
	flagPrefix := flagName[1:3]

	// TODO: Should we support -flag-name (single hyphen) syntax as well?
	for i, arg := range os.Args {
		if strings.Contains(arg, flagPrefix) {
			// format '--driver foo' or '-d foo'
			if arg == flagPrefix || arg == flagName {
				if i+1 < len(os.Args) {
					return os.Args[i+1]
				}
			}

			// format '--driver=foo' or '-d=foo'
			if strings.HasPrefix(arg, flagPrefix+"=") || strings.HasPrefix(arg, flagName+"=") {
				return strings.Split(arg, "=")[1]
			}
		}
	}

	return ""
}

func IsCredentialFlag(s string, nfs *pflag.FlagSet) bool {
	found := false
	nfs.VisitAll(func(f *pflag.Flag) {
		if strings.EqualFold(s, f.Name) {
			found = true
		}
	})
	return found
}
