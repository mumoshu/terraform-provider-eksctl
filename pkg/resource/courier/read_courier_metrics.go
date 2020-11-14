package courier

import (
	"github.com/mumoshu/terraform-provider-eksctl/pkg/courier"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/resource/cluster"
)

func readMetrics(d cluster.Read) ([]courier.Metric, error) {
	var metrics []courier.Metric

	if v := d.Get("datadog_metric"); v != nil {
		ms, err := courier.LoadMetrics(v.([]interface{}))
		if err != nil {
			return nil, err
		}

		for i := range ms {
			ms[i].Provider = "datadog"
		}

		metrics = ms
	}

	if v := d.Get("cloudwatch_metric"); v != nil {
		ms, err := courier.LoadMetrics(v.([]interface{}))
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
