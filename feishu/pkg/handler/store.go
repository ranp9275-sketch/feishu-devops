package handler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// RequestStore 用于在内存中存储发送的卡片请求数据，以便在回调中重建卡片
type RequestStore struct {
	data sync.Map
	mu   sync.Mutex // 保护写文件操作和复杂对象的更新
}

var GlobalStore = &RequestStore{}

const storageDir = "data/requests"

func init() {
	_ = os.MkdirAll(storageDir, 0755)
}

type StoredRequest struct {
	OriginalRequest GrayCardRequest
	DisabledActions map[string]bool // key: "serviceName:action"
	ActionCounts    map[string]int  // key: "serviceName:action"
}

// saveToDisk 将请求数据持久化到磁盘
// 注意：调用此方法前必须持有锁 s.mu
func (s *RequestStore) saveToDisk(id string, req *StoredRequest) {
	filePath := filepath.Join(storageDir, id+".json")
	data, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		fmt.Printf("Failed to marshal request data: %v\n", err)
		return
	}
	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		fmt.Printf("Failed to write request data to disk: %v\n", err)
	}
}

// loadFromDisk 从磁盘加载请求数据
func (s *RequestStore) loadFromDisk(id string) *StoredRequest {
	filePath := filepath.Join(storageDir, id+".json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}
	var req StoredRequest
	if err := json.Unmarshal(data, &req); err != nil {
		fmt.Printf("Failed to unmarshal request data: %v\n", err)
		return nil
	}
	// 确保 map 被初始化，防止 nil panic
	if req.DisabledActions == nil {
		req.DisabledActions = make(map[string]bool)
	}
	if req.ActionCounts == nil {
		req.ActionCounts = make(map[string]int)
	}
	return &req
}

func (s *RequestStore) Save(id string, req GrayCardRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()

	stored := &StoredRequest{
		OriginalRequest: req,
		DisabledActions: make(map[string]bool),
		ActionCounts:    make(map[string]int),
	}
	s.data.Store(id, stored)
	// 持久化
	s.saveToDisk(id, stored)
}

func (s *RequestStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.Delete(id)
	// 删除磁盘文件
	filePath := filepath.Join(storageDir, id+".json")
	_ = os.Remove(filePath)
}

func (s *RequestStore) Get(id string) (*StoredRequest, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 先查内存
	val, ok := s.data.Load(id)
	if ok {
		// 返回深拷贝，防止外部并发修改导致 panic
		return val.(*StoredRequest), true
	}

	// 查磁盘
	loaded := s.loadFromDisk(id)
	if loaded != nil {
		s.data.Store(id, loaded) // 回填内存
		return loaded, true
	}

	return nil, false
}

// MarkActionDisabled 标记某个动作已禁用
func (s *RequestStore) MarkActionDisabled(id, serviceName, action string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 内部直接获取原始对象，避免深拷贝带来的开销和状态不一致
	val, ok := s.data.Load(id)
	var req *StoredRequest

	if !ok {
		// 尝试从磁盘加载
		req = s.loadFromDisk(id)
		if req == nil {
			fmt.Printf("MarkActionDisabled: ID %s not found\n", id)
			return
		}
		s.data.Store(id, req)
	} else {
		req = val.(*StoredRequest)
	}

	key := serviceName + ":" + action
	req.DisabledActions[key] = true

	// 更新持久化
	s.saveToDisk(id, req)
}

// IncrementActionCount 增加动作执行次数
func (s *RequestStore) IncrementActionCount(id, serviceName, action string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	val, ok := s.data.Load(id)
	var req *StoredRequest

	if !ok {
		req = s.loadFromDisk(id)
		if req == nil {
			return
		}
		s.data.Store(id, req)
	} else {
		req = val.(*StoredRequest)
	}

	key := serviceName + ":" + action
	if req.ActionCounts == nil {
		req.ActionCounts = make(map[string]int)
	}
	req.ActionCounts[key]++

	// 更新持久化
	s.saveToDisk(id, req)
}

// GetActionCount 获取动作执行次数
func (s *RequestStore) GetActionCount(id, serviceName, action string) int {
	req, ok := s.Get(id)
	if !ok {
		return 0
	}

	key := serviceName + ":" + action
	if req.ActionCounts == nil {
		return 0
	}
	return req.ActionCounts[key]
}

// IsActionDisabled 检查某个动作是否已执行
func (s *RequestStore) IsActionDisabled(id, serviceName, action string) bool {
	req, ok := s.Get(id)
	if !ok {
		return false
	}

	key := serviceName + ":" + action
	return req.DisabledActions[key]
}
