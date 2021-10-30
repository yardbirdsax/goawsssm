package integration

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/gruntwork-io/terratest/modules/random"
	"github.com/gruntwork-io/terratest/modules/terraform"
	test_structure "github.com/gruntwork-io/terratest/modules/test-structure"

	"github.com/yardbirdsax/goawsssm/tunnel"
)

var regionName string = "us-east-2"

func TestGoAWSSSM(t *testing.T) {
	service := random.UniqueId()
	terraformOptions := &terraform.Options{
		TerraformDir: ".",
		Vars: map[string]interface{}{
			"service": service,
			"region": regionName,
		},
		RetryableTerraformErrors: terraform.DefaultRetryableTerraformErrors,		
	}

	defer test_structure.RunTestStage(t, "terraform_destroy", func() {
		_ = terraform.Destroy(t, terraformOptions)
	})
	test_structure.RunTestStage(t, "terraform_apply", func() {
		_ = terraform.InitAndApply(t, terraformOptions)
	})

	test_structure.RunTestStage(t, "test_ssm", func() {
		instanceID := terraform.Output(t, terraformOptions, "instance_id")
		tunnelIsOpen := make(chan bool, 1)
		tunnelCanClose := make(chan bool, 1)
		localPortNumber := random.Random(32768, 65535)
		createTunnelInput := tunnel.CreateSSMTunnelInput{
			TunnelIsOpen: tunnelIsOpen,
			TunnelCanClose: tunnelCanClose,
			InstanceID: instanceID,
			LocalPortNumber: localPortNumber,
			RemotePortNumber: 80,
			RegionName: regionName,	
			MaxRetries: 30,
			RetryWaitInterval: 5 * time.Second,		
		}
		waitGroup := new(sync.WaitGroup)
		waitGroup.Add(1)
		logger, _ := zap.NewDevelopment()
		sugar := logger.Sugar()
		ctx, cancel := context.WithTimeout(
			context.WithValue(
				context.Background(), 
				"logger", 
				sugar,
			),
			180*time.Second,
		)
		defer cancel()
		go func(ctx context.Context) {
			_, err := tunnel.CreateSSMTunnelE(ctx, createTunnelInput)
			if err != nil {
				sugar.Error(err)
			}
			sugar.Debug("goroutine exiting")
			waitGroup.Done()
		}(ctx)

		var tunnelOpened bool
		select {
		case tunnelOpened = <- tunnelIsOpen:
			sugar.Debug("Received signal from tunnel open function...")
		case <- ctx.Done():
			sugar.Error("Tunnel did not open after a reasonable period.")
			t.Fail()
		}

		if !tunnelOpened {
			sugar.Error("Tunnel was not able to be opened.")
			t.Fail()
			waitGroup.Wait()
			sugar.Debug("test function returning")
			return
		}

		url := fmt.Sprintf("localhost:%d", localPortNumber)
		time.Sleep(10*time.Second)
		conn, err := net.DialTimeout("tcp", url, 60*time.Second)
		if err != nil {
			sugar.Error(err)
			t.Fail()
		}
		if conn != nil {
			defer conn.Close()
		}
		tunnelCanClose <- true
		sugar.Debug("About to wait for goroutine exit")
		waitGroup.Wait()
	})
}