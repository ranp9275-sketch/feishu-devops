package feishu

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"devops/feishu/config"
	"devops/tools/logger"
)

// Client 飞书客户端封装
type Client struct {
	appID         string
	appSecret     string
	logger        *logger.Logger
	httpClient    *http.Client
	tenantToken   string
	tokenExpireAt time.Time
	mu            sync.RWMutex
}

// NewClient 创建新的飞书客户端
func NewClient(cfg *config.Config) *Client {
	log := logger.NewLogger(cfg.LogLevel)

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        cfg.MaxIdleConns,
			MaxIdleConnsPerHost: cfg.MaxIdleConnsPerHost,
			IdleConnTimeout:     cfg.IdleConnTimeout,
		},
	}

	log.Info("Feishu client initialized")

	return &Client{
		appID:      cfg.FeishuAppID,
		appSecret:  cfg.FeishuAppSecret,
		logger:     log,
		httpClient: httpClient,
	}
}

// generateUUID 生成UUID
func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// getTenantAccessToken 获取飞书应用的tenant_access_token
func (c *Client) getTenantAccessToken(ctx context.Context) (string, error) {
	// 读锁检查缓存
	c.mu.RLock()
	valid := c.tenantToken != "" && time.Now().Before(c.tokenExpireAt)
	token := c.tenantToken
	c.mu.RUnlock()
	if valid {
		c.logger.Debug("Using cached tenant access token")
		return token, nil
	}

	c.logger.Info("Fetching new tenant access token")

	// 飞书获取tenant_access_token的API
	tokenURL := "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal"

	// 构建请求体
	payload := map[string]string{
		"app_id":     c.appID,
		"app_secret": c.appSecret,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		c.logger.Error("Failed to marshal token request: %v", err)
		return "", fmt.Errorf("failed to marshal token request: %w", err)
	}
	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, bytes.NewBuffer(data))
	if err != nil {
		c.logger.Error("Failed to create token request: %v", err)
		return "", fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("Failed to send token request: %v", err)
		return "", fmt.Errorf("failed to send token request: %w", err)
	}
	defer resp.Body.Close()

	// 解析响应
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.logger.Error("Failed to decode token response: %v", err)
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		c.logger.Error("Token request failed: status=%d, response=%v", resp.StatusCode, result)
		return "", fmt.Errorf("token request failed: status=%d, response=%v", resp.StatusCode, result)
	}

	// 检查错误码
	if code, ok := result["code"].(float64); ok && code != 0 {
		msg, _ := result["msg"].(string)
		c.logger.Error("Token API error: code=%v, msg=%s", code, msg)
		return "", fmt.Errorf("token API error: code=%v, msg=%s", code, msg)
	}

	// 提取token
	token, ok := result["tenant_access_token"].(string)
	if !ok {
		c.logger.Error("Invalid token format in response: %v", result)
		return "", fmt.Errorf("invalid token format in response")
	}

	// 设置过期时间（默认1小时，提前10分钟刷新）
	expire, _ := result["expire"].(float64)
	c.mu.Lock()
	if expire > 0 {
		c.tokenExpireAt = time.Now().Add(time.Duration(expire-600) * time.Second)
	} else {
		c.tokenExpireAt = time.Now().Add(50 * time.Minute)
	}
	c.tenantToken = token
	c.mu.Unlock()
	c.logger.Info("Successfully obtained tenant access token, expires at: %v", c.tokenExpireAt)

	return token, nil
}

// CreateCard 创建飞书卡片
func (c *Client) CreateCard(ctx context.Context, title, content string) (string, error) {
	c.logger.Debug("Creating card with title: %s", title)

	// 构建卡片JSON内容（用于日志记录）
	cardJSON := fmt.Sprintf(`{
		"schema":"2.0",
		"header":{
			"title":{
				"content":"%s",
				"tag":"plain_text"
			}
		},
		"body":{
			"elements":[
				{
					"tag":"markdown",
					"content":"%s"
				}
			]
		}
	}`, title, content)

	c.logger.Debug("Card content: %s", cardJSON)

	// 使用自定义UUID作为卡片实例ID
	cardID := fmt.Sprintf("card_%s", generateUUID())

	c.logger.Info("Card created successfully, card_id: %s", cardID)
	return cardID, nil
}

// SendMessage 发送消息（使用飞书应用真实API）
func (c *Client) SendMessage(ctx context.Context, receiveID, receiveIdType, msgType, content string) error {
	c.logger.Debug("Sending message to %s, type: %s", receiveID, msgType)

	// 获取tenant_access_token
	token, err := c.getTenantAccessToken(ctx)
	if err != nil {
		c.logger.Error("Failed to get tenant access token: %v", err)
		return fmt.Errorf("failed to get tenant access token: %w", err)
	}

	// 飞书发送消息API（receive_id_type 需作为查询参数）
	sendURL := fmt.Sprintf("https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=%s", receiveIdType)

	// 构建消息请求体
	messagePayload := map[string]interface{}{
		"receive_id": receiveID,
		"msg_type":   msgType,
	}

	// 根据消息类型处理content
	switch msgType {
	case "text":
		// 文本消息：content直接传入 JSON 字符串，例如 {"text":"..."}
		messagePayload["content"] = content

	case "interactive":
		// 交互式卡片：content直接是卡片JSON字符串
		messagePayload["content"] = content

	default:
		c.logger.Error("Unsupported message type: %s", msgType)
		return fmt.Errorf("unsupported message type: %s", msgType)
	}

	// 序列化请求体
	payloadData, err := json.Marshal(messagePayload)
	if err != nil {
		c.logger.Error("Failed to marshal message payload: %v", err)
		return fmt.Errorf("failed to marshal message payload: %w", err)
	}

	c.logger.Debug("Message payload: %s", string(payloadData))

	// 创建HTTP请求

	req, err := http.NewRequestWithContext(ctx, "POST", sendURL, bytes.NewBuffer(payloadData))
	if err != nil {
		c.logger.Error("Failed to create message request: %v", err)
		return fmt.Errorf("failed to create message request: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Error("Failed to send message request: %v", err)
		return fmt.Errorf("failed to send message request: %w", err)
	}
	defer resp.Body.Close()

	// 解析响应
	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		c.logger.Error("Failed to decode message response: %v", err)
		return fmt.Errorf("failed to decode message response: %w", err)
	}

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		c.logger.Error("Message send failed: status=%d, response=%v", resp.StatusCode, response)
		return fmt.Errorf("message send failed: status=%d, response=%v", resp.StatusCode, response)
	}

	// 检查业务错误码
	if code, ok := response["code"].(float64); ok && code != 0 {
		msg, _ := response["msg"].(string)
		c.logger.Error("Message API error: code=%v, msg=%s", code, msg)
		return fmt.Errorf("message API error: code=%v, msg=%s", code, msg)
	}

	// 提取message_id
	data, ok := response["data"].(map[string]interface{})
	if !ok {
		c.logger.Error("Invalid response data format: %v", response)
		return fmt.Errorf("invalid response data format")
	}

	messageID, ok := data["message_id"].(string)
	if !ok {
		c.logger.Error("Message ID not found in response: %v", data)
		return fmt.Errorf("message ID not found in response")
	}

	c.logger.Info("Message sent successfully, message_id: %s", messageID)

	return nil
}

// GetLogger 获取日志记录器
func (c *Client) GetLogger() *logger.Logger {
	return c.logger
}
