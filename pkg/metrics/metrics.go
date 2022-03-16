package metrics

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/cnrancher/autok3s/pkg/common"

	"github.com/Jason-ZW/autok3s-geo/pkg/types"
)

const (
	metricsEndpoint  = "http://metrics.cnrancher.com:8080/v1/geoIPs"
	metricsSourceTag = "AutoK3s"
)

func ReportMetrics() {
	logger := common.NewLogger(common.Debug, nil)

	client := &http.Client{}

	b, err := json.Marshal(types.GeoIP{})
	if err != nil {
		logger.Debugf("failed to collected usage metrics: %s", err.Error())
		return
	}

	req, err := http.NewRequest(http.MethodPost, metricsEndpoint, bytes.NewBuffer(b))
	if err != nil {
		logger.Debugf("failed to collected usage metrics: %s", err.Error())
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Req-Source", metricsSourceTag)

	resp, err := client.Do(req)
	if err != nil {
		logger.Debugf("failed to collected usage metrics: %s", err.Error())
		return
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		logger.Debugf("failed to collected usage metrics: %s", resp.Status)
	}
}
