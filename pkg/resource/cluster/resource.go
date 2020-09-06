package cluster

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/resource"
	"gopkg.in/yaml.v3"
	"log"
)

func ResourceCluster() *schema.Resource {
	m := &Manager{
		DisableClusterNameSuffix: true,
	}
	return &schema.Resource{
		Create: func(d *schema.ResourceData, meta interface{}) error {
			set, err := m.createCluster(d)
			if err != nil {
				return err
			}

			d.SetId(set.ClusterID)

			return nil
		},
		CustomizeDiff: func(d *schema.ResourceDiff, meta interface{}) error {
			_ = m.readCluster(&DiffReadWrite{D: d})

			v := d.Get(KeyKubeconfigPath)

			var kp string

			if v != nil {
				kp = v.(string)
			}

			if d.Id() == "" || kp == "" {
				d.SetNewComputed(KeyKubeconfigPath)
			}

			return nil
		},
		Update: func(d *schema.ResourceData, meta interface{}) error {
			log.Printf("udapting existing cluster...")

			if err := m.updateCluster(d); err != nil {
				return err
			}

			return nil
		},
		Delete: func(d *schema.ResourceData, meta interface{}) error {
			if err := m.deleteCluster(d); err != nil {
				return err
			}

			d.SetId("")

			return nil
		},
		Read: func(d *schema.ResourceData, meta interface{}) error {
			return m.readCluster(d)
		},
		Schema: map[string]*schema.Schema{
			// "ForceNew" fields
			//
			// the provider does not support zero-downtime updates of these fields so they are set to `ForceNew`,
			// which results recreating cluster without traffic management.
			KeyRegion: {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
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
			KeyKubeconfigPath: {
				Type:     schema.TypeString,
				Computed: true,
			},
			// spec is the string containing the part of eksctl cluster.yaml
			// Over time the provider adds HCL-native syntax for any of cluster.yaml items.
			// Until then, this is the primary place you configure the cluster as you like.
			KeySpec: {
				Type:     schema.TypeString,
				Required: true,
				ValidateFunc: func(v interface{}, name string) ([]string, []error) {
					s := v.(string)

					if strings.TrimSpace(s) == "" {
						return nil, nil
					}

					configForVaildation := EksctlClusterConfig{
						Rest: map[string]interface{}{},
					}
					if err := yaml.Unmarshal([]byte(s), &configForVaildation); err != nil {
						return nil, []error{fmt.Errorf("vaidating eksctl_cluster's \"spec\": %w: INPUT:\n%s", err, s)}
					}

					if configForVaildation.VPC.ID != "" {
						return nil, []error{fmt.Errorf("validating eksctl_cluster's \"spec\": vpc.id must not be set within the spec yaml. use \"vpc_id\" attribute instead, becaues the provider uses it for generating the final eksctl cluster config yaml")}
					}

					return nil, nil
				},
			},
			resource.KeyOutput: {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func ResourceClusterDeployment() *schema.Resource {
	ALBSupportedProtocols := []string{"http", "https", "tcp", "tls", "udp", "tcp_udp"}

	metrics := &schema.Schema{
		Type:       schema.TypeList,
		Optional:   true,
		ConfigMode: schema.SchemaConfigModeBlock,
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"provider": {
					Type:     schema.TypeString,
					Required: true,
				},
				"address": {
					Type:     schema.TypeString,
					Optional: true,
					Default:  "",
				},
				"query": {
					Type:     schema.TypeString,
					Required: true,
				},
				"max": {
					Type:     schema.TypeFloat,
					Optional: true,
				},
				"min": {
					Type:     schema.TypeInt,
					Optional: true,
				},
				"interval": {
					Type:     schema.TypeString,
					Optional: true,
					Default:  "1m",
				},
			},
		},
	}

	m := &Manager{}

	return &schema.Resource{
		Create: func(d *schema.ResourceData, meta interface{}) error {
			set, err := m.createCluster(d)
			if err != nil {
				return err
			}

			d.SetId(set.ClusterID)

			return nil
		},
		CustomizeDiff: func(d *schema.ResourceDiff, meta interface{}) error {
			_ = m.readCluster(&DiffReadWrite{D: d})

			v := d.Get(KeyKubeconfigPath)

			var kp string

			if v != nil {
				kp = v.(string)
			}

			if d.Id() == "" || kp == "" {
				d.SetNewComputed(KeyKubeconfigPath)
			}

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

				set, err := m.createCluster(d)
				if err != nil {
					return err
				}

				if err := graduallyShiftTraffic(set, set.CanaryOpts); err != nil {
					return err
				}

				if err := m.deleteCluster(d); err != nil {
					return err
				}

				// TODO If requested, delete remaining stray clusters that didn't complete previous canary deployments

				d.SetId(set.ClusterID)

				return nil
			}

			log.Printf("udapting existing cluster...")

			if err := m.updateCluster(d); err != nil {
				return err
			}

			return nil
		},
		Delete: func(d *schema.ResourceData, meta interface{}) error {
			if err := m.deleteCluster(d); err != nil {
				return err
			}

			d.SetId("")

			return nil
		},
		Read: func(d *schema.ResourceData, meta interface{}) error {
			return m.readCluster(d)
		},
		Schema: map[string]*schema.Schema{
			// "ForceNew" fields
			//
			// the provider does not support zero-downtime updates of these fields so they are set to `ForceNew`,
			// which results recreating cluster without traffic management.
			KeyRegion: {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
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
			KeyKubeconfigPath: {
				Type:     schema.TypeString,
				Computed: true,
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
						KeyMetrics: metrics,
					},
				},
			},
			KeyMetrics: metrics,
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
