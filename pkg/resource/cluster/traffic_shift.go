package cluster

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/courier"
	"golang.org/x/sync/errgroup"
	"golang.org/x/xerrors"
)

func graduallyShiftTraffic(set *ClusterSet, opts courier.CanaryOpts) error {
	cluster := set.Cluster

	svc := elbv2.New(AWSSessionFromCluster(cluster))

	listenerStatuses := set.ListenerStatuses

	m := &ALBRouter{ELBV2: svc}

	{
		var err error

		m.Analyzers, err = courier.MetricsToAnalyzers(cluster.Region, cluster.Profile, cluster.AssumeRoleConfig, cluster.Metrics)
		if err != nil {
			return xerrors.Errorf("initializing analyzer: %w", err)
		}
	}

	return m.SwitchTargetGroup(listenerStatuses, opts)
}

type ALBRouter struct {
	ELBV2 elbv2iface.ELBV2API

	Analyzers []*courier.Analyzer
}

type CanaryConfig struct {
	Region      string
	ClusterName string
}

func (m *ALBRouter) SwitchTargetGroup(listenerStatuses ListenerStatuses, opts courier.CanaryOpts) error {
	svc := m.ELBV2

	if len(listenerStatuses) == 0 {
		return nil
	}

	tCtx, cancel := context.WithCancel(context.Background())
	g, gctx := errgroup.WithContext(tCtx)

	wg := &sync.WaitGroup{}

	for i := range listenerStatuses {
		l := listenerStatuses[i]

		g.Go(func() error {
			return courier.DoGradualTrafficShift(tCtx, svc, l, 1, opts)
		})
	}

	// Check per cluster metrics
	for i := range m.Analyzers {
		a := m.Analyzers[i]

		g.Go(func() error {
			ticker := time.NewTicker(courier.DefaultAnalyzeInterval)
			defer ticker.Stop()

			for {
				select {
				case <-gctx.Done():
					// Deployment finished. Stop checking as not necessary anymore
					return nil
				case <-ticker.C:
					if err := a.Analyze(opts); err != nil {
						return xerrors.Errorf("analyze: %w", err)
					}
				}
			}
		})
	}

	go func() {
		defer cancel()

		wg.Wait()
	}()

	var err error
	{
		defer cancel()

		err = g.Wait()
	}

	if err == nil {
		log.Printf("Traffic shifting finished successfully.")
	} else if err == context.Canceled {
		log.Printf("Traffic shifting canceled externally.")

		return err
	} else {
		log.Printf("Traffic shifting canceled due to error: %s", err.Error())

		return err
	}

	return nil
}
