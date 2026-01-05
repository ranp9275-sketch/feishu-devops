package oa

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Helper for converting various types to string
func anyToString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case json.Number:
		return strings.TrimSpace(t.String())
	case float64:
		if t == math.Trunc(t) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case uint64:
		return strconv.FormatUint(t, 10)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

// FindOriginalData attempts to extract the true original_data map from various nested structures
func FindOriginalData(jsonData map[string]interface{}) map[string]interface{} {
	var originalData map[string]interface{}

	// Case 1: Direct original_data
	if od, exists := jsonData["original_data"].(map[string]interface{}); exists {
		// Verify if this is the REAL original_data (must contain fwm or similar fields)
		// Or if it's a wrapper that happens to be named "original_data"
		if _, ok := od["fwm"]; ok {
			originalData = od
		} else {
			// If it looks like a wrapper (has "data" or "latest_file"), try to extract from it
			if innerData, ok := od["data"].(map[string]interface{}); ok {
				if latestFile, ok := innerData["latest_file"].(map[string]interface{}); ok {
					if innerOD, ok := latestFile["original_data"].(map[string]interface{}); ok {
						originalData = innerOD
					}
				}
			}
		}
	}

	// Case 5: Nested inside data (Some API responses)
	if originalData == nil {
		if data, ok := jsonData["data"].(map[string]interface{}); ok {
			if od, exists := data["original_data"].(map[string]interface{}); exists {
				originalData = od
			}
		}
	}

	// Case 2: Nested inside data.latest_file (Standard API response)
	if originalData == nil {
		if data, ok := jsonData["data"].(map[string]interface{}); ok {
			// Also check if data itself IS the wrapper for latest_file (without "data" key inside)
			if latestFile, ok := data["latest_file"].(map[string]interface{}); ok {
				// Try to get original_data from latest_file
				if od, exists := latestFile["original_data"].(map[string]interface{}); exists {
					originalData = od
				}
			}
		}
	}

	// Case 3: Nested inside latest_file (Some unwrapped formats)
	if originalData == nil {
		if latestFile, ok := jsonData["latest_file"].(map[string]interface{}); ok {
			if od, exists := latestFile["original_data"].(map[string]interface{}); exists {
				originalData = od
			}
		}
	}

	// Case 4: Fallback - jsonData itself IS the original_data (e.g. fwm exists at top level)
	if originalData == nil {
		if val, ok := jsonData["fwm"]; ok && val != nil {
			originalData = jsonData
		}
	}

	// Case 6: Absolute fallback for broken recursion
	if originalData == nil {
		if data, ok := jsonData["data"].(map[string]interface{}); ok {
			if _, ok := data["fwm"]; ok {
				originalData = data
			}
		}
	}

	return originalData
}

// ExtractRequestDetails extracts common fields from the JSON data
func ExtractRequestDetails(jsonData map[string]interface{}) map[string]string {
	result := map[string]string{
		"request_name": "-",
		"applicant":    "-",
		"request_time": "-",
		"job_name":     "-",
	}

	originalData := FindOriginalData(jsonData)
	if originalData == nil {
		return result
	}

	// Extract Job Name (fwm)
	if val, ok := originalData["fwm"]; ok {
		result["job_name"] = anyToString(val)
	}

	// Extract Request Info
	var requestName, applicant, requestTime string
	
	// Try requestManager first
	if rm, ok := originalData["requestManager"].(map[string]interface{}); ok {
		requestName = anyToString(rm["requestname"])
		applicant = anyToString(rm["sqr"])
		if applicant == "" {
			applicant = anyToString(rm["creater"])
		}
		requestTime = anyToString(rm["sqsj"])
	}

	// Fallback to top level of originalData
	if requestName == "" {
		requestName = anyToString(originalData["requestname"])
	}
	if applicant == "" {
		applicant = anyToString(originalData["sqr"])
	}
	if requestTime == "" {
		requestTime = anyToString(originalData["sqsj"])
	}

	if requestName != "" {
		result["request_name"] = requestName
	}
	if applicant != "" {
		result["applicant"] = applicant
	}
	if requestTime != "" {
		result["request_time"] = requestTime
	}

	return result
}
