package oafeishujenkins

import (
	"context"
	"devops/feishu/config"
	"devops/feishu/pkg/feishu"
	oajenkins "devops/jenkins/oa-jenkins"
	"os"
	"strings"
	"testing"
)

// MockSender implements feishu.Sender for testing
type MockSender struct {
	SendFunc func(ctx context.Context, receiveID, receiveIDType, msgType, content string) error
}

func (m *MockSender) Send(ctx context.Context, receiveID, receiveIDType, msgType, content string) error {
	if m.SendFunc != nil {
		return m.SendFunc(ctx, receiveID, receiveIDType, msgType, content)
	}
	return nil
}

func TestSendCard(t *testing.T) {
	// Backup and restore original functions
	origLoadConfig := loadConfigFunc
	origNewSender := newSenderFunc
	defer func() {
		loadConfigFunc = origLoadConfig
		newSenderFunc = origNewSender
		os.RemoveAll("data") // Clean up created files if any
	}()

	// Mock LoadConfig
	loadConfigFunc = func() (*config.Config, error) {
		return &config.Config{
			FeishuAppID:     "test-app-id",
			FeishuAppSecret: "test-app-secret",
		}, nil
	}

	// Mock Sender
	mockSender := &MockSender{}
	newSenderFunc = func(cfg *config.Config) feishu.Sender {
		return mockSender
	}

	// Test data
	jobs := []*oajenkins.JenkinsJob{
		{
			JobName:   "test-job",
			JobBranch: "master",
		},
	}

	// Expectation
	mockSender.SendFunc = func(ctx context.Context, receiveID, receiveIDType, msgType, content string) error {
		if receiveID != "" {
			t.Errorf("expected receiveID user123, got %s", receiveID)
		}
		if receiveIDType != "open_id" {
			t.Errorf("expected receiveIDType open_id, got %s", receiveIDType)
		}
		if msgType != "interactive" {
			t.Errorf("expected msgType interactive, got %s", msgType)
		}

		// Verify content contains job info
		if !strings.Contains(content, "test-job") {
			t.Errorf("content should contain job name")
		}

		return nil
	}

	err := SendCard(context.Background(), "user123", "open_id", jobs)
	if err != nil {
		t.Fatalf("SendCard failed: %v", err)
	}
}
