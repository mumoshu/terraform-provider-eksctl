package courier

import (
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk/api"
)

func DeleteCourierALB(d api.Getter) error {
	conf, err := ReadCourierALB(d)
	if err != nil {
		return err
	}

	alb := &ALB{}

	return alb.Delete(conf)
}

func CreateOrUpdateCourierALB(d api.Getter) error {
	conf, err := ReadCourierALB(d)
	if err != nil {
		return err
	}

	alb := &ALB{}

	return alb.Apply(conf)
}
