package feishu

import (
	"context"
	cfg "devops/feishu/config"
	"testing"
	"time"
)

func TestGetTenantAccessToken_CachedConcurrent(t *testing.T) {
	c := NewClient(&cfg.Config{FeishuAppID: "", FeishuAppSecret: "", LogLevel: "debug", MaxIdleConns: 10, MaxIdleConnsPerHost: 5, IdleConnTimeout: time.Second * 60})
	c.mu.Lock()
	c.tenantToken = "token123"
	c.tokenExpireAt = time.Now().Add(10 * time.Minute)
	c.mu.Unlock()

	ctx := context.Background()
	ch := make(chan error, 20)
	for i := 0; i < 20; i++ {
		go func() {
			_, err := c.getTenantAccessToken(ctx)
			ch <- err
		}()
	}
	for i := 0; i < 20; i++ {
		if err := <-ch; err != nil {
			t.Fatalf("expected cached token without error, got %v", err)
		}
	}
}
