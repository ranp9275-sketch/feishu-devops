package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"devops/feishu/config"
	"devops/feishu/pkg/feishu"
	"devops/tools/ioc"
	"devops/tools/logger"

	"github.com/gin-gonic/gin"
)

type ApiHandler struct {
	handler *Handler
}

func init() {
	ioc.Api.RegisterContainer("FHandler", &ApiHandler{})
}

func (h *ApiHandler) Init() error {
	c, err := config.LoadConfig()
	if err != nil {
		return err
	}

	client := feishu.NewClient(c)
	h.handler = NewHandler(client)

	root := c.Application.GinRootRouter().Group("feishu")
	h.Register(root)

	return nil
}

func (h *ApiHandler) Register(appRouter gin.IRouter) {
	appRouter.POST("/api/send-card", h.handler.SendCard)
	appRouter.GET("/version", h.handler.Version)
}

func mapErrorCode(status int) int {
	if status >= 500 {
		return 50000
	}
	return 40000
}

// Handler HTTP处理器
type Handler struct {
	client *feishu.Client
	logger *logger.Logger
	sender feishu.Sender
}

type BadRequestError struct{ Msg string }

func (e BadRequestError) Error() string { return e.Msg }

// NewHandler 创建新的HTTP处理器
func NewHandler(client *feishu.Client) *Handler {
	// 初始化回调处理器
	InitCallbackHandler(client)

	return &Handler{
		client: client,
		logger: logger.NewLogger("INFO"),
		sender: feishu.NewAPISender(client),
	}
}

