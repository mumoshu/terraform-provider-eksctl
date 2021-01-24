package cluster

import (
	"fmt"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk/tfsdk"
	"log"
	"regexp"
	"runtime/debug"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"gopkg.in/yaml.v3"
)

func ResourceCluster() *schema.Resource {
	m := &Manager{
		DisableClusterNameSuffix: true,
	}
	return &schema.Resource{
		Create: func(d *schema.ResourceData, meta interface{}) (finalErr error) {
			defer func() {
				if err := recover(); err != nil {
					finalErr = fmt.Errorf("unhandled error: %v\n%s", err, debug.Stack())
				}
			}()

			set, err := m.createCluster(d)
			if err != nil {
				return fmt.Errorf("creating cluster: %w", err)
			}

			d.SetId(set.ClusterID)

			if err := loadOIDCProviderURLAndARN(d, set.Cluster); err != nil {
				return fmt.Errorf("loading oidc issuer url: %w", err)
			}

			return nil
		},
		CustomizeDiff: func(d *schema.ResourceDiff, meta interface{}) (finalErr error) {
			defer func() {
				if err := recover(); err != nil {
					finalErr = fmt.Errorf("unhandled error: %v\n%s", err, debug.Stack())
				}
			}()

			if err := m.planCluster(&DiffReadWrite{D: d}); err != nil {
				return fmt.Errorf("diffing cluster: %w", err)
			}

			v := d.Get(KeyKubeconfigPath)

			var kp string

			if v != nil {
				kp = v.(string)
			}

			if d.Id() == "" || kp == "" {
				d.SetNewComputed(KeyKubeconfigPath)
			}

			if err := validateDrainNodeGroups(d); err != nil {
				return fmt.Errorf("drain error: %s", err)
			}

			return nil
		},
		Update: func(d *schema.ResourceData, meta interface{}) (finalErr error) {
			defer func() {
				if err := recover(); err != nil {
					finalErr = fmt.Errorf("unhandled error: %v\n%s", err, debug.Stack())
				}
			}()

			log.Printf("udapting existing cluster...")

			set, err := m.updateCluster(d)
			if err != nil {
				return fmt.Errorf("updating cluster: %w", err)
			}

			if err := loadOIDCProviderURLAndARN(d, set.Cluster); err != nil {
				return fmt.Errorf("loading oidc issuer url: %w", err)
			}

			return nil
		},
		Delete: func(d *schema.ResourceData, meta interface{}) (finalErr error) {
			defer func() {
				if err := recover(); err != nil {
					finalErr = fmt.Errorf("unhandled error: %v\n%s", err, debug.Stack())
				}
			}()

			if err := m.deleteCluster(d); err != nil {
				return err
			}

			d.SetId("")

			return nil
		},
		Read: func(d *schema.ResourceData, meta interface{}) (finalErr error) {
			defer func() {
				if err := recover(); err != nil {
					finalErr = fmt.Errorf("unhandled error: %v\n%s", err, debug.Stack())
				}
			}()

			_, err := m.readCluster(d)
			if err != nil {
				return fmt.Errorf("reading cluster: %w", err)
			}

			return nil
		},
		Importer: &schema.ResourceImporter{
			State: func(data *schema.ResourceData, i interface{}) ([]*schema.ResourceData, error) {
				data, err := m.importCluster(data)
				if err != nil {
					return nil, fmt.Errorf("importing cluster: %w", err)
				}

				return []*schema.ResourceData{data}, nil
			},
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
			KeyProfile: {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			tfsdk.KeyAssumeRole: tfsdk.AssumeRoleSchema(),
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
			// Tags is the metadata.tags in the cluster config
			KeyTags: {
				Type:     schema.TypeMap,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Default:  map[string]interface{}{},
				ForceNew: true,
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
			KeyEksctlVersion: {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			KeyKubectlBin: {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "kubectl",
			},
			KeyKubeconfigPath: {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
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
			KeyDrainNodeGroups: {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeBool,
				},
			},
			KeyIAMIdentityMapping: {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"iamarn": {
							Required: true,
							Type:     schema.TypeString,
						},
						"username": {
							Required: true,
							Type:     schema.TypeString,
						},
						"groups": {
							Required: true,
							Type:     schema.TypeList,
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
						},
					},
				},
			},
			KeyAWSAuthConfigMap: {
				Type:     schema.TypeSet,
				Computed: true,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"iamarn": {
							Required: true,
							Type:     schema.TypeString,
						},
						"username": {
							Required: true,
							Type:     schema.TypeString,
						},
						"groups": {
							Required: true,
							Type:     schema.TypeList,
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
						},
					},
				},
			},
			sdk.KeyOutput: {
				Type:     schema.TypeString,
				Computed: true,
			},
			KeyOIDCProviderURL: {
				Type:     schema.TypeString,
				Computed: true,
			},
			KeyOIDCProviderARN: {
				Type:     schema.TypeString,
				Computed: true,
			},
			KeySecurityGroupIDs: {
				Computed: true,
				Type:     schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}
}

func validateDrainNodeGroups(d *schema.ResourceDiff) error {

	if v, ok := d.GetOk(KeyDrainNodeGroups); ok {

		spec := ""

		if s, ok := d.GetOk(KeySpec); ok {
			spec = s.(string)
		}

		nodegroups := v.(map[string]interface{})
		for k := range nodegroups {
			reg := regexp.MustCompile(`- name: ` + k)
			if !reg.MatchString(spec) {
				return fmt.Errorf("no such nodegroup to drain '%s'", k)
			}
		}
	}

	return nil
}
