package cluster

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/awsclicompat"
)

func AWSSessionFromCluster(cluster *Cluster) *session.Session {
	sess := awsclicompat.NewSession(cluster.Region, cluster.Profile)

	if cluster.AssumeRoleConfig == nil {
		return sess
	}

	newSess, err := awsclicompat.AssumeRole(sess, *cluster.AssumeRoleConfig)
	if err != nil {
		panic(err)
	}

	return newSess
}
