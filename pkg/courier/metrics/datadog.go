package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

// https://docs.datadoghq.com/api/
const (
	datadogDefaultHost = "https://api.datadoghq.com"

	datadogMetricsQueryPath     = "/api/v1/query"
	datadogAPIKeyValidationPath = "/api/v1/validate"

	datadogAPIKeySecretKey = "datadog_api_key"
	DatadogAPIKeyHeaderKey = "DD-API-KEY"

	datadogApplicationKeySecretKey = "datadog_application_key"
	DatadogApplicationKeyHeaderKey = "DD-APPLICATION-KEY"

	datadogFromDeltaMultiplierOnMetricInterval = 10
)

type Datadog struct {
	metricsQueryEndpoint     string
	apiKeyValidationEndpoint string

	timeout        time.Duration
	apiKey         string
	applicationKey string
	fromDelta      int64
}

type datadogResponse struct {
	Series []struct {
		Pointlist [][]float64 `json:"pointlist"`
	}
}

type DatadogOpts struct {
	APIKey         string
	ApplicationKey string
}

func NewDatadogProvider(
	provider ProviderOpts,
	credentials DatadogOpts) (*Datadog, error) {

	address := provider.Address
	if address == "" {
		address = datadogDefaultHost
	}

	dd := Datadog{
		timeout:                  5 * time.Second,
		metricsQueryEndpoint:     address + datadogMetricsQueryPath,
		apiKeyValidationEndpoint: address + datadogAPIKeyValidationPath,
	}

	if b := credentials.APIKey; b != "" {
		dd.apiKey = b
	} else {
		return nil, fmt.Errorf("DATADOG_API_KEY is not set")
	}

	if b := credentials.ApplicationKey; b != "" {
		dd.applicationKey = b
	} else {
		return nil, fmt.Errorf("DATADOG_APPLICATION_KEY is not set")
	}

	dd.fromDelta = int64(datadogFromDeltaMultiplierOnMetricInterval * provider.Interval.Seconds())
	return &dd, nil
}

// Execute executes the datadog query against Datadog.metricsQueryEndpoint
// and returns the the first result as float64
func (p *Datadog) Execute(query string) (float64, error) {

	req, err := http.NewRequest("GET", p.metricsQueryEndpoint, nil)
	if err != nil {
		return 0, fmt.Errorf("error http.NewRequest: %w", err)
	}

	req.Header.Set(DatadogAPIKeyHeaderKey, p.apiKey)
	req.Header.Set(DatadogApplicationKeyHeaderKey, p.applicationKey)
	now := time.Now().Unix()
	q := req.URL.Query()
	q.Add("query", query)
	q.Add("from", strconv.FormatInt(now-p.fromDelta, 10))
	q.Add("to", strconv.FormatInt(now, 10))
	req.URL.RawQuery = q.Encode()

	ctx, cancel := context.WithTimeout(req.Context(), p.timeout)
	defer cancel()
	r, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return 0, fmt.Errorf("request failed: %w", err)
	}

	defer r.Body.Close()
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return 0, fmt.Errorf("error reading body: %w", err)
	}

	if r.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("error response: %s: %w", string(b), err)
	}

	var res datadogResponse
	if err := json.Unmarshal(b, &res); err != nil {
		return 0, fmt.Errorf("error unmarshaling result: %w, '%s'", err, string(b))
	}

	if len(res.Series) < 1 {
		return 0, fmt.Errorf("invalid response: %s: %w", string(b), ErrNoValuesFound)
	}

	pl := res.Series[0].Pointlist
	if len(pl) < 1 {
		return 0, fmt.Errorf("invalid response: %s: %w", string(b), ErrNoValuesFound)
	}

	vs := pl[len(pl)-1]
	if len(vs) < 1 {
		return 0, fmt.Errorf("invalid response: %s: %w", string(b), ErrNoValuesFound)
	}

	return vs[1], nil
}
