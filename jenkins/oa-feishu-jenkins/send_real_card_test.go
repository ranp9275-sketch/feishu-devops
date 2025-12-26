package oafeishujenkins

import (
	"context"
	"devops/feishu/config"
	"devops/feishu/pkg/feishu"
	oajenkins "devops/jenkins/oa-jenkins"
	"fmt"
	"testing"
)

// RealSender uses the actual implementation but allows for logging or interception if needed
type RealSender struct {
	sender feishu.Sender
}

func (r *RealSender) Send(ctx context.Context, receiveID, receiveIDType, msgType, content string) error {
	fmt.Printf("Sending real message to %s\n", receiveID)
	return r.sender.Send(ctx, receiveID, receiveIDType, msgType, content)
}

// TestSendRealCard sends a real card to the specified user ID using the actual environment configuration.
// CAUTION: This will send a real notification.
func TestSendRealCard(t *testing.T) {
	// Restore original functions after test to avoid side effects if run in suite
	origLoadConfig := loadConfigFunc
	origNewSender := newSenderFunc
	defer func() {
		loadConfigFunc = origLoadConfig
		newSenderFunc = origNewSender
	}()

	// Use real config loading (assumes environment variables or default config are set correctly)
	loadConfigFunc = config.LoadConfig

	// Use real sender
	newSenderFunc = func(cfg *config.Config) feishu.Sender {
		client := feishu.NewClient(cfg)
		return feishu.NewAPISender(client)
	}

	// Real user ID provided by user
	realReceiveID := "ou_792407cf78ecf995f7d2cdf99a556900"

	// Construct real job data for the card
	jobs := []*oajenkins.JenkinsJob{
		{
			JobName:   "test-real-job-001",
			JobBranch: "feature/test-branch",
		},
		{
			JobName:   "test-real-job-002",
			JobBranch: "hotfix/urgent-fix",
		},
	}

	// Execute SendCard
	fmt.Println("Attempting to send real card...")
	err := SendCard(context.Background(), realReceiveID, "open_id", jobs)
	if err != nil {
		t.Fatalf("Failed to send real card: %v", err)
	}
	fmt.Println("Successfully sent real card!")
}
