package cluster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/resource"
	"github.com/rs/xid"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
)

const KeyName = "name"
const KeyRegion = "region"
const KeyAPIVersion = "api_version"
const KeyVersion = "version"
const KeySpec = "spec"
const KeyBin = "eksctl_bin"
const KeyKubectlBin = "kubectl_bin"
const KeyPodsReadinessCheck = "pods_readiness_check"
const KeyLoadBalancerAttachment = "lb_attachment"
const KeyVPCID = "vpc_id"
const KeyManifests = "manifests"

const DefaultAPIVersion = "eksctl.io/v1alpha5"
const DefaultVersion = "1.16"

type CheckPodsReadiness struct {
	namespace  string
	labels     map[string]string
	timeoutSec int
}

func doCheckPodsReadiness(cluster *Cluster, id string) error {
	kubeconfig, err := ioutil.TempFile("", "terraform-provider-eksctl-kubeconfig-")
	if err != nil {
		return err
	}

	kubeconfigPath := kubeconfig.Name()

	if err := kubeconfig.Close(); err != nil {
		return err
	}

	clusterName := cluster.Name + "-" + id

	writeKubeconfigCmd := exec.Command(cluster.EksctlBin, "utils", "write-kubeconfig", "--kubeconfig", kubeconfigPath, "--cluster", clusterName, "--region", cluster.Region)
	if _, err := resource.Run(writeKubeconfigCmd); err != nil {
		return err
	}

	for _, r := range cluster.CheckPodsReadinessConfigs {
		args := []string{"wait", "--namespace", r.namespace, "--for", "condition=ready", "pod",
			"--timeout", fmt.Sprintf("%ds", r.timeoutSec),
		}

		var matches []string
		for k, v := range r.labels {
			matches = append(matches, k+"="+v)
		}

		args = append(args, "-l", strings.Join(matches, ","))

		var selectorArgs []string

		args = append(args, selectorArgs...)

		kubectlCmd := exec.Command(cluster.KubectlBin, args...)

		for _, env := range os.Environ() {
			if !strings.HasPrefix(env, "KUBECONFIG=") {
				kubectlCmd.Env = append(kubectlCmd.Env, env)
			}
		}

		kubectlCmd.Env = append(kubectlCmd.Env, "KUBECONFIG="+kubeconfigPath)

		if _, err := resource.Run(kubectlCmd); err != nil {
			return err
		}
	}

	return nil
}

func doApplyKubernetesManifests(cluster *Cluster, id string) error {
	kubeconfig, err := ioutil.TempFile("", "terraform-provider-eksctl-kubeconfig-")
	if err != nil {
		return err
	}

	kubeconfigPath := kubeconfig.Name()

	if err := kubeconfig.Close(); err != nil {
		return err
	}

	clusterName := cluster.Name + "-" + id

	writeKubeconfigCmd := exec.Command(cluster.EksctlBin, "utils", "write-kubeconfig", "--kubeconfig", kubeconfigPath, "--cluster", clusterName, "--region", cluster.Region)
	if _, err := resource.Run(writeKubeconfigCmd); err != nil {
		return err
	}

	all := strings.Join(cluster.Manifests, "\n---\n")

	kubectlCmd := exec.Command(cluster.KubectlBin, "apply", "-f", "-")

	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, "KUBECONFIG=") {
			kubectlCmd.Env = append(kubectlCmd.Env, env)
		}
	}

	kubectlCmd.Env = append(kubectlCmd.Env, "KUBECONFIG="+kubeconfigPath)

	kubectlCmd.Stdin = bytes.NewBufferString(all)

	if _, err := resource.Run(kubectlCmd); err != nil {
		return err
	}

	return nil
}

func createCluster(d *schema.ResourceData) (string, error) {
	id := newClusterID()

	log.Printf("[DEBUG] creating eksctl cluster with id %q", id)

	cluster, clusterConfig := PrepareClusterConfig(d, id)

	cmd := exec.Command(cluster.EksctlBin, "create", "cluster", "-f", "-")

	cmd.Stdin = bytes.NewReader(clusterConfig)

	if err := resource.Create(cmd, d, id); err != nil {
		return "", err
	}

	if err := doApplyKubernetesManifests(cluster, id); err != nil {
		return "", err
	}

	if err := doCheckPodsReadiness(cluster, id); err != nil {
		return "", err
	}

	return id, nil
}

