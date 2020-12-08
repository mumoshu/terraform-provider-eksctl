package courier

import (
	"errors"
	"fmt"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk/api"
	"golang.org/x/xerrors"
	"strconv"
	"time"
)

func ReadListenerRule(m api.Lister, schema *ALBSchema) (*ListenerRule, error) {
	var hosts []string
	if r := m.List(schema.Hosts); r != nil {
		for _, h := range r {
			hosts = append(hosts, h.(string))
		}
	}

	var pathPatterns []string
	if r := m.List(schema.PathPatterns); r != nil {
		for _, p := range r {
			pathPatterns = append(pathPatterns, p.(string))
		}
	}

	var methods []string
	if r := m.List(schema.Methods); r != nil {
		for _, p := range r {
			methods = append(methods, p.(string))
		}
	}

	var sourceIPs []string
	if r := m.List(schema.SourceIPs); r != nil {
		for _, p := range r {
			sourceIPs = append(sourceIPs, p.(string))
		}
	}

	var headers map[string][]string

	if v := m.Get(schema.Headers); v != nil {
		if r := v.(map[string]interface{}); r != nil {
			for k, rawVals := range r {
				var vs []string
				for _, rawVal := range rawVals.([]interface{}) {
					vs = append(vs, rawVal.(string))
				}
				headers[k] = vs
			}
		}
	}

	var querystrings map[string]string
	if v := m.Get(schema.QueryStrings); v != nil {
		if r := v.(map[string]interface{}); r != nil {
			for k, rawVal := range r {
				querystrings[k] = rawVal.(string)
			}
		}
	}

	if len(hosts) == 0 && len(pathPatterns) == 0 && len(methods) == 0 && len(sourceIPs) == 0 && len(headers) == 0 &&
		len(querystrings) == 0 {

		return nil, errors.New("one ore more rule condition(s) are required. Specify `hosts`, `path_patterns`, `methods`, `source_ips`, `headers`, or `querystrings`")
	}

	var priority int

	switch typed := m.Get(schema.Priority).(type) {
	case int:
		priority = typed
	case string:
		intv, err := strconv.Atoi(typed)
		if err != nil {
			return nil, xerrors.Errorf("converting priority %q into int: %w", typed, err)
		}

		priority = intv
	default:
		return nil, xerrors.Errorf("unsupported type of priority: %v(%T)", typed)
	}

	return &ListenerRule{
		ListenerARN:  m.Get(schema.ListenerARN).(string),
		Priority:     priority,
		Hosts:        hosts,
		PathPatterns: pathPatterns,
		Methods:      methods,
		SourceIPs:    sourceIPs,
		Headers:      headers,
		QueryStrings: querystrings,
	}, nil
}

func LoadMetrics(metrics []interface{}, schema *MetricSchema) ([]Metric, error) {
	var result []Metric

	for _, r := range metrics {
		m := r.(map[string]interface{})

		var max *float64

		if v, set := m[schema.Max]; set {
			vv := v.(float64)
			max = &vv
		}

		var min *float64

		if v, minSet := m[schema.Min]; minSet {
			vv := v.(float64)
			min = &vv
		}

		var interval time.Duration

		if v, set := m[schema.Interval]; set {
			d, err := time.ParseDuration(v.(string))
			if err != nil {
				return nil, fmt.Errorf("parsing metric.interval %q: %v", v, err)
			}

			interval = d
		} else {
			interval = 1 * time.Minute
		}

		metric := Metric{
			Address:    m[schema.Address].(string),
			Query:      m[schema.Query].(string),
			AWSRegion:  m[schema.AWSRegion].(string),
			AWSProfile: m[schema.AWSProfile].(string),
			Max:        max,
			Min:        min,
			Interval:   interval,
		}

		if v := m["provider"]; v != nil {
			metric.Provider = v.(string)
		}

		result = append(result, metric)
	}

	return result, nil
}
