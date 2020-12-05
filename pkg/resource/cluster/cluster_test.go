package cluster

import (
	"github.com/google/go-cmp/cmp"
	"gopkg.in/yaml.v3"
	"testing"
)

func TestConfig_MarshalUnmarshal(t *testing.T) {
	input := `vpc:
  id: ""
  subnets:
    public: {}
    private: {}
nodeGroups: []
iam:
  withOIDC: true
  serviceAccounts:
    - name: foo
apiVersion: example.com/v1alpha1
kind: ClusterConfig
metadata:
  name: mycluster
  region: us-east-2
  tags:
    foo: bar
  version: "1.18"
`

	c := clusterConfigNew()

	if err := yaml.Unmarshal([]byte(input), &c); err != nil {
		t.Fatalf("%v", err)
	}

	output, err := clusterConfigToYAML(c)
	if err != nil {
		t.Fatalf("%v", err)
	}

	if d := cmp.Diff(string(input), string(output)); d != "" {
		t.Errorf("unexpected diff: want (-), got (+)\n%s", d)
	}
}
