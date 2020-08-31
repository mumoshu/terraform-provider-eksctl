package courier

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"time"
)

type MapReader struct {
	M map[string]interface{}
}

func (r *MapReader) Get(k string) interface{} {
	return r.M[k]
}

type ResourceReader interface {
	Get(string) interface{}
}

func ReadListenerRule(m ResourceReader) (*ListenerRule, error) {
	var hosts []string
	if r := m.Get("hosts").(*schema.Set); r != nil {
		for _, h := range r.List() {
			hosts = append(hosts, h.(string))
		}
	}

	var pathPatterns []string
	if r := m.Get("path_patterns").(*schema.Set); r != nil {
		for _, p := range r.List() {
			pathPatterns = append(pathPatterns, p.(string))
		}
	}

	var methods []string
	if r := m.Get("methods").(*schema.Set); r != nil {
		for _, p := range r.List() {
			methods = append(methods, p.(string))
		}
	}

	var sourceIPs []string
	if r := m.Get("source_ips").(*schema.Set); r != nil {
		for _, p := range r.List() {
			sourceIPs = append(sourceIPs, p.(string))
		}
	}

	var headers map[string][]string
	if r := m.Get("headers").(map[string]interface{}); r != nil {
		for k, rawVals := range r {
			var vs []string
			for _, rawVal := range rawVals.([]interface{}) {
				vs = append(vs, rawVal.(string))
			}
			headers[k] = vs
		}
	}

	var querystrings map[string]string
	if r := m.Get("querystrings").(map[string]interface{}); r != nil {
		for k, rawVal := range r {
			querystrings[k] = rawVal.(string)
		}
	}

	return &ListenerRule{
		ListenerARN:  m.Get("listener_arn").(string),
		Priority:     m.Get("priority").(int),
		Hosts:        hosts,
		PathPatterns: pathPatterns,
		Methods:      methods,
		SourceIPs:    sourceIPs,
		Headers:      headers,
		QueryStrings: querystrings,
	}, nil
}

func LoadMetrics(metrics []interface{}) ([]Metric, error) {
	var result []Metric

	for _, r := range metrics {
		m := r.(map[string]interface{})

		var max *float64

		if v, set := m["max"]; set {
			vv := v.(float64)
			max = &vv
		}

		var min *float64

		if v, minSet := m["min"]; minSet {
			vv := v.(float64)
			min = &vv
		}

		var interval time.Duration

		if v, set := m["interval"]; set {
			d, err := time.ParseDuration(v.(string))
			if err != nil {
				return nil, fmt.Errorf("parsing metric.interval %q: %v", v, err)
			}

			interval = d
		} else {
			interval = 1 * time.Minute
		}

		metric := Metric{
			Address:  m["address"].(string),
			Query:    m["query"].(string),
			Max:      max,
			Min:      min,
			Interval: interval,
		}

		if v := m["provider"]; v != nil {
			metric.Provider = v.(string)
		}

		result = append(result, metric)
	}

	return result, nil
}

