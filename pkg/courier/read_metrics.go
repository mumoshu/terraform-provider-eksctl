package courier

import (
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk/api"
)

func ReadMetrics(d api.Getter) ([]Metric, error) {
	var metrics []Metric

	if v := d.Get("datadog_metric"); v != nil {
		ms, err := LoadMetrics(v.([]interface{}))
		if err != nil {
			return nil, err
		}

		for i := range ms {
			ms[i].Provider = "datadog"
		}

		metrics = ms
	}

	if v := d.Get("cloudwatch_metric"); v != nil {
		ms, err := LoadMetrics(v.([]interface{}))
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
