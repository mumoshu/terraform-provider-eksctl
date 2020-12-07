package courier

import (
	"fmt"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk/api"
	"time"
)

func ReadCourierALB(d api.Getter) (*CourierALB, error) {
	region, profile := sdk.GetAWSRegionAndProfile(d)

	conf := CourierALB{
		Region:  region,
		Profile: profile,
	}

	if v := d.Get("address"); v != nil {
		conf.Address = v.(string)
	}

	conf.ListenerARN = d.Get("listener_arn").(string)

	conf.Priority = d.Get("priority").(int)

	var destinations []Destination

	if v := d.Get("destination"); v != nil {
		for _, arrayItem := range v.([]interface{}) {
			m := arrayItem.(map[string]interface{})
			tgARN := m["target_group_arn"].(string)
			weight := m["weight"].(int)

			d := Destination{
				TargetGroupARN: tgARN,
				Weight:         weight,
			}

			destinations = append(destinations, d)
		}
	}

	conf.Destinations = destinations

	stepWeight := 50
	if v := d.Get("step_weight"); v != nil {
		stepWeight = v.(int)
	}

	conf.StepWeight = stepWeight

	stepInterval := 1 * time.Second
	if v := d.Get("step_interval"); v != nil {
		d, err := time.ParseDuration(v.(string))
		if err != nil {
			return nil, fmt.Errorf("error parsing step_interval %v: %w", v, err)
		}

		stepInterval = d
	}

	conf.StepInterval = stepInterval

	metrics, err := ReadMetrics(d)
	if err != nil {
		return nil, err
	}

	conf.Metrics = metrics

	lr, err := ReadListenerRule(d)
	if err != nil {
		return nil, err
	}

	conf.ListenerRule = lr

	return &conf, nil
}

