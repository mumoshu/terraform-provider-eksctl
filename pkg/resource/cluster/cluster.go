package cluster

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/resource"
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

func Resource() *schema.Resource {
	ALBSupportedProtocols := []string{"http", "https", "tcp", "tls", "udp", "tcp_udp"}

	return &schema.Resource{
		Create: func(d *schema.ResourceData, meta interface{}) error {
			set, err := createCluster(d)
			if err != nil {
				return err
			}

			d.SetId(set.ClusterID)

			return nil
		},
		Update: func(d *schema.ResourceData, meta interface{}) error {
			// TODO shift back 100% traffic to the current cluster before update so that you can use `terraform apply` to
			// cancel previous canary deployment that hang in the middle of the process.

			info, err := getLiveClusterInfo(d)
			if err != nil {
				return err
			}

			k8sVerCurrent := info.KubernetesVersion
			k8sVerDesired := d.Get(KeyVersion).(string)

			revisionCurrent := info.Revision
			revisionDesired := d.Get(KeyRevision).(int)

			log.Printf("determining if a blue-green cluster deploymnet is needed: k8sVer current=%v, desired=%v, rev current=%v, desired=%v", k8sVerCurrent, k8sVerDesired, revisionCurrent, revisionDesired)

			if k8sVerCurrent != k8sVerDesired || revisionCurrent != revisionDesired {
				log.Printf("creating new cluster...")

				set, err := createCluster(d)
				if err != nil {
					return err
				}

				if err := graduallyShiftTraffic(set, set.CanaryOpts); err != nil {
					return err
				}

				if err := deleteCluster(d); err != nil {
					return err
				}

				// TODO If requested, delete remaining stray clusters that didn't complete previous canary deployments

				d.SetId(set.ClusterID)

				return nil
			}

			log.Printf("udapting existing cluster...")

			if err := updateCluster(d); err != nil {
				return err
			}

			return nil
		},
		Delete: func(d *schema.ResourceData, meta interface{}) error {
			if err := deleteCluster(d); err != nil {
				return err
			}

			d.SetId("")

			return nil
		},
		Read: func(d *schema.ResourceData, meta interface{}) error {
			return readCluster(d)
		},
		Schema: map[string]*schema.Schema{
			// "ForceNew" fields
			//
			// the provider does not support zero-downtime updates of these fields so they are set to `ForceNew`,
			// which results recreating cluster without traffic management.
			KeyRegion: {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				DefaultFunc: schema.EnvDefaultFunc("AWS_DEFAULT_REGION", nil),
			},
			KeyName: {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			KeyVPCID: {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			// The below fields can be updated with `terraform apply`, without cluster recreation
			KeyAPIVersion: {
				Type:     schema.TypeString,
				Optional: true,
				Default:  DefaultAPIVersion,
			},
			// TODO EksctlVersion: {...}

			// Version is the K8s version (e.g. 1.15, 1.16) that EKS supports
			// Changing this results in zero-downtime blue-green cluster upgrade.
			KeyVersion: {
				Type:     schema.TypeString,
				Optional: true,
				Default:  DefaultVersion,
			},
			// revision is the manually bumped revision number of the cluster.
			// Increment this so that any changes made to `spec` are deployed via a blue-green cluster deployment.
			KeyRevision: {
				Type:     schema.TypeInt,
				Optional: true,
			},
			// To allow upgrading eksctl and kubectl binaries without upgrading the provider,
			// you can specify the path to the binary.
			KeyBin: {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "eksctl",
			},
			KeyKubectlBin: {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "kubectl",
			},
			// spec is the string containing the part of eksctl cluster.yaml
			// Over time the provider adds HCL-native syntax for any of cluster.yaml items.
			// Until then, this is the primary place you configure the cluster as you like.
			KeySpec: {
				Type:     schema.TypeString,
				Required: true,
				ValidateFunc: func(v interface{}, name string) ([]string, []error) {
					s := v.(string)

					configForVaildation := EksctlClusterConfig{
						Rest: map[string]interface{}{},
					}
					if err := yaml.Unmarshal([]byte(s), &configForVaildation); err != nil {
						return nil, []error{err}
					}

					if configForVaildation.VPC.ID != "" {
						return nil, []error{fmt.Errorf("validating attribute \"spec\": vpc.id must not be set within the spec yaml. use \"vpc_id\" attribute instead, becaues the provider uses it for generating the final eksctl cluster config yaml")}
					}

					return nil, nil
				},
			},
			// The provider runs the following command to ensure that the required pods are up and ready before
			// completing `terraform apply`.
			//
			//   kubectl wait --namespace=${namespace} --for=condition=ready pod
			//     --timeout=${timeout_sec}s -l ${selector generated from labels}`
			KeyPodsReadinessCheck: {
				Type:       schema.TypeList,
				Optional:   true,
				ConfigMode: schema.SchemaConfigModeBlock,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"namespace": {
							Type:     schema.TypeString,
							Required: true,
						},
						"labels": {
							Type:     schema.TypeMap,
							Required: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"timeout_sec": {
							Type:     schema.TypeInt,
							Optional: true,
							Default:  300,
						},
					},
				},
			},
			KeyKubernetesResourceDeletionBeforeDestroy: {
				Type:       schema.TypeList,
				Optional:   true,
				ConfigMode: schema.SchemaConfigModeBlock,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"namespace": {
							Type:     schema.TypeString,
							Required: true,
						},
						"kind": {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.StringInSlice(ValidDeleteK8sResourceKinds, true),
						},
						"name": {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},
			KeyALBAttachment: {
				Type:        schema.TypeList,
				Description: "vpc_id is also required in order to use this configuration",
				Optional:    true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"node_group_name": {
							Type:     schema.TypeString,
							Required: true,
						},
						"weight": {
							Type:     schema.TypeInt,
							Optional: true,
						},
						"listener_arn": {
							Type:     schema.TypeString,
							Required: true,
						},
						"protocol": {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.StringInSlice(ALBSupportedProtocols, true),
						},
						"node_port": {
							Type:     schema.TypeInt,
							Required: true,
						},
						// TODO Expose matching pods IPs via target group. Maybe require the provider to deploy a
						// operator for that.
						"pod_labels": {
							Type:     schema.TypeMap,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
							// https://github.com/hashicorp/terraform-plugin-sdk/issues/71
							//ConflictsWith: []string{"node_port"},
						},
						// Listener rule settings
						"priority": {
							Type:     schema.TypeInt,
							Optional: true,
							Default:  10,
						},
						"hosts": {
							Type:          schema.TypeSet,
							Optional:      true,
							Set:           schema.HashString,
							Elem:          &schema.Schema{Type: schema.TypeString},
							ConflictsWith: []string{"alb_attachment.methods", "alb_attachment.path_patterns", "alb_attachment.source_ips"},
							Description:   "ALB listener rule condition values for host-header condition, e.g. hosts = [\"example.com\", \"*.example.com\"]",
						},
						"methods": {
							Type:          schema.TypeSet,
							Optional:      true,
							Set:           schema.HashString,
							Elem:          &schema.Schema{Type: schema.TypeString},
							ConflictsWith: []string{"alb_attachment.hosts", "alb_attachment.path_patterns", "alb_attachment.source_ips"},
							Description:   "ALB listener rule condition values for http-request-method condition, e.g. methods = [\"get\"]",
						},
						"path_patterns": {
							Type:          schema.TypeSet,
							Optional:      true,
							Set:           schema.HashString,
							Elem:          &schema.Schema{Type: schema.TypeString},
							ConflictsWith: []string{"alb_attachment.hosts", "alb_attachment.methods", "alb_attachment.source_ips"},
							Description: `
PAthPatternConfig values of ALB listener rule condition "path-pattern" field.

Example:

path_patterns = ["/prefix/*"]

produces:

[
  {
      "Field": "path-pattern",
      "PathPatternConfig": {
          "Values": ["/prefix/*"]
      }
  }
]
`,
						},
						"source_ips": {
							Type:     schema.TypeSet,
							Optional: true,
							Set:      schema.HashString,
							// TF fails with `ValidateFunc is not yet supported on lists or sets.`
							//ValidateFunc:  validation.IPRange(),
							Elem:          &schema.Schema{Type: schema.TypeString},
							ConflictsWith: []string{"alb_attachment.hosts", "alb_attachment.methods", "alb_attachment.path_patterns"},
							Description: `
SourceIpConfig values of ALB listener rule condition "source-ip" field.

Example:

headers = ["MYIPD/CIDR"]

produces:

[
  {
      "Field": "source-ip",
      "SourceIpConfig": {
          "Values": ["MYIP/CIDR"]
      }
  }
]
`,
						},
						"headers": {
							Type: schema.TypeMap,
							Elem: &schema.Schema{
								Type: schema.TypeList,
								Elem: &schema.Schema{Type: schema.TypeString},
							},
							Optional: true,
							Description: `HttpHeaderConfig values of ALB listener rule condition "http-header" field.

Example:

headers = {
 Cookie = "condition=foobar"
}

produces:

[
  {
      "Field": "http-header",
      "HttpHeaderConfig": {
          "HttpHeaderName": "Cookie",
          "Values": ["condition=foobar"]
      }
  }
]
`,
						},
						"querystrings": {
							Type: schema.TypeMap,
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
							Optional: true,
							Description: `QueryStringConfig values of ALB listener rule condition "query-string" field.

Example:

querystrings = {
 foo = "bar"
}

produces:

{
     "Field": "query-string",
     "QueryStringConfig": {
         "Values": [
           {
               "Key": "foo",
               "Value": "bar"
           }
         ]
     }
 }
`,
						},
						"analysis": {
							Type: schema.TypeList,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									// The provider waits until healthy target counts becomes greater than 0 and then
									// queries ELB metrics to determine to ensure that
									// - The targetgroup's 5xx count in the interval is LESS THAN max_5xx_count
									// - The targetgroup's 5xx count in the interval is GREATER THAN min_2xx_count
									"interval_sec": {
										Type:     schema.TypeInt,
										Optional: true,
										// ELB emits metrics every 60 sec
										// https://docs.aws.amazon.com/ja_jp/elasticloadbalancing/latest/application/load-balancer-cloudwatch-metrics.html
										Default: 60,
									},
									"max_5xx_count": {
										Type:     schema.TypeInt,
										Optional: true,
									},
									"min_2xx_count": {
										Type:     schema.TypeInt,
										Optional: true,
									},
								},
							},
							Optional: true,
						},
					},
				},
			},
			KeyManifests: {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			KeyTargetGroupARNs: {
				Type:     schema.TypeList,
				Optional: true,
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			resource.KeyOutput: {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
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
		return nil, err
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

		t := ALBAttachment{
			NodeGroupName: m["node_group_name"].(string),
			Weght:         m["weight"].(int),
			ListenerARN:   m["listener_arn"].(string),
			Protocol:      m["protocol"].(string),
			NodePort:      m["node_port"].(int),
			Priority:      m["priority"].(int),
			Hosts:         hosts,
			PathPatterns:  pathPatterns,
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

	fmt.Printf("Read Cluster:\n%+v", a)

	return &a, nil
}
