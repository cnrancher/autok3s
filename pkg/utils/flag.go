package utils

import (
	"github.com/cnrancher/autok3s/pkg/types"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const BashCompEnvVarFlag = "cobra_annotation_bash_env_var_flag"

// ConvertFlags change autok3s flags to FlagSet, will mark required annotation if possible.
func ConvertFlags(cmd *cobra.Command, fs []types.Flag) *pflag.FlagSet {
	for _, f := range fs {
		if f.ShortHand == "" {
			if cmd.Flags().Lookup(f.Name) == nil {
				pf := cmd.Flags()
				switch t := f.V.(type) {
				case bool:
					pf.BoolVar(f.P.(*bool), f.Name, t, f.Usage)
				case string:
					pf.StringVar(f.P.(*string), f.Name, t, f.Usage)
				default:
					continue
				}
				if f.Required {
					cobra.MarkFlagRequired(pf, f.Name)
				}
			}
		} else {
			if cmd.Flags().Lookup(f.Name) == nil {
				pf := cmd.Flags()
				switch t := f.V.(type) {
				case bool:
					pf.BoolVarP(f.P.(*bool), f.Name, f.ShortHand, t, f.Usage)
				case string:
					pf.StringVarP(f.P.(*string), f.Name, f.ShortHand, t, f.Usage)
				default:
					continue
				}
				if f.Required {
					cobra.MarkFlagRequired(pf, f.Name)
				}
			}
		}

		if f.EnvVar != "" {
			cmd.Flags().SetAnnotation(f.Name, BashCompEnvVarFlag, []string{f.EnvVar})
		}
	}

	return cmd.Flags()
}
