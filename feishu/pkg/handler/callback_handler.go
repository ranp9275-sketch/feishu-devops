package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"devops/feishu/pkg/feishu"
	"devops/jenkins"

	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
)

var GlobalClient *feishu.Client

// InitCallbackHandler 初始化回调处理器
func InitCallbackHandler(client *feishu.Client) {
	GlobalClient = client
	feishu.SetCardActionHandler(handleCardAction)

}

func handleCardAction(ctx context.Context, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {

	// 1. 解析 action value
	// 注意：SDK 解析后的 Value 是 interface{}，通常是 map[string]interface{}
	action := event.Event.Action
	if action == nil || action.Value == nil {
		return toast("无效的操作数据"), nil
	}

	valueMap := action.Value
	requestID, _ := valueMap["request_id"].(string)
	serviceName, _ := valueMap["service"].(string)
	actionName, _ := valueMap["action"].(string)
	branch, _ := valueMap["branch"].(string)

	if requestID == "" {
		// 如果没有 requestID，可能是旧卡片或者未适配的卡片，直接返回成功但不处理
		return toast("无法获取请求ID，请重试"), nil
	}
	// 2. 检查是否重复点击
	if GlobalStore.IsActionDisabled(requestID, serviceName, actionName) {
		return toast("该操作已执行，请勿重复点击"), nil
	}

	// 3. 标记为已执行 (除了重启操作，重启允许重复执行)
	// 记录点击次数（排除批量操作和回滚操作）
	if actionName != "batch_release_all" && actionName != "stop_batch_release" && actionName != "do_rollback" {
		GlobalStore.IncrementActionCount(requestID, serviceName, actionName)
	}

	// 修改逻辑：正式发布也像灰度发布一样，允许重复点击，不立即禁用按钮
	if actionName != "do_restart" && actionName != "do_gray_release" && actionName != "batch_release_all" && actionName != "do_official_release" {
		GlobalStore.MarkActionDisabled(requestID, serviceName, actionName)
	}
	switch actionName {
	case "do_gray_release":
		// 3. 执行灰度发布操作
		fmt.Printf("Triggering Gray Release: %s, %s\n", serviceName, branch)
		go triggerAndMonitorBuild(context.Background(), serviceName, branch, "Gray", requestID)
	case "do_official_release":
		// 3. 执行正式发布操作
		fmt.Printf("Triggering Official Release: %s, %s\n", serviceName, branch)
		// 显式增加正式发布计数 (上面统一逻辑已处理，这里移除)
		// GlobalStore.IncrementActionCount(requestID, serviceName, actionName)
		go triggerAndMonitorBuild(context.Background(), serviceName, branch, "Deploy", requestID)
	case "do_rollback":
		// 3. 执行回滚操作
		fmt.Printf("Triggering Rollback: %s, %s\n", serviceName, branch)
		go triggerAndMonitorBuild(context.Background(), serviceName, branch, "Rollback", requestID)
	case "do_restart":
		// 3. 执行重启操作
		fmt.Printf("Triggering Restart: %s, %s\n", serviceName, branch)
		go triggerAndMonitorBuild(context.Background(), serviceName, branch, "Restart", requestID)
	}

	// 同时，如果点击了其中一个批量按钮，另一个批量按钮也应该被禁用
	if actionName == "batch_release_all" || actionName == "stop_batch_release" {
		// 获取携带的分支信息
		branchMap := make(map[string]string)
		if branches, ok := valueMap["all_branches"]; ok {
			// fmt.Printf("Batch Action '%s' carried branches: %v\n", actionName, branches)
			if bm, ok := branches.(map[string]interface{}); ok {
				for k, v := range bm {
					if s, ok := v.(string); ok {
						branchMap[k] = s
					}
				}
			}
		}

		switch actionName {
		case "batch_release_all":
			// 3. 执行批量灰度发布操作
			fmt.Println("--------------------------------------------------------------")
			fmt.Printf("BatchReleaseService(ctx, branches=%v)\n", branchMap)

			// 获取请求数据以判断发布类型
			reqData, ok := GlobalStore.Get(requestID)
			if !ok {
				fmt.Printf("Error: RequestID %s not found\n", requestID)
				return toast("请求数据不存在"), nil
			}

			for svc, br := range branchMap {
				deployType := "Deploy" // 默认为正式发布

				// 查找服务定义
				var targetService *Service
				for _, s := range reqData.OriginalRequest.Services {
					if s.Name == svc {
						targetService = &s
						break
					}
				}

				if targetService != nil {
					// 检查是否包含灰度动作
					for _, act := range targetService.Actions {
						if strings.EqualFold(act, "gray") || act == "灰度" {
							deployType = "Gray"
							break
						}
					}
				}

				fmt.Printf("Batch triggering %s for %s (Branch: %s)\n", deployType, svc, br)
				go triggerAndMonitorBuild(context.Background(), svc, br, deployType, requestID)
			}

		case "stop_batch_release":
			// 3. 执行批量结束灰度发布操作
			fmt.Println("--------------------------------------------------------------")
			fmt.Printf("StopBatchReleaseService(ctx, branches=%v)\n", branchMap)
			// 发送新的卡片，把灰度发布按钮改成正式发布按钮
			// 同时过滤掉已经完成正式发布的服务
			if reqData, ok := GlobalStore.Get(requestID); ok && GlobalClient != nil {
				// 1. 创建新请求ID
				newrequestID := fmt.Sprintf("req_%d", time.Now().UnixNano())

				// 2. 复制并修改数据
				newCardReq := reqData.OriginalRequest
				// newServices := make([]Service, len(reqData.OriginalRequest.Services)) // 不能直接make len，因为可能过滤
				var filteredServices []Service

				for _, s := range reqData.OriginalRequest.Services {
					// 检查该服务是否已经完成了正式发布
					isOfficialDone := false
					if reqData.ActionCounts != nil {
						if count, ok := reqData.ActionCounts[s.Name+":do_official_release"]; ok && count > 0 {
							isOfficialDone = true
						}
					}

					// 如果已经完成正式发布，则不添加到新卡片中
					if isOfficialDone {
						continue
					}

					// Deep copy service
					newService := s
					actions := make([]string, len(s.Actions))
					copy(actions, s.Actions)
					newService.Actions = actions
					branches := make([]string, len(s.Branches))
					copy(branches, s.Branches)
					newService.Branches = branches

					filteredServices = append(filteredServices, newService)
				}
				newCardReq.Services = filteredServices

				updated := false
				// 如果所有服务都过滤掉了，就不发送新卡片了？或者发送一个空的提示？
				// 这里假设至少有一个服务需要处理，或者如果为空则updated=false自然不发送（需检查逻辑）
				if len(newCardReq.Services) > 0 {
					updated = true // 至少有服务存在，可能需要发送
				} else {
					// 全部完成了，直接提示？
					updated = false
				}

				if len(newCardReq.Services) > 0 {
					for i, s := range newCardReq.Services {
						newActions := []string{}
						seenOfficial := false // 用于去重 official

						for _, a := range s.Actions {
							if strings.EqualFold(a, "gray") || a == "灰度" {
								if !seenOfficial {
									newActions = append(newActions, "official")
									seenOfficial = true
								}
							} else if strings.EqualFold(a, "official") || strings.EqualFold(a, "release") || a == "正式" {
								if !seenOfficial {
									newActions = append(newActions, "official")
									seenOfficial = true
								}
							} else {
								newActions = append(newActions, a)
							}
						}
						newCardReq.Services[i].Actions = newActions
					}

					// 只要有服务保留下来，我们就认为需要更新发送
					updated = true
				}

				if updated {
					// 3. 保存新请求
					GlobalStore.Save(newrequestID, newCardReq)

					// 4. 构建并发送新卡片
					if newCardReq.ReceiveID != "" && newCardReq.ReceiveIDType != "" {
						cardContent := BuildCard(newCardReq, newrequestID, nil, nil)
						cardBytes, _ := json.Marshal(cardContent)
						GlobalClient.SendMessage(ctx, newCardReq.ReceiveID, newCardReq.ReceiveIDType, "interactive", string(cardBytes))
					}
				}
			}

		}

		// 1. 禁用另一个批量按钮 (互斥)
		if actionName == "stop_batch_release" {
			GlobalStore.MarkActionDisabled(requestID, serviceName, "batch_release_all")
		}
		// 注意：如果 actionName 是 batch_release_all，我们不应该禁用它自己，因为我们要允许重复点击增加计数
		// 之前的代码: if actionName == "stop_batch_release" || actionName == "batch_release_all" { ... }
		// 这会导致 batch_release_all 被禁用，从而阻止后续点击 //else {
		// 	GlobalStore.MarkActionDisabled(requestID, serviceName, "stop_batch_release")
		// }

		// 2. 禁用所有子服务的按钮
		var serverList map[string]string
		if reqData, ok := GlobalStore.Get(requestID); ok {
			for _, service := range reqData.OriginalRequest.Services {
				// 当 actionName == "stop_batch_release" 时，禁用所有按钮
				if actionName == "stop_batch_release" {
					// 遍历该服务的所有可能动作并禁用
					// 我们需要将配置中的动作名映射回按钮的 action value (例如 "gray" -> "do_gray_release")
					actionsToDisable := []string{"do_rollback", "do_restart", "do_gray_release"} // 默认总是包含这两个，并且禁用灰度

					for _, act := range service.Actions {
						var valueAction string
						switch strings.ToLower(act) {
						case "gray", "灰度":
							valueAction = "do_gray_release"
						case "official", "release", "正式":
							valueAction = "do_official_release"
							//continue // 不要禁用正式发布按钮
						case "check", "验收":
							valueAction = "do_check"
						case "rollback", "回滚":
							continue // 已经在默认列表中
						case "restart", "重启":
							continue // 已经在默认列表中
						default:
							valueAction = "do_" + act
						}
						actionsToDisable = append(actionsToDisable, valueAction)
					}

					for _, act := range actionsToDisable {
						GlobalStore.MarkActionDisabled(requestID, service.Name, act)
					}
				} else {
					// 批量发布时：
					// 1. 禁用灰度发布按钮?
					// 2. 增加灰度发布计数
					GlobalStore.IncrementActionCount(requestID, service.Name, "do_gray_release")
					if strings.EqualFold(actionName, "official") || strings.EqualFold(actionName, "release") || actionName == "正式" {
						GlobalStore.IncrementActionCount(requestID, service.Name, "do_official_release")
					}
				}

				if serverList == nil {
					serverList = make(map[string]string)
				}
				if len(service.Branches) > 0 {
					serverList[service.Name] = service.Branches[0]
				}
			}
		}
	}

	// 4. 获取原始请求数据并重新构建卡片
	storedReq, exists := GlobalStore.Get(requestID)

	if !exists {
		return toast("请求数据已过期或不存在"), nil
	}

	// 检查是否需要过滤显示（灰度模式）
	// 原始请求包含灰度服务，我们需要保持灰度视图（隐藏正式发布按钮）
	displayRequest := storedReq.OriginalRequest
	hasGray := false
	for _, s := range displayRequest.Services {
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
		var filteredServices []Service
		for _, s := range displayRequest.Services {
			hasGrayAction := false
			for _, a := range s.Actions {
				if strings.EqualFold(a, "gray") || a == "灰度" {
					hasGrayAction = true
					break
				}
			}

			if hasGrayAction {
				newService := s
				newActions := []string{}
				for _, a := range s.Actions {
					if strings.EqualFold(a, "official") || strings.EqualFold(a, "release") || a == "正式" {
						continue
					}
					newActions = append(newActions, a)
				}
				newService.Actions = newActions
				filteredServices = append(filteredServices, newService)
			}
		}
		displayRequest.Services = filteredServices
	}

	// 重新构建卡片（按钮会被禁用）
	// 注意：这里需要传入最新的 disabledActions，已经在 Store 中更新了
	// Store.Get 返回的是指针，所以 MarkActionDisabled 修改的是同一个对象
	// BuildGrayCard 会读取这个 map
	newCard := BuildCard(displayRequest, requestID, storedReq.DisabledActions, storedReq.ActionCounts)

	// 如果是结束发布，删除持久化文件（清理数据）
	// if actionName == "stop_batch_release" {
	// 	GlobalStore.Delete(requestID)
	// }

	// 5. 返回更新后的卡片
	// Card 字段在 SDK 中通常定义为 interface{}，可以直接传入 map
	return &callback.CardActionTriggerResponse{
		Toast: &callback.Toast{
			Type:    "success",
			Content: "操作成功",
		},
		Card: &callback.Card{
			Type: "raw",
			Data: newCard,
		},
	}, nil
}

func toast(msg string) *callback.CardActionTriggerResponse {
	return &callback.CardActionTriggerResponse{
		Toast: &callback.Toast{
			Type:    "info",
			Content: msg,
		},
	}
}

// triggerAndMonitorBuild 触发 Jenkins 构建并监控直到完成
func triggerAndMonitorBuild(ctx context.Context, jobName, branch, deployType, requestID string) {
	// 获取发送消息的 ID
	var receiveID, receiveIDType string
	if reqData, ok := GlobalStore.Get(requestID); ok {
		receiveID = reqData.OriginalRequest.ReceiveID
		receiveIDType = reqData.OriginalRequest.ReceiveIDType
	} else {
		fmt.Printf("Error: RequestID %s not found in store, cannot send notifications\n", requestID)
		return
	}

	client := jenkins.NewClient()
	if client == nil {
		sendFeishuMessage(ctx, receiveID, receiveIDType, fmt.Sprintf("❌ Jenkins 初始化失败: %s", jobName))
		return
	}

	req := jenkins.BuildRequest{
		JobName:    jobName,
		Branch:     branch,
		DeployType: deployType,
	}

	// 触发构建
	queueID, err := client.Build(ctx, req)
	if err != nil {
		sendFeishuMessage(ctx, receiveID, receiveIDType, fmt.Sprintf("❌ 构建触发失败: %s\nBranch: %s\nType: %s\nError: %v", jobName, branch, deployType, err))
		return
	}

	sendFeishuMessage(ctx, receiveID, receiveIDType, fmt.Sprintf("⏳ 正在排队: %s\nBranch: %s\nType: %s\nQueueID: %d", jobName, branch, deployType, queueID))

	// 等待构建开始
	buildNum, err := client.WaitForBuildToStart(ctx, queueID)
	if err != nil {
		sendFeishuMessage(ctx, receiveID, receiveIDType, fmt.Sprintf("❌ 等待构建开始超时: %s\nQueueID: %d\nError: %v", jobName, queueID, err))
		return
	}

	sendFeishuMessage(ctx, receiveID, receiveIDType, fmt.Sprintf("🚀 构建已开始: %s #%d\nBranch: %s\nType: %s", jobName, buildNum, branch, deployType))

	// 监控构建
	build, err := client.MonitorBuildUntilCompletion(ctx, jobName, buildNum)
	if err != nil {
		sendFeishuMessage(ctx, receiveID, receiveIDType, fmt.Sprintf("❌ 监控构建出错: %s #%d\nError: %v", jobName, buildNum, err))
		return
	}

	result := build.GetResult()
	duration := build.Raw.Duration / 1000 // ms -> s

	if result == "SUCCESS" {
		sendFeishuMessage(ctx, receiveID, receiveIDType, fmt.Sprintf("✅ 构建成功: %s #%d\nBranch: %s\nType: %s\nDuration: %ds", jobName, buildNum, branch, deployType, int64(duration)))
	} else {
		sendFeishuMessage(ctx, receiveID, receiveIDType, fmt.Sprintf("❌ 构建失败: %s #%d\nBranch: %s\nType: %s\nResult: %s", jobName, buildNum, branch, deployType, result))
	}
}

func sendFeishuMessage(ctx context.Context, receiveID, receiveIDType, content string) {
	if GlobalClient == nil {
		fmt.Println("GlobalClient is nil, cannot send message:", content)
		return
	}
	// 构造简单的文本消息
	msgContent := map[string]interface{}{
		"text": content,
	}
	msgBytes, _ := json.Marshal(msgContent)

	err := GlobalClient.SendMessage(ctx, receiveID, receiveIDType, "text", string(msgBytes))
	if err != nil {
		fmt.Printf("Failed to send Feishu message: %v\n", err)
	}
}
