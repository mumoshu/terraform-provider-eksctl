package cluster

import (
	"fmt"
)

type ClusterName string

func getClusterName(cluster *Cluster, id string) ClusterName {
	return ClusterName(fmt.Sprintf("%s-%s", cluster.Name, id))
}
