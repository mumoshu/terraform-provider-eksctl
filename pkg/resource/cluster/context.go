package cluster

import (
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk"
)

func mustNewContext(cluster *Cluster) *sdk.Context {
	sess, creds := AWSCredsFromConfig(cluster.Region, cluster.Profile, cluster.AssumeRoleConfig)

	return &sdk.Context{Sess: sess, Creds: creds}
}
