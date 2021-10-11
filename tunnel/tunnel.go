package tunnel

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	awsssm "github.com/aws/aws-sdk-go-v2/service/ssm"
	"gopkg.in/square/go-jose.v2/json"

	"github.com/mitchellh/iochan"

	"github.com/yardbirdsax/goawsssm/logging"
	"github.com/yardbirdsax/goawsssm/session"
)

type CreateSSMTunnelInput struct {
	// A channel to signal the caller that the tunnel has been opened.
	TunnelIsOpen chan bool
	// A channel to receive a signal that the tunnel can be closed.
	TunnelCanClose chan bool
	// The AWS Instance ID that the connection should be made to.
	InstanceID string
	// The remote port number for the tunnel.
	RemotePortNumber int
	// The local port number for the tunnel.
	LocalPortNumber int
	// The AWS region where the instance resides.
	RegionName string
	// The maximum number of retries to open the tunnel.
	MaxRetries int
	// The wait time between retries.
	RetryWaitInterval time.Duration
}

// CreateSSMTunnelE is used to create an SSM based port-forwarding tunnel to an AWS EC2 instance. It will log various tidbits if the context input includes a key call "logger" 
// that matches the logging.Logger interface.
func CreateSSMTunnelE(ctx context.Context, input CreateSSMTunnelInput) (string, error) {

	if input.MaxRetries == 0 {
		input.MaxRetries = 1
	}
	
	logger := ctx.Value(logging.LOGGER_CONTEXT_KEY).(logging.Logger)
	
	logging.Infof(logger, "Loading AWS config")

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return "", err
	}
	cfg.Region = input.RegionName

	documentName := "AWS-StartPortForwardingSession"
	remotePortNumberStr := fmt.Sprintf("%v", input.RemotePortNumber)
	localPortNumberStr := fmt.Sprintf("%v", input.LocalPortNumber)
	instanceID := input.InstanceID
	startSessionInput := session.StartSessionInput{
		InstanceID: instanceID,
		MaxRetries: input.MaxRetries,
		RetryWaitInterval: input.RetryWaitInterval,
		DocumentName: documentName,
		Parameters: map[string][]string{
			"portNumber":      {remotePortNumberStr},
			"localPortNumber": {localPortNumberStr},
		},
	}
	sessionInput := &awsssm.StartSessionInput{
		Target:       &instanceID,
		DocumentName: &documentName,
		Parameters: map[string][]string{
			"portNumber":      {remotePortNumberStr},
			"localPortNumber": {localPortNumberStr},
		},
	} 

	ssmClient := awsssm.NewFromConfig(cfg)

	var sessionOutput *awsssm.StartSessionOutput
	for retryCount := 1; retryCount <= input.MaxRetries; retryCount ++ {
		sessionOutput, err = session.Start(ctx, ssmClient, startSessionInput)
		if err != nil {
			logger.Infof("Tunnel could not be opened, error is: %s. Retry count is %d, max count is %d.", err, retryCount, input.MaxRetries)
		}
		if retryCount < input.MaxRetries {
			time.Sleep(input.RetryWaitInterval)
		}
	}
	if sessionOutput == nil {
		input.TunnelIsOpen <- false
		return "", err
	}

	termSessionInput := awsssm.TerminateSessionInput{
		SessionId: sessionOutput.SessionId,
	}
	defer ssmClient.TerminateSession(ctx, &termSessionInput)

	sessionOutputData, err := json.Marshal(sessionOutput)
	if err != nil {
		input.TunnelIsOpen <- false
		return *sessionOutput.SessionId, err
	}
	sessionInputData, err := json.Marshal(sessionInput)
	if err != nil {
		input.TunnelIsOpen <- false
		return *sessionOutput.SessionId, err
	}

	args := []string{
		string(sessionOutputData),
		input.RegionName,
		"StartSession",
		"", // profile name
		string(sessionInputData),
		*sessionOutput.StreamUrl,
	}

	// This logic borrowed heavily from Hashicorp Packer's AWS plugin, see https://github.com/hashicorp/packer-plugin-amazon/blob/main/builder/common/ssm/session.go and
	// https://github.com/hashicorp/packer-plugin-sdk/blob/main/shell-local/localexec/run_and_stream.go.
	cmd := exec.Command("session-manager-plugin", args...)
	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()
	defer stdoutW.Close()
	defer stderrW.Close()

	cmd.Stdout = stdoutW
	cmd.Stderr = stderrW

	logging.Infof(logger, "Starting session manager plugin process.")
	err = cmd.Start()
	if err != nil {
		input.TunnelIsOpen <- false
		return *sessionOutput.SessionId, err
	}

	stdOutChan := iochan.DelimReader(stdoutR, '\n')
	stdErrChan := iochan.DelimReader(stderrR, '\n')

	var streamWg sync.WaitGroup
	streamWg.Add(2)

	streamFunc := func(ch <-chan string) {
		defer streamWg.Done()

		for line := range ch {
			logging.Infof(logger, line)
		}
	}

	go streamFunc(stdOutChan)
	go streamFunc(stdErrChan)

	logging.Infof(logger, "Senging signal that tunnel is open.")
	input.TunnelIsOpen <- true

	logging.Infof(logger, "Waiting for signal that tunnel can close.")
	<-input.TunnelCanClose
	logging.Infof(logger, "Received signal that tunnel can close.")

	cmd.Process.Kill()

	logging.Infof(logger, "Sending signal that tunnel has been closed.")
	input.TunnelIsOpen <- false

	return *sessionOutput.SessionId, err

}
