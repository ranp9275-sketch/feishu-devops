package api

import (
	"devops/feishu/config"
	"devops/feishu/pkg/robot"
	"devops/tools/ioc"
	"devops/tools/middleware"
	"net/http"

	"github.com/gin-gonic/gin"
)

func init() {
	ioc.Api.RegisterContainer(robot.AppName, &RobotHandler{})
}

type RobotHandler struct {
	RobotApi robot.Service
}

func (h *RobotHandler) Init() error {
	h.RobotApi = ioc.ConController.GetMapContainer(robot.AppName).(robot.Service)

	c, err := config.LoadConfig()
	if err != nil {
		return err
	}
	subr := c.Application.GinRootRouter().Group("robot")
	h.Register(subr)

	return nil
}

func (h *RobotHandler) Register(appRouter gin.IRouter) {
	appRouter.POST("/addrobot", h.CreateFeishuRobot)
	appRouter.GET("/describe", h.DescribeFeishuRobot)
	appRouter.GET("/query", h.QueryFeishuRobot)
	appRouter.POST("/delrobot", h.DeleteFeishuRobot)
	appRouter.POST("/updaterobot", h.UpdateFeishuRobot)

}

type apiResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func writeSuccess(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

func (h *RobotHandler) CreateFeishuRobot(gin *gin.Context) {
	req := robot.NewCreateFeishuRobotRequest()
	if err := gin.BindJSON(&req); err != nil {
		middleware.Failed(err, gin)
		return
	}

	//检查是否存在
	robot, err := h.RobotApi.DescribeRobot(gin.Request.Context(), robot.NewDescribeFeishuRobotRequest(req.Name))
	if err != nil {
		middleware.Failed(err, gin)
		return
	}
	if robot != nil {
		middleware.Failed(middleware.ErrValidateFailed("robot already exists"), gin)
		return
	}

	//创建机器人
	in, err := h.RobotApi.CreateRobot(gin.Request.Context(), req)
	if err != nil {
		middleware.Failed(err, gin)
		return
	}

	writeSuccess(gin, in)
}

func (h *RobotHandler) DescribeFeishuRobot(gin *gin.Context) {
	name := gin.Query("name")
	robot, err := h.RobotApi.DescribeRobot(gin.Request.Context(), robot.NewDescribeFeishuRobotRequest(name))
	if err != nil {
		middleware.Failed(err, gin)
		return
	}
	if robot == nil {
		middleware.Failed(middleware.ErrValidateFailed("robot not found"), gin)
		return
	}
	writeSuccess(gin, robot)
}

func (h *RobotHandler) QueryFeishuRobot(gin *gin.Context) {
	req := robot.NewQueryFeishuRobotRequest()
	req.PageRequest = middleware.NewPageRequestFromContext(gin)
	req.BotName = gin.Query("name")

	robots, err := h.RobotApi.GetRobot(gin.Request.Context(), req)
	if err != nil {
		middleware.Failed(err, gin)
		return
	}

	writeSuccess(gin, robots)
}

func (h *RobotHandler) DeleteFeishuRobot(gin *gin.Context) {
	req := robot.NewDeleteFeishuRobotRequest("")
	if err := gin.BindJSON(&req); err != nil {
		middleware.Failed(err, gin)
		return
	}

	in, err := h.RobotApi.DeleteRobot(gin.Request.Context(), req)
	if err != nil {
		middleware.Failed(err, gin)
		return
	}

	writeSuccess(gin, in)
}

func (h *RobotHandler) UpdateFeishuRobot(gin *gin.Context) {
	req := robot.NewUpdateFeishuRobotRequest("")
	if err := gin.BindJSON(&req); err != nil {
		middleware.Failed(err, gin)
		return
	}

	// Ensure Name is set
	if req.Name == "" && req.CreateFeishuRobotRequest != nil {
		req.Name = req.CreateFeishuRobotRequest.Name
	}

	if req.Name == "" {
		middleware.Failed(middleware.ErrValidateFailed("name is empty"), gin)
		return
	}

	// Check if robot exists
	existing, err := h.RobotApi.DescribeRobot(gin.Request.Context(), robot.NewDescribeFeishuRobotRequest(req.Name))
	if err != nil {
		middleware.Failed(err, gin)
		return
	}
	if existing == nil {
		middleware.Failed(middleware.ErrValidateFailed("robot not found"), gin)
		return
	}

	ins, err := h.RobotApi.UpdateRobot(gin.Request.Context(), req)
	if err != nil {
		middleware.Failed(err, gin)
		return
	}

	writeSuccess(gin, ins)
}
