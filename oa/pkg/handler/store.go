package oa

import (
	log "devops/tools/logger"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

var (
	storageDir = "data/requests"
	Logger     = log.NewLogger("ERROR")
)

// saveToDisk 将请求数据持久化到磁盘
func SaveToDisk(id string, req interface{}) error {
	filePath := filepath.Join(storageDir, id+".json")

	// 确保目录存在
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		Logger.Error("Failed to create directory: %v", err)
		return err
	}

	data, err := json.MarshalIndent(req, "", "  ")

	if err != nil {
		Logger.Error("Failed to marshal request data: %v", err)
		return err
	}
	err = os.WriteFile(filePath, data, 0644)
	if err != nil {

		Logger.Error("Failed to write request data to disk: %v", err)
		return err
	}
	return nil
}

// LoadFromDisk 从磁盘加载请求数据
func LoadFromDisk(id string) (map[string]interface{}, error) {
	filePath := filepath.Join(storageDir, id+".json")
	data, err := os.ReadFile(filePath)
	if err != nil {

		Logger.Error("Failed to read request data from disk: %v", err)
		return nil, err
	}

	var req map[string]interface{}
	if err := json.Unmarshal(data, &req); err != nil {
		Logger.Error("Failed to unmarshal request data: %v", err)
		return nil, err
	}
	return req, nil
}

// LoadJsonFromDiskALL 从磁盘加载所有请求数据
func LoadJsonFromDiskALL() (map[string]interface{}, error) {
	reqs := make(map[string]interface{})
	// 遍历目录下所有文件
	err := filepath.Walk(storageDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			Logger.Error("Failed to walk through directory: %v", err)
			return err
		}
		if info.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}

		id := filepath.Base(path)[:len(filepath.Base(path))-5]
		req, err := LoadFromDisk(id)
		if err != nil {
			Logger.Error("Failed to load request data from disk for id %s: %v", id, err)
			// Skip this file and continue
			return nil
		}
		reqs[id] = req
		return nil
	})
	if err != nil {
		Logger.Error("Failed to walk through directory: %v", err)
		return nil, err
	}
	return reqs, nil
}

// 获取最新的json文件内容
func GetLatestJsonFileContent() (map[string]interface{}, error) {
	// 遍历目录下所有文件
	var latestFile string
	var latestModTime int64

	// 如果文件很多呢？
	// 可以考虑使用并发处理，或者使用 os.ReadDir 来获取目录下的所有文件
	dirEntries, err := os.ReadDir(storageDir)
	if err != nil {
		Logger.Error("Failed to read directory: %v", err)
		return nil, err
	}

	for _, entry := range dirEntries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			Logger.Error("Failed to get file info for %s: %v", entry.Name(), err)
			continue
		}

		if info.ModTime().Unix() > latestModTime {
			latestModTime = info.ModTime().Unix()
			latestFile = filepath.Join(storageDir, entry.Name())
		}
	}

	// 如果没有找到最新文件，返回错误
	if latestFile == "" {
		Logger.Error("No valid JSON files found in directory")
		return nil, os.ErrNotExist
	}

	// 加载最新文件内容
	req, err := LoadFromDisk(filepath.Base(latestFile)[:len(filepath.Base(latestFile))-5])
	if err != nil {
		Logger.Error("Failed to load request data from disk for id %s: %v", filepath.Base(latestFile)[:len(filepath.Base(latestFile))-5], err)
		return nil, err
	}
	return req, nil
}

// 获取 最新的json文件内容
func GetLatestJsonFromApi() (map[string]interface{}, error) {
	// 发送GET请求
	resp, err := http.Get("")
	if err != nil {
		Logger.Error("Failed to send GET request: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		Logger.Error("Unexpected status code: %d", resp.StatusCode)
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// 解析JSON响应
	var req map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&req); err != nil {
		Logger.Error("Failed to decode JSON response: %v", err)
		return nil, err
	}

	return req, nil
}
