package oafeishujenkins

import (
	"context"
	c "devops/feishu/config"
	feishu "devops/feishu/pkg/feishu"
	h "devops/feishu/pkg/handler"
	oajenkins "devops/jenkins/oa-jenkins"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Define variables to allow mocking in tests
var (
	loadConfigFunc = c.LoadConfig
	newSenderFunc  = func(cfg *c.Config) feishu.Sender {
		return feishu.NewAPISender(feishu.NewClient(cfg))
	}
)

func SendCard(ctx context.Context, receive_id, receive_id_type string, jobs []*oajenkins.JenkinsJob) error {
	// 发送卡片的逻辑
	// 生成唯一请求ID
	var req h.SendGrayCardRequest
	cfg, err := loadConfigFunc()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	sender := newSenderFunc(cfg)

	requestID := fmt.Sprintf("req_%d", time.Now().UnixNano())

	req.CardData.ReceiveID = receive_id
	req.CardData.ReceiveIDType = receive_id_type

	// 填充 CardData
	services := make([]h.Service, 0, len(jobs))
	for _, job := range jobs {
		services = append(services, h.Service{
			Name:     job.JobName,
			ObjectID: job.JobName,
			Branches: []string{job.JobBranch},
			Actions:  []string{"check", "gray", "official"},
		})
	}
	req.CardData.Services = services

	// 保存请求数据以便回调使用
	h.GlobalStore.Save(requestID, req.CardData)

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
		var filteredServices []h.Service
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

	cardContent := h.BuildCard(displayCardData, requestID, nil, nil)
	// 2. 序列化为 JSON 字符串
	cardBytes, err := json.Marshal(cardContent)
	if err != nil {
		return err
	}

	err = sender.Send(ctx, receive_id, receive_id_type, "interactive", string(cardBytes))
	if err != nil {
		return err
	}
	return nil
}
