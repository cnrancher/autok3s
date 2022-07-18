package metrics

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/prometheus/common/expfmt"
	"github.com/sirupsen/logrus"
)

const (
	// metricsEndpoint will send metrics to https://telemetry.rancher.cn/metrics/job/autok3s
	metricsEndpoint = "https://telemetry.rancher.cn"
	jobName         = "autok3s"
)

var (
	ClusterCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: "autok3s",
		Name:      "cluster_count",
		Help:      "the cluster count for the current autok3s setup, label by provider and k3s version.",
	}, []string{"provider", "k3sversion", "install_uuid"})

	TemplateCount = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: "autok3s",
		Name:      "cluster_template_count",
		Help:      "the cluster template count for the current autok3s setup, label by provider and k3s version.",
	}, []string{"provider", "k3sversion", "install_uuid"})

	Active = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: "autok3s",
		Name:      "up",
		Help:      "the autok3s running status",
	}, []string{"install_uuid", "version"})

	defaultRegistry = prometheus.NewRegistry()
	enableFunc      func() bool
	once            = &sync.Once{}
	pusher          *push.Pusher
)

func init() {
	defaultRegistry.MustRegister(ClusterCount, TemplateCount, Active)
	pusher = push.New(metricsEndpoint, jobName).
		Format(expfmt.FmtText).
		Gatherer(defaultRegistry)
}

func SetupEnableFunc(f func() bool) {
	once.Do(func() {
		enableFunc = f
	})
}

func Report() {
	if enableFunc == nil || !enableFunc() {
		return
	}
	logrus.Debug("Reporting metrics")
	if err := pusher.Push(); err != nil {
		// telegraf always returns 204 instead of 200/202
		if !strings.Contains(err.Error(), "unexpected status code 204") {
			logrus.Debug("failed to push metrics to telemetry")
		}
	}
}

func ReportEach(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-t.C:
				Report()
			case <-ctx.Done():
				t.Stop()
				return
			}
		}
	}()
}
