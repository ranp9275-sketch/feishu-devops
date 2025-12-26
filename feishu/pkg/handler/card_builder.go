package handler

import (
	log "devops/tools/logger"
	"fmt"
	"strings"
)

// BuildGrayCard 构建灰度发布卡片
// 使用 V1 Message Card 格式以支持 action 模块的多组件布局
// requestID: 用于追踪卡片交互状态的唯一ID
// disabledActions: 已禁用的动作集合，key为 "serviceName:action"
func BuildCard(req GrayCardRequest, requestID string, disabledActions map[string]bool, actionCounts map[string]int) map[string]interface{} {
	Logger := log.NewLogger("ERROR")
	// 检查 Services 是否为空
	if len(req.Services) == 0 {
		Logger.Error("Services list is empty")
		return nil
	}
	// 检查 ObjectID 是否为空
	if req.Services[0].ObjectID == "" {
		Logger.Error("ObjectID is empty")
		return nil
	}
	// 检查 Actions 是否为空
	if len(req.Services[0].Actions) == 0 {
		Logger.Error("Actions list is empty")
		return nil
	}

	// 检查 Branches 是否为空
	if len(req.Services[0].Branches) == 0 {
		Logger.Error("Branches list is empty")
		return nil
	}

	//服务名称
	req.ObjectID = req.Services[0].ObjectID

	req.Title = fmt.Sprintf("🚀%s-服务发布通知", req.ObjectID)

	elements := []interface{}{
		map[string]interface{}{
			"tag": "div",
			"text": map[string]interface{}{
				"tag": "lark_md",
				//"content": "**服务发布通知**",
				//"content": req.Services[0].ObjectID,
			},
		},
		map[string]interface{}{
			"tag": "hr",
		},
		map[string]interface{}{
			"tag": "div",
			"text": map[string]interface{}{
				"tag":     "lark_md",
				"content": "📋 **服务列表与操作**",
			},
		},
	}

	for i, service := range req.Services {
		// 1. 服务名称行
		elements = append(elements, map[string]interface{}{
			"tag": "div",
			"text": map[string]interface{}{
				"tag":     "lark_md",
				"content": fmt.Sprintf("**%d. 服务名称：** `%s`", i+1, service.Name),
			},
		})

		// 2. 操作行（显示分支 + 动作按钮）
		// 显示传入的分支信息（不可选择）
		var branchDisplay string
		if len(service.Branches) == 0 {
			branchDisplay = "无分支"
			Logger.Error(fmt.Sprintf("Service %s has no branches", service.Name))
		} else {
			// 显示第一个分支（如果有多个分支，可以按需调整）
			branchDisplay = service.Branches[0]
		}

		// 添加分支显示（在action外部）
		elements = append(elements, map[string]interface{}{
			"tag": "div",
			"text": map[string]interface{}{
				"tag":     "lark_md",
				"content": fmt.Sprintf("📦 **发布分支：** `%s`", branchDisplay),
			},
		})

		// 构建操作区（只包含按钮）
		actionsList := []interface{}{}

		// 根据 Actions 列表生成按钮
		// 创建一个新的切片，避免修改原始数据
		// 过滤掉验收功能 (check/验收)
		var currentActions []string
		hasRollback := false
		hasRestart := false

		for _, a := range service.Actions {
			if strings.EqualFold(a, "check") || strings.EqualFold(a, "验收") {
				continue
			}
			if strings.EqualFold(a, "rollback") || strings.EqualFold(a, "回滚") {
				hasRollback = true
			}
			if strings.EqualFold(a, "restart") || strings.EqualFold(a, "重启") {
				hasRestart = true
			}
			currentActions = append(currentActions, a)
		}

		if !hasRollback {
			currentActions = append(currentActions, "rollback")
		}
		if !hasRestart {
			currentActions = append(currentActions, "restart")
		}

		for _, action := range currentActions {
			var text string
			var valueAction string
			var btnType string = "primary"

			switch strings.ToLower(action) {
			case "gray", "灰度":
				text = "🚀 灰度"
				valueAction = "do_gray_release"
			case "official", "release", "正式":
				text = "🎉 正式"
				valueAction = "do_official_release"
				btnType = "danger" // 正式发布可能需要警示色
			case "rollback", "回滚":
				text = "🔙 回滚"
				valueAction = "do_rollback"
				btnType = "danger"
			case "restart", "重启":
				text = "🔄 重启"
				valueAction = "do_restart"
				btnType = "primary"

			default:
				text = action
				valueAction = "do_" + action
			}

			// 检查是否禁用
			isDisabled := false
			count := 0
			key := fmt.Sprintf("%s:%s", service.Name, valueAction)

			if disabledActions != nil {
				if disabledActions[key] {
					isDisabled = true
					btnType = "default"
				}
			}

			if actionCounts != nil {
				count = actionCounts[key]
			}

			// 根据动作类型和计数更新文本
			// 统一逻辑：如果 count > 0，则显示计数
			if count > 0 {
				text = fmt.Sprintf("%s (%d)", text, count)
			}

			// 构建按钮（包含确认对话框和防重复点击）
			button := map[string]interface{}{
				"tag": "button",
				"text": map[string]interface{}{
					"tag":     "plain_text",
					"content": text,
				},
				"type":     btnType,
				"disabled": isDisabled,
				"value": map[string]interface{}{
					"action":     valueAction,
					"service":    service.Name,
					"request_id": requestID,
					"branch":     branchDisplay,
				},
				"confirm": map[string]interface{}{
					"title": map[string]interface{}{
						"tag":     "plain_text",
						"content": "是否确认？",
					},
					"ok_text": map[string]interface{}{
						"tag":     "plain_text",
						"content": "确认",
					},
					"cancel_text": map[string]interface{}{
						"tag":     "plain_text",
						"content": "取消",
					},
				},
			}
			actionsList = append(actionsList, button)

		}

		actionElement := map[string]interface{}{
			"tag":     "action",
			"actions": actionsList,
		}
		elements = append(elements, actionElement)

		// 3. 分割线（除了最后一个）
		if i < len(req.Services)-1 {
			elements = append(elements, map[string]interface{}{
				"tag": "hr",
			})
		}
	}

	// 4. 添加批量操作按钮
	elements = append(elements, map[string]interface{}{
		"tag": "hr",
	})
	elements = append(elements, map[string]interface{}{
		"tag": "div",
		"text": map[string]interface{}{
			"tag":     "lark_md",
			"content": "⚡ **批量操作**",
		},
	})

	// 收集所有服务的发布分支信息
	allBranches := make(map[string]string)
	for _, svc := range req.Services {
		if len(svc.Branches) > 0 {
			allBranches[svc.Name] = svc.Branches[0]
		}
	}

	// 批量发布按钮
	batchActions := []interface{}{}

	// 定义批量按钮配置
	batchBtns := []struct {
		Text   string
		Type   string
		Action string
	}{
		{Text: "🚀 批量发布", Type: "primary", Action: "batch_release_all"},
		{Text: "⏹️ 结束批量发布", Type: "danger", Action: "stop_batch_release"},
	}

	for _, btn := range batchBtns {
		text := btn.Text
		btnType := btn.Type
		isDisabled := false

		// 检查是否禁用 (使用 "BATCH" 作为特殊的 service name)
		if disabledActions != nil {
			key := fmt.Sprintf("BATCH:%s", btn.Action)
			if disabledActions[key] {
				isDisabled = true
				text = text + " (已执行)"
				btnType = "default"
			}
		}

		batchActions = append(batchActions, map[string]interface{}{
			"tag": "button",
			"text": map[string]interface{}{
				"tag":     "plain_text",
				"content": text,
			},
			"type":     btnType,
			"disabled": isDisabled,
			"value": map[string]interface{}{
				"action":       btn.Action,
				"service":      "BATCH",
				"request_id":   requestID,
				"all_branches": allBranches,
			},
			"confirm": map[string]interface{}{
				"title": map[string]interface{}{
					"tag":     "plain_text",
					"content": fmt.Sprintf("是否确认%s所有服务？", strings.TrimPrefix(strings.TrimPrefix(btn.Text, "🚀 "), "⏹️ ")),
				},
				"ok_text": map[string]interface{}{
					"tag":     "plain_text",
					"content": "确认",
				},
				"cancel_text": map[string]interface{}{
					"tag":     "plain_text",
					"content": "取消",
				},
			},
		})
	}

	elements = append(elements, map[string]interface{}{
		"tag":     "action",
		"actions": batchActions,
	})

	return map[string]interface{}{
		"header": map[string]interface{}{
			"title": map[string]interface{}{
				"content": req.Title,
				"tag":     "plain_text",
			},
			"template": "blue",
		},
		"elements": elements,
	}
}
