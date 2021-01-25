package common

import (
	"fmt"
	"os"
	"strings"

	"github.com/cnrancher/autok3s/pkg/common"
	"github.com/cnrancher/autok3s/pkg/providers"
	"github.com/cnrancher/autok3s/pkg/providers/alibaba"
	"github.com/cnrancher/autok3s/pkg/providers/amazone"
	"github.com/cnrancher/autok3s/pkg/providers/tencent"
	"github.com/cnrancher/autok3s/pkg/types"
	typesAli "github.com/cnrancher/autok3s/pkg/types/alibaba"
	typesAmazone "github.com/cnrancher/autok3s/pkg/types/amazone"
	typesTencent "github.com/cnrancher/autok3s/pkg/types/tencent"
	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/ghodss/yaml"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func bindPFlags(cmd *cobra.Command, p providers.Provider) {
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

func InitPFlags(cmd *cobra.Command, p providers.Provider) {
	// bind env to flags
	bindEnvFlags(cmd)
	bindPFlags(cmd, p)

	// read options from config.
	if err := viper.ReadInConfig(); err != nil {
		logrus.Fatalln(err)
	}

	// sync config data to local cfg path.
	if err := viper.WriteConfig(); err != nil {
		logrus.Fatalln(err)
	}
}

func bindEnvFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		envAnnotation := f.Annotations[utils.BashCompEnvVarFlag]
		if len(envAnnotation) == 0 {
			return
		}

		if os.Getenv(envAnnotation[0]) != "" {
			cmd.Flags().Set(f.Name, fmt.Sprintf("%v", os.Getenv(envAnnotation[0])))
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

func MakeSureCredentialFlag(flags *pflag.FlagSet, p providers.Provider) error {
	flags.VisitAll(func(flag *pflag.Flag) {
		// if viper has set the value, make sure flag has the value set to pass require check
		if IsCredentialFlag(flag.Name, p.BindCredentialFlags()) && viper.IsSet(fmt.Sprintf(common.BindPrefix, p.GetProviderName(), flag.Name)) {
			flags.Set(flag.Name, viper.GetString(fmt.Sprintf(common.BindPrefix, p.GetProviderName(), flag.Name)))
		}
	})

	return nil
}

func GetProviderByState(c types.Cluster) (providers.Provider, error) {
	b, err := yaml.Marshal(c.Options)
	if err != nil {
		return nil, err
	}
	switch c.Provider {
	case "alibaba":
		option := &typesAli.Options{}
		if err := yaml.Unmarshal(b, option); err != nil {
			return nil, err
		}
		return &alibaba.Alibaba{
			Metadata: c.Metadata,
			Options:  *option,
			Status:   c.Status,
		}, nil
	case "tencent":
		option := &typesTencent.Options{}
		if err := yaml.Unmarshal(b, option); err != nil {
			return nil, err
		}
		return &tencent.Tencent{
			Metadata: c.Metadata,
			Options:  *option,
			Status:   c.Status,
		}, nil
	case "amazone":
		option := &typesAmazone.Options{}
		if err := yaml.Unmarshal(b, option); err != nil {
			return nil, err
		}
		return &amazone.Amazone{
			Metadata: c.Metadata,
			Options:  *option,
			Status:   c.Status,
		}, nil
	default:
		return nil, fmt.Errorf("invalid provider name %s", c.Provider)
	}
}
