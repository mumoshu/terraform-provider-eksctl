package cluster

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"log"
)

func createVPCResourceTags(cluster *Cluster, clusterName ClusterName) error {
	if cluster.VPCID == "" {
		log.Printf("Skipped VPC resource tagging due to that VPC and subnets management are handled to eksctl")

		return nil
	}

	ec2session := ec2.New(AWSSessionFromCluster(cluster))

	tagKey := fmt.Sprintf("kubernetes.io/cluster/%s", clusterName)
	tagValue := "shared"

	resources := getVpcResources(cluster)

	if _, err := ec2session.CreateTags(&ec2.CreateTagsInput{
		Resources: resources,
		Tags: []*ec2.Tag{
			{
				Key:   aws.String(tagKey),
				Value: aws.String(tagValue),
			},
		},
	}); err != nil {
		return err
	}

	return nil
}

func getVpcResources(cluster *Cluster) []*string {
	var resources []*string

	for _, subnetID := range cluster.PublicSubnetIDs {
		resources = append(resources, aws.String(subnetID))
	}

	for _, subnetID := range cluster.PrivateSubnetIDs {
		resources = append(resources, aws.String(subnetID))
	}

	if cluster.VPCID == "" {
		log.Printf("vpc id isnt configured via vpc_id attribute: tagging of ekslct-managed vpc is not supported yet")
	} else {
		resources = append(resources, aws.String(cluster.VPCID))
	}
	return resources
}

func deleteVPCResourceTags(cluster *Cluster, clusterName ClusterName) error {
	if cluster.VPCID == "" {
		log.Printf("Skipped VPC resource de-tagging due to that VPC and subnets management are handled to eksctl")

		return nil
	}

	ec2session := ec2.New(AWSSessionFromCluster(cluster))

	tagKey := fmt.Sprintf("kubernetes.io/cluster/%s", clusterName)

	resources := getVpcResources(cluster)

	if _, err := ec2session.DeleteTags(&ec2.DeleteTagsInput{
		Resources: resources,
		Tags: []*ec2.Tag{
			{
				Key: aws.String(tagKey),
			},
		},
	}); err != nil {
		return err
	}

	return nil
}
