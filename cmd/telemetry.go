package cmd

import (
	"strconv"

	"github.com/cnrancher/autok3s/pkg/common"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	telemetryCommand = &cobra.Command{
		Use:   "telemetry",
		Short: "Telemetry status for autok3s",
	}
	enable string
)

func init() {
	telemetryCommand.Flags().StringVar(&enable, "set", "", "to set telemetry status, true of false")
}

func TelemetryCommand() *cobra.Command {
	telemetryCommand.PreRunE = func(cmd *cobra.Command, _ []string) error {
		_, err := getValidatedEnable(cmd)
		if err != nil {
			return errors.Wrap(err, "invalid set flag")
		}
		return nil
	}
	telemetryCommand.Run = func(cmd *cobra.Command, _ []string) {
		rtn, _ := getValidatedEnable(cmd)
		if rtn == nil {
			getCurrentStatus(cmd)
		} else {
			if err := common.SetTelemetryStatus(*rtn); err != nil {
				logrus.Fatal(err)
			}
			cmd.Printf("telemetry status set to %v\n", *rtn)
		}
	}
	return telemetryCommand
}

func getValidatedEnable(cmd *cobra.Command) (*bool, error) {
	setFlag := cmd.Flag("set")
	if setFlag == nil {
		return nil, nil
	}
	if !setFlag.Changed {
		return nil, nil
	}
	rtn, err := strconv.ParseBool(enable)
	if err != nil {
		return nil, err
	}
	return &rtn, nil
}

func getCurrentStatus(cmd *cobra.Command) {
	enable := common.GetTelemetryEnable()
	status := "promote"
	if enable != nil {
		status = strconv.FormatBool(*enable)
	}
	cmd.Printf("current telemetry status is %s, it can be changed via --set flag\n", status)
}
