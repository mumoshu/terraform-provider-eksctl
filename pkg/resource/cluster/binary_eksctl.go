package cluster

import (
	"bufio"
	"fmt"
	"github.com/mumoshu/shoal"
	"golang.org/x/sync/errgroup"
	"io"
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

	log.Print("Started taking exclusive lock on shoal")

	prepareEksctlMu.Lock()
	defer prepareEksctlMu.Unlock()

	log.Print("Took exclusive lock on shoal")

	logReader, logWriter := io.Pipe()

	s, err := shoal.New(shoal.LogOutput(logWriter))
	if err != nil {
		return nil, err
	}

	log.Print("Shoal instance created")

	if len(conf.Dependencies) > 0 {
		eg  := errgroup.Group{}

		scanner := bufio.NewScanner(logReader)

		eg.Go(func() error {
			for scanner.Scan() {
				log.Printf("shoal] %s", scanner.Text())
			}

			return nil
		})

		if err := s.Init(); err != nil {
			return nil, fmt.Errorf("initializing shoal: %w", err)
		}

		log.Print("Shoal initialized")

		if err := s.InitGitProvider(conf); err != nil {
			return nil, fmt.Errorf("initializing shoal git provider: %w", err)
		}

		log.Print("Shoal's Git provider initialized")

		eg.Go(func() error {
			defer logWriter.Close()

			if err := s.Sync(conf); err != nil {
				return err
			}

			return nil
		})

		if err := eg.Wait(); err != nil {
			return nil, err
		}

		log.Println("Shoal sync finished")
	}

	binPath := s.BinPath()

	if eksctlVersion != "" {
		eksctlBin = filepath.Join(binPath, "eksctl")
	}

	return &eksctlBin, nil
}