func updateCluster(d *schema.ResourceData) error {
	log.Printf("[DEBUG] updating eksctl cluster with id %q", d.Id())

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

	applyKubernetesManifests := func(id string) func() error {
		return func() error {
			return doApplyKubernetesManifests(cluster, id)
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

	checkPodsReadiness := func(id string) func() error {
		return func() error {
			return doCheckPodsReadiness(cluster, id)
		}
	}

	id := d.Id()

	tasks := []func() error{
		createNew("nodegroup"),
		associateIAMOIDCProvider(),
		createNew("iamserviceaccount"),
		createNew("fargateprofile"),
		enableRepo(),
		deleteMissing("nodegroup", "--drain"),
		deleteMissing("iamserviceaccount"),
		deleteMissing("fargateprofile"),
		applyKubernetesManifests(id),
		checkPodsReadiness(id),
	}

	for _, t := range tasks {
		if err := t(); err != nil {
			return err
		}
	}

	return nil
}

func deleteCluster(d *schema.ResourceData) error {
	log.Printf("[DEBUG] deleting eksctl cluster with id %q", d.Id())

	cluster, clusterConfig := PrepareClusterConfig(d)

	args := []string{
		"delete",
		"cluster",
		"-f", "-",
		"--wait",
	}

	cmd := exec.Command(cluster.EksctlBin, args...)

	cmd.Stdin = bytes.NewReader(clusterConfig)

	if err := resource.Delete(cmd, d); err != nil {
		return err
	}

	return nil
}

func getClusterKubernetesVersion(d *schema.ResourceData) (string, error) {
	log.Printf("[DEBUG] getting eksctl cluster k8s version with id %q", d.Id())

	cluster, clusterConfig := PrepareClusterConfig(d)

	clusterName := cluster.Name + "-" + d.Id()

	args := []string{
		"get",
		"cluster",
		"--name", clusterName,
		"--region", cluster.Region,
		"-o", "json",
	}

	cmd := exec.Command(cluster.EksctlBin, args...)

	cmd.Stdin = bytes.NewReader(clusterConfig)

	res, err := resource.Run(cmd)
	if err != nil {
		return "", err
	}

	type ClusterData struct {
		Version string `json:"Version"`
	}

	var data []ClusterData

	if err := json.Unmarshal([]byte(res.Output), &data); err != nil {
		return "", err
	}

	if len(data) != 1 {
		return "", fmt.Errorf("BUG: expected number of clusters found by running eksctl get cluster: %d\n\n%v", len(data), data)
	}

	return data[0].Version, nil
}

func Resource() *schema.Resource {
	return &schema.Resource{
		Create: func(d *schema.ResourceData, meta interface{}) error {
			id, err := createCluster(d)
			if err != nil {
				return err
			}

			d.SetId(id)

			return nil
		},
		Update: func(d *schema.ResourceData, meta interface{}) error {
			// TODO shift back 100% traffic to the current cluster before update so that you can use `terraform apply` to
			// cancel previous canary deployment that hang in the middle of the process.

			currentVer, err := getClusterKubernetesVersion(d)
			if err != nil {
				return err
			}

			desiredVer := d.Get(KeyVersion).(string)

			if currentVer != desiredVer {
				newID, err := createCluster(d)
				if err != nil {
					return err
				}

				newTGs, err := getTGs(d, newID)
				if err != nil {
					return err
				}

				curTGs, err := getTGs(d)
				if err != nil {
					return err
				}

				albs, err := getALBs(d)
				if err != nil {
					return err
				}

				for key, alb := range albs {
					newTG := newTGs[key]
					curTG := curTGs[key]
					if newTG != nil {
						if curTG != nil {
							if err := shiftTraffic(alb, newTG, curTG); err != nil {
								return err
							}
						} else {
							if err := attachTG(alb, newTG); err != nil {
								return err
							}
						}
					}
				}

				// TODO Do canary deployment

				if err := deleteCluster(d); err != nil {
					return err
				}

				// TODO If requested, delete remaining stray clusters that didn't complete previous canary deployments

				d.SetId(newID)

				return nil
			}

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
			KeyAPIVersion: {
				Type:     schema.TypeString,
				Optional: true,
				Default:  DefaultAPIVersion,
			},
			KeyVersion: {
				Type:     schema.TypeString,
				Optional: true,
				Default:  DefaultVersion,
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
}

func PrepareClusterConfig(d *schema.ResourceData, newId ...string) (*Cluster, []byte) {
	a := ReadCluster(d)

	spec := map[string]interface{}{}

	if err := yaml.Unmarshal([]byte(a.Spec), spec); err != nil {
		panic(err)
	}

	if a.VPCID != "" {
		switch vpc := spec["vpc"].(type) {
		case map[interface{}]interface{}:
			vpc["id"] = a.VPCID
		}
	}

	var buf bytes.Buffer

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)

	err := enc.Encode(spec)
	if err != nil {
		panic(err)
	}

	specStr := buf.String()

	var id string

	if len(newId) > 0 {
		id = newId[0]
	} else {
		id = d.Id()
	}

	if id == "" {
		panic("Missing Resource ID. This must be a bug!")
	}

	clusterName := fmt.Sprintf("%s-%s", a.Name, id)

	clusterConfig := []byte(fmt.Sprintf(`
apiVersion: %s
kind: ClusterConfig

metadata:
  name: %q
  region: %q
  version: %q

%s
`, a.APIVersion, clusterName, a.Region, a.Version, specStr))

	return a, clusterConfig
}

func ReadCluster(d *schema.ResourceData) *Cluster {
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

	rawManifests := d.Get(KeyManifests).([]interface{})
	for _, m := range rawManifests {
		a.Manifests = append(a.Manifests, m.(string))
	}

	return &a
}
