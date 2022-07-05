package metrics

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/sirupsen/logrus"
)

const (
	metricsEndpoint  = "http://metrics.cnrancher.com:8080/v1/geoIPs"
	metricsSourceTag = "AutoK3s"
)

func ReportMetrics() {
	client := &http.Client{}

	b, err := json.Marshal(map[string]string{})
	if err != nil {
		logrus.Debugf("failed to collected usage metrics: %s", err.Error())
		return
	}

	req, err := http.NewRequest(http.MethodPost, metricsEndpoint, bytes.NewBuffer(b))
	if err != nil {
		logrus.Debugf("failed to collected usage metrics: %s", err.Error())
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Req-Source", metricsSourceTag)

	resp, err := client.Do(req)
	if err != nil {
		logrus.Debugf("failed to collected usage metrics: %s", err.Error())
		return
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		logrus.Debugf("failed to collected usage metrics: %s", resp.Status)
	}
}
