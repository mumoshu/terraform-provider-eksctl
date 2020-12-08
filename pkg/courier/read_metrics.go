package courier

import (
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk/api"
)

type MetricSchema struct {
	DatadogMetric      string
	CloudWatchMetric   string
	Min, Max, Interval string
	Address            string
	Query              string
	AWSProfile         string
	AWSRegion          string
}

func ReadMetrics(d api.Getter, schema *MetricSchema) ([]Metric, error) {
	var metrics []Metric

	if v := d.Get(schema.DatadogMetric); v != nil {
		ms, err := LoadMetrics(v.([]interface{}), schema)
		if err != nil {
			return nil, err
		}

		for i := range ms {
			ms[i].Provider = "datadog"
		}

		metrics = ms
	}

	if v := d.Get(schema.CloudWatchMetric); v != nil {
		ms, err := LoadMetrics(v.([]interface{}), schema)
		if err != nil {
			return nil, err
		}

		for i := range ms {
			ms[i].Provider = "cloudwatch"
		}

		metrics = append(metrics, ms...)
	}

	return metrics, nil
}
