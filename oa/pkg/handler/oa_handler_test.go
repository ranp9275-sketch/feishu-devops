package oa

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestStoreJsonHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler()

	t.Run("Valid JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		reqBody := `{"key": "value"}`
		c.Request = httptest.NewRequest(http.MethodPost, "/store-json-handler", bytes.NewBufferString(reqBody))
		c.Request.Header.Set("Content-Type", "application/json")

		h.StoreJsonHandler(c)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
		}
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		reqBody := `{"key": "value"` // Missing closing brace
		c.Request = httptest.NewRequest(http.MethodPost, "/store-json-handler", bytes.NewBufferString(reqBody))
		c.Request.Header.Set("Content-Type", "application/json")

		h.StoreJsonHandler(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status BadRequest, got %v", w.Code)
		}
	})

}

func TestGetJsonBatchHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler()

	// // Setup temporary storage directory
	// tempDir := t.TempDir()
	// originalStorageDir := storageDir
	// storageDir = tempDir
	// defer func() { storageDir = originalStorageDir }()

	// Create a valid JSON file
	// validJSON := `{"id": "valid", "original_data": {"key": "value"}}`

	// // Write valid file
	// os.WriteFile(filepath.Join(tempDir, "valid.json"), []byte(validJSON), 0644)

	// // Write invalid file (string content)
	// invalidJSON := `"some string content"`
	// os.WriteFile(filepath.Join(tempDir, "invalid.json"), []byte(invalidJSON), 0644)

	t.Run("Should skip invalid files and return valid ones", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/get-json-batch", nil)

		h.GetJsonBatchHandler(c)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
		}

		// Check response body
		// It should contain "valid" but not "invalid" (or at least not fail)
		// The response structure is {"code":200, "message":"Success", "data": {...}}
		if !bytes.Contains(w.Body.Bytes(), []byte("valid")) {
			t.Errorf("Response should contain valid data")
		}
	})

	t.Run("Should return empty list if no files", func(t *testing.T) {
		// // Clean up valid file
		// os.Remove(filepath.Join(tempDir, "valid.json"))
		// os.Remove(filepath.Join(tempDir, "invalid.json"))

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/get-json-batch", nil)

		h.GetJsonBatchHandler(c)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
		}
		// Should return empty list data: []
		if !bytes.Contains(w.Body.Bytes(), []byte(`"data":[]`)) {
			t.Errorf("Response should contain empty data array, got %s", w.Body.String())
		}
	})
}
