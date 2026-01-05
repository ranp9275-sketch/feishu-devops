package oajenkins

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Add mutex for locking
var processMutex sync.Mutex

// StartScheduler 启动定时任务
func (h *JKServer) StartScheduler(ctx context.Context) {
	fmt.Println("Starting OA Scheduler...")

	// 1 minute ticker for OA Data
	oaTicker := time.NewTicker(1 * time.Minute)
	// 1 hour ticker for Token Refresh
	tokenTicker := time.NewTicker(1 * time.Hour)

	go func() {
		// Run once immediately
		h.checkOAData(ctx)
		h.refreshToken(ctx)

		for {
			select {
			case <-ctx.Done():
				fmt.Println("Scheduler stopped")
				oaTicker.Stop()
				tokenTicker.Stop()
				return
			case <-oaTicker.C:
				h.checkOAData(ctx)
			case <-tokenTicker.C:
				h.refreshToken(ctx)
			}
		}
	}()
}

func (h *JKServer) checkOAData(ctx context.Context) {
	// Try to acquire lock
	if !processMutex.TryLock() {
		fmt.Println("Scheduler: Previous checkOAData still running, skipping this round.")
		return
	}
	defer processMutex.Unlock()

	// 获取所有未处理的请求
	reqs, err := GetUnprocessedRequests()
	if err != nil {
		fmt.Printf("Scheduler: Failed to get unprocessed OA requests: %v\n", err)
		return
	}

	if len(reqs) > 0 {
		fmt.Printf("Scheduler: Found %d unprocessed requests\n", len(reqs))
	}

	for _, req := range reqs {
		// Extract ID for marking as processed later
		id, _ := req["id"].(string)
		if id == "" {
			// Fallback if ID is missing (should not happen with DB)
			if t, ok := req["received_at"].(string); ok {
				id = t
			}
		}

		// Double check processing status inside loop to prevent race conditions
		// if multiple instances were running (though mutex handles single instance)
		// For distributed locking, we would need Redis/DB lock here.

		fmt.Printf("Scheduler: Processing request %s...\n", id)

		// 触发流程
		err := h.processOARequest(ctx, req, "", "")
		if err != nil {
			fmt.Printf("Scheduler: Failed to process request %s: %v\n", id, err)
			// Continue to next request. This one remains unprocessed in DB.
			continue
		}

		if id != "" {
			if err := MarkRequestAsProcessed(id); err != nil {
				fmt.Printf("Scheduler: Failed to mark request %s as processed: %v\n", id, err)
			} else {
				fmt.Printf("Scheduler: Request %s processed and marked.\n", id)
			}
		}
	}
}

func (h *JKServer) refreshToken(ctx context.Context) {
	fmt.Println("Scheduler: Refreshing tokens...")

	// Refresh Tenant Token (Client 内部会自动判断是否过期，这里强制调用一次以确保活跃)
	// feishu.Client 没有直接暴露 Refresh 方法，但 GetTenantAccessToken 会处理
	// 我们需要访问 h.feishuClient (它是 *feishu.Client 类型)
	// 但 feishu.Client 的方法需要 check 它的定义
	// 查看 feishu/client.go, GetTenantAccessToken 是私有的吗？不，是大写的。
	// 但它是 *Client 的方法。

	// 这里我们实际上想刷新 groupChatClient 的 User Token
	if h.groupChatClient != nil {
		_, err := h.groupChatClient.GetAndRefreshUserToken(ctx)
		if err != nil {
			fmt.Printf("Scheduler: Failed to refresh user token: %v\n", err)
		} else {
			fmt.Println("Scheduler: User token refreshed successfully")
		}
	}
}
