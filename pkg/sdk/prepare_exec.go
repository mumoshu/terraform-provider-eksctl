package sdk

import (
	"bufio"
	"fmt"
	"github.com/mumoshu/shoal"
	"golang.org/x/sync/errgroup"
	"golang.org/x/xerrors"
	"io"
	"log"
	"path/filepath"
	"sync"
)

var prepareExecMu sync.Mutex

func PrepareExecutable(defaultPath, pkgAndCmdName, pkgVersion string) (*string, error) {
	log.Printf("Preparing %s binary", pkgAndCmdName)

	conf := shoal.Config{
		Git: shoal.Git{
			Provider: "go-git",
		},
	}

	rig := "https://github.com/fishworks/fish-food"

	installEksctl := pkgVersion != ""

	if installEksctl {
		log.Printf("Installing %s %s", pkgAndCmdName, pkgVersion)

		conf.Dependencies = append(conf.Dependencies,
			shoal.Dependency{
				Rig:     rig,
				Food:    pkgAndCmdName,
				Version: pkgVersion,
			},
		)
	}

	log.Print("Started taking exclusive lock on shoal")

	prepareExecMu.Lock()
	defer prepareExecMu.Unlock()

	log.Print("Took exclusive lock on shoal")

	logReader, logWriter := io.Pipe()

	s, err := shoal.New(shoal.LogOutput(logWriter))
	if err != nil {
		return nil, err
	}

	log.Print("Shoal instance created")

	if len(conf.Dependencies) > 0 {
		eg := errgroup.Group{}

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
				return xerrors.Errorf("running shoal-sync: %w", err)
			}

			return nil
		})

		if err := eg.Wait(); err != nil {
			return nil, xerrors.Errorf("calling shoal: %w", err)
		}

		log.Println("Shoal sync finished")
	}

	binPath := s.BinPath()

	if pkgVersion != "" {
		defaultPath = filepath.Join(binPath, pkgAndCmdName)
	}

	return &defaultPath, nil
}
