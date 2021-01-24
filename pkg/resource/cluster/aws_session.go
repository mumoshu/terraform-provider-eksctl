package cluster

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk"
)

func AWSSessionFromCluster(cluster *Cluster) *session.Session {
	sess, _ := sdk.AWSCredsFromConfig(cluster.Region, cluster.Profile, cluster.AssumeRoleConfig)

	return sess
}

