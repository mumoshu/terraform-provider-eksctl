package iamserviceaccount

import (
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk/tfsdk"
	"os/exec"
)

const KeyNamespace = "namespace"
const KeyName = "name"
const KeyRegion = "region"
const KeyProfile = "profile"
const KeyCluster = "cluster"
const KeyOverrideExistingServiceAccounts = "override_existing_serviceaccounts"
const KeyAttachPolicyARN = "attach_policy_arn"

func Resource() *schema.Resource {
	return &schema.Resource{
		Create: func(d *schema.ResourceData, meta interface{}) error {
			a := ReadIAMServiceAccount(d)

			ctx := mustContext(a)

			args := []string{
				"create",
				"iamserviceaccount",
				"--cluster", a.Cluster,
				"--name", a.Name,
				"--namespace", a.Namespace,
			}

			if a.OverrideExistingServiceAccounts {
				args = append(args,
					"--override-existing-serviceaccounts",
				)
			}

			if a.AttachPolicyARN != "" {
				args = append(args,
					"--attach-policy-arn", a.AttachPolicyARN,
				)
			}

			return ctx.Create(exec.Command("eksctl", args...), d, "")
		},
		Delete: func(d *schema.ResourceData, meta interface{}) error {
			a := ReadIAMServiceAccount(d)

			ctx := mustContext(a)

			args := []string{
				"delete",
				"iamserviceaccount",
				"--cluster", a.Cluster,
				"--name", a.Name,
				"--namespace", a.Namespace,
			}

			return ctx.Delete(exec.Command("eksctl", args...))
		},
		Read: func(d *schema.ResourceData, meta interface{}) error {
			return nil
		},
		Update: func(data *schema.ResourceData, i interface{}) error {
			return nil
		},
		Schema: map[string]*schema.Schema{
			KeyNamespace: {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Default:  "default",
			},
			KeyName: {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			KeyRegion: {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			KeyProfile: {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: false,
			},
			KeyCluster: {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			KeyOverrideExistingServiceAccounts: {
				Type:     schema.TypeBool,
				Optional: true,
				ForceNew: true,
			},
			KeyAttachPolicyARN: {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			tfsdk.KeyAssumeRole: tfsdk.SchemaAssumeRole(),
			sdk.KeyOutput: {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

type IAMServiceAccount struct {
	Name                            string
	Namespace                       string
	Region                          string
	Profile                         string
	Cluster                         string
	AttachPolicyARN                 string
	OverrideExistingServiceAccounts bool
	Output                          string
	AssumeRoleConfig                *sdk.AssumeRoleConfig
}

func ReadIAMServiceAccount(d *schema.ResourceData) *IAMServiceAccount {
	a := IAMServiceAccount{}
	a.Namespace = d.Get(KeyNamespace).(string)
	a.Name = d.Get(KeyName).(string)
	a.Region = d.Get(KeyRegion).(string)
	a.Cluster = d.Get(KeyCluster).(string)
	a.AttachPolicyARN = d.Get(KeyAttachPolicyARN).(string)
	a.OverrideExistingServiceAccounts = d.Get(KeyOverrideExistingServiceAccounts).(bool)
	if cfg := tfsdk.GetAssumeRoleConfig(d); cfg != nil {
		a.AssumeRoleConfig = cfg
	}

	return &a
}