// SendCard 发送卡片消息处理函数
func (h *Handler) SendYCard(c *gin.Context) {
	var req SendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid request body: %v", err)
		h.writeError(c, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	ctx := c.Request.Context()

	// 验证必要参数
	if err := h.validateSendRequest(&req); err != nil {
		h.logger.Warn("Validation failed: %v", err)
		h.writeError(c, http.StatusBadRequest, err.Error())
		return
	}

	var err error
	var result map[string]interface{}

	switch req.MsgType {
	case "interactive":
		result, err = h.handleInteractiveMessage(ctx, req)
	case "text":
		result, err = h.handleTextMessage(ctx, req)
	default:
		h.writeError(c, http.StatusBadRequest, fmt.Sprintf("Unsupported message type: %s", req.MsgType))
		return
	}

	if err != nil {
		h.logger.Error("Failed to process message: %v", err)
		if _, ok := err.(BadRequestError); ok {
			h.writeError(c, http.StatusBadRequest, fmt.Sprintf("Failed to process message: %v", err))
		} else {
			h.writeError(c, http.StatusInternalServerError, fmt.Sprintf("Failed to process message: %v", err))
		}
		return
	}

	h.writeSuccess(c, result)
}

// SendGrayCard 发送动态生成的灰度发布卡片
func (h *Handler) SendCard(c *gin.Context) {
	var req SendGrayCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid request body: %v", err)
		h.writeError(c, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	if req.ReceiveID == "" || req.ReceiveIDType == "" {
		h.writeError(c, http.StatusBadRequest, "receive_id and receive_id_type are required")
		return
	}

	// 生成唯一请求ID
	requestID := fmt.Sprintf("req_%d", time.Now().UnixNano())

	// 填充接收者信息到 GrayCardRequest
	req.CardData.ReceiveID = req.ReceiveID
	req.CardData.ReceiveIDType = req.ReceiveIDType

	// 保存请求数据以便回调使用
	GlobalStore.Save(requestID, req.CardData)

	// 1. 动态构建卡片内容 (V1 Message Card)
	// 检查是否包含灰度服务，如果包含，则过滤显示
	displayCardData := req.CardData
	hasGray := false
	for _, s := range req.CardData.Services {
		for _, a := range s.Actions {
			if strings.EqualFold(a, "gray") || a == "灰度" {
				hasGray = true
				break
			}
		}
		if hasGray {
			break
		}
	}

	if hasGray {
		// 灰度模式：只显示灰度服务，且只显示灰度按钮（或非正式按钮）
		var filteredServices []Service
		for _, s := range req.CardData.Services {
			hasGrayAction := false
			for _, a := range s.Actions {
				if strings.EqualFold(a, "gray") || a == "灰度" {
					hasGrayAction = true
					break
				}
			}

			if hasGrayAction {
				// 复制服务并过滤动作
				newService := s
				newActions := []string{}
				for _, a := range s.Actions {
					// 过滤掉正式发布按钮
					if strings.EqualFold(a, "official") || strings.EqualFold(a, "release") || a == "正式" {
						continue
					}
					newActions = append(newActions, a)
				}
				newService.Actions = newActions
				filteredServices = append(filteredServices, newService)
			}
		}
		displayCardData.Services = filteredServices
	}

	cardContent := BuildCard(displayCardData, requestID, nil, nil)

	// 2. 序列化为 JSON 字符串
	cardBytes, err := json.Marshal(cardContent)
	if err != nil {
		h.logger.Error("Failed to marshal card content: %v", err)
		h.writeError(c, http.StatusInternalServerError, "Failed to build card content")
		return
	}

	// 3. 发送消息 (MsgType=interactive)
	ctx := c.Request.Context()
	err = h.sender.Send(ctx, req.ReceiveID, req.ReceiveIDType, "interactive", string(cardBytes))
	if err != nil {
		h.logger.Error("Failed to send gray card: %v", err)
		h.writeError(c, http.StatusInternalServerError, fmt.Sprintf("Failed to send gray card: %v", err))
		return
	}

	h.writeSuccess(c, map[string]string{
		"message": "Gray release card sent successfully",
	})
}

// validateSendRequest 验证发送请求
func (h *Handler) validateSendRequest(req *SendRequest) error {
	if req.ReceiveID == "" {
		return fmt.Errorf("receive_id is required")
	}
	if req.ReceiveIDType == "" {
		return fmt.Errorf("receive_id_type is required")
	}
	if req.MsgType == "" {
		return fmt.Errorf("msg_type is required")
	}
	if req.Content == nil {
		return fmt.Errorf("content is required")
	}
	return nil
}

// handleInteractiveMessage 处理交互式消息（卡片）
func (h *Handler) handleInteractiveMessage(ctx context.Context, req SendRequest) (map[string]interface{}, error) {
	if len(req.Content) == 0 {
		return nil, BadRequestError{"invalid card content format"}
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(req.Content, &raw); err != nil {
		return nil, BadRequestError{"invalid card content format"}
	}

	// 兼容两种传法：顶层 card 或直接卡片结构
	var card map[string]interface{}
	if c, ok := raw["card"].(map[string]interface{}); ok {
		card = c
	} else {
		card = raw
	}

	// 检查是否包含 V2 schema 标识
	_, hasSchema := card["schema"]

	// 如果没有 schema，检查是否是 V1 (Message Card)
	// V1 特征：包含 config/card_link/header/modules (elements) 但无 schema/body
	// 如果用户没有提供 schema，我们不强制注入 "2.0"，除非它看起来像 V2
	// 但如果用户提供了 "tag": "action" (Action Module)，这在 V2 是不支持的，必须保持为 V1

	// 我们采取保守策略：
	// 1. 如果有 schema，则认为是 V2，进行 V2 校验
	// 2. 如果无 schema，但有 body，认为是 V2
	// 3. 否则，认为是 V1，原样透传，不注入 schema

	isV2 := hasSchema
	if _, ok := card["body"]; ok {
		isV2 = true
	}

	if isV2 {
		// 确保 schema 存在
		if _, ok := card["schema"]; !ok {
			card["schema"] = "2.0"
		}

		// V2 校验：elements 必须在 body 下
		// 如果顶层有 elements，尝试迁移到 body (仅当 body 不存在时)
		if _, hasBody := card["body"].(map[string]interface{}); !hasBody {
			if elems, ok := card["elements"].([]interface{}); ok && len(elems) > 0 {
				card["body"] = map[string]interface{}{"elements": elems}
				delete(card, "elements")
			}
		}

		// 校验 body.elements 是否存在且非空
		var elementsLen int
		if body, ok := card["body"].(map[string]interface{}); ok {
			if elems, ok := body["elements"].([]interface{}); ok {
				elementsLen = len(elems)
			}
		}
		if elementsLen == 0 {
			return nil, BadRequestError{"card V2 must include body.elements"}
		}
	} else {
		// V1 校验：必须包含 elements (或 modules)
		// V1 中 modules 和 elements 是别名，通常用 elements
		var elementsLen int
		if elems, ok := card["elements"].([]interface{}); ok {
			elementsLen = len(elems)
		} else if modules, ok := card["modules"].([]interface{}); ok {
			elementsLen = len(modules)
		}

		if elementsLen == 0 {
			return nil, BadRequestError{"card must include elements or modules"}
		}
	}

	// 发送为卡片 JSON 字符串
	b, _ := json.Marshal(card)
	contentStr := string(b)

	err := h.sender.Send(ctx, req.ReceiveID, req.ReceiveIDType, req.MsgType, contentStr)
	if err != nil {
		return nil, fmt.Errorf("failed to send card message: %w", err)
	}

	return map[string]interface{}{
		"message": "Card message sent successfully",
	}, nil
}

// handleTextMessage 处理文本消息
func (h *Handler) handleTextMessage(ctx context.Context, req SendRequest) (map[string]interface{}, error) {
	var obj struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(req.Content, &obj); err == nil && strings.TrimSpace(obj.Text) != "" {
		contentJSON, _ := json.Marshal(map[string]string{"text": obj.Text})
		if err := h.sender.Send(ctx, req.ReceiveID, req.ReceiveIDType, req.MsgType, string(contentJSON)); err != nil {
			return nil, fmt.Errorf("failed to send text message: %w", err)
		}
		return map[string]interface{}{"message": "Text message sent successfully"}, nil
	}

	var s string
	if err := json.Unmarshal(req.Content, &s); err == nil && strings.TrimSpace(s) != "" {
		contentJSON, _ := json.Marshal(map[string]string{"text": s})
		if err := h.sender.Send(ctx, req.ReceiveID, req.ReceiveIDType, req.MsgType, string(contentJSON)); err != nil {
			return nil, fmt.Errorf("failed to send text message: %w", err)
		}
		return map[string]interface{}{"message": "Text message sent successfully"}, nil
	}

	return nil, BadRequestError{"invalid text content format"}
}

// Health 健康检查接口
func (h *Handler) Health(c *gin.Context) {
	h.writeSuccess(c, map[string]string{
		"status":  "healthy",
		"service": "feishu-message-service",
	})
}

// Version 版本信息接口
func (h *Handler) Version(c *gin.Context) {
	h.writeSuccess(c, map[string]string{
		"version": os.Getenv("VERSION"),
	})
}

// writeSuccess 写入成功响应
func (h *Handler) writeSuccess(c *gin.Context, data interface{}) {
	response := APIResponse{
		Code:    0,
		Message: "success",
		Data:    data,
	}
	c.JSON(http.StatusOK, response)
}

// writeError 写入错误响应
func (h *Handler) writeError(c *gin.Context, statusCode int, message string) {
	response := APIResponse{
		Code:    mapErrorCode(statusCode),
		Message: message,
		Data:    nil,
	}
	c.JSON(statusCode, response)
}

// GetLogger 获取日志记录器
func (h *Handler) GetLogger() *logger.Logger {
	return h.logger
}
