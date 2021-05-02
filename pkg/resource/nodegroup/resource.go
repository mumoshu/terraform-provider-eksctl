package nodegroup

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk/api"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk/tfsdk"
	"math/rand"
	"os/exec"
	"runtime/debug"
	"strings"
)

type Op uint8

const (
	Create Op = 1 << iota
	Delete
)

type Attr struct {
	Key    string
	Schema *schema.Schema
	Args   func(d api.Getter) []string
	Ops    Op
}

type SchemaOption func(*schema.Schema)

type Tpe struct {
	Type schema.ValueType
	Elem *schema.Schema
	Args func(key, flag string, def interface{}, d api.Getter) (args []string)
}

func NewAttr(name string, tpe Tpe, ops Op, opts ...SchemaOption) Attr {
	key := strings.ReplaceAll(name, "-", "_")
	flag := strings.ReplaceAll(name, "_", "-")

	sc := &schema.Schema{
		Type:     tpe.Type,
		Elem:     tpe.Elem,
		ForceNew: true,
	}

	for _, o := range opts {
		o(sc)
	}

	if !sc.Required {
		sc.Optional = true
	}

	return Attr{
		Key:    key,
		Schema: sc,
		Args: func(d api.Getter) []string {
			return tpe.Args(key, flag, sc.Default, d)
		},
		Ops: ops,
	}
}

var String = Tpe{
	Type: schema.TypeString,
	Args: func(key, flag string, def interface{}, d api.Getter) (args []string) {
		if v, ok := d.Get(key).(string); ok && v != "" {
			args = append(args, "--"+flag, v)
		}
		return
	},
}

var Bool = Tpe{
	Type: schema.TypeBool,
	Args: func(key, flag string, def interface{}, d api.Getter) (args []string) {
		if v, ok := d.Get(key).(bool); ok {
			if def != nil {
				if def.(bool) != v {
					args = append(args, "--"+flag)
				}
			} else if v {
				args = append(args, "--"+flag)
			}
		}
		return
	},
}

var Int = Tpe{
	Type: schema.TypeInt,
	Args: func(key, flag string, def interface{}, d api.Getter) (args []string) {
		if v, ok := d.Get(key).(int); ok {
			if def != nil {
				if def.(int) != v {
					args = append(args, "--"+flag)
				}
			} else if v > 0 {
				args = append(args, "--"+flag, fmt.Sprintf("%d", v))
			}
		}
		return
	},
}

var Strings = Tpe{
	Type: schema.TypeList,
	Elem: &schema.Schema{
		Type: schema.TypeString,
	},
	Args: func(key, flag string, def interface{}, d api.Getter) (args []string) {
		if vs, ok := d.Get(key).([]interface{}); ok && len(vs) > 0 {
			var ss []string

			for _, v := range vs {
				ss = append(ss, fmt.Sprintf("%v", v))
			}

			args = append(args, "--"+flag, strings.Join(ss, ","))
		}
		return
	},
}

var StringMap = Tpe{
	Type: schema.TypeMap,
	Elem: &schema.Schema{
		Type: schema.TypeString,
	},
	Args: func(key, flag string, def interface{}, d api.Getter) (args []string) {
		if m, ok := d.Get(key).(map[string]interface{}); ok && len(m) > 0 {
			var vs []string

			for k, v := range m {
				vs = append(vs, fmt.Sprintf("%s=%v", k, v))
			}

			args = append(args, "--"+flag, strings.Join(vs, ","))
		}
		return
	},
}

func Required() func(sc *schema.Schema) {
	return func(sc *schema.Schema) {
		sc.Required = true
	}
}

func Default(v interface{}) func(sc *schema.Schema) {
	return func(sc *schema.Schema) {
		sc.Default = v
	}
}

const (
	KeyEksctlVersion = "eksctl_version"
)

