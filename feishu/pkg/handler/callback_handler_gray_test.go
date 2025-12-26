package handler

import (
	"context"
	"devops/feishu/pkg/feishu"
	"encoding/json"
	"strings"
	"testing"

	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
)

func TestCallbackHandler_GrayPhase(t *testing.T) {
	// 初始化
	client := &feishu.Client{}
	InitCallbackHandler(client)

	// 测试灰度阶段过滤逻辑持久化
	t.Run("Gray Phase Persistence", func(t *testing.T) {
		reqID := "test-req-gray-phase-001"
		serviceName := "service-gray-official"

		// 准备数据：包含灰度和正式的服务
		reqData := GrayCardRequest{
			Title: "Gray Phase Test Card",
			Services: []Service{
				{
					Name:     serviceName,
					ObjectID: "obj-1",
					Branches: []string{"master"},
					Actions:  []string{"gray", "official"},
				},
			},
		}
		GlobalStore.Save(reqID, reqData)

		// 模拟点击灰度按钮
		event := &callback.CardActionTriggerEvent{
			Event: &callback.CardActionTriggerRequest{
				Action: &callback.CallBackAction{
					Value: map[string]interface{}{
						"request_id": reqID,
						"service":    serviceName,
						"action":     "do_gray_release",
					},
				},
			},
		}
		resp, _ := handleCardAction(context.Background(), event)

		if resp.Toast.Type != "success" {
			t.Errorf("Expected success toast, got %+v", resp.Toast)
		}

		// 检查返回的卡片内容
		// 卡片内容应该是过滤后的（只有灰度按钮，没有正式按钮）
		cardData, ok := resp.Card.Data.(map[string]interface{})
		if !ok {
			t.Errorf("Expected card data to be map[string]interface{}, got %T", resp.Card.Data)
			return
		}

		// 序列化后检查字符串
		cardBytes, _ := json.Marshal(cardData)
		cardJson := string(cardBytes)

		if strings.Contains(cardJson, "do_official_release") {
			t.Errorf("Card should not contain official release button in Gray Phase. Content: %s", cardJson)
		}
		if !strings.Contains(cardJson, "do_gray_release") {
			t.Errorf("Card should contain gray release button. Content: %s", cardJson)
		}

		// 验证 GlobalStore 中的 OriginalRequest 仍然包含 "official" (未被破坏)
		stored, _ := GlobalStore.Get(reqID)
		hasOfficialInStore := false
		for _, a := range stored.OriginalRequest.Services[0].Actions {
			if a == "official" {
				hasOfficialInStore = true
				break
			}
		}
		if !hasOfficialInStore {
			t.Errorf("Original request in store was modified! Official action lost.")
		}
	})

	// 测试批量结束操作后，旧卡片仍然保持灰度视图（只是按钮被禁用）
	t.Run("Stop Batch Old Card Persistence", func(t *testing.T) {
		reqID := "test-req-transition-001"
		serviceName := "service-gray-official-trans"

		// 准备数据
		reqData := GrayCardRequest{
			Title: "Transition Test Card",
			Services: []Service{
				{
					Name:     serviceName,
					ObjectID: "obj-1",
					Branches: []string{"master"},
					Actions:  []string{"gray", "official"},
				},
			},
		}
		GlobalStore.Save(reqID, reqData)

		// 模拟点击批量结束按钮
		event := &callback.CardActionTriggerEvent{
			Event: &callback.CardActionTriggerRequest{
				Action: &callback.CallBackAction{
					Value: map[string]interface{}{
						"request_id": reqID,
						"service":    "BATCH",
						"action":     "stop_batch_release",
					},
				},
			},
		}

		resp, _ := handleCardAction(context.Background(), event)

		// 检查返回的旧卡片内容
		cardData, ok := resp.Card.Data.(map[string]interface{})
		if !ok {
			t.Errorf("Expected card data to be map[string]interface{}")
			return
		}

		cardBytes, _ := json.Marshal(cardData)
		cardJson := string(cardBytes)

		// 旧卡片应该仍然不包含 official 按钮
		if strings.Contains(cardJson, "do_official_release") {
			t.Errorf("Old Card should NOT reveal official release button even after stop batch. Content: %s", cardJson)
		}
	})
}
