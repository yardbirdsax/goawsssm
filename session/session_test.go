package session

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	//"github.com/yardbirdsax/goawsssm/client"
	"github.com/yardbirdsax/goawsssm/mock"
)


func TestStart(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockClient := mock.NewMockSSMClient(ctrl)
	instanceID := "i-abcd123456"
	sessionID := "abcd123456"
	streamURL := "http://127.0.0.1"
	tokenValue := "xyz0123456"
	documentName := "document"
	parameters := map[string][]string{
		"hello": {"world"},
	}
	maxRetries := 5
	retryWaitInterval := 5*time.Second
	expectedDurationSeconds := retryWaitInterval.Seconds() * float64(maxRetries - 1)
	ctx := context.TODO()
	startSesionInput := &StartSessionInput{
		InstanceID: instanceID,
		MaxRetries: maxRetries,
		RetryWaitInterval: retryWaitInterval,
		DocumentName: documentName,
		Parameters: parameters,
	}
	startSessionInputClient := &ssm.StartSessionInput{
		Target: &instanceID,
		DocumentName: &documentName,
		Parameters: parameters,
	}
	callCounter := 0
	mockClient.EXPECT().StartSession(ctx, startSessionInputClient).
		Times(maxRetries).
		DoAndReturn(
		func (ctx context.Context, input *ssm.StartSessionInput) (output *ssm.StartSessionOutput, err error) {
			callCounter ++
			if callCounter < 5 {
				output = nil
				err = fmt.Errorf("I'm an error")
			} else {
				output = &ssm.StartSessionOutput{
					SessionId: &sessionID,
					StreamUrl: &streamURL,
					TokenValue: &tokenValue,
				}
			}
			return
		},
	)

	timeStarted := time.Now()
	output, err := Start(ctx, mockClient, *startSesionInput)
	timeAfter := time.Now()
	actualDurationSeconds := timeAfter.Sub(timeStarted).Seconds()

	ctrl.Finish()
	assert.NotNil(t, output)
	assert.Nil(t, err, "Start function returned an error unexpectedly.")
	assert.Equal(t, sessionID, *(output.SessionId))
	assert.Equal(t, streamURL, *(output.StreamUrl))
	assert.Equal(t, tokenValue, *(output.TokenValue))
	assert.GreaterOrEqual(t, actualDurationSeconds, expectedDurationSeconds, "Function call did not take the minimum time expected based on minimum retry count and wait duration.")
}