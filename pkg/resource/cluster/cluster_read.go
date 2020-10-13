package cluster

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"log"
	"os"
)

type Read interface {
	Get(string) interface{}
}

type ReadWrite interface {
	Read

	Id() string

	Set(string, interface{}) error
}

type DiffReadWrite struct {
	D *schema.ResourceDiff
}

func (d *DiffReadWrite) Get(k string) interface{} {
	return d.D.Get(k)
}

func (d *DiffReadWrite) Set(k string, v interface{}) error {
	return d.D.SetNew(k, v)
}

func (d *DiffReadWrite) SetNewComputed(k string) error {
	return d.D.SetNewComputed(k)
}

func (d *DiffReadWrite) Id() string {
	return d.D.Id()
}

func (m *Manager) readCluster(d ReadWrite) error {
	cluster, err := m.readClusterInternal(d)

	if err != nil {
		return fmt.Errorf("reading cluster: %w", err)
	}

	var path string

	if v := d.Get(KeyKubeconfigPath); v != nil {
		path = v.(string)
	}

	// `kubeconfig_path` persistend in a Terraform remote backend might refer to an inexistent local path, meaning that
	// the file is created on another machine and the tfstate had been changed there.
	// Another resource that depends on this eksctl_cluster(_deployment)'s kubeconfig_path might use the kubeconfig while
	// in `terraform plan`, so I believe we need to "reproduce" the kubeconfig before `plan`.
	if path != "" {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			log.Printf("running customdiff: no kubeconfig file found at kubeconfig_path=%s: recreating it", path)
			if err := doWriteKubeconfig(d, string(m.getClusterName(cluster, d.Id())), cluster.Region); err != nil {
				return fmt.Errorf("writing missing kubeconfig on plan: %w", err)
			}
		}
	}

	return nil
}

func (m *Manager) readClusterInternal(d ReadWrite) (*Cluster, error) {
	clusterNamePrefix := d.Get("name").(string)

	sess := AWSSessionFromResourceData(d)

	arns, err := getTargetGroupARNs(sess, clusterNamePrefix)
	if err != nil {
		return nil, fmt.Errorf("reading cluster: %w", err)
	}

	var v []interface{}

	for _, arn := range arns {
		v = append(v, arn)
	}

	if err := d.Set(KeyTargetGroupARNs, v); err != nil {
		log.Printf("setting resource data value for key %v: %w", KeyTargetGroupARNs, err)
	}

	c, err := ReadCluster(d)
	if err != nil {
		return nil, err
	}

	return c, err
}

func (m *Manager) planCluster(d *DiffReadWrite) error {
	_, err := m.readClusterInternal(d)
	if err != nil {
		return err
	}

	if err := m.doPlanKubeconfig(d); err != nil {
		return err
	}

	return nil
}
