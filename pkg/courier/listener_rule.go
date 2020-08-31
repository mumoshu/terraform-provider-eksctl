package courier

type ListenerRule struct {
	ListenerARN  string
	Priority     int
	Hosts        []string
	PathPatterns []string
	Methods      []string
	SourceIPs    []string
	Headers      map[string][]string
	QueryStrings map[string]string
	Destinations []Destination
}
