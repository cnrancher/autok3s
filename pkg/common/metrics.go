package common

import (
	"strconv"
	"syscall"

	"github.com/cnrancher/autok3s/pkg/metrics"
	"github.com/cnrancher/autok3s/pkg/types"
	"github.com/cnrancher/autok3s/pkg/utils"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	enableMetricsSettingName = "enable-metrics"
)

func SetupPrometheusMetrics(version string) {
	labels := uuidLabels()
	labels["version"] = version
	metrics.Active.With(labels).Set(1)
	clusters, err := DefaultDB.ListCluster("")
	if err != nil {
		logrus.Debugf("failed to list cluster from db, %v", err)
		return
	}
	for _, cluster := range clusters {
		metrics.ClusterCount.With(getLabelsFromMeta(cluster.Metadata)).Add(1)
	}
	templates, err := DefaultDB.ListTemplates()
	if err != nil {
		logrus.Debugf("failed to list template from db, %v", err)
		return
	}
	for _, template := range templates {
		metrics.TemplateCount.With(getLabelsFromMeta(template.Metadata)).Add(1)
	}
	metrics.SetupEnableFunc(func() bool {
		enable, err := GetTelemetryEnable()
		if err != nil {
			return false
		}
		if enable != nil && *enable {
			return true
		}
		return false
	})
}

func getLabelsFromMeta(meta types.Metadata) prometheus.Labels {
	version := meta.K3sVersion
	if version == "" {
		version = "unknown"
	}
	uuid, err := GetUUID()
	if err != nil {
		logrus.Debugf("failed to get install uuid from db, %v", err)
		uuid = "unknown"
	}
	return prometheus.Labels{
		"provider":     meta.Provider,
		"k3sversion":   version,
		"install_uuid": uuid,
	}
}

func GetTelemetryEnable() (*bool, error) {
	setting, err := DefaultDB.GetSetting(enableMetricsSettingName)
	if err != nil {
		return nil, err
	}
	var rtn bool
	switch setting.Value {
	case "true":
		rtn = true
	case "false":
		rtn = false
	// default case indicates that we should promote to user
	default:
		return nil, nil
	}
	return &rtn, nil
}

func MetricsPrompt(cmd *cobra.Command) {
	if cmd.Use == "version" ||
		cmd.Use == "serve" ||
		cmd.Use == "completion" ||
		cmd.Use == "explorer" {
		return
	}
	if !term.IsTerminal(int(syscall.Stdin)) {
		logrus.Debug("disable promoting telemetry in non-terminal environment")
		return
	}
	if should, _ := GetTelemetryEnable(); should != nil {
		return
	}

	rtn := utils.AskForConfirmation("This is the very first time using autok3s,\n  would you like to share metrics with us?\n  You can always your mind with telemetry command", true)

	if err := SetTelemetryStatus(rtn); err != nil {
		logrus.Warnf("failed to set telemetry enable status, %v", err)
	}
}

func SetTelemetryStatus(enable bool) error {
	return DefaultDB.SaveSetting(&Setting{
		Name:  enableMetricsSettingName,
		Value: strconv.FormatBool(enable),
	})
}

func uuidLabels() map[string]string {
	uuid, err := GetUUID()
	if err != nil {
		logrus.Debugf("failed to get install uuid from db, %v", err)
		uuid = "unknown"
	}
	return map[string]string{
		"install_uuid": uuid,
	}
}
