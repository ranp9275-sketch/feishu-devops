package oa

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetLatestJsonFileContent(t *testing.T) {
	// Setup temporary directory
	tmpDir := "data/requests_test"
	storageDir = tmpDir // Override global storageDir
	os.MkdirAll(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	// Create 3 files with different timestamps
	files := []struct {
		name    string
		content map[string]interface{}
		modTime time.Time
	}{
		{"file1", map[string]interface{}{"id": "file1"}, time.Now().Add(-3 * time.Hour)},
		{"file2", map[string]interface{}{"id": "file2"}, time.Now().Add(-1 * time.Hour)}, // This should be the latest
		{"file3", map[string]interface{}{"id": "file3"}, time.Now().Add(-2 * time.Hour)},
	}

	for _, f := range files {
		data, _ := json.Marshal(f.content)
		path := filepath.Join(tmpDir, f.name+".json")
		os.WriteFile(path, data, 0644)
		os.Chtimes(path, f.modTime, f.modTime)
	}

	// Call the function
	result, err := GetLatestJsonFileContent()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify the result
	if result["id"] != "file2" {
		t.Errorf("Expected file2, got %v. The function failed to identify the latest file.", result["id"])
	} else {
		fmt.Println("Successfully identified the latest file: file2")
	}
}
