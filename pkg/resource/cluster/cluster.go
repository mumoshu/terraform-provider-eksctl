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

func Resource() *schema.Resource {
	return &schema.Resource{
		Create: func(d *schema.ResourceData, meta interface{}) error {
			clusterConfig := RenderClusterConfig(d)

			cmd := exec.Command("eksctl", "create", "cluster", "-f", "-")

			cmd.Stdin = bytes.NewReader(clusterConfig)

			return resource.Create(cmd, d)
		},
		Update: func(d *schema.ResourceData, meta interface{}) error {
			clusterConfig := RenderClusterConfig(d)

			createNew := func(kind string) func() error {
				return func() error {
					cmd := exec.Command("eksctl", "create", kind, "-f", "-")

					cmd.Stdin = bytes.NewReader(clusterConfig)

					if err := resource.Update(cmd, d); err != nil {
						return fmt.Errorf("%v\n\nCLUSTER CONFIG:\n%s", clusterConfig)
					}

					return nil
				}
			}

			deleteMissing := func(kind string, extraArgs ...string) func() error {
				return func() error {
					args := append([]string{"delete", kind, "-f", "-", "--only-missing"}, extraArgs...)

					cmd := exec.Command("eksctl", args...)

					cmd.Stdin = bytes.NewReader(clusterConfig)

					if err := resource.Update(cmd, d); err != nil {
						return fmt.Errorf("%v\n\nCLUSTER CONFIG:\n%s", clusterConfig)
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
			clusterConfig := RenderClusterConfig(d)

			args := []string{
				"delete",
				"cluster",
				"-f", "-",
			}

			cmd := exec.Command("eksctl", args...)

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
			resource.KeyOutput: {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

type Cluster struct {
	Name   string
	Region string
	Spec   string
	Output string
}

func RenderClusterConfig(d *schema.ResourceData) []byte {
	a := ReadCluster(d)

	clusterConfig := []byte(fmt.Sprintf(`
apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig

metadata:
  name: %q
  region: %q

%s
`, a.Name, a.Region, a.Spec))

	return clusterConfig
}

func ReadCluster(d *schema.ResourceData) *Cluster {
	a := Cluster{}
	a.Name = d.Get(KeyName).(string)
	a.Region = d.Get(KeyRegion).(string)
	a.Spec = d.Get(KeySpec).(string)
	return &a
}
