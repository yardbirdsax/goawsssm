package tunnel

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"gopkg.in/square/go-jose.v2/json"

	"github.com/mitchellh/iochan"

	"github.com/yardbirdsax/goawsssm/logging"
)

type CreateSSMTunnelInput struct {
	// A channel to signal the caller that the tunnel has been opened.
	tunnelIsOpen chan bool
	// A channel to receive a signal that the tunnel can be closed.
	tunnelCanClose chan bool
	// The AWS Instance ID that the connection should be made to.
	instanceID string
	// The remote port number for the tunnel.
	remotePortNumber int
	// The local port number for the tunnel.
	localPortNumber int
	// The AWS region where the instance resides.
	regionName string
}

// CreateSSMTunnelE is used to create an SSM based port-forwarding tunnel to an AWS EC2 instance. It will log various tidbits if the context input includes a key call "logger" 
// that matches the logging.Logger interface.
func CreateSsmTunnelE(ctx context.Context, input CreateSSMTunnelInput) (string, error) {

	logger := ctx.Value(logging.LOGGER_CONTEXT_KEY).(logging.Logger)
	
	logging.Infof(logger, "Loading AWS config")

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return "", err
	}
	cfg.Region = input.regionName

	documentName := "AWS-StartPortForwardingSession"
	remotePortNumberStr := fmt.Sprintf("%v", input.remotePortNumber)
	localPortNumberStr := fmt.Sprintf("%v", input.localPortNumber)
	instanceID := input.instanceID
	sessionInput := &ssm.StartSessionInput{
		Target:       &instanceID,
		DocumentName: &documentName,
		Parameters: map[string][]string{
			"portNumber":      {remotePortNumberStr},
			"localPortNumber": {localPortNumberStr},
		},
	}

	ssmClient := ssm.NewFromConfig(cfg)
	sessionOutput, err := ssmClient.StartSession(context.Background(), sessionInput)
	if err != nil {
		input.tunnelIsOpen <- false
		return "", err
	}
	termSessionInput := ssm.TerminateSessionInput{
		SessionId: sessionOutput.SessionId,
	}
	defer ssmClient.TerminateSession(context.Background(), &termSessionInput)

	sessionOutputData, err := json.Marshal(sessionOutput)
	if err != nil {
		input.tunnelIsOpen <- false
		return *sessionOutput.SessionId, err
	}
	sessionInputData, err := json.Marshal(sessionInput)
	if err != nil {
		input.tunnelIsOpen <- false
		return *sessionOutput.SessionId, err
	}

	args := []string{
		string(sessionOutputData),
		"", // region, will default to profile
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
		input.tunnelIsOpen <- false
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
	input.tunnelIsOpen <- true

	logging.Infof(logger, "Waiting for signal that tunnel can close.")
	<-input.tunnelCanClose
	logging.Infof(logger, "Received signal that tunnel can close.")

	cmd.Process.Kill()

	logging.Infof(logger, "Sending signal that tunnel has been closed.")
	input.tunnelIsOpen <- false

	return *sessionOutput.SessionId, err

}
