package cluster

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/resource"
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
	if err := readIAMIdentityMapping(d, cluster); err != nil {
		return fmt.Errorf("reading aws-auth via eksctl get iamidentitymaping: %w", err)
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

func readIAMIdentityMapping(d ReadWrite, cluster *Cluster) error {
	iams, err := runGetIAMIdentityMapping(cluster)

	if err != nil {
		return fmt.Errorf("can not get iamidentitymapping from eks cluster: %w", err)
	}

	current := make([]map[string]interface{}, 0)

	for _, v := range d.Get(KeyAWSAuthConfigMap).(*schema.Set).List() {
		current = append(current, v.(map[string]interface{}))
	}

	// sort for diff
	sort.Slice(current, func(i, j int) bool { return current[i]["iamarn"].(string) < current[j]["iamarn"].(string) })
	sort.Slice(iams, func(i, j int) bool { return iams[i]["iamarn"].(string) < iams[j]["iamarn"].(string) })

	if diff := cmp.Diff(iams, current); diff != "" {
		log.Printf("aws-auth diff remote (-remote +current):\n%s", diff)
	} else {
		log.Printf("have diff between remote source and param")
	}

	return nil
}

func runGetIAMIdentityMapping(cluster *Cluster) ([]map[string]interface{}, error) {

	//get iamidentitymapping
	args := []string{
		"get",
		"iamidentitymapping",
		"--cluster",
		cluster.Name,
		"-o",
		"json",
	}
	cmd, err := newEksctlCommandWithAWSProfile(cluster, args...)

	if err != nil {
		return nil, fmt.Errorf("creating get imaidentitymapping command: %w", err)
	}
	iamJson, err := resource.Run(cmd)

	//replace rolearn and userarn to iamarn
	iamJson1 := strings.Replace(iamJson.Output, "rolearn", "iamarn", -1)
	iamJson2 := strings.Replace(iamJson1, "userarn", "iamarn", -1)

	if err != nil {
		return nil, fmt.Errorf("running get iamidentitymapping : %w", err)
	}

	var iams []map[string]interface{}
	if err := json.Unmarshal([]byte(iamJson2), &iams); err != nil {
		return nil, fmt.Errorf("parse iamidentitymapping : %w", err)
	}

	return iams, nil
}

func sortSlice() {

}
