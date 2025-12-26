package oajenkins

import (
	"fmt"
	"testing"
)

func TestGetLatestJson(t *testing.T) {
	t.Run("GetLatestJson", func(t *testing.T) {
		req, err := GetLatestJson()
		if err != nil {
			t.Errorf("Failed to get latest json: %v", err)
		}
		if req == nil {
			t.Errorf("Latest json is nil")
		}
	})

	t.Run("ExtractProjectNames", func(t *testing.T) {
		req, err := GetLatestJson()
		if err != nil {
			t.Errorf("Failed to get latest json: %v", err)
		}
		if req == nil {
			t.Errorf("Latest json is nil")
		}
	})

	t.Run("HandleLatestJson", func(t *testing.T) {
		req, err := GetLatestJson()

		if err != nil {
			t.Errorf("Failed to handle latest json: %v", err)
		}
		if req == nil {
			t.Errorf("Latest json is nil")
		}

		jenkinsJobs, err := NewJenkinsJob("tustin-construction-assistant-admin-web", "master").HandleLatestJson(req)
		if err != nil {
			t.Errorf("Failed to handle latest json: %v", err)
		}
		if jenkinsJobs == nil {
			t.Errorf("Jenkins jobs is nil")
		}
		for _, job := range jenkinsJobs {
			fmt.Printf("Jenkins job: %v\n", job)
		}

	})
}
