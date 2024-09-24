package common

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/providers"
	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// BindEnvFlags used for bind env to flag.
func BindEnvFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		envAnnotation := f.Annotations[utils.BashCompEnvVarFlag]
		if len(envAnnotation) == 0 {
			return
		}
		v, _ := cmd.Flags().GetString(f.Name)
		if v == "" && os.Getenv(envAnnotation[0]) != "" {
			_ = cmd.Flags().Set(f.Name, fmt.Sprintf("%v", os.Getenv(envAnnotation[0])))
		}
	})
}

// FlagHackLookup hack lookup function.
// Borrowed from https://github.com/docker/machine/blob/master/commands/create.go#L267.
func FlagHackLookup(flagName string) string {
	// i.e. "-d" for "--driver"
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

// MakeSureCredentialFlag ensure credential is provided.
func MakeSureCredentialFlag(flags *pflag.FlagSet, p providers.Provider) error {
	flags.VisitAll(func(flag *pflag.Flag) {
		if isCredentialFlag(flag.Name, p) {
			v, err := flags.GetString(flag.Name)
			if err != nil || v == "" {
				credentials, err := common.DefaultDB.GetCredentialByProvider(p.GetProviderName())
				if err != nil {
					logrus.Errorf("failed to get credential by provider %s: %v", p.GetProviderName(), err)
					return
				}
				if len(credentials) > 0 {
					cred := credentials[0]
					secrets := map[string]string{}
					err = json.Unmarshal(cred.Secrets, &secrets)
					if err != nil {
						logrus.Errorf("failed to convert credential value: %v", err)
						return
					}
					_ = flags.Set(flag.Name, secrets[flag.Name])
				}
			}
		}
	})
	return nil
}

func isCredentialFlag(s string, p providers.Provider) bool {
	found := false
	credFlags := p.GetCredentialFlags()
	for _, flag := range credFlags {
		if strings.EqualFold(s, flag.Name) {
			found = true
		}
	}
	return found
}
