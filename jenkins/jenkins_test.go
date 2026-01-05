package jenkins

import (
	"context"
	"os"
	"testing"
)

func TestJenkinsClient(t *testing.T) {
	t.Run("TestNewClient", func(t *testing.T) {
		client := NewClient()
		t.Logf("NewClient() returned %v", client)
		if client == nil {
			t.Errorf("NewClient() returned nil")
		}
	})

	t.Run("TestBuildHandler", func(t *testing.T) {
		if os.Getenv("RUN_JENKINS_INTEGRATION_TESTS") != "1" {
			t.Skip("skipping Jenkins integration tests")
		}

		client := NewClient()
		if client == nil {
			t.Fatal("NewClient() returned nil")
		}

		tests := []struct {
			name string
			req  BuildRequest
		}{
			{
				name: "Deploy Build",
				req: BuildRequest{
					JobName:    "tustin-construction-assistant-admin-web-prod",
					Branch:     "master",
					DeployType: "Deploy",
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				queueID, err := client.Build(context.Background(), tt.req)
				t.Logf("Build() for %s returned queueID: %d, err: %v", tt.name, queueID, err)
				if err != nil {
					t.Errorf("Build() for %s returned error: %v", tt.name, err)
				}
				if queueID == 0 {
					t.Errorf("Build() for %s returned invalid queueID: 0", tt.name)
				}
			})
		}
	})
}
