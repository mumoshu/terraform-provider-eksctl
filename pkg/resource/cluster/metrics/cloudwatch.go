package metrics

import (
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/service/cloudwatch/cloudwatchiface"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
)

const (
	cloudWatchStartDeltaMultiplierOnMetricInterval = 10
)

type CloudWatch struct {
	client     cloudWatchClient
	startDelta time.Duration
}

// for the testing purpose
type cloudWatchClient interface {
	GetMetricData(input *cloudwatch.GetMetricDataInput) (*cloudwatch.GetMetricDataOutput, error)
}

type ProviderOpts struct {
	Address  string
	Interval time.Duration
}

func NewCloudWatchProvider(client cloudwatchiface.CloudWatchAPI, provider ProviderOpts) *CloudWatch {
	return &CloudWatch{
		client:     client,
		startDelta: cloudWatchStartDeltaMultiplierOnMetricInterval * provider.Interval,
	}
}

func (p *CloudWatch) Execute(query string) (float64, error) {
	var cq []*cloudwatch.MetricDataQuery
	if err := json.Unmarshal([]byte(query), &cq); err != nil {
		return 0, fmt.Errorf("error unmarshaling query: %s", err.Error())
	}

	end := time.Now()
	start := end.Add(-p.startDelta)
	res, err := p.client.GetMetricData(&cloudwatch.GetMetricDataInput{
		EndTime:           aws.Time(end),
		MaxDatapoints:     aws.Int64(20),
		StartTime:         aws.Time(start),
		MetricDataQueries: cq,
	})

	if err != nil {
		return 0, fmt.Errorf("error requesting cloudwatch: %s", err.Error())
	}

	mr := res.MetricDataResults
	if len(mr) < 1 {
		return 0, fmt.Errorf("invalid response: %s: %w", res.String(), ErrNoValuesFound)
	}

	vs := mr[0].Values
	if len(vs) < 1 {
		return 0, fmt.Errorf("invalid reponse %s: %w", res.String(), ErrNoValuesFound)
	}

	return aws.Float64Value(vs[0]), nil
}
