package register

import (
	"devops/feishu/config"
	"devops/tools/ioc"
	"net/http"

	_ "devops/feishu/pkg/handler"

	"github.com/gin-gonic/gin"
)

type RegisterHandler struct{}

func init() {
	ioc.Api.RegisterContainer("FeishuRegister", &RegisterHandler{})
}

func (h *RegisterHandler) Init() error {
	c, err := config.LoadConfig()
	if err != nil {
		return err
	}
	record := c.Application.GinRootRouter()

	record.GET("/health", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{
			"message": "hello ok！",
		})
	})
	return nil
}
