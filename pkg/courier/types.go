package courier

import "time"

var DefaultAnalyzeInterval = 10 * time.Second

type Destination struct {
	TargetGroupARN string
	Weight         int
}

type DestinationRecordSet struct {
	SetIdentifier string
	Weight        int
}

type ALBAttachment struct {
	NodeGroupName string
	Weght         int
	ListenerARN   string

	// TargetGroup settings

	NodePort int
	Protocol string

	// ALB Listener Rule settings
	Priority     int
	Hosts        []string
	PathPatterns []string
	Methods      []string
	SourceIPs    []string
	Headers      map[string][]string
	QueryStrings map[string]string
	Metrics      []Metric
}
