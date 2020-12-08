package cluster

import (
	"fmt"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk/api"
)

func ReadCluster(d api.Getter) (*Cluster, error) {
	a := Cluster{}
	a.EksctlBin = d.Get(KeyBin).(string)
	a.EksctlVersion = d.Get(KeyEksctlVersion).(string)
	a.KubectlBin = d.Get(KeyKubectlBin).(string)
	a.Name = d.Get(KeyName).(string)
	a.Region = d.Get(KeyRegion).(string)
	a.Profile = d.Get(KeyProfile).(string)
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

	if v := d.Get(KeyPodsReadinessCheck); v != nil {
		rawCheckPodsReadiness := v.([]interface{})
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
	}

	if v := d.Get(KeyKubernetesResourceDeletionBeforeDestroy); v != nil {
		resourceDeletions := v.([]interface{})
		for _, r := range resourceDeletions {
			m := r.(map[string]interface{})

			d := DeleteKubernetesResource{
				Namespace: m["namespace"].(string),
				Name:      m["name"].(string),
				Kind:      m["kind"].(string),
			}

			a.DeleteKubernetesResourcesBeforeDestroy = append(a.DeleteKubernetesResourcesBeforeDestroy, d)
		}
	}

	if v := d.Get(KeyManifests); v != nil {
		rawManifests := v.([]interface{})
		for _, m := range rawManifests {
			a.Manifests = append(a.Manifests, m.(string))
		}
	}

	if v := d.Get(KeyTargetGroupARNs); v != nil {
		tgARNs := v.([]interface{})
		for _, arn := range tgARNs {
			a.TargetGroupARNs = append(a.TargetGroupARNs, arn.(string))
		}
	}

	fmt.Printf("Read Cluster:\n%+v", a)

	return &a, nil
}
