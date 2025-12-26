package oajenkins

import (
	oa "devops/oa/pkg/handler"
	"fmt"
	"strings"
)

type JenkinsJob struct {
	JobName   string `json:"jobName"`
	JobBranch string `json:"jobBranch"`
}

func NewJenkinsJob(jobName, jobBranch string) *JenkinsJob {
	return &JenkinsJob{
		JobName:   jobName,
		JobBranch: jobBranch,
	}
}

// GetLatestJson 获取最新的 JSON 数据
func GetLatestJson() (map[string]interface{}, error) {
	req, err := oa.GetLatestJsonFromApi()
	if err != nil {
		oa.Logger.Error("Failed to get latest json from api: %v", err)
		return nil, err
	}
	return req, nil
}

// HandleLatestJson 处理最新的 JSON 数据
func (j *JenkinsJob) HandleLatestJson(jsonData map[string]interface{}) ([]*JenkinsJob, error) {
	// 处理 JSON 数据的逻辑
	// 提取需要的字段
	//获取fwm
	data, ok := jsonData["data"].(map[string]interface{})
	if !ok {
		oa.Logger.Error("Failed to extract data from jsonData")
		return nil, fmt.Errorf("failed to extract data from jsonData")
	}

	latestFile, ok := data["latest_file"].(map[string]interface{})
	if !ok {
		oa.Logger.Error("Failed to extract latest_file from data")
		return nil, fmt.Errorf("failed to extract latest_file from data")
	}

	originalData, ok := latestFile["original_data"].(map[string]interface{})
	if !ok {
		oa.Logger.Error("Failed to extract original_data from latest_file")
		return nil, fmt.Errorf("failed to extract original_data from latest_file")
	}

	jobName, ok := originalData["fwm"].(string)
	if !ok {
		oa.Logger.Error("Failed to extract jobName from original_data")
		return nil, fmt.Errorf("failed to extract jobName from original_data")
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
		oa.Logger.Info("Project Name: %s, Branch: %s", projectName, branch)
		jenkinsJobs = append(jenkinsJobs, &JenkinsJob{
			JobName:   projectName,
			JobBranch: branch,
		})
	}

	// ...
	return jenkinsJobs, nil
}
