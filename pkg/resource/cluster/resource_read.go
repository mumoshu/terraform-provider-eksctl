package cluster

import (
	"fmt"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/courier"
)

func ReadCluster(d Read) (*Cluster, error) {
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

		r, err := courier.ReadListenerRule(&courier.MapReader{M: m})
		if err != nil {
			return nil, err
		}

		ms := m[KeyMetrics]

		var metrics []courier.Metric

		if ms != nil {
			var err error

			metrics, err = courier.LoadMetrics(ms.([]interface{}))
			if err != nil {
				return nil, err
			}
		}

		t := courier.ALBAttachment{
			NodeGroupName: m["node_group_name"].(string),
			Weght:         m["weight"].(int),
			ListenerARN:   r.ListenerARN,
			Protocol:      m["protocol"].(string),
			NodePort:      m["node_port"].(int),
			Priority:      r.Priority,
			Hosts:         r.Hosts,
			PathPatterns:  r.PathPatterns,
			Methods:       r.Methods,
			SourceIPs:     r.SourceIPs,
			Headers:       r.Headers,
			QueryStrings:  r.QueryStrings,
			Metrics:       metrics,
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

	{
		metrics := d.Get(KeyMetrics).([]interface{})

		var err error

		a.Metrics, err = courier.LoadMetrics(metrics)
		if err != nil {
			return nil, err
		}
	}

	fmt.Printf("Read Cluster:\n%+v", a)

	return &a, nil
}
