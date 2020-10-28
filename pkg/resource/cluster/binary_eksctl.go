package cluster

import (
	"fmt"
	"github.com/mumoshu/shoal"
	"log"
	"path/filepath"
	"sync"
)

var prepareEksctlMu sync.Mutex

func prepareEksctlBinary(cluster *Cluster) (*string, error) {
	return prepareEksctlBinaryInternal(cluster.EksctlBin, cluster.EksctlVersion)
}

func prepareEksctlBinaryInternal(eksctlBin, eksctlVersion string) (*string, error) {
	log.Print("Preparing eksctl binary")

	conf := shoal.Config{
		Git: shoal.Git{
			Provider: "go-git",
		},
	}

	rig := "https://github.com/fishworks/fish-food"

	installEksctl := eksctlVersion != ""

	if installEksctl {
		log.Printf("Installing eksctl %s", eksctlVersion)

		conf.Dependencies = append(conf.Dependencies,
			shoal.Dependency{
				Rig:     rig,
				Food:    "eksctl",
				Version: eksctlVersion,
			},
		)
	}

	prepareEksctlMu.Lock()
	defer prepareEksctlMu.Unlock()

	s, err := shoal.New()
	if err != nil {
		return nil, err
	}

	if len(conf.Dependencies) > 0 {
		if err := s.Init(); err != nil {
			return nil, fmt.Errorf("initializing shoal: %w", err)
		}

		if err := s.InitGitProvider(conf); err != nil {
			return nil, fmt.Errorf("initializing shoal git provider: %w", err)
		}

		if err := s.Sync(conf); err != nil {
			return nil, err
		}
	}

	binPath := s.BinPath()

	if eksctlVersion != "" {
		eksctlBin = filepath.Join(binPath, "eksctl")
	}

	return &eksctlBin, nil
}
