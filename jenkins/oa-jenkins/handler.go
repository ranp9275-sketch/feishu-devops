package oajenkins

import (
	"context"
	"devops/feishu/config"
	"devops/feishu/pkg/feishu"
	"devops/feishu/pkg/handler"
	"devops/jenkins"
	"devops/tools/ioc"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func init() {
	ioc.Api.RegisterContainer("jk_server", &JKServer{})
}

type JKServer struct {
	jenkins      *jenkins.Client
	feishuClient *feishu.Client
}

func (h *JKServer) Init() error {
	// h.jenkins = ioc.ConController.GetMapContainer(string(jenkins.AppNameJenkins)).(*jenkins.Client)
	// 因为 Jenkins 客户端现在是注册在 Api 容器中，而不是 ConController 容器中
	// 而且在当前架构下，JKServer 和 Jenkins Client 都是在 ioc.Api 容器中注册的
	// 所以我们应该从 Api 容器获取，或者直接使用 jenkins.NewClient() (如果单例模式)
	// 但这里我们尝试从 ioc.Api 获取
	if obj := ioc.Api.GetMapContainer(string(jenkins.AppNameJenkins)); obj != nil {
		h.jenkins = obj.(*jenkins.Client)
	} else {
		// 如果获取不到，尝试新建一个
		h.jenkins = jenkins.NewClient()
	}

	c, err := config.LoadConfig()
	if err != nil {
		return err
	}
	// Initialize Feishu client for notifications
	h.feishuClient = feishu.NewClient(c)

	subr := c.Application.GinRootRouter().Group("jk")
	h.Register(subr)

	return nil
}

func (h *JKServer) Register(r *gin.RouterGroup) {
	r.POST("/test-flow", h.TestFlow)
}

type TestFlowRequest struct {
	ReceiveID     string `json:"receive_id"`      // 接收通知的用户ID/群ID
	ReceiveIDType string `json:"receive_id_type"` // ID类型: open_id, chat_id, etc.
}

func (h *JKServer) TestFlow(c *gin.Context) {
	var req TestFlowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.ReceiveID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "receive_id is required"})
		return
	}

	// 异步执行完整流程模拟
	go func() {
		ctx := context.Background()
		h.simulateOAFlow(ctx, req.ReceiveID, req.ReceiveIDType)
	}()

	c.JSON(http.StatusOK, gin.H{"message": "Test flow started"})
}

// simulateOAFlow 模拟 OA 推送 -> 生成卡片 -> 发送卡片 的流程
func (h *JKServer) simulateOAFlow(ctx context.Context, receiveID, receiveIDType string) {
	// 1. 获取 OA 数据
	oaData, err := GetLatestJson()
	if err != nil {
		h.sendFeishuMessage(ctx, receiveID, receiveIDType, fmt.Sprintf("❌ 获取 OA 数据失败: %v", err))
		return
	}

	// 2. 解析 OA 数据
	dummy := &JenkinsJob{}
	jobs, err := dummy.HandleLatestJson(oaData)
	if err != nil {
		h.sendFeishuMessage(ctx, receiveID, receiveIDType, fmt.Sprintf("❌ 解析 OA 数据失败: %v", err))
		return
	}

	if len(jobs) == 0 {
		h.sendFeishuMessage(ctx, receiveID, receiveIDType, "⚠️ OA 数据中没有找到 Job")
		return
	}

	// 3. 构建 CardRequest
	var services []handler.Service
	for _, job := range jobs {
		// 为了让 BuildCard 通过校验，我们需要确保 ObjectID 不为空
		// 在 BuildCard 中，req.Services[0].ObjectID 被用作服务的标识
		// 这里我们临时借用 JobName 作为 ObjectID，或者你可以根据实际情况调整
		// 实际上 ObjectID 应该是整个发布的 ID，但这里为了测试，我们每个 Service 都填上

		// 根据 Job 名称或配置判断动作
		// 如果没有特别标识，默认为 "release" (正式发布)
		// 如果需要灰度，必须在 Job 信息或配置中有所体现，这里为了测试，我们简单地默认只给 release
		// 除非你需要测试灰度流程，可以手动修改这里
		actions := []string{"gray", "rollback", "restart"}

		// 示例：如果 Job 名称包含 "gray"，则添加灰度动作
		// if strings.Contains(job.JobName, "gray") {
		// 	actions = []string{"gray", "release", "rollback", "restart"}
		// }

		services = append(services, handler.Service{
			Name:     job.JobName + "-prod",
			ObjectID: job.JobName, // 关键修复：确保 ObjectID 不为空
			Actions:  actions,     // 修正：默认只给 release，有需要再加 gray
			Branches: []string{job.JobBranch},
		})
	}

	requestID := fmt.Sprintf("req_test_%d", time.Now().UnixNano())
	cardReq := handler.GrayCardRequest{
		Title:         "应用发布申请 (测试)",
		Services:      services,
		ReceiveID:     receiveID,
		ReceiveIDType: receiveIDType,
	}

	// 4. 保存到 GlobalStore (这一步对于回调处理是必须的)
	handler.GlobalStore.Save(requestID, cardReq)

	// 5. 构建并发送卡片
	cardContent := handler.BuildCard(cardReq, requestID, nil, nil)
	cardBytes, _ := json.Marshal(cardContent)

	err = h.feishuClient.SendMessage(ctx, receiveID, receiveIDType, "interactive", string(cardBytes))
	if err != nil {
		h.sendFeishuMessage(ctx, receiveID, receiveIDType, fmt.Sprintf("❌ 发送卡片失败: %v", err))
		return
	}

	h.sendFeishuMessage(ctx, receiveID, receiveIDType, "✅ 卡片已发送，请点击卡片按钮测试 Jenkins 触发")
}

func (h *JKServer) sendFeishuMessage(ctx context.Context, receiveID, receiveIDType, content string) {
	if h.feishuClient == nil {
		fmt.Println("Feishu client is nil, cannot send message:", content)
		return
	}
	// 构造简单的文本消息
	msgContent := map[string]interface{}{
		"text": content,
	}
	msgBytes, _ := json.Marshal(msgContent)

	h.feishuClient.SendMessage(ctx, receiveID, receiveIDType, "text", string(msgBytes))
}
