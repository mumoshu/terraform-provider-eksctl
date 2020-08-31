package courier

import "time"

type Metric struct {
	Provider string
	Address  string
	Query    string
	Max      *float64
	Min      *float64
	Interval time.Duration
}