func Resource() *schema.Resource {
	subresource := "nodegroup"

	attrs := []Attr{
		NewAttr("cluster", String, Create|Delete, Required()),
		NewAttr("name", String, Create|Delete, Required()),
		NewAttr(KeyEksctlVersion, String, Create|Delete),
		NewAttr("tags", StringMap, Create),
		NewAttr("region", String, Create|Delete),
		NewAttr("version", String, Create),
		NewAttr("node-type", String, Create),
		NewAttr("nodes", Int, Create),
		NewAttr("nodes-min", Int, Create),
		NewAttr("nodes-max", Int, Create),
		NewAttr("node-volume-size", Int, Create),
		NewAttr("node-volume-type", Int, Create),
		NewAttr("ssh-access", Bool, Create),
		NewAttr("ssh-public-key", String, Create),
		NewAttr("enable-ssm", Bool, Create),
		NewAttr("node-ami", String, Create),
		NewAttr("node-ami-family", String, Create),
		NewAttr("node-private-networking", Bool, Create),
		NewAttr("node-security-groups", Strings, Create),
		NewAttr("node-labels", StringMap, Create),
		NewAttr("node-zones", Strings, Create),
		NewAttr("instance-prefix", String, Create),
		NewAttr("instance-name", String, Create),
		NewAttr("disable-pod-imds", Bool, Create),
		NewAttr("managed", Bool, Create),
		NewAttr("spot", Bool, Create),
		NewAttr("drain", Bool, Delete, Default(true)),
		NewAttr("instance-types", Strings, Create),
		NewAttr("asg-access", Bool, Create),
		NewAttr("external-dns-access", Bool, Create),
		NewAttr("full-ecr-access", Bool, Create),
		NewAttr("appmesh-access", Bool, Create),
		NewAttr("appmesh-preview-access", Bool, Create),
		NewAttr("alb-ingress-access", Bool, Create),
		NewAttr("install-neuron-plugin", Bool, Create),
		NewAttr("install-nvidia-plugin", Bool, Create),
		NewAttr("cfn-role-arn", String, Create),
		NewAttr("cfn-disable-rollback", Bool, Create),
	}

	sc := map[string]*schema.Schema{
		tfsdk.KeyAssumeRole: tfsdk.SchemaAssumeRole(),
		sdk.KeyOutput: {
			Type:     schema.TypeString,
			Computed: true,
		},
	}

	for _, attr := range attrs {
		sc[attr.Key] = attr.Schema
	}

	return &schema.Resource{
		Create: func(d *schema.ResourceData, meta interface{}) (finalErr error) {
			defer func() {
				if err := recover(); err != nil {
					finalErr = fmt.Errorf("unhandled error: %v\n%s", err, debug.Stack())
				}
			}()

			ctx := mustContext(d)

			args := []string{
				"create",
				subresource,
			}

			for _, attr := range attrs {
				if Create&attr.Ops != 0 {
					args = append(args, attr.Args(d)...)
				}
			}

			id := fmt.Sprintf("%d", rand.Int())

			if err := ctx.Create(createCommand(d, args), d, id); err != nil {
				return err
			}

			d.SetId(id)

			return nil
		},
		Delete: func(d *schema.ResourceData, meta interface{}) (finalErr error) {
			defer func() {
				if err := recover(); err != nil {
					finalErr = fmt.Errorf("unhandled error: %v\n%s", err, debug.Stack())
				}
			}()

			ctx := mustContext(d)

			args := []string{
				"delete",
				subresource,
			}

			for _, attr := range attrs {
				if Delete&attr.Ops != 0 {
					args = append(args, attr.Args(d)...)
				}
			}

			return ctx.Delete(createCommand(d, args))
		},
		Read: func(d *schema.ResourceData, meta interface{}) error {
			return nil
		},
		Update: func(d *schema.ResourceData, meta interface{}) error {
			return nil
		},
		Schema: sc,
	}
}

func createCommand(d api.Getter, args []string) *exec.Cmd {
	eksctlVersion := d.Get(KeyEksctlVersion).(string)

	eksctlBin, err := sdk.PrepareExecutable("eksctl", "eksctl", eksctlVersion)
	if err != nil {
		panic(fmt.Errorf("creating eksctl command: %w", err))
	}

	return exec.Command(*eksctlBin, args...)
}
