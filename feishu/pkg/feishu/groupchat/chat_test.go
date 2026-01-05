package groupchat

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestCreateGroupChat(t *testing.T) {
	if os.Getenv("RUN_FEISHU_INTEGRATION_TESTS") != "1" {
		t.Skip("skipping Feishu integration tests")
	}

	// 初始化 Client
	// 注意：NewClient 是作为 method 定义的，所以需要先实例化 Client
	// 这是一个 Factory 模式的变体
	cli := NewClient()
	if cli == nil {
		t.Fatalf("Failed to initialize Feishu Client")
	}

	// 打印调试信息
	t.Logf("FEISHU_APP_ID: %s", cli.Config.FeishuAppID)
	// 注意：不要打印 Secret

	t.Run("CreateGroupChat", func(t *testing.T) {
		if os.Getenv("FEISHU_TENANT_ACCESS_TOKEN") == "" && os.Getenv("FEISHU_REFRESH_TOKEN") == "" && os.Getenv("FEISHU_USER_ACCESS_TOKEN") == "" {
			t.Skip("missing Feishu credentials env vars")
		}

		// 优先从环境变量获取测试用的 OpenID
		// 608307895 是已确认可见的用户 ID
		testUserIDs := []string{"608307895"}
		if envID := os.Getenv("TEST_OPEN_ID"); envID != "" {
			testUserIDs = strings.Split(envID, ",")
		}

		// 允许从环境变量覆盖 ID 类型 (默认 user_id)
		userIDType := os.Getenv("TEST_USER_ID_TYPE")
		if userIDType == "" {
			userIDType = "user_id"
		}

		t.Logf("Testing with UserIDs: %v, Type: %s", testUserIDs, userIDType)

		// 使用随机 UUID 防止重复创建失败
		chatUUID := uuid.New().String()

		req := NewCreateGroupChatRequest(
			userIDType,
			chatUUID,
			"Test Group Chat "+chatUUID[:8], // 群名带上随机后缀
			"Description of test group",
			testUserIDs,
		)

		t.Logf("Creating group chat with UUID: %s, UserIDs: %v", chatUUID, testUserIDs)

		// 调用创建群聊函数 (使用 Tenant Token)
		chatID, err := cli.CreateGroupChat(context.Background(), "", req)
		if err != nil {
			t.Fatalf("CreateGroupChat failed: %v", err)
		}
		t.Logf("Successfully created group chat with ID: %s", chatID)

		// 测试获取群成员
		t.Logf("Getting members for group chat: %s", chatID)
		members, err := cli.GetGroupChatMembers(context.Background(), chatID)
		if err != nil {
			t.Errorf("GetGroupChatMembers failed: %v", err)
		} else {
			t.Logf("Successfully got members (Type: %T): %+v", members, members)
		}

		// 设置 User Access Token，以便 AddGroupChatMembers 使用用户身份拉人
		if os.Getenv("FEISHU_USER_ACCESS_TOKEN") == "" && os.Getenv("FEISHU_REFRESH_TOKEN") == "" {
			t.Skip("missing FEISHU_USER_ACCESS_TOKEN or FEISHU_REFRESH_TOKEN")
		}

		// 测试拉人进群
		t.Logf("Adding members to group chat: %s", chatID)
		testUserIDs = []string{"608307895", "612177649"}
		// 尝试拉取同一个用户 (幂等性测试) 或其他用户
		invalidIDs, err := cli.AddGroupChatMembers(context.Background(), chatID, testUserIDs)
		if err != nil {
			t.Errorf("AddGroupChatMembers failed: %v", err)
		} else {
			if len(invalidIDs) > 0 {
				t.Logf("AddGroupChatMembers partially succeeded. Invalid IDs: %v", invalidIDs)
			} else {
				t.Logf("AddGroupChatMembers succeeded.")
			}
		}
	})

	// 机器人应用(Tenant Token)无法调用搜索用户接口(99991663错误)
	// 或者 Contact.V3.User.List 需要开通 '获取用户姓名' 权限才能匹配名字
	t.Run("SearchUser", func(t *testing.T) {
		if os.Getenv("FEISHU_USER_ACCESS_TOKEN") == "" && os.Getenv("FEISHU_REFRESH_TOKEN") == "" {
			t.Skip("missing FEISHU_USER_ACCESS_TOKEN or FEISHU_REFRESH_TOKEN")
		}

		// Postman 测试用例: "张嘉伟" (需要 User Access Token)
		// 默认: "肖磊" (如果只有 Tenant Token，可能需要权限才能看到)
		username := "张嘉伟"
		if os.Getenv("TEST_SEARCH_USERNAME") != "" {
			username = os.Getenv("TEST_SEARCH_USERNAME")
		}

		t.Logf("Searching user with username: %s", username)

		// 调用搜索用户函数
		// GetUserIDByUsernameOrEmpty 内部会调用 GetAndRefreshUserToken
		// 优先级: FEISHU_USER_ACCESS_TOKEN > FEISHU_REFRESH_TOKEN > FEISHU_USER_CODE > App/Tenant Token
		user, err := cli.GetUserIDByUsernameOrEmpty(context.Background(), username)
		if err != nil {
			// 允许测试通过，如果只是因为用户不可见（但功能逻辑是正常的）
			// 但我们仍然让它 Fail，以便用户看到错误信息去修复权限
			t.Fatalf("GetUserIDByUsernameOrEmpty failed: %v", err)
		}
		t.Logf("Successfully searched user with username: %s, user: %+v", username, user)
	})

	t.Run("GetAndRefreshUserToken", func(t *testing.T) {
		if os.Getenv("FEISHU_USER_ACCESS_TOKEN") == "" && os.Getenv("FEISHU_REFRESH_TOKEN") == "" {
			t.Skip("missing FEISHU_USER_ACCESS_TOKEN or FEISHU_REFRESH_TOKEN")
		}

		// 调用获取和刷新用户token函数
		token, err := cli.GetAndRefreshUserToken(context.Background())
		if err != nil {
			t.Fatalf("GetAndRefreshUserToken failed: %v", err)
		}
		t.Logf("Successfully refreshed user token: %s", token)
	})
}

