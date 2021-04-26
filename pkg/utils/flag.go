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
				case map[string]string:
					pf.StringToStringVar(f.P.(*map[string]string), f.Name, t, f.Usage)
				case []string:
					pf.StringArrayVar(f.P.(*[]string), f.Name, t, f.Usage)
				default:
					continue
				}
				if f.Required {
					_ = cobra.MarkFlagRequired(pf, f.Name)
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
				case map[string]string:
					pf.StringToStringVarP(f.P.(*map[string]string), f.Name, f.ShortHand, t, f.Usage)
				case []string:
					pf.StringArrayVarP(f.P.(*[]string), f.Name, f.ShortHand, t, f.Usage)
				default:
					continue
				}
				if f.Required {
					_ = cobra.MarkFlagRequired(pf, f.Name)
				}
			}
		}

		if f.EnvVar != "" {
			_ = cmd.Flags().SetAnnotation(f.Name, BashCompEnvVarFlag, []string{f.EnvVar})
		}
	}

	return cmd.Flags()
}

// ValidateRequiredFlags set `flag.Change` if the required flag has default value
// but not changed by flags.Set to pass the required check
// https://github.com/spf13/cobra/blob/v1.1.1/command.go#L1001
func ValidateRequiredFlags(flags *pflag.FlagSet) {
	flags.VisitAll(func(flag *pflag.Flag) {
		requiredAnnotation, found := flag.Annotations[cobra.BashCompOneRequiredFlag]
		if !found {
			return
		}
		if (requiredAnnotation[0] == "true") && flag.Value.String() != "" && !flag.Changed {
			flag.Changed = true
		}
	})
}
