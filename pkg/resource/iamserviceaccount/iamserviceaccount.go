package iamserviceaccount

import (
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/resource"
	"os/exec"
)

const KeyNamespace = "namespace"
const KeyName = "name"
const KeyCluster = "cluster"
const KeyOverrideExistingServiceAccounts = "override_existing_serviceaccounts"
const KeyAttachPolicyARN = "attach_policy_arn"

func Resource() *schema.Resource {
	return &schema.Resource{
		Create: func(d *schema.ResourceData, meta interface{}) error {
			a := ReadIAMServiceAccount(d)

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

			return resource.Create(exec.Command("eksctl", args...), d, "")
		},
		Delete: func(d *schema.ResourceData, meta interface{}) error {
			a := ReadIAMServiceAccount(d)

			args := []string{
				"delete",
				"iamserviceaccount",
				"--cluster", a.Cluster,
				"--name", a.Name,
				"--namespace", a.Namespace,
			}

			return resource.Delete(exec.Command("eksctl", args...), d)
		},
		Read: func(d *schema.ResourceData, meta interface{}) error {
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
			resource.KeyOutput: {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

type IAMServiceAccount struct {
	Name                            string
	Namespace                       string
	Cluster                         string
	AttachPolicyARN                 string
	OverrideExistingServiceAccounts bool
	Output                          string
}

func ReadIAMServiceAccount(d *schema.ResourceData) *IAMServiceAccount {
	a := IAMServiceAccount{}
	a.Namespace = d.Get(KeyNamespace).(string)
	a.Name = d.Get(KeyName).(string)
	a.Cluster = d.Get(KeyCluster).(string)
	a.AttachPolicyARN = d.Get(KeyAttachPolicyARN).(string)
	a.OverrideExistingServiceAccounts = d.Get(KeyOverrideExistingServiceAccounts).(bool)
	return &a
}
