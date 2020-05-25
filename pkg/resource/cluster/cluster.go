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

func Resource() *schema.Resource {
	return &schema.Resource{
		Create: func(d *schema.ResourceData, meta interface{}) error {
			cluster, clusterConfig := PrepareClusterConfig(d)

			cmd := exec.Command(cluster.EksctlBin, "create", "cluster", "-f", "-")

			cmd.Stdin = bytes.NewReader(clusterConfig)

			return resource.Create(cmd, d)
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

			tasks := []func() error{
				createNew("nodegroup"),
				createNew("iamserviceaccount"),
				createNew("fargateprofile"),
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
			resource.KeyOutput: {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

type Cluster struct {
	EksctlBin string
	Name      string
	Region    string
	Spec      string
	Output    string
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
	a.Name = d.Get(KeyName).(string)
	a.Region = d.Get(KeyRegion).(string)
	a.Spec = d.Get(KeySpec).(string)
	return &a
}
