package cluster

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/rs/xid"
	"gopkg.in/yaml.v3"
	"log"
	"time"
)

const KeyName = "name"
const KeyRegion = "region"
const KeyAPIVersion = "api_version"
const KeyVersion = "version"
const KeyRevision = "revision"
const KeySpec = "spec"
const KeyBin = "eksctl_bin"
const KeyKubectlBin = "kubectl_bin"
const KeyPodsReadinessCheck = "pods_readiness_check"
const KeyKubernetesResourceDeletionBeforeDestroy = "kubernetes_resource_deletion_before_destroy"
const KeyALBAttachment = "alb_attachment"
const KeyVPCID = "vpc_id"
const KeyManifests = "manifests"
const KeyMetrics = "metrics"

const (
	KeyTargetGroupARNs = "target_group_arns"
)

const DefaultAPIVersion = "eksctl.io/v1alpha5"
const DefaultVersion = "1.16"

var ValidDeleteK8sResourceKinds = []string{"deployment", "deploy", "pod", "service", "svc", "statefulset", "job"}

type CheckPodsReadiness struct {
	namespace  string
	labels     map[string]string
	timeoutSec int
}

func newClusterID() string {
	return xid.New().String()
}

type Cluster struct {
	EksctlBin  string
	KubectlBin string
	Name       string
	Region     string
	APIVersion string
	Version    string
	VPCID      string
	Spec       string
	Output     string
	Manifests  []string

	CheckPodsReadinessConfigs []CheckPodsReadiness

	DeleteKubernetesResourcesBeforeDestroy []DeleteKubernetesResource

	PublicSubnetIDs  []string
	PrivateSubnetIDs []string
	ALBAttachments   []ALBAttachment
	TargetGroupARNs  []string
	Metrics          []Metric
}

type Metric struct {
	Provider string
	Address  string
	Query    string
	Max      *float64
	Min      *float64
	Interval time.Duration
}

type DeleteKubernetesResource struct {
	Namespace string
	Name      string
	Kind      string
}

type EksctlClusterConfig struct {
	VPC        VPC                    `yaml:"vpc"`
	NodeGroups []NodeGroup            `yaml:"nodeGroups"`
	Rest       map[string]interface{} `yaml:",inline"`
}

type VPC struct {
	ID      string  `yaml:"id"`
	Subnets Subnets `yaml:"subnets"`
}

type Subnets struct {
	Public  map[string]Subnet `yaml:"public"`
	Private map[string]Subnet `yaml:"private"`
}

type Subnet struct {
	ID string `yaml:"id"`
}

type ALBAttachment struct {
	NodeGroupName string
	Weght         int
	ListenerARN   string

	// TargetGroup settings

	NodePort int
	Protocol string

	// ALB Listener Rule settings
	Priority     int
	Hosts        []string
	PathPatterns []string
	Methods      []string
	SourceIPs    []string
	Headers      map[string][]string
	QueryStrings map[string]string
	Metrics      []Metric
}

type ClusterSet struct {
	ClusterID        string
	ClusterName      ClusterName
	Cluster          *Cluster
	ClusterConfig    []byte
	ListenerStatuses ListenerStatuses
	CanaryOpts       CanaryOpts
}

type NodeGroup struct {
	Name            string                 `yaml:"name"`
	TargetGroupARNS []string               `yaml:"targetGroupARNS"`
	Rest            map[string]interface{} `yaml:",inline"`
}

func PrepareClusterSet(d *schema.ResourceData, optNewId ...string) (*ClusterSet, error) {
	a, err := ReadCluster(d)
	if err != nil {
		return nil, err
	}

	spec := map[string]interface{}{}

	if err := yaml.Unmarshal([]byte(a.Spec), spec); err != nil {
		return nil, err
	}

	if a.VPCID != "" {
		if _, ok := spec["vpc"]; !ok {
			spec["vpc"] = map[string]interface{}{}
		}

		switch vpc := spec["vpc"].(type) {
		case map[interface{}]interface{}:
			vpc["id"] = a.VPCID
		}
	}

	var specStr string
	{
		var buf bytes.Buffer

		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)

		if err := enc.Encode(spec); err != nil {
			return nil, err
		}

		specStr = buf.String()
	}

	var id string
	var newId string

	if len(optNewId) > 0 {
		id = optNewId[0]
		newId = optNewId[0]
	} else {
		id = d.Id()
	}

	if id == "" {
		return nil, errors.New("Missing Resource ID. This must be a bug!")
	}

	clusterName := fmt.Sprintf("%s-%s", a.Name, id)

	listenerStatuses, err := planListenerChanges(a, d.Id(), newId)
	if err != nil {
		return nil, fmt.Errorf("planning listener changes: %v", err)
	}

	seedClusterConfig := []byte(fmt.Sprintf(`
apiVersion: %s
kind: ClusterConfig

metadata:
  name: %q
  region: %q
  version: %q

%s
`, a.APIVersion, clusterName, a.Region, a.Version, specStr))

	c := EksctlClusterConfig{
		VPC: VPC{
			ID: "",
			Subnets: Subnets{
				Public:  map[string]Subnet{},
				Private: map[string]Subnet{},
			},
		},
		NodeGroups: []NodeGroup{},
		Rest:       map[string]interface{}{},
	}

	if err := yaml.Unmarshal(seedClusterConfig, &c); err != nil {
		return nil, err
	}
	//
	//for i := range c.NodeGroups {
	//	ng := c.NodeGroups[i]
	//
	//	for _, l := range listenerStatuses {
	//		for _, a := range l.ALBAttachments {
	//			if ng.Name == a.NodeGroupName {
	//				ng.TargetGroupARNS = append(ng.TargetGroupARNS, *l.DesiredTG.TargetGroupArn)
	//			}
	//		}
	//	}
	//}

	var mergedClusterConfig []byte
	{
		var buf bytes.Buffer

		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)

		if err := enc.Encode(c); err != nil {
			return nil, err
		}

		mergedClusterConfig = buf.Bytes()
	}

	log.Printf("seed cluster config:\n%s", string(seedClusterConfig))
	log.Printf("merged cluster config:\n%s", string(mergedClusterConfig))

	for _, s := range c.VPC.Subnets.Public {
		a.PublicSubnetIDs = append(a.PublicSubnetIDs, s.ID)
	}

	for _, s := range c.VPC.Subnets.Public {
		a.PrivateSubnetIDs = append(a.PrivateSubnetIDs, s.ID)
	}

	a.VPCID = c.VPC.ID

	return &ClusterSet{
		ClusterID:        id,
		ClusterName:      getClusterName(a, id),
		Cluster:          a,
		ClusterConfig:    mergedClusterConfig,
		ListenerStatuses: listenerStatuses,
		CanaryOpts: CanaryOpts{
			CanaryAdvancementInterval: 5 * time.Second,
			CanaryAdvancementStep:     5,
		},
	}, nil
}
