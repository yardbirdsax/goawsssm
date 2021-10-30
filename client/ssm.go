package client

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

type SSMClient interface {
	StartSession(context.Context, *ssm.StartSessionInput, ...func(*ssm.Options)) (*ssm.StartSessionOutput, error)
}