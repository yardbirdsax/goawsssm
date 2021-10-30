package session

import (
	"context"
	"os/exec"
	"time"

	//"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"gopkg.in/square/go-jose.v2/json"

	//"github.com/mitchellh/iochan"

	//"github.com/yardbirdsax/goawsssm/logging"
	"github.com/yardbirdsax/goawsssm/client"
	"github.com/yardbirdsax/goawsssm/logging"
)

type StartSessionInput struct {
	// The AWS Instance ID that the connection should be made to.
	InstanceID string
	// The maximum number of retries to open the tunnel.
	MaxRetries int
	// The wait time between retries.
	RetryWaitInterval time.Duration
	// The SSM Document Name to pass to the AWS API
	DocumentName string
	// The parameters to pass to the start session call
	Parameters map[string][]string
}

// Start begins a new SSM session with the instance and region. If connectivity cannot be established 
// after the designated number of retries, the most recent error message captured is returned.
func Start(ctx context.Context, client client.SSMClient, input StartSessionInput) (output *ssm.StartSessionOutput, err error) {
	if input.MaxRetries == 0 {
		input.MaxRetries = 1
	}
	logger := logging.GetLogger(ctx)
	startSessionInput := &ssm.StartSessionInput{
		Target: &input.InstanceID,
		DocumentName: &input.DocumentName,
		Parameters: input.Parameters,
	}
	for currentRetryCount := 0; currentRetryCount < input.MaxRetries; currentRetryCount++ {
		logging.Infof(logger, "Attempting to start SSM session, attempt %d of %d", currentRetryCount + 1, input.MaxRetries)
		output, err = client.StartSession(ctx, startSessionInput)
		if err != nil {
			logging.Errorf(logger, "Error attempting to start SSM session: %v; attempt %d of %d.", err, currentRetryCount, input.MaxRetries)
			time.Sleep(input.RetryWaitInterval)
		} else {
			break
		}
	}
	return
}

type GetPluginCommandInput struct {
	StartSessionOuput *ssm.StartSessionOutput

	RegionName string
	
	AWSProfileName string

	StartSessionInput *ssm.StartSessionInput
}

func GetPluginCommand(ctx context.Context, executor client.Exec, input GetPluginCommandInput) (cmd *exec.Cmd, err error) {
	logger := logging.GetLogger(ctx)
	logging.Infof(logger, "Hello there")

	sessionOutputData, err := json.Marshal(input.StartSessionOuput)
	if err != nil {
		return
	}
	sessionInputData, err := json.Marshal(input.StartSessionInput)
	if err != nil {
		return
	}

	args := []string{
		string(sessionOutputData),
		input.RegionName,
		"StartSession",
		input.AWSProfileName,
		string(sessionInputData),
		*input.StartSessionOuput.StreamUrl,
	}
	
	cmd = executor.Command("session-manager-plugin", args)

	return
}