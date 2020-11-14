package courier

import (
	"fmt"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/courier"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/resource"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/resource/cluster"
	"time"
)

type Read interface {
	Get(string) interface{}
}

func toConf(d Read) (*courier.CourierALB, error) {
	region, profile := resource.GetAWSRegionAndProfile(d)

	conf := courier.CourierALB{
		Region:  region,
		Profile: profile,
	}

	if v := d.Get("address"); v != nil {
		conf.Address = v.(string)
	}

	conf.ListenerARN = d.Get("listener_arn").(string)

	conf.Priority = d.Get("priority").(int)

	var destinations []courier.Destination

	if v := d.Get("destination"); v != nil {
		for _, arrayItem := range v.([]interface{}) {
			m := arrayItem.(map[string]interface{})
			tgARN := m["target_group_arn"].(string)
			weight := m["weight"].(int)

			d := courier.Destination{
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

	metrics, err := readMetrics(d)
	if err != nil {
		return nil, err
	}

	conf.Metrics = metrics

	lr, err := courier.ReadListenerRule(d)
	if err != nil {
		return nil, err
	}

	conf.ListenerRule = lr

	return &conf, nil
}

func deleteCourierALB(d cluster.Read) error {
	conf, err := toConf(d)
	if err != nil {
		return err
	}

	alb := &courier.ALB{}

	return alb.Delete(conf)
}

func createOrUpdateCourierALB(d Read) error {
	conf, err := toConf(d)
	if err != nil {
		return err
	}

	alb := &courier.ALB{}

	return alb.Apply(conf)
}
