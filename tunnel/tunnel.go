package tunnel

import (
	//"github.com/stretchr/testify/assert"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"gopkg.in/square/go-jose.v2/json"

	"github.com/gruntwork-io/terratest/modules/logger"

	"github.com/mitchellh/iochan"
)

func CreateSsmTunnelE(tunnelOpen chan bool, tunnelCanClose chan bool, t *testing.T, instanceId string, port int, region string) (string, error) {

	
	logger.Logf(t, "Loading AWS config")
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return "", err
	}
	cfg.Region = region

	documentName := "AWS-StartPortForwardingSession"
	portNumberStr := fmt.Sprintf("%v", port)
	sessionInput := &ssm.StartSessionInput{
		Target:       &instanceId,
		DocumentName: &documentName,
		Parameters: map[string][]string{
			"portNumber":      {portNumberStr},
			"localPortNumber": {portNumberStr},
		},
	}

	ssmClient := ssm.NewFromConfig(cfg)
	sessionOutput, err := ssmClient.StartSession(context.Background(), sessionInput)
	if err != nil {
		return "", err
	}
	termSessionInput := ssm.TerminateSessionInput{
		SessionId: sessionOutput.SessionId,
	}
	defer ssmClient.TerminateSession(context.Background(), &termSessionInput)

	sessionOutputData, err := json.Marshal(sessionOutput)
	if err != nil {
		return *sessionOutput.SessionId, err
	}
	sessionInputData, err := json.Marshal(sessionInput)
	if err != nil {
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

	logger.Logf(t, "Starting session manager plugin process.")
	err = cmd.Start()
	if err != nil {
		return *sessionOutput.SessionId, err
	}

	stdOutChan := iochan.DelimReader(stdoutR, '\n')
	stdErrChan := iochan.DelimReader(stderrR, '\n')

	var streamWg sync.WaitGroup
	streamWg.Add(2)

	streamFunc := func(ch <-chan string) {
		defer streamWg.Done()

		for line := range ch {
			logger.Logf(t, line)
		}
	}

	go streamFunc(stdOutChan)
	go streamFunc(stdErrChan)

	logger.Logf(t, "Senging signal that tunnel is open.")
	tunnelOpen <- true

	logger.Logf(t, "Waiting for signal that tunnel can close.")
	<-tunnelCanClose
	logger.Logf(t, "Received signal that tunnel can close.")

	cmd.Process.Kill()

	logger.Logf(t, "Sending signal that tunnel has been closed.")
	tunnelOpen <- false

	return *sessionOutput.SessionId, err

}
