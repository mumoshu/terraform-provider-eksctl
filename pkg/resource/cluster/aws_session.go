package cluster

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/awsclicompat"
)

func AWSSessionFromCluster(cluster *Cluster) *session.Session {
	sess := awsclicompat.NewSession(cluster.Region, cluster.Profile)

	if cluster.AssumeRoleConfig == nil {
		return sess
	}

	newSess, _, err := awsclicompat.AssumeRole(sess, *cluster.AssumeRoleConfig)
	if err != nil {
		panic(err)
	}

	return newSess
}

func AWSCredsFromConfig(region, profile string, assumeRole *awsclicompat.AssumeRoleConfig) (*session.Session, *sts.Credentials) {
	sess := awsclicompat.NewSession(region, profile)

	if assumeRole == nil {
		return sess, nil
	}

	assumed, creds, err := awsclicompat.AssumeRole(sess, *assumeRole)
	if err != nil {
		panic(err)
	}

	return assumed, creds
}
