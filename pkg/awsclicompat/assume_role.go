package awsclicompat

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"golang.org/x/xerrors"
	"math/rand"
)

type AssumeRoleConfig struct {
	RoleARN           string
	DurationSeconds   int64
	ExternalID        string
	Policy            string
	PolicyARNs        []string
	SessionName       string
	Tags              map[string]string
	TransitiveTagKeys []string
}

func AssumeRole(sess *session.Session, config AssumeRoleConfig) (*session.Session, error) {
	var awsDurationSeconds *int64

	if config.DurationSeconds != 0 {
		awsDurationSeconds = &config.DurationSeconds
	}

	stsSvc := sts.New(sess)

	sessionName := fmt.Sprintf("tf-eksctl-session-%d", rand.Int())
	if config.SessionName != "" {
		sessionName = config.SessionName
	}

	input := &sts.AssumeRoleInput{
		DurationSeconds: awsDurationSeconds,
		RoleArn:         aws.String(config.RoleARN),
		RoleSessionName: aws.String(sessionName),
	}

	if config.ExternalID != "" {
		input.ExternalId = aws.String(config.ExternalID)
	}

	if config.Policy != "" {
		input.Policy = aws.String(config.Policy)
	}

	if len(config.PolicyARNs) > 0 {
		for _, a := range config.PolicyARNs {
			input.PolicyArns = append(input.PolicyArns, &sts.PolicyDescriptorType{Arn: aws.String(a)})
		}
	}

	if len(config.Tags) > 0 {
		for k, v := range config.Tags {
			input.Tags = append(input.Tags, &sts.Tag{
				Key:   aws.String(k),
				Value: aws.String(v),
			})
		}
	}

	if len(config.TransitiveTagKeys) > 0 {
		input.TransitiveTagKeys = aws.StringSlice(config.TransitiveTagKeys)
	}

	assumedRole, err := stsSvc.AssumeRole(input)
	if err != nil {
		return nil, xerrors.Errorf("failed assuming role: %w", err)
	}

	return session.NewSession(&aws.Config{
		Credentials: credentials.NewStaticCredentials(
			*assumedRole.Credentials.AccessKeyId,
			*assumedRole.Credentials.SecretAccessKey,
			*assumedRole.Credentials.SessionToken,
		),
		Region: sess.Config.Region,
	})
}
