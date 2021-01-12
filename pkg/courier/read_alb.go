package courier

import (
	"fmt"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk/api"
	"golang.org/x/xerrors"
	"strconv"
	"time"
)

type ALBSchema struct {
	Address                   string
	ListenerARN               string
	Priority                  string
	Destination               string
	DestinationTargetGroupARN string
	DestinationWeight         string
	StepWeight                string
	StepInterval              string

	Hosts        string
	PathPatterns string
	Methods      string
	SourceIPs    string
	Headers      string
	QueryStrings string
}

func ReadCourierALB(d api.Lister, schema *ALBSchema, metricSchema *MetricSchema) (*CourierALB, error) {
	region, profile := sdk.GetAWSRegionAndProfile(d)

	sess := sdk.AWSSessionFromResourceData(d)

	conf := CourierALB{
		Region:           region,
		Profile:          profile,
		AssumeRoleConfig: sdk.GetAssumeRoleConfig(d),
		Session:          sess,
	}

	if v := d.Get(schema.Address); v != nil {
		conf.Address = v.(string)
	}

	conf.ListenerARN = d.Get(schema.ListenerARN).(string)

	priority := d.Get(schema.Priority)
	switch typed := priority.(type) {
	case int:
		conf.Priority = typed
	case string:
		intv, err := strconv.Atoi(typed)
		if err != nil {
			return nil, xerrors.Errorf("converting priority %q into int: %w", typed, err)
		}

		conf.Priority = intv
	default:
		return nil, xerrors.Errorf("unsupported type of priority: %v(%T)", typed)
	}

	var destinations []Destination

	if v := d.Get(schema.Destination); v != nil {
		for _, arrayItem := range v.([]interface{}) {
			m := arrayItem.(map[string]interface{})
			tgARN := m[schema.DestinationTargetGroupARN].(string)
			rawWeight := m[schema.DestinationWeight]

			var weight int

			switch typed := rawWeight.(type) {
			case int:
				weight = typed
			case string:
				intv, err := strconv.Atoi(typed)
				if err != nil {
					return nil, xerrors.Errorf("converting weight %q into int: %w", typed, err)
				}

				weight = intv
			default:
				return nil, xerrors.Errorf("unsupported type of weight: %v(%T)", typed)
			}

			d := Destination{
				TargetGroupARN: tgARN,
				Weight:         weight,
			}

			destinations = append(destinations, d)
		}
	}

	conf.Destinations = destinations

	stepWeight := 50

	if v := d.Get(schema.StepWeight); v != nil {
		switch typed := v.(type) {
		case int:
			stepWeight = typed
		case string:
			intv, err := strconv.Atoi(typed)
			if err != nil {
				return nil, xerrors.Errorf("converting stepWeight %q into int: %w", typed, err)
			}

			stepWeight = intv
		default:
			return nil, xerrors.Errorf("unsupported type of stepWeight: %v(%T)", typed)
		}
	}

	conf.StepWeight = stepWeight

	stepInterval := 1 * time.Second
	if v := d.Get(schema.StepInterval); v != nil {
		d, err := time.ParseDuration(v.(string))
		if err != nil {
			return nil, fmt.Errorf("error parsing step_interval %v: %w", v, err)
		}

		stepInterval = d
	}

	conf.StepInterval = stepInterval

	metrics, err := ReadMetrics(d, metricSchema)
	if err != nil {
		return nil, err
	}

	conf.Metrics = metrics

	lr, err := ReadListenerRule(d, schema)
	if err != nil {
		return nil, err
	}

	conf.ListenerRule = lr

	return &conf, nil
}
