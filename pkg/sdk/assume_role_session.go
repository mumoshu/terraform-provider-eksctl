package sdk

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
)

func AWSCredsFromConfig(region, profile string, assumeRole *AssumeRoleConfig) (*session.Session, *sts.Credentials) {
	sess := NewSession(region, profile)

	if assumeRole == nil {
		return sess, nil
	}

	assumed, creds, err := AssumeRole(sess, *assumeRole)
	if err != nil {
		panic(err)
	}

	return assumed, creds
}
