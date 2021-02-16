package sdk

import (
	"os"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

type Context struct {
	Creds *sts.Credentials
	Sess  *session.Session
}

func (e *Context) Run(cmd *exec.Cmd) (*CommandResult, error) {
	e.setEnv(cmd)

	return Run(cmd)
}

func (e *Context) setEnv(cmd *exec.Cmd) {
	var env []string

	if len(cmd.Env) == 0 {
		for _, kv := range os.Environ() {
			if e.Creds == nil || (!strings.HasPrefix(kv, "KUBECONFIG=") &&
				!strings.HasPrefix(kv, "AWS_SESSION_TOKEN=") &&
				!strings.HasPrefix(kv, "AWS_SECRET_ACCESS_KEY=") &&
				!strings.HasPrefix(kv, "AWS_ACCESS_KEY_ID=")) {

				env = append(env, kv)
			}
		}
	} else {
		env = cmd.Env
	}

	if e.Creds != nil {
		env = append(env, "AWS_SESSION_TOKEN="+*e.Creds.SessionToken)
		env = append(env, "AWS_SECRET_ACCESS_KEY="+*e.Creds.SecretAccessKey)
		env = append(env, "AWS_ACCESS_KEY_ID="+*e.Creds.AccessKeyId)
	}

	cmd.Env = env
}

func (e *Context) Update(cmd *exec.Cmd, d *schema.ResourceData) error {
	st, err := e.Run(cmd)
	if err != nil {
		return err
	}

	SetOutput(d, st.Output)

	return nil
}

func (e *Context) Delete(cmd *exec.Cmd) error {
	_, err := e.Run(cmd)
	if err != nil {
		return err
	}

	return nil
}

func (e *Context) Create(cmd *exec.Cmd, d *schema.ResourceData, id string) error {
	d.MarkNewResource()

	st, err := e.Run(cmd)
	if err != nil {
		return err
	}

	SetOutput(d, st.Output)

	return nil
}

func (e *Context) Session() *session.Session {
	return e.Sess
}
