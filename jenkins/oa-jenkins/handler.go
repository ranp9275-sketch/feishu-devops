package oajenkins

import (
	"context"
	"devops/feishu/config"
	"devops/feishu/pkg/feishu"
	"devops/feishu/pkg/feishu/groupchat"
	"devops/feishu/pkg/handler"
	"devops/jenkins"
	"devops/tools/ioc"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func init() {
	ioc.Api.RegisterContainer("jk_server", &JKServer{})
}

type JKServer struct {
	jenkins         *jenkins.Client
	feishuClient    *feishu.Client
	groupChatClient *groupchat.Client
	lastProcessedID string
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
	h.groupChatClient = groupchat.NewClient()
	if h.groupChatClient == nil {
		return fmt.Errorf("failed to initialize group chat client")
	}

	// Start Scheduler
	h.StartScheduler(context.Background())

	subr := c.Application.GinRootRouter().Group("jk")
	h.Register(subr)

	return nil
}

func (h *JKServer) Register(r *gin.RouterGroup) {
	r.POST("/test-flow", h.TestFlow)
	r.POST("/feishu/token", h.UpdateFeishuToken)
}

type UpdateTokenRequest struct {
	UserAccessToken  string `json:"user_access_token"`
	UserRefreshToken string `json:"user_refresh_token"`
}

func (h *JKServer) UpdateFeishuToken(c *gin.Context) {
	var req UpdateTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.groupChatClient != nil {
		// Default expire to 2 hours (7200 seconds)
		h.groupChatClient.UpdateTokenCache(req.UserAccessToken, req.UserRefreshToken, 7200)
		c.JSON(http.StatusOK, gin.H{"message": "Token cache updated"})
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Group chat client not initialized"})
	}
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

	if err := h.processOARequest(ctx, oaData, receiveID, receiveIDType); err != nil {
		fmt.Printf("processOARequest failed: %v\n", err)
	}
}

func (h *JKServer) processOARequest(ctx context.Context, oaData map[string]interface{}, receiveID, receiveIDType string) error {
	logReceiveID := receiveID
	logReceiveIDType := receiveIDType
	cardReceiveID := receiveID
	cardReceiveIDType := receiveIDType

	// 2. 解析 OA 数据
	dummy := &JenkinsJob{}
	jobs, err := dummy.HandleLatestJson(oaData)
	if err != nil {
		h.sendFeishuMessage(ctx, logReceiveID, logReceiveIDType, fmt.Sprintf("❌ 解析 OA 数据失败: %v", err))
		return err
	}

	if len(jobs) == 0 {
		h.sendFeishuMessage(ctx, logReceiveID, logReceiveIDType, "⚠️ OA 数据中没有找到 Job")
		return nil
	}

	// 尝试根据发起人建群
	if jobs[0].Initiator != "" {
		initiatorName := jobs[0].Initiator
		// 如果 initiatorName 是数字（OA ID），尝试用 RequestName 里的名字
		// 假设 RequestName 格式 "系统变更申请-姓名-日期"
		if len(initiatorName) > 0 && initiatorName[0] >= '0' && initiatorName[0] <= '9' {
			if jobs[0].RequestName != "" {
				parts := strings.Split(jobs[0].RequestName, "-")
				if len(parts) >= 2 {
					// 假设第二部分是名字
					possibleName := parts[1]
					// 简单的中文名字检查（可选）
					fmt.Printf("SimulateOAFlow: Initiator '%s' looks like ID, trying to use name '%s' from RequestName '%s'\n", initiatorName, possibleName, jobs[0].RequestName)
					initiatorName = possibleName
				}
			}
		}

		fmt.Printf("SimulateOAFlow: Found initiator '%s'\n", initiatorName)
		if logReceiveID != "" {
			h.sendFeishuMessage(ctx, logReceiveID, logReceiveIDType, fmt.Sprintf("🔍 正在查找发起人: %s", initiatorName))
		}

		userID, err := h.groupChatClient.GetUserIDByUsername(ctx, initiatorName)
		if err != nil {
			fmt.Printf("SimulateOAFlow: Failed to find user ID for '%s': %v\n", initiatorName, err)
			if logReceiveID != "" {
				h.sendFeishuMessage(ctx, logReceiveID, logReceiveIDType, fmt.Sprintf("⚠️ 无法找到发起人 '%s' 的 ID: %v", initiatorName, err))
			}
		} else {
			fmt.Printf("SimulateOAFlow: Found UserID '%s' for '%s'\n", userID, initiatorName)
			cardReceiveID = userID
			cardReceiveIDType = "user_id"

			reqName := jobs[0].RequestName
			if reqName == "" {
				reqName = "OA Release"
			}
			groupName := fmt.Sprintf("🚀 发布群 - %s", reqName)
			desc := fmt.Sprintf("OA发布申请: %s\n发起人: %s", reqName, initiatorName)

			// 尝试在群里查找已存在的群
			// UUID 用于去重，但为了避免频繁建群，我们可以先不传 UUID，依靠群名或其他逻辑判断
			// 不过 CreateGroupChat 接口如果有 UUID 会自动幂等
			// 使用 RequestID 或类似的作为 UUID

			// 提取 RequestID 用于去重 (假设 jobs[0].RequestName 是唯一的，或者用 OA ID)
			// 这里我们用 jobs[0].RequestName 作为基础，如果能拿到 OA ID 更好
			uniqueKey := jobs[0].RequestID
			if uniqueKey == "" {
				// Fallback to RequestName if ID is missing
				uniqueKey = jobs[0].RequestName
			}
			if uniqueKey == "" {
				uniqueKey = fmt.Sprintf("%d", time.Now().UnixNano())
			}

			// groupchat.NewCreateGroupChatRequest 参数顺序: userIDType, uuid, name, description, userIDs
			// 将 uniqueKey 作为 uuid 传入
			createReq := groupchat.NewCreateGroupChatRequest("user_id", uniqueKey, groupName, desc, []string{userID})

			chatID, err := h.groupChatClient.CreateGroupChat(ctx, userID, createReq)
			if err != nil {
				fmt.Printf("SimulateOAFlow: Failed to create group: %v\n", err)
				if logReceiveID != "" {
					h.sendFeishuMessage(ctx, logReceiveID, logReceiveIDType, fmt.Sprintf("❌ 创建群失败: %v", err))
				}
			} else {
				fmt.Printf("SimulateOAFlow: Group created successfully. ChatID: %s\n", chatID)
				cardReceiveID = chatID
				cardReceiveIDType = "chat_id"
				h.sendFeishuMessage(ctx, chatID, "chat_id", fmt.Sprintf("✅ 群已创建，欢迎 %s", initiatorName))
			}
		}
	} else {
		fmt.Println("SimulateOAFlow: No initiator found in job")
	}

	// 3. 构建 CardRequest
	// 如果 receiveID 为空（自动触发且没建群），则无法发送卡片
	if cardReceiveID == "" {
		fmt.Println("SimulateOAFlow: Warning - receiveID is empty. Cannot send Feishu card.")
		// 仍然继续，以便保存到 GlobalStore 供调试？或者直接返回？
		// 我们可以保存，但无法发送
		return nil
	}

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
		ReceiveID:     cardReceiveID,
		ReceiveIDType: cardReceiveIDType,
	}

	// 4. 保存到 GlobalStore (这一步对于回调处理是必须的)
	handler.GlobalStore.Save(requestID, cardReq)

	// 5. 构建并发送卡片
	if cardReceiveID == "" {
		fmt.Printf("Info: receiveID is empty, cannot send card. (Initiator not found or group creation failed)\n")
		// 如果因为没建群导致发不了卡片，是否应该算处理成功？
		// 如果算失败，会一直重试；如果算成功，则静默忽略
		// 建议：如果是因为找不到人或建群失败，视为“已处理但失败”，不再重试
		return nil
	}
	cardContent := handler.BuildCard(cardReq, requestID, nil, nil)
	cardBytes, _ := json.Marshal(cardContent)

	err = h.feishuClient.SendMessage(ctx, cardReceiveID, cardReceiveIDType, "interactive", string(cardBytes))
	if err != nil {
		h.sendFeishuMessage(ctx, logReceiveID, logReceiveIDType, fmt.Sprintf("❌ 发送卡片失败: %v", err))
		// 如果发送失败，返回 nil 以防止无限重试（特别是在群已解散等不可恢复的场景下）。
		// 这样会标记请求为 processed，停止骚扰用户。
		fmt.Printf("Error sending card: %v. Marking as processed to avoid loops.\n", err)
		return nil
	}

	h.sendFeishuMessage(ctx, logReceiveID, logReceiveIDType, "✅ 卡片已发送，请点击卡片按钮测试 Jenkins 触发")
	// 如果发送成功，返回 nil 以触发重试机制。

	return nil
}

func (h *JKServer) sendFeishuMessage(ctx context.Context, receiveID, receiveIDType, content string) {
	if h.feishuClient == nil {
		fmt.Println("Feishu client is nil, cannot send message:", content)
		return
	}
	if receiveID == "" {
		fmt.Printf("Info: receiveID is empty, skipping Feishu message: %s\n", content)
		return
	}
	// 构造简单的文本消息
	msgContent := map[string]interface{}{
		"text": content,
	}
	msgBytes, _ := json.Marshal(msgContent)

	h.feishuClient.SendMessage(ctx, receiveID, receiveIDType, "text", string(msgBytes))
}
