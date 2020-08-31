package courier

import (
	"bytes"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/awsclicompat"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/courier/metrics"
	"os"
	"text/template"
	"time"
)

func MetricsToAnalyzers(region string, ms []Metric) ([]*Analyzer, error) {
	var analyzers []*Analyzer

	for _, m := range ms {
		var provider MetricProvider

		var err error

		switch m.Provider {
		case "cloudwatch":
			s := awsclicompat.NewSession(region)
			s.Config.Endpoint = aws.String(m.Address)
			c := cloudwatch.New(s)
			provider = metrics.NewCloudWatchProvider(c, metrics.ProviderOpts{
				Address:  m.Address,
				Interval: 1 * time.Minute,
			})
		case "datadog":
			provider, err = metrics.NewDatadogProvider(metrics.ProviderOpts{
				Address:  m.Address,
				Interval: 1 * time.Minute,
			}, metrics.DatadogOpts{
				APIKey:         os.Getenv("DATADOG_API_KEY"),
				ApplicationKey: os.Getenv("DATADOG_APPLICATION_KEY"),
			})
		default:
			return nil, fmt.Errorf("creating metrics provider: unknown and unsupported provider %q specified", m.Provider)
		}

		if err != nil {
			return nil, fmt.Errorf("creating metrics provider %q: %v", m.Provider, err)
		}

		analyzers = append(analyzers, &Analyzer{
			MetricProvider: provider,
			Query:          m.Query,
			Min:            m.Min,
			Max:            m.Max,
		})
	}

	return analyzers, nil
}

type MetricProvider interface {
	Execute(string) (float64, error)
}

type Analyzer struct {
	MetricProvider
	Query string
	Min   *float64
	Max   *float64
}

func (a *Analyzer) Analyze(data interface{}) error {
	maxRetries := 3

	var v float64

	var err error

	var query string

	{
		tmpl, err := template.New("query").Parse(a.Query)
		if err != nil {
			return fmt.Errorf("parsing query template: %w", err)
		}

		var buf bytes.Buffer

		if err := tmpl.Execute(&buf, data); err != nil {
			return fmt.Errorf("executing query template: %w", err)
		}

		query = buf.String()
	}

	for i := 0; i < maxRetries; i++ {
		v, err = a.MetricProvider.Execute(query)
		if err == nil {
			break
		}
	}

	if err != nil {
		return err
	}

	if a.Min != nil && *a.Min > v {
		return fmt.Errorf("checking value against threshold: %v is below %v", v, *a.Min)
	}

	if a.Max != nil && *a.Max < v {
		return fmt.Errorf("checking value against threshold: %v is beyond %v", v, *a.Max)
	}

	return nil
}
