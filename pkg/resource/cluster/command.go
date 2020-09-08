package cluster

import (
	"fmt"
	"os/exec"
)

func newEksctlCommandFromResource(resource Read, args ...string) (*exec.Cmd, error) {
	eksctlBin := resource.Get(KeyBin).(string)
	eksctlVersion := resource.Get(KeyEksctlVersion).(string)

	bin, err := prepareEksctlBinaryInternal(eksctlBin, eksctlVersion)
	if err != nil {
		return nil, fmt.Errorf("preparing eksctl binary: %w", err)
	}

	cmd := exec.Command(*bin, args...)

	return cmd, nil
}

func newEksctlCommand(cluster *Cluster, args ...string) (*exec.Cmd, error) {
	eksctlBin, err := prepareEksctlBinary(cluster)
	if err != nil {
		return nil, fmt.Errorf("creating eksctl command: %w", err)
	}

	cmd := exec.Command(*eksctlBin, args...)

	return cmd, nil
}
