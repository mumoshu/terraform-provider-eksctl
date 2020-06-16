package cluster

import (
	"fmt"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
)

type mockedAWS struct {
	elbv2iface.ELBV2API

	CreateRuleFunc func(*elbv2.CreateRuleInput) (*elbv2.CreateRuleOutput, error)
	ModifyRuleFunc func(*elbv2.ModifyRuleInput) (*elbv2.ModifyRuleOutput, error)
}

func (m mockedAWS) CreateRule(i *elbv2.CreateRuleInput) (*elbv2.CreateRuleOutput, error) {
	if m.CreateRuleFunc == nil {
		return nil, fmt.Errorf("creating rule: unexpected call")
	}
	return m.CreateRuleFunc(i)
}

func (m mockedAWS) ModifyRule(i *elbv2.ModifyRuleInput) (*elbv2.ModifyRuleOutput, error) {
	if m.ModifyRuleFunc == nil {
		return nil, fmt.Errorf("modifying rule: unexpected call")
	}

	return m.ModifyRuleFunc(i)
}
