package middleware

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

type PageRequest struct {
	// 分页大小
	PageSize int `json:"pageSize" form:"pageSize" query:"pageSize"`
	// 分页页码
	PageNum int `json:"pageNum" form:"pageNum" query:"pageNum"`
}

// Offset 计算分页偏移量
func (p *PageRequest) Offset() int {
	return (p.PageNum - 1) * p.PageSize
}

// NewPageRequest 创建默认分页请求
func NewPageRequest() *PageRequest {
	return &PageRequest{
		PageSize: 10,
		PageNum:  1,
	}
}

// NewPageRequestFromContext 从上下文创建分页请求
func NewPageRequestFromContext(c *gin.Context) *PageRequest {
	p := NewPageRequest()
	pnStr := c.Query("pageNum")
	psStr := c.Query("pageSize")

	if pnStr != "" {
		p.PageNum, _ = strconv.Atoi(pnStr)
	}
	if psStr != "" {
		p.PageSize, _ = strconv.Atoi(psStr)
	}

	return p
}
