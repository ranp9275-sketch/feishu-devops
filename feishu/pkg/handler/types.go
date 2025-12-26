package handler

import "encoding/json"

// APIResponse 统一API响应结构
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// SendRequest 发送消息请求结构
type SendRequest struct {
	ReceiveID     string          `json:"receive_id"`
	ReceiveIDType string          `json:"receive_id_type"`
	MsgType       string          `json:"msg_type"`
	Content       json.RawMessage `json:"content"`
}

// Service 定义服务信息
type Service struct {
	Name     string   `json:"name"`
	ObjectID string   `json:"object_id"`
	Branches []string `json:"branches"`
	Actions  []string `json:"actions"` // 支持多个动作，如 ["gray", "official"]
}

// GrayCardRequest 定义灰度卡片构建请求
type GrayCardRequest struct {
	Title         string    `json:"title"`
	Services      []Service `json:"services"`
	ObjectID      string    `json:"object_id"`
	ReceiveID     string    `json:"receive_id,omitempty"`
	ReceiveIDType string    `json:"receive_id_type,omitempty"`
}

// SendGrayCardRequest 发送灰度卡片请求结构
type SendGrayCardRequest struct {
	ReceiveID     string          `json:"receive_id"`
	ReceiveIDType string          `json:"receive_id_type"`
	CardData      GrayCardRequest `json:"card_data"`
}
