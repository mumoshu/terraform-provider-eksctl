package cluster

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"time"
)

func ReadCluster(d *schema.ResourceData) (*Cluster, error) {
	a := Cluster{}
	a.EksctlBin = d.Get(KeyBin).(string)
	a.KubectlBin = d.Get(KeyKubectlBin).(string)
	a.Name = d.Get(KeyName).(string)
	a.Region = d.Get(KeyRegion).(string)
	a.Spec = d.Get(KeySpec).(string)

	a.APIVersion = d.Get(KeyAPIVersion).(string)
	// For migration from older version of the provider that didn't had api_version attribute
	if a.APIVersion == "" {
		a.APIVersion = DefaultAPIVersion
	}

	a.Version = d.Get(KeyVersion).(string)
	// For migration from older version of the provider that didn't had api_version attribute
	if a.Version == "" {
		a.Version = DefaultVersion
	}

	a.VPCID = d.Get(KeyVPCID).(string)

	rawCheckPodsReadiness := d.Get(KeyPodsReadinessCheck).([]interface{})
	for _, r := range rawCheckPodsReadiness {
		m := r.(map[string]interface{})

		labels := map[string]string{}

		rawLabels := m["labels"].(map[string]interface{})
		for k, v := range rawLabels {
			labels[k] = v.(string)
		}

		ccc := CheckPodsReadiness{
			namespace:  m["namespace"].(string),
			labels:     labels,
			timeoutSec: m["timeout_sec"].(int),
		}

		a.CheckPodsReadinessConfigs = append(a.CheckPodsReadinessConfigs, ccc)
	}

	resourceDeletions := d.Get(KeyKubernetesResourceDeletionBeforeDestroy).([]interface{})
	for _, r := range resourceDeletions {
		m := r.(map[string]interface{})

		d := DeleteKubernetesResource{
			Namespace: m["namespace"].(string),
			Name:      m["name"].(string),
			Kind:      m["kind"].(string),
		}

		a.DeleteKubernetesResourcesBeforeDestroy = append(a.DeleteKubernetesResourcesBeforeDestroy, d)
	}

	albAttachments := d.Get(KeyALBAttachment).([]interface{})
	for _, r := range albAttachments {
		m := r.(map[string]interface{})

		var hosts []string
		if r := m["hosts"].(*schema.Set); r != nil {
			for _, h := range r.List() {
				hosts = append(hosts, h.(string))
			}
		}

		var pathPatterns []string
		if r := m["path_patterns"].(*schema.Set); r != nil {
			for _, p := range r.List() {
				pathPatterns = append(pathPatterns, p.(string))
			}
		}

		var methods []string
		if r := m["methods"].(*schema.Set); r != nil {
			for _, p := range r.List() {
				methods = append(methods, p.(string))
			}
		}

		var sourceIPs []string
		if r := m["source_ips"].(*schema.Set); r != nil {
			for _, p := range r.List() {
				sourceIPs = append(sourceIPs, p.(string))
			}
		}

		var headers map[string][]string
		if r := m["headers"].(map[string]interface{}); r != nil {
			for k, rawVals := range r {
				var vs []string
				for _, rawVal := range rawVals.([]interface{}) {
					vs = append(vs, rawVal.(string))
				}
				headers[k] = vs
			}
		}

		var querystrings map[string]string
		if r := m["querystrings"].(map[string]interface{}); r != nil {
			for k, rawVal := range r {
				querystrings[k] = rawVal.(string)
			}
		}

		t := ALBAttachment{
			NodeGroupName: m["node_group_name"].(string),
			Weght:         m["weight"].(int),
			ListenerARN:   m["listener_arn"].(string),
			Protocol:      m["protocol"].(string),
			NodePort:      m["node_port"].(int),
			Priority:      m["priority"].(int),
			Hosts:         hosts,
			PathPatterns:  pathPatterns,
			Methods:       methods,
			SourceIPs:     sourceIPs,
			Headers:       headers,
			QueryStrings:  querystrings,
		}

		a.ALBAttachments = append(a.ALBAttachments, t)
	}

	rawManifests := d.Get(KeyManifests).([]interface{})
	for _, m := range rawManifests {
		a.Manifests = append(a.Manifests, m.(string))
	}

	tgARNs := d.Get(KeyTargetGroupARNs).([]interface{})
	for _, arn := range tgARNs {
		a.TargetGroupARNs = append(a.TargetGroupARNs, arn.(string))
	}

	metrics := d.Get(KeyMetrics).([]interface{})
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
			Provider: m["provider"].(string),
			Address:  m["address"].(string),
			Query:    m["query"].(string),
			Max:      max,
			Min:      min,
			Interval: interval,
		}
		a.Metrics = append(a.Metrics, metric)
	}

	fmt.Printf("Read Cluster:\n%+v", a)

	return &a, nil
}
