package cluster

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/awsclicompat"
)

func GetAWSRegionAndProfile(d Read) (string, string) {
	var region string

	if v := d.Get("region"); v != nil {
		region = v.(string)
	}

	var profile string

	if v := d.Get("profile"); v != nil {
		profile = v.(string)
	}

	return region, profile
}

func AWSSessionFromResourceData(d Read) *session.Session {
	region, profile := GetAWSRegionAndProfile(d)

	return awsclicompat.NewSession(region, profile)
}

func AWSSessionFromCluster(cluster *Cluster) *session.Session {
	return awsclicompat.NewSession(cluster.Region, cluster.Profile)
}

