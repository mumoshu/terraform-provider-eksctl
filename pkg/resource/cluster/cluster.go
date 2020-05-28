package cluster

import (
	"bytes"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/resource"
	"os/exec"
)

const KeyName = "name"
const KeyRegion = "region"
const KeySpec = "spec"
const KeyBin = "eksctl_bin"
const KeyKubectlBin = "kubectl_bin"
const KeyCheckPodsReadiness = "check_pods_readiness"
const KeyLoadBalancerAttachment = "lb_attachment"
const KeyVPCID = "vpc_id"
const KeyManifests = "manifests"

type CheckPodsReadiness struct {
	namespace  string
	labels     map[string]string
	timeoutSec int
}

func Resource() *schema.Resource {
	return &schema.Resource{
		Create: func(d *schema.ResourceData, meta interface{}) error {
			cluster, clusterConfig := PrepareClusterConfig(d)

			cmd := exec.Command(cluster.EksctlBin, "create", "cluster", "-f", "-")

			cmd.Stdin = bytes.NewReader(clusterConfig)

			if err := resource.Create(cmd, d); err != nil {
				return err
			}

			for _, r := range cluster.CheckPodsReadinessConfigs {
				args := []string{"wait", "--namespace", r.namespace, "--for", "condition=ready", "pod",
					"--timeout", fmt.Sprintf("%ds", r.timeoutSec),
				}

				var selectorArgs []string

				args = append(args, selectorArgs...)

				kubectlCmd := exec.Command(cluster.KubectlBin, args...)
				if _, err := resource.Run(kubectlCmd); err != nil {
					return err
				}
			}

			return nil
		},
		Update: func(d *schema.ResourceData, meta interface{}) error {
			cluster, clusterConfig := PrepareClusterConfig(d)

			createNew := func(kind string) func() error {
				return func() error {
					cmd := exec.Command(cluster.EksctlBin, "create", kind, "-f", "-")

					cmd.Stdin = bytes.NewReader(clusterConfig)

					if err := resource.Update(cmd, d); err != nil {
						return fmt.Errorf("%v\n\nCLUSTER CONFIG:\n%s", err, string(clusterConfig))
					}

					return nil
				}
			}

			deleteMissing := func(kind string, extraArgs ...string) func() error {
				return func() error {
					args := append([]string{"delete", kind, "-f", "-", "--only-missing"}, extraArgs...)

					cmd := exec.Command(cluster.EksctlBin, args...)

					cmd.Stdin = bytes.NewReader(clusterConfig)

					if err := resource.Update(cmd, d); err != nil {
						return fmt.Errorf("%v\n\nCLUSTER CONFIG:\n%s", err, string(clusterConfig))
					}

					return nil
				}
			}

			associateIAMOIDCProvider := func() func() error {
				return func() error {
					cmd := exec.Command(cluster.EksctlBin, "utils", "associate-iam-oidc-provider", "-f", "-", "--approve")
					cmd.Stdin = bytes.NewReader(clusterConfig)

					if err := resource.Update(cmd, d); err != nil {
						return fmt.Errorf("%v\n\nCLUSTER CONFIG:\n%s", err, string(clusterConfig))
					}

					return nil
				}
			}

			enableRepo := func() func() error {
				return func() error {
					cmd := exec.Command(cluster.EksctlBin, "enable", "repo", "-f", "-")
					cmd.Stdin = bytes.NewReader(clusterConfig)

					if err := resource.Update(cmd, d); err != nil {
						return fmt.Errorf("%v\n\nCLUSTER CONFIG:\n%s", err, string(clusterConfig))
					}

					return nil
				}
			}

			tasks := []func() error{
				createNew("nodegroup"),
				associateIAMOIDCProvider(),
				createNew("iamserviceaccount"),
				createNew("fargateprofile"),
				enableRepo(),
				deleteMissing("nodegroup", "--drain"),
				deleteMissing("iamserviceaccount"),
				deleteMissing("fargateprofile"),
			}

			for _, t := range tasks {
				if err := t(); err != nil {
					return err
				}
			}

			return nil
		},
		Delete: func(d *schema.ResourceData, meta interface{}) error {
			cluster, clusterConfig := PrepareClusterConfig(d)

			args := []string{
				"delete",
				"cluster",
				"-f", "-",
				"--wait",
			}

			cmd := exec.Command(cluster.EksctlBin, args...)

			cmd.Stdin = bytes.NewReader(clusterConfig)

			return resource.Delete(cmd, d)
		},
		Read: func(d *schema.ResourceData, meta interface{}) error {
			return nil
		},
		Schema: map[string]*schema.Schema{
			KeyRegion: {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			KeyName: {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			KeySpec: {
				Type:     schema.TypeString,
				Required: true,
			},
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
			// The provider runs the following command to ensure that the required pods are up and ready before
			// completing `terraform apply`.
			//
			//   kubectl wait --namespace=${namespace} --for=condition=ready pod
			//     --timeout=${timeout_sec}s -l ${selector generated from labels}`
			KeyCheckPodsReadiness: {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"namespace": {
							Type:     schema.TypeString,
							Required: true,
						},
						"labels": {
							Type:     schema.TypeMap,
							Optional: true,
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
			KeyVPCID: {
				Type:     schema.TypeString,
				Optional: true,
			},
			KeyLoadBalancerAttachment: {
				Type:        schema.TypeList,
				Description: "vpc_id is also required in order to use this configuration",
				Optional:    true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"arn": {
							Type:     schema.TypeString,
							Required: true,
						},
						"protocol": {
							Type:     schema.TypeString,
							Required: true,
						},
						"port": {
							Type:     schema.TypeInt,
							Required: true,
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
			resource.KeyOutput: {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

type Cluster struct {
	EksctlBin  string
	KubectlBin string
	Name       string
	Region     string
	Spec       string
	Output     string

	CheckPodsReadinessConfigs []CheckPodsReadiness
}

func PrepareClusterConfig(d *schema.ResourceData) (*Cluster, []byte) {
	a := ReadCluster(d)

	clusterConfig := []byte(fmt.Sprintf(`
apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig

metadata:
  name: %q
  region: %q

%s
`, a.Name, a.Region, a.Spec))

	return a, clusterConfig
}

func ReadCluster(d *schema.ResourceData) *Cluster {
	a := Cluster{}
	a.EksctlBin = d.Get(KeyBin).(string)
	a.KubectlBin = d.Get(KeyKubectlBin).(string)
	a.Name = d.Get(KeyName).(string)
	a.Region = d.Get(KeyRegion).(string)
	a.Spec = d.Get(KeySpec).(string)

	rawCheckPodsReadiness := d.Get(KeyCheckPodsReadiness).([]interface{})
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

	return &a
}
