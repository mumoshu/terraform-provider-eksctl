package provider

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/courier"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/resource"
	"golang.org/x/sync/errgroup"
	"time"
)

func createOrUpdateCourierRoute53Record(d *schema.ResourceData) error {
	ctx := context.Background()

	sess := resource.AWSSessionFromResourceData(d)

	if v := d.Get("address"); v != nil {
		sess.Config.Endpoint = aws.String(v.(string))
	}

	svc := route53.New(sess)

	zoneID := d.Get("zone_id").(string)

	_, err := svc.GetHostedZone(&route53.GetHostedZoneInput{Id: aws.String(zoneID)})
	if err != nil {
		return err
	}

	region, profile := resource.GetAWSRegionAndProfile(d)

	recordName := d.Get("name").(string)

	metrics, err := readMetrics(d)
	if err != nil {
		return err
	}

	var destinations []courier.DestinationRecordSet

	if v := d.Get("destination"); v != nil {
		for _, arrayItem := range v.([]interface{}) {
			m := arrayItem.(map[string]interface{})
			setIdentifier := m["set_identifier"].(string)
			weight := m["weight"].(int)

			d := courier.DestinationRecordSet{
				SetIdentifier: setIdentifier,
				Weight:        weight,
			}

			destinations = append(destinations, d)
		}
	}

	stepInterval := 1 * time.Second
	if v := d.Get("step_interval"); v != nil {
		d, err := time.ParseDuration(v.(string))
		if err != nil {
			return fmt.Errorf("error parsing step_interval %v: %w", v, err)
		}

		stepInterval = d
	}

	stepWeight := 50
	if v := d.Get("step_weight"); v != nil {
		stepWeight = v.(int)
	}

	r := &courier.Route53RecordSetRouter{
		Service:                   svc,
		RecordName:                recordName,
		HostedZoneID:              zoneID,
		Destinations:              destinations,
		CanaryAdvancementInterval: stepInterval,
		CanaryAdvancementStep:     stepWeight,
	}

	ctx, cancel := context.WithCancel(ctx)
	e, errctx := errgroup.WithContext(ctx)

	e.Go(func() error {
		defer cancel()
		return r.TrafficShift(errctx)
	})

	type templateData struct {
	}

	e.Go(func() error {
		return courier.Analyze(errctx, region, profile, metrics, &templateData{})
	})

	return e.Wait()
}
