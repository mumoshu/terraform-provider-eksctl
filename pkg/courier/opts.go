package courier

import "time"

type CanaryOpts struct {
	CanaryAdvancementInterval time.Duration
	CanaryAdvancementStep     int
	Region                    string
	ClusterName               string
}
