package cluster

import (
	"fmt"
	"os/exec"
)

func newEksctlCommand(cluster *Cluster, args ...string) (*exec.Cmd, error) {
	eksctlBin, err := prepareBinaries(cluster)
	if err != nil {
		return nil, fmt.Errorf("creating eksctl command: %w", err)
	}

	cmd := exec.Command(*eksctlBin, args...)

	return cmd, nil
}

