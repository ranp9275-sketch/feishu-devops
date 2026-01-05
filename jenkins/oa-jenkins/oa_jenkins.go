package oajenkins

import (
	oa "devops/oa/pkg/handler"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

type JenkinsJob struct {
	JobName     string `json:"jobName"`
	JobBranch   string `json:"jobBranch"`
	Initiator   string `json:"initiator"`
	RequestName string `json:"requestName"`
	RequestID   string `json:"requestID"`
}

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

func NewJenkinsJob(jobName, jobBranch, initiator, requestName string) *JenkinsJob {
	return &JenkinsJob{
		JobName:     jobName,
		JobBranch:   jobBranch,
		Initiator:   initiator,
		RequestName: requestName,
	}
}

// GetLatestJson 获取最新的 JSON 数据
func GetLatestJson() (map[string]interface{}, error) {
	req, err := oa.GetLatestJsonFromDB()
	if err != nil {
		oa.Logger.Error("Failed to get latest json from database: %v", err)
		return nil, err
	}

	// Double check: Try to unwrap if it matches the wrapped format (data.latest_file)
	// This handles cases where LoadFromDB might have returned the raw wrapped data
	// or if the data was double-wrapped.
	if data, ok := req["data"].(map[string]interface{}); ok {
		if latestFile, ok := data["latest_file"].(map[string]interface{}); ok {
			return latestFile, nil
		}
	}

	return req, nil
}

// GetUnprocessedRequests 获取所有未处理的请求
func GetUnprocessedRequests() ([]map[string]interface{}, error) {
	reqs, err := oa.GetUnprocessedRequestsFromDB()
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	for _, req := range reqs {
		// Unwrap logic
		unwrapped := req
		if latestFile, ok := req["latest_file"].(map[string]interface{}); ok {
			unwrapped = latestFile
		} else if data, ok := req["data"].(map[string]interface{}); ok {
			if latestFile, ok := data["latest_file"].(map[string]interface{}); ok {
				unwrapped = latestFile
			}
		}

		// Preserve ID from the outer wrapper (Database ID) to ensure we can update status later.
		// Even if unwrapped has an "id" (from JSON), we MUST overwrite it with the DB ID
		// because MarkRequestAsProcessed uses this ID to query the DB.
		if dbID, ok := req["id"]; ok {
			unwrapped["id"] = dbID
		}
		results = append(results, unwrapped)
	}
	return results, nil
}

// MarkRequestAsProcessed 标记请求为已处理
func MarkRequestAsProcessed(id string) error {
	return oa.MarkRequestAsProcessed(id)
}

// HandleLatestJson 处理最新的 JSON 数据
func (j *JenkinsJob) HandleLatestJson(jsonData map[string]interface{}) ([]*JenkinsJob, error) {

	// 处理 JSON 数据的逻辑
	// 提取需要的字段
	// 尝试直接获取 original_data (当从本地文件读取时)
	var originalData map[string]interface{}
	// Debug logging
	// oa.Logger.Info("Processing JSON data: %+v", jsonData)

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

			// If still nil, maybe allow other cases to run?
			// But since "original_data" key existed, other cases might not match jsonData structure.
			// Let's leave originalData as nil if we couldn't verify it, so other Cases (like Case 2/3/4) *might* have a chance
			// (though unlikely if structure is totally different).
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
	// IMPORTANT: This must be checked carefully to avoid matching the top-level wrapper if it doesn't actually have fwm
	if originalData == nil {
		if val, ok := jsonData["fwm"]; ok && val != nil {
			originalData = jsonData
		}
	}

	// Case 6: Absolute fallback for broken recursion
	// If originalData is still nil, but jsonData["data"] exists, it might be that jsonData["data"] is the original_data
	// (Though this overlaps with Case 5, Case 5 requires "original_data" key)
	if originalData == nil {
		if data, ok := jsonData["data"].(map[string]interface{}); ok {
			if _, ok := data["fwm"]; ok {
				originalData = data
			}
		}
	}

	// Case 7: Direct latest_file access (Common in some responses)
	if originalData == nil {
		if latestFile, ok := jsonData["latest_file"].(map[string]interface{}); ok {
			// This covers the case where jsonData IS the "data" object from the response
			if od, exists := latestFile["original_data"].(map[string]interface{}); exists {
				originalData = od
			}
		}
	}

	if originalData == nil {
		oa.Logger.Error("Failed to extract original_data from jsonData")
		return nil, fmt.Errorf("failed to extract original_data from jsonData")
	}

	jobName, ok := originalData["fwm"].(string)
	if !ok {
		oa.Logger.Error("Failed to extract jobName from original_data. data=%+v", originalData)
		return nil, fmt.Errorf("failed to extract jobName from original_data")
	}

	var initiator, requestName, requestID string
	if rm, ok := originalData["requestManager"].(map[string]interface{}); ok {
		initiator = anyToString(rm["sqr"])
		if initiator == "" {
			initiator = anyToString(rm["creater"])
		}
		requestName = anyToString(rm["requestname"])
		requestID = anyToString(rm["requestid"])
	}
	if initiator == "" {
		initiator = anyToString(originalData["sqr"])
	}
	if requestName == "" {
		requestName = anyToString(originalData["requestname"])
	}
	if requestID == "" {
		requestID = anyToString(originalData["requestId"])
	}

	//提取项目名称
	projectNames := strings.Split(jobName, "<br>")
	if len(projectNames) == 0 {
		oa.Logger.Error("Failed to extract projectName from fwm")
		return nil, fmt.Errorf("failed to extract projectName from fwm")
	}
	jenkinsJobs := make([]*JenkinsJob, 0)

	//提取项目名称和分支
	for _, project := range projectNames {
		project = strings.TrimSpace(project)
		if project == "" {
			continue
		}

		// Replace &nbsp; with space
		project = strings.ReplaceAll(project, "&nbsp;", " ")

		// 提取项目名称和分支
		parts := strings.Fields(project)
		if len(parts) < 2 {
			oa.Logger.Warn("Skipping invalid project format: %s", project)
			continue
		}

		projectName := parts[0]
		branch := parts[1]
		oa.Logger.Info("Project Name: %s, Branch: %s, Initiator: %s", projectName, branch, initiator)
		jenkinsJobs = append(jenkinsJobs, &JenkinsJob{
			JobName:     projectName,
			JobBranch:   branch,
			Initiator:   initiator,
			RequestName: requestName,
			RequestID:   requestID,
		})
	}

	return jenkinsJobs, nil
}
