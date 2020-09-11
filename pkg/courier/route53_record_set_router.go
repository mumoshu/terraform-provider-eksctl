package courier

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/k-kinzal/progressived/pkg/provider"
	"log"
	"time"
)

type Route53RecordSetRouter struct {
	Service                   *route53.Route53
	HostedZoneID              string
	RecordName                string
	Destinations              []DestinationRecordSet
	CanaryAdvancementInterval time.Duration
	CanaryAdvancementStep     int
}

func (r *Route53RecordSetRouter) TrafficShift(ctx context.Context) error {
	var src, dst DestinationRecordSet

	switch len(r.Destinations) {
	case 2:
		if r.Destinations[0].Weight < r.Destinations[1].Weight {
			src = r.Destinations[0]
			dst = r.Destinations[1]
		} else if r.Destinations[0].Weight > r.Destinations[1].Weight {
			src = r.Destinations[1]
			dst = r.Destinations[0]
		} else {
			return fmt.Errorf("two destinations' weights must have different values: %v", r.Destinations)
		}
	default:
		return fmt.Errorf("unsupported number of destinations: %d", len(r.Destinations))
	}

	rp, err := provider.NewRoute53Provider(&provider.Route53Confg{
		Client:                r.Service,
		HostedZoneId:          r.HostedZoneID,
		RecordName:            r.RecordName,
		SourceIdentifier:      src.SetIdentifier,
		DestinationIdentifier: dst.SetIdentifier,
	})

	if err != nil {
		return err
	}

	// Gradually shift traffic from current tg to desired tg by
	// updating rule
	var step int

	if r.CanaryAdvancementStep > 0 {
		step = r.CanaryAdvancementStep
	} else {
		step = 5
	}

	var advancementInterval time.Duration

	if r.CanaryAdvancementInterval != 0 {
		advancementInterval = r.CanaryAdvancementInterval
	} else {
		advancementInterval = 30 * time.Second
	}

	ticker := time.NewTicker(advancementInterval)
	defer ticker.Stop()

	p := step

	for {
		select {
		case <-ticker.C:
			if p >= 100 {
				p = 100
			}

			log.Printf("Setting weight to %v", p)

			if err := rp.Update(float64(p)); err != nil {
				return err
			}

			if p == 100 {
				fmt.Printf("Done.")
				return nil
			}

			p += step
		case <-ctx.Done():
			if p != 100 {
				log.Printf("Rolling back traffic for record %s", r.RecordName)

				if err := rp.Update(0); err != nil {
					return err
				}

				break
			}

			return nil
		}
	}

	log.Printf("Rolling back traffic shift for Route 53 record %s", r.RecordName)

	return nil
}
