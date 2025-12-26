package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// CrsMiddleware 跨域中间件
func CrsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}

// 成功, 怎么把 对象 -->  HTTP Reponse
func Success(data any, c *gin.Context) {
	// 其他逻辑
	// 脱敏
	// Desense()
	c.JSON(http.StatusOK, data)
}

// 成功, 怎么把 对象 -->  HTTP Reponse
// 统一返回的数据结构: ApiException
func Failed(err error, c *gin.Context) {
	// 非200 状态, 接口报错, 返回内容: ApiException对象

	httpCode := http.StatusInternalServerError
	if v, ok := err.(*ApiException); ok {
		if v.HttpCode != 0 {
			httpCode = v.HttpCode
		}
	} else {
		// 非业务异常，支持转化为 指定的内部报错异常
		err = ErrServerInternal(err.Error())
	}

	c.JSON(httpCode, err)
	c.Abort()
}

func NewApiException(code int, message string) *ApiException {
	return &ApiException{
		Code:    code,
		Message: message,
	}
}

// 用于描述业务异常
// 实现自定义异常
// return error
type ApiException struct {
	// 业务异常的编码, 50001 表示Token过期
	Code int `json:"code"`
	// 异常描述信息
	Message string `json:"message"`
	// 不会出现在Boyd里面, 序列画成JSON, http response 进行set
	HttpCode int `json:"-"`
}

// The error built-in interface type is the conventional interface for
// representing an error condition, with the nil value representing no error.
//
//	type error interface {
//		Error() string
//	}
func (e *ApiException) Error() string {
	return e.Message
}

func (e *ApiException) String() string {
	dj, _ := json.MarshalIndent(e, "", "  ")
	return string(dj)
}

func (e *ApiException) WithMessage(msg string) *ApiException {
	e.Message = msg
	return e
}

func (e *ApiException) WithHttpCode(httpCode int) *ApiException {
	e.HttpCode = httpCode
	return e
}

func ErrServerInternal(format string, a ...any) *ApiException {
	return &ApiException{
		Code:    50000,
		Message: fmt.Sprintf(format, a...),
	}
}
func ErrNotFound(format string, a ...any) *ApiException {
	return &ApiException{
		Code:    404,
		Message: fmt.Sprintf(format, a...),
	}
}
func ErrValidateFailed(format string, a ...any) *ApiException {
	return &ApiException{
		Code:    400,
		Message: fmt.Sprintf(format, a...),
	}
}
