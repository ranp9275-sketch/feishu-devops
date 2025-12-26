package handler

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestBuildGrayCard(t *testing.T) {
	services := []Service{
		{
			Name:     "service-order-center",
			ObjectID: "service-order-center",
			Branches: []string{"master", "feature/gray-v1", "fix/bug-001"},
			Actions:  []string{"gray", "official"},
		},
		{
			Name:     "service-user-center",
			ObjectID: "service-user-center",
			Branches: []string{"master", "feature/user-opt"},
			Actions:  []string{"gray"},
		},
		{
			Name:     "service-payment",
			ObjectID: "service-payment",
			Branches: []string{"master", "hotfix/pay-error"},
			Actions:  []string{"check", "release"}, // release = official
		},
	}

	req := GrayCardRequest{
		Title:    "🚀 批量灰度发布",
		Services: services,
	}

	card := BuildCard(req, "test-req-id", nil, nil)

	// 验证基本结构
	if card["header"] == nil {
		t.Error("header is missing")
	}
	elements, ok := card["elements"].([]interface{})
	if !ok {
		t.Error("elements is missing or invalid type")
	}

	// 验证元素数量：
	// 初始: div(服务发布通知) + hr + div(服务列表) = 3
	// 每个服务: div(名称) + div(分支显示) + action(操作按钮) = 3
	// 分割线: n-1 个
	// 批量操作: hr + div(批量操作) + action(批量按钮) = 3
	// 总数 = 3 + 3*3 + 2 + 3 = 17
	expectedCount := 3 + len(services)*3 + (len(services) - 1) + 3
	if len(elements) != expectedCount {
		t.Errorf("expected %d elements, got %d", expectedCount, len(elements))
	}

	// 打印 JSON 以供人工核对
	b, _ := json.MarshalIndent(card, "", "  ")
	fmt.Printf("Generated Card JSON:\n%s\n", string(b))
}

func TestCheckButtonFiltered(t *testing.T) {
	reqID := "test-check-filtered"
	serviceName := "service-test"
	req := GrayCardRequest{
		Services: []Service{
			{
				Name:     serviceName,
				ObjectID: serviceName,
				Branches: []string{"master"},
				Actions:  []string{"gray", "check"}, // check requested
			},
		},
	}

	// 验证 "check" 按钮被过滤掉
	card := BuildCard(req, reqID, nil, nil)
	checkBtn := findButton(card, "do_check")
	if checkBtn != nil {
		t.Error("Check button should be filtered out")
	}
	
	// 验证其他按钮正常
	grayBtn := findButton(card, "do_gray_release")
	if grayBtn == nil {
		t.Error("Gray button should be present")
	}
}

// Helper to find a button by action value in the card
func findButton(card map[string]interface{}, actionValue string) map[string]interface{} {
	elements, _ := card["elements"].([]interface{})
	for _, el := range elements {
		eMap, _ := el.(map[string]interface{})
		if eMap["tag"] == "action" {
			actions, _ := eMap["actions"].([]interface{})
			for _, a := range actions {
				aMap, _ := a.(map[string]interface{})
				valMap, _ := aMap["value"].(map[string]interface{})
				if valMap["action"] == actionValue {
					return aMap
				}
			}
		}
	}
	return nil
}