func TestGetCronAndRefreshUserToken(t *testing.T) {
	if os.Getenv("RUN_FEISHU_INTEGRATION_TESTS") != "1" {
		t.Skip("skipping Feishu integration tests")
	}

	cli := NewClient()
	if cli == nil {
		t.Fatalf("Failed to initialize Feishu Client")
	}

	t.Run("StartTokenRefresh", func(t *testing.T) {
		// Mock context with cancel to stop background loop
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// 设置 User Access Token，避免 GetAndRefreshUserToken 报错
		if os.Getenv("FEISHU_USER_ACCESS_TOKEN") == "" && os.Getenv("FEISHU_REFRESH_TOKEN") == "" {
			t.Skip("missing FEISHU_USER_ACCESS_TOKEN or FEISHU_REFRESH_TOKEN")
		}

		token, err := cli.GetCronAndRefreshUserToken(ctx)
		if err != nil {
			t.Fatalf("GetCronAndRefreshUserToken failed: %v", err)
		}

		if token == nil {
			t.Fatal("Expected token, got nil")
		}

		t.Logf("Got initial token: Tenant=%s..., User=%s...",
			limitStr(token.TenantAccessToken, 10),
			limitStr(token.UserAccessToken, 10))

		// Check cache
		cached := cli.GetCachedToken()
		if cached == nil {
			t.Fatal("Cache should be populated")
		}
		if cached.TenantAccessToken != token.TenantAccessToken {
			t.Errorf("Cache mismatch: expected %s, got %s", token.TenantAccessToken, cached.TenantAccessToken)
		}
		t.Logf("Cache matches initial token Successfully: Tenant=%s..., User=%s...",
			limitStr(token.TenantAccessToken, 10),
			limitStr(token.UserAccessToken, 10))

		// 验证非阻塞：测试应该立即完成，而不是等待 30 分钟
		// 无需额外代码，如果阻塞，测试框架会超时
	})
}

func limitStr(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
