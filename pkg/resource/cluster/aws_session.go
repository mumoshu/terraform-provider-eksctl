package cluster

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/awsclicompat"
)

func AWSSessionFromCluster(cluster *Cluster) *session.Session {
	return awsclicompat.NewSession(cluster.Region, cluster.Profile)
}
