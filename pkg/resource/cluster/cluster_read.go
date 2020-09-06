package cluster

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"log"
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
	_, err:= m.readClusterInternal(d)
	return err
}

func (m *Manager) readClusterInternal(d ReadWrite) (*Cluster, error) {
	clusterNamePrefix := d.Get("name").(string)
	region := d.Get("region").(string)

	arns, err := getTargetGroupARNs(region, clusterNamePrefix)
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
		return err
	}

	clusterName := m.getClusterName(c, d.Id())

	if err := doWriteKubeconfig(d, string(clusterName), c.Region); err != nil {
		return err
	}

	return nil
}
