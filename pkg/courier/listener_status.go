package courier

import (
	"github.com/aws/aws-sdk-go/service/elbv2"
)

type ListenerStatus struct {
	Listener       *elbv2.Listener
	Rule           *elbv2.Rule
	ALBAttachments []ALBAttachment

	DesiredTG  *elbv2.TargetGroup
	CurrentTG  *elbv2.TargetGroup
	DeletedTGs *elbv2.TargetGroup

	// Common listener rule settings
	RulePriority int64
	Hosts        []string
	PathPatterns []string
	Methods      []string
	SourceIPs    []string
	Headers      map[string][]string
	QueryStrings map[string]string
	Metrics      []Metric
}

func ListerStatusToTemplateData(l ListenerStatus) interface{} {
	targetGroupARN := *l.DesiredTG.TargetGroupArn
	var loadBalancerARNs []string

	for _, a := range l.DesiredTG.LoadBalancerArns {
		loadBalancerARNs = append(loadBalancerARNs, *a)
	}

	data := struct {
		TargetGroupARN   string
		LoadBalancerARNs []string
	}{
		TargetGroupARN:   targetGroupARN,
		LoadBalancerARNs: loadBalancerARNs,
	}

	return data
}
