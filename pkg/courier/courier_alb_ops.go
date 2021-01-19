package courier

import (
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk/api"
	"golang.org/x/xerrors"
)

func DeleteCourierALB(d api.Lister, schema *ALBSchema, metricSchema *MetricSchema) error {
	conf, err := ReadCourierALB(d, schema, metricSchema)
	if err != nil {
		return xerrors.Errorf("reading courier ALB for deletion: %w", err)
	}

	alb := &ALB{}

	return alb.Delete(conf)
}

func CreateOrUpdateCourierALB(d api.Lister, schema *ALBSchema, metricSchema *MetricSchema) error {
	conf, err := ReadCourierALB(d, schema, metricSchema)
	if err != nil {
		return xerrors.Errorf("reading courier ALB for create/update: %w", err)
	}

	alb := &ALB{}

	return alb.Apply(conf)
}
