package handler

import (
	"devops/feishu/config"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"gorm.io/gorm"
)

// RequestStore 用于在内存中存储发送的卡片请求数据，以便在回调中重建卡片
type RequestStore struct {
	data sync.Map
	mu   sync.Mutex // 保护 DB 操作和复杂对象的更新
}

var GlobalStore = &RequestStore{}

type FeishuRequestModel struct {
	ID        string `gorm:"primaryKey;size:191"`
	Data      string `gorm:"type:longtext"` // Stores StoredRequest as JSON
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (FeishuRequestModel) TableName() string {
	return "feishu_requests"
}

func init() {
	// Try to ensure table exists on startup, but ignore errors if config not ready
	if cfg, err := config.LoadConfig(); err == nil {
		if db := cfg.GetDB(); db != nil {
			if !db.Migrator().HasTable(&FeishuRequestModel{}) {
				db.AutoMigrate(&FeishuRequestModel{})
			}
		}
	}
}

type StoredRequest struct {
	OriginalRequest GrayCardRequest
	DisabledActions map[string]bool // key: "serviceName:action"
	ActionCounts    map[string]int  // key: "serviceName:action"
}

func (s *RequestStore) getDB() *gorm.DB {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		return nil
	}
	return cfg.GetDB()
}

// ensureTable ensures the table exists (lazy init)
func (s *RequestStore) ensureTable(db *gorm.DB) {
	if !db.Migrator().HasTable(&FeishuRequestModel{}) {
		db.AutoMigrate(&FeishuRequestModel{})
	}
}

// saveToDB 将请求数据持久化到数据库
// 注意：调用此方法前必须持有锁 s.mu
func (s *RequestStore) saveToDB(id string, req *StoredRequest) {
	db := s.getDB()
	if db == nil {
		fmt.Println("Failed to get DB connection")
		return
	}
	s.ensureTable(db)

	data, err := json.Marshal(req)
	if err != nil {
		fmt.Printf("Failed to marshal request data: %v\n", err)
		return
	}

	var model FeishuRequestModel
	if err := db.First(&model, "id = ?", id).Error; err == nil {
		// Update
		model.Data = string(data)
		model.UpdatedAt = time.Now()
		if err := db.Save(&model).Error; err != nil {
			fmt.Printf("Failed to update request data in DB: %v\n", err)
		}
	} else {
		// Create
		model = FeishuRequestModel{
			ID:        id,
			Data:      string(data),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := db.Create(&model).Error; err != nil {
			fmt.Printf("Failed to create request data in DB: %v\n", err)
		}
	}
}

// loadFromDB 从数据库加载请求数据
func (s *RequestStore) loadFromDB(id string) *StoredRequest {
	db := s.getDB()
	if db == nil {
		return nil
	}
	s.ensureTable(db)

	var model FeishuRequestModel
	if err := db.First(&model, "id = ?", id).Error; err != nil {
		return nil
	}

	var req StoredRequest
	if err := json.Unmarshal([]byte(model.Data), &req); err != nil {
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
	s.saveToDB(id, stored)
}

func (s *RequestStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.Delete(id)

	db := s.getDB()
	if db != nil {
		db.Delete(&FeishuRequestModel{}, "id = ?", id)
	}
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

	// 查数据库
	loaded := s.loadFromDB(id)
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
		// 尝试从数据库加载
		req = s.loadFromDB(id)
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
	s.saveToDB(id, req)
}

// IncrementActionCount 增加动作执行次数
func (s *RequestStore) IncrementActionCount(id, serviceName, action string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	val, ok := s.data.Load(id)
	var req *StoredRequest

	if !ok {
		req = s.loadFromDB(id)
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
	s.saveToDB(id, req)
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
