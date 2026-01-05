package oa

import (
	"devops/feishu/config"
	"devops/tools/ioc"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ApiHandler struct {
	handler *Handler
}

func init() {
	ioc.Api.RegisterContainer("OAHandler", &ApiHandler{})
}

func (h *ApiHandler) Init() error {
	c, err := config.LoadConfig()
	if err != nil {
		return err
	}

	h.handler = NewHandler()

	root := c.Application.GinRootRouter().Group("oa")
	h.Register(root)

	return nil
}

func (h *ApiHandler) Register(r gin.IRouter) {
	r.POST("/store-json", h.handler.StoreJsonHandler)
	r.GET("/get-json/:id", h.handler.GetJsonHandler)
	r.GET("/get-json-all", h.handler.GetJsonBatchHandler)
	r.GET("/get-latest-json", h.handler.GetLatestJsonHandler)
}

type Handler struct {
}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) StoreJsonHandler(c *gin.Context) {
	// Gin automatically handles Method Not Allowed if routed correctly,
	// but if we want strict content-type check:
	if c.GetHeader("Content-Type") != "application/json" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    http.StatusBadRequest,
			Message: "Content-Type must be application/json",
		})
		return
	}

	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    http.StatusInternalServerError,
			Message: "Failed to read request body",
		})
		return
	}

	// 验证json是否有效
	if !json.Valid(body) {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    http.StatusBadRequest,
			Message: "Invalid JSON",
		})
		return
	}
	var originalData map[string]interface{}
	// 尝试直接解析
	if err1 := json.Unmarshal(body, &originalData); err1 != nil {
		// 如果失败，可能是JSON字符串，尝试二次解析
		var jsonString string
		if err2 := json.Unmarshal(body, &jsonString); err2 == nil {
			// 成功解析为字符串，再尝试解析字符串内容
			if err3 := json.Unmarshal([]byte(jsonString), &originalData); err3 != nil {
				// 如果还是失败，存储为原始字符串
				originalData = map[string]interface{}{
					"_raw_json_string": jsonString,
					"_parse_error":     err3.Error(),
				}
			}
		} else {
			// 不是JSON字符串，尝试解析为其他类型
			var genericData interface{}
			if err4 := json.Unmarshal(body, &genericData); err4 == nil {
				// 成功解析为其他类型（如数组、数字等）
				originalData = map[string]interface{}{
					"_parsed_data": genericData,
					"_data_type":   fmt.Sprintf("%T", genericData),
				}
			} else {
				// 完全无法解析
				c.JSON(http.StatusBadRequest, APIResponse{
					Code:    http.StatusBadRequest,
					Message: "无效的JSON格式: " + err.Error(),
					Data: map[string]interface{}{
						"raw_input": string(body),
					},
				})
				return
			}
		}
	}

	id := uuid.New().String()[:8]
	timestamp := time.Now().Format("20060102_150405")
	uniqueID := fmt.Sprintf("%s_%s", timestamp, id)

	storedJSON := StoredJSON{
		ID:           id,
		ReceivedAt:   time.Now().Format(time.RFC3339),
		IPAddress:    c.ClientIP(),
		UserAgent:    c.GetHeader("User-Agent"),
		OriginalData: originalData,
	}

	// 讲数据存储到数据库
	err = SaveToDB(uniqueID, storedJSON)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    http.StatusInternalServerError,
			Message: "Failed to save data to database",
		})
		return
	}

	// 返回成功响应
	c.JSON(http.StatusOK, APIResponse{
		Code:    http.StatusOK,
		Message: "Success",
		Data: map[string]interface{}{
			"id": uniqueID,
		},
	})
}

func (h *Handler) GetJsonHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    http.StatusBadRequest,
			Message: "ID is required",
		})
		return
	}

	// 从数据库加载数据
	req, err := LoadFromDB(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    http.StatusInternalServerError,
			Message: "Failed to load data from database",
		})
		return
	}

	// 返回成功响应
	c.JSON(http.StatusOK, APIResponse{
		Code:    http.StatusOK,
		Message: "Success",
		Data:    req,
	})
}

func (h *Handler) GetJsonBatchHandler(c *gin.Context) {
	// 从数据库加载所有数据
	reqs, err := LoadAllJsonFromDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    http.StatusInternalServerError,
			Message: "Failed to load data from database",
		})
		return
	}

	// 即使为空也返回空数组，而不是404
	if reqs == nil {
		reqs = []map[string]interface{}{}
	}

	// Enrich data with parsed details
	for i, req := range reqs {
		if originalData, ok := req["original_data"].(map[string]interface{}); ok {
			details := ExtractRequestDetails(originalData)
			req["parsed_request_name"] = details["request_name"]
			req["parsed_applicant"] = details["applicant"]
			req["parsed_request_time"] = details["request_time"]
			req["parsed_job_name"] = details["job_name"]
		} else {
			// Try parsing the top-level req if original_data is missing or empty
			// (Though StoredJSON structure usually has it)
			details := ExtractRequestDetails(req)
			req["parsed_request_name"] = details["request_name"]
			req["parsed_applicant"] = details["applicant"]
			req["parsed_request_time"] = details["request_time"]
			req["parsed_job_name"] = details["job_name"]
		}
		reqs[i] = req
	}

	// 返回成功响应
	c.JSON(http.StatusOK, APIResponse{
		Code:    http.StatusOK,
		Message: "Success",
		Data:    reqs,
	})
}

// 获取最新的json文件内容
func (h *Handler) GetLatestJsonHandler(c *gin.Context) {
	// 从数据库加载最新数据
	latestFile, err := GetLatestJsonFromDB()
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Code:    http.StatusInternalServerError,
			Message: "Failed to load latest data from database",
		})
		return
	}

	// 返回成功响应
	c.JSON(http.StatusOK, APIResponse{
		Code:    http.StatusOK,
		Message: "Success",
		Data: map[string]interface{}{
			"latest_file": latestFile,
		},
	})
}
