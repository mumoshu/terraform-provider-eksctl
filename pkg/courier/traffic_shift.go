package courier

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
	"golang.org/x/sync/errgroup"
	"log"
	"time"
)

func DoGradualTrafficShift(ctx context.Context, svc elbv2iface.ELBV2API, l ListenerStatus, opts CanaryOpts) error {
	ctx2, finish := context.WithCancel(ctx)
	g, gctx := errgroup.WithContext(ctx2)

	region := opts.Region

	var analyzers []*Analyzer
	{
		var err error

		analyzers, err = MetricsToAnalyzers(region, l.Metrics)
		if err != nil {
			return err
		}
	}

	if l.Rule.Actions != nil && len(l.Rule.Actions) > 0 {
		if len(l.Rule.Actions) != 1 {
			return fmt.Errorf("unexpected number of actions in rule %q: want 2, got %d", *l.Rule.RuleArn, len(l.Rule.Actions))
		}

		// Gradually shift traffic from current tg to desired tg by
		// updating rule
		var step int

		if opts.CanaryAdvancementStep > 0 {
			step = opts.CanaryAdvancementStep
		} else {
			step = 5
		}

		var advancementInterval time.Duration

		if opts.CanaryAdvancementInterval != 0 {
			advancementInterval = opts.CanaryAdvancementInterval
		} else {
			advancementInterval = 30 * time.Second
		}

		g.Go(func() error {
			ticker := time.NewTicker(advancementInterval)
			defer ticker.Stop()
			defer finish()

			p := 1

			for {
				select {
				case <-ticker.C:
					if p >= 100 {
						fmt.Printf("Done.")
						p = 100

						if err := SetDesiredTGTrafficPercentage(svc, l, 100); err != nil {
							return err
						}
						return nil
					}

					if err := SetDesiredTGTrafficPercentage(svc, l, p); err != nil {
						return err
					}

					p += step
				case <-gctx.Done():
					if p != 100 {
						log.Printf("Rolling back traffic for listener %s", *l.Listener.ListenerArn)

						if err := SetDesiredTGTrafficPercentage(svc, l, 0); err != nil {
							return err
						}

						break
					}

					// Shouldn't this be `return nil`?
					return gctx.Err()
				}
			}

			log.Printf("Rolling back traffic shift for rule on listener %s", *l.Listener.ListenerArn)

			return nil
		})

		// Check per alb, per target group metrics
		for i := range analyzers {
			a := analyzers[i]

			// TODO Check Datadog metrics and return non-nil error on check failure to cancel all the traffic shift
			g.Go(func() error {
				ticker := time.NewTicker(DefaultAnalyzeInterval)
				defer ticker.Stop()

				for {
					select {
					case <-gctx.Done():
						// Deployment finished. Stop checking as not necessary anymore
						return nil
					case <-ticker.C:
						if err := a.Analyze(ListerStatusToTemplateData(l)); err != nil {
							return err
						}
					}
				}
			})
		}
	}

	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}
