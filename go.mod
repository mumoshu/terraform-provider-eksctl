module github.com/mumoshu/terraform-provider-eksctl

go 1.13

require (
	github.com/armon/circbuf v0.0.0-20190214190532-5111143e8da2
	github.com/aws/aws-sdk-go v1.34.16
	github.com/google/go-cmp v0.4.0
	github.com/hashicorp/terraform-plugin-sdk v1.0.0
	github.com/k-kinzal/progressived v0.0.0-20200909013205-9522de740306
	github.com/mitchellh/go-linereader v0.0.0-20190213213312-1b945b3263eb
	github.com/mumoshu/shoal v0.2.10
	github.com/rs/xid v1.2.1
	github.com/stretchr/testify v1.5.1
	golang.org/x/sync v0.0.0-20190423024810-112230192c58
	gopkg.in/yaml.v3 v3.0.0-20200506231410-2ff61e1afc86
)

replace github.com/fishworks/gofish => github.com/mumoshu/gofish v0.13.1-0.20200908033248-ab2d494fb15c

replace git.apache.org/thrift.git => github.com/apache/thrift v0.0.0-20180902110319-2566ecd5d999
