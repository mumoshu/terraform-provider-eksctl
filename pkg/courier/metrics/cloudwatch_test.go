package metrics

import (
	"errors"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

type cloudWatchClientMock struct {
	o   *cloudwatch.GetMetricDataOutput
	err error
}

func (c cloudWatchClientMock) GetMetricData(_ *cloudwatch.GetMetricDataInput) (*cloudwatch.GetMetricDataOutput, error) {
	return c.o, c.err
}

func TestCloudWatchProvider_RunQuery(t *testing.T) {
	// ref: https://aws.amazon.com/premiumsupport/knowledge-center/cloudwatch-getmetricdata-api/
	query := `
[
    {
        "Id": "e1",
        "Expression": "m1 / m2",
        "Label": "ErrorRate"
    },
    {
        "Id": "m1",
        "MetricStat": {
            "Metric": {
                "Namespace": "MyApplication",
                "MetricName": "Errors",
                "Dimensions": [
                    {
                        "Name": "FunctionName",
                        "Value": "MyFunc"
                    }
                ]
            },
            "Period": 300,
            "Stat": "Sum",
            "Unit": "Count"
        },
        "ReturnData": false
    },
    {
        "Id": "m2",
        "MetricStat": {
            "Metric": {
                "Namespace": "MyApplication",
                "MetricName": "Invocations",
                "Dimensions": [
                    {
                        "Name": "FunctionName",
                        "Value": "MyFunc"
                    }
                ]
            },
            "Period": 300,
            "Stat": "Sum",
            "Unit": "Count"
        },
        "ReturnData": false
    }
]`

	t.Run("ok", func(t *testing.T) {
		var exp float64 = 100
		p := CloudWatch{client: cloudWatchClientMock{
			o: &cloudwatch.GetMetricDataOutput{
				MetricDataResults: []*cloudwatch.MetricDataResult{
					{Values: []*float64{aws.Float64(exp)}},
				},
			},
		}}

		actual, err := p.Execute(query)
		assert.NoError(t, err)
		assert.Equal(t, exp, actual)
	})

	t.Run("no values", func(t *testing.T) {
		p := CloudWatch{client: cloudWatchClientMock{
			o: &cloudwatch.GetMetricDataOutput{
				MetricDataResults: []*cloudwatch.MetricDataResult{
					{Values: []*float64{}},
				},
			},
		}}

		_, err := p.Execute(query)
		require.Error(t, err)
		require.True(t, errors.Is(err, ErrNoValuesFound))

		p = CloudWatch{client: cloudWatchClientMock{
			o: &cloudwatch.GetMetricDataOutput{}}}

		_, err = p.Execute(query)
		require.Error(t, err)
		require.True(t, errors.Is(err, ErrNoValuesFound))
	})
}
