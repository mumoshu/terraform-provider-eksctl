module github.com/mumoshu/terraform-provider-eksctl

go 1.16

require (
	github.com/armon/circbuf v0.0.0-20190214190532-5111143e8da2
	github.com/aws/aws-sdk-go v1.38.35
	github.com/google/go-cmp v0.5.5
	github.com/hashicorp/terraform-plugin-sdk v1.0.0
	github.com/k-kinzal/progressived v0.0.0-20200911065552-afe494a1cc18
	github.com/mitchellh/go-linereader v0.0.0-20190213213312-1b945b3263eb
	github.com/mumoshu/shoal v0.2.18
	github.com/rs/xid v1.2.1
	github.com/stretchr/testify v1.5.1
	golang.org/x/net v0.0.0-20210405180319-a5a99cb37ef4 // indirect
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20210403161142-5e06dd20ab57 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1
	gopkg.in/yaml.v3 v3.0.0-20200506231410-2ff61e1afc86
)

replace github.com/fishworks/gofish => github.com/mumoshu/gofish v0.13.1-0.20200908033248-ab2d494fb15c

replace git.apache.org/thrift.git => github.com/apache/thrift v0.0.0-20180902110319-2566ecd5d999
