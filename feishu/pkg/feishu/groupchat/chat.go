package groupchat

import (
	"bytes"
	"context"
	c "devops/feishu/config"
	"devops/tools/logger"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkcontact "github.com/larksuite/oapi-sdk-go/v3/service/contact/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

var (
	loadConfigFunc = c.LoadConfig
)

type CreateGroupChatRequest struct {
	UserIdType  string   `json:"user_id_type"`
	Uuid        string   `json:"uuid"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	UserIDs     []string `json:"user_ids"`
	OwnerID     string   `json:"owner_id,omitempty"` // 可选
}

// 定义响应结构体
type SearchUserRespBody struct {
	HasMore   bool                      `json:"has_more"`
	PageToken string                    `json:"page_token"`
	Users     *[]SearchUserRespBodyUser `json:"users"`
}

type SearchUserRespBodyUser struct {
	DepartmentIDs []string `json:"department_ids"`
	Name          string   `json:"name"`
	OpenID        string   `json:"open_id"`
	UserID        string   `json:"user_id"`
}

type Token struct {
	UserAccessToken   string `json:"user_access_token"`
	TenantAccessToken string `json:"tenant_access_token"`
}

func NewCreateGroupChatRequest(userIDType, uuid, name, description string, userIDs []string) *CreateGroupChatRequest {
	return &CreateGroupChatRequest{
		UserIdType:  userIDType,
		Uuid:        uuid,
		Name:        name,
		Description: description,
		UserIDs:     userIDs,
	}
}

type Client struct {
	Client *lark.Client
	Config *c.Config
	Clog   *logger.Logger

	// tokenCache 缓存最新的 Token
	tokenCache *Token
	mu         sync.RWMutex
}

func (c *Client) NewClient() *Client {
	// 加载配置
	cfg, err := loadConfigFunc()
	if err != nil {
		return nil
	}

	// 创建 Feishu 客户端
	client := lark.NewClient(cfg.FeishuAppID, cfg.FeishuAppSecret)
	return &Client{
		Client: client,
		Config: cfg,
	}
}

// 用自建应用发起群聊
func (c *Client) CreateGroupChat(ctx context.Context, ownerId string, req *CreateGroupChatRequest) (string, error) {
	// 使用 Client 中的配置
	// cfg := c.Config

	// 使用 Client 中的 Feishu 客户端
	client := c.Client

	// 尝试获取 User Access Token (u-token) 以便作为用户身份创建群聊
	// 这样可以规避机器人的可见性限制
	userToken, err := c.GetAndRefreshUserToken(ctx)

	// 构造请求体 (无论使用 SDK 还是手动 HTTP，body 内容是一样的)
	bodyBuilder := larkim.NewCreateChatReqBodyBuilder().
		Name(req.Name).
		Description(req.Description).OwnerId(ownerId).
		UserIdList(req.UserIDs).
		GroupMessageType(`chat`).
		ChatMode(`group`).
		ChatType(`private`).
		JoinMessageVisibility(`all_members`).
		LeaveMessageVisibility(`all_members`).
		MembershipApproval(`no_approval_required`).
		EditPermission(`all_members`)

	// 如果指定了 OwnerID，则设置
	if req.OwnerID != "" {
		bodyBuilder.OwnerId(req.OwnerID)
	}

	reqBody := bodyBuilder.Build()

	// 如果有 User Token，使用手动 HTTP 请求 (仿照 GetUserIDByUsernameOrEmpty)
	// 这是为了避免 SDK 在 User Token 和 Tenant Token 混用时的兼容性问题
	if err == nil && userToken != "" {

		// 构造 URL
		// 注意: user_id_type 和 uuid 是 query 参数
		baseUrl := "https://open.feishu.cn/open-apis/im/v1/chats"
		params := url.Values{}
		if req.UserIdType != "" {
			params.Add("user_id_type", req.UserIdType)
		} else {
			params.Add("user_id_type", "user_id")
		}
		if req.Uuid != "" {
			params.Add("uuid", req.Uuid)
		}
		fullUrl := fmt.Sprintf("%s?%s", baseUrl, params.Encode())

		// 序列化 Body
		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			c.Clog.Error(err.Error(), "marshal request body failed")
			return "", fmt.Errorf("marshal request body failed: %w", err)
		}

		// 创建请求
		httpReq, err := http.NewRequestWithContext(ctx, "POST", fullUrl, bytes.NewBuffer(bodyBytes))
		if err != nil {
			c.Clog.Error(err.Error(), "create http request failed")
			return "", fmt.Errorf("create http request failed: %w", err)
		}
		httpReq.Header.Set("Authorization", "Bearer "+userToken)
		httpReq.Header.Set("Content-Type", "application/json; charset=utf-8")

		// 发送请求
		httpClient := &http.Client{}
		httpResp, err := httpClient.Do(httpReq)
		if err != nil {
			c.Clog.Error(err.Error(), "do http request failed")
			return "", fmt.Errorf("do http request failed: %w", err)
		}
		defer httpResp.Body.Close()

		respBytes, _ := io.ReadAll(httpResp.Body)
		// fmt.Printf("Debug CreateGroupChat Response: %s\n", string(respBytes))

		var respData struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
			Data *struct {
				ChatId *string `json:"chat_id"`
			} `json:"data"`
		}

		if err := json.Unmarshal(respBytes, &respData); err != nil {
			c.Clog.Error(err.Error(), "decode response failed")
			return "", fmt.Errorf("decode response failed: %w", err)
		}

		if respData.Code != 0 {
			if respData.Code == 99991672 {
				c.Clog.Error(respData.Msg, "permission denied (User Token)")
				return "", fmt.Errorf("permission denied (User Token): The App lacks required User Scopes. \nPlease add one of these permissions in 'User Scopes' (not Bot Scopes) in Developer Console: [im:chat, im:chat:create, im:chat:create_by_user].\nThen 'Create Version' and 'Publish'.\nOriginal error: %s", respData.Msg)
			}
			if respData.Code == 12070 {
				c.Clog.Error(respData.Msg, "token scope error")
				return "", fmt.Errorf("token scope error: The User Access Token provided does NOT have the required scopes (im:chat, im:chat:create). \nEven if the App has these permissions, the Token itself was generated WITHOUT them. \nACTION: Please RE-GENERATE the User Access Token and ensure you check/authorize the 'im:chat' and 'im:chat:create' scopes during generation.\nOriginal error: %s", respData.Msg)
			}
			c.Clog.Error(respData.Msg, "feishu api error")
			return "", fmt.Errorf("feishu api error: code=%d, msg=%s", respData.Code, respData.Msg)
		}

		if respData.Data != nil && respData.Data.ChatId != nil {
			return *respData.Data.ChatId, nil
		}
		c.Clog.Error("chat_id not found in response", "CreateGroupChat")
		return "", fmt.Errorf("chat_id not found in response")
	} else {
		c.Clog.Error(err.Error(), "failed to get user token")
		fmt.Printf("Info: User Access Token not available (%v). Using default Tenant Access Token via SDK.\n", err)
	}

	// 创建请求对象
	re := larkim.NewCreateChatReqBuilder().
		UserIdType(req.UserIdType).
		SetBotManager(false).
		Uuid(req.Uuid).
		Body(reqBody).
		Build()

	// 发起请求
	var opts []larkcore.RequestOptionFunc

	// 尝试从缓存或获取 Tenant Token
	tenantToken, err := c.GetTenantAccessToken(ctx)
	if err == nil && tenantToken != "" {
		// fmt.Println("Info: Using Tenant Access Token from method.")
		opts = append(opts, larkcore.WithTenantAccessToken(tenantToken))
	} else if tenantToken := os.Getenv("FEISHU_TENANT_ACCESS_TOKEN"); tenantToken != "" {
		c.Clog.Error("tenant access token not found in cache or environment variable", "CreateGroupChat")

		opts = append(opts, larkcore.WithTenantAccessToken(tenantToken))
	} else {
		c.Clog.Error("tenant access token not found in cache or environment variable", "CreateGroupChat")
	}

	resp, err := client.Im.V1.Chat.Create(ctx, re, opts...)
	if err != nil {
		c.Clog.Error(err.Error(), "failed to create group chat")
		return "", fmt.Errorf("failed to create group chat: %w", err)
	}

	if !resp.Success() {
		c.Clog.Error(resp.Msg, "feishu api error")
		return "", fmt.Errorf("feishu api error: code=%d, msg=%s, req_id=%s", resp.Code, resp.Msg, resp.RequestId())
	}

	if resp.Data != nil && resp.Data.ChatId != nil {
		c.Clog.Info(fmt.Sprintf("Info: Created Group Chat ID: %s", *resp.Data.ChatId), "CreateGroupChat")
		return *resp.Data.ChatId, nil
	}

	return "", fmt.Errorf("chat_id not found in response")
}

// 通过用户名获取用户 ID (注意：这会遍历所有用户，效率较低，建议使用手机号或邮箱)
func (c *Client) GetUserIDByUsername(ctx context.Context, username string) (string, error) {
	// 使用 Client 中的 Feishu 客户端
	client := c.Client

	// 使用 Contact List API 遍历用户
	// 注意：/search/v1/user 接口不支持机器人(Tenant Token)，只能用 List 接口遍历

	req := larkcontact.NewListUserReqBuilder().
		PageSize(50).
		DepartmentIdType("open_department_id").
		UserIdType("user_id").
		Build()

	resp, err := client.Contact.V3.User.List(ctx, req)
	if err != nil {
		c.Clog.Error(err.Error(), "list users failed")
		return "", fmt.Errorf("list users failed: %w", err)
	}

	if !resp.Success() {
		c.Clog.Error(resp.Msg, "feishu api error")
		return "", fmt.Errorf("list users failed: code=%d, msg=%s", resp.Code, resp.Msg)
	}

	if resp.Data == nil || resp.Data.Items == nil {
		c.Clog.Error("user list is nil", "GetUserIDByUsername")
		//fmt.Printf("Debug: User list is nil. Check 'Contacts Scope' (可访问的数据范围) in Developer Console.\n")
		return "", fmt.Errorf("user not found: %s", username)
	}

	// 遍历用户列表
	c.Clog.Info(fmt.Sprintf("Found %d users in the list. Scanning for match...", len(resp.Data.Items)), "GetUserIDByUsername")

	for _, user := range resp.Data.Items {
		name := ""
		if user.Name != nil {
			name = *user.Name
			c.Clog.Info(fmt.Sprintf("Scanning user: %s (ID: %s)", name, *user.UserId), "GetUserIDByUsername")
		} else {
			// 如果名字为空，说明权限不足
			if user.UserId != nil {
				c.Clog.Warn(fmt.Sprintf("Found user ID %s but name is empty. Check 'Access user name' permission.", *user.UserId), "GetUserIDByUsername")
				c.Clog.Fatal("CRITICAL: If you have added permissions, you MUST click 'Create Version' and 'Publish' in the Developer Console for changes to take effect!", "GetUserIDByUsername")
			}
			// 尝试打印完整 user 对象看是否有其他字段可用
			userJSON, _ := json.Marshal(user)
			c.Clog.Error(fmt.Sprintf("User: %s", string(userJSON)), "GetUserIDByUsername")
		}

		if name == username {
			if user.UserId != nil {
				return *user.UserId, nil
			}
			c.Clog.Error(fmt.Sprintf("User %s has no user_id field", name), "GetUserIDByUsername")
			return "", fmt.Errorf("found user %s but user_id is missing", username)
		}
	}

	return "", fmt.Errorf("user not found: %s (Note: Bots cannot use search API, and List API requires 'user name' field permission)", username)
}

// GetTenantAccessToken 获取 Tenant Access Token (优先从缓存获取)
func (c *Client) GetTenantAccessToken(ctx context.Context) (string, error) {
	c.mu.RLock()
	if c.tokenCache != nil && c.tokenCache.TenantAccessToken != "" {
		token := c.tokenCache.TenantAccessToken
		c.mu.RUnlock()
		return token, nil
	}
	c.mu.RUnlock()

	// 缓存未命中，降级为直接获取
	c.Clog.Info("Tenant Token cache miss, fetching directly...", "GetTenantAccessToken")
	return c.fetchTenantAccessToken(ctx)
}

// GetAndRefreshUserToken 获取 User Access Token (优先从缓存获取)
func (c *Client) GetAndRefreshUserToken(ctx context.Context) (string, error) {
	c.mu.RLock()
	if c.tokenCache != nil && c.tokenCache.UserAccessToken != "" {
		token := c.tokenCache.UserAccessToken
		c.mu.RUnlock()
		return token, nil
	}
	c.mu.RUnlock()

	// 缓存未命中，降级为直接获取
	c.Clog.Info("User Token cache miss, fetching directly...", "GetAndRefreshUserToken")
	return c.fetchUserAccessToken(ctx)
}

// fetchTenantAccessToken 获取 Tenant Access Token (Internal)
func (c *Client) fetchTenantAccessToken(ctx context.Context) (string, error) {
	cfg := c.Config
	url := "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal"
	body := map[string]string{
		"app_id":     cfg.FeishuAppID,
		"app_secret": cfg.FeishuAppSecret,
	}
	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		c.Clog.Error(err.Error(), "create tenant token request failed")
		return "", fmt.Errorf("create tenant token request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.Clog.Error(err.Error(), "do tenant token request failed")
		return "", fmt.Errorf("do tenant token request failed: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		c.Clog.Error(err.Error(), "decode tenant token response failed")
		return "", fmt.Errorf("decode tenant token response failed: %w", err)
	}
	if tokenResp.Code != 0 {
		c.Clog.Error(fmt.Sprintf("get tenant token failed: code=%d, msg=%s", tokenResp.Code, tokenResp.Msg), "fetchTenantAccessToken")
		return "", fmt.Errorf("get tenant token failed: code=%d, msg=%s", tokenResp.Code, tokenResp.Msg)
	}

	return tokenResp.TenantAccessToken, nil
}

// fetchUserAccessToken 获取和刷新用户token (Internal)
func (c *Client) fetchUserAccessToken(ctx context.Context) (string, error) {
	cfg := c.Config
	// 0. 优先尝试从环境变量获取 User Access Token (用于调试或手动提供)
	if userToken := os.Getenv("FEISHU_USER_ACCESS_TOKEN"); userToken != "" {
		c.Clog.Info("Using User Access Token from environment variable.", "fetchUserAccessToken")
		return userToken, nil
	}

	url := "https://open.feishu.cn/open-apis/authen/v2/oauth/token"

	// 1. 优先尝试使用 Refresh Token 刷新
	refreshToken := os.Getenv("FEISHU_REFRESH_TOKEN")
	if refreshToken != "" {
		tokenBody := map[string]string{
			"grant_type":    "refresh_token",
			"client_id":     cfg.FeishuAppID,
			"client_secret": cfg.FeishuAppSecret,
			"refresh_token": refreshToken,
		}
		tokenBytes, _ := json.Marshal(tokenBody)

		reqToken, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(tokenBytes))
		if err != nil {
			c.Clog.Error(err.Error(), "create refresh token request failed")
			return "", fmt.Errorf("create refresh token request failed: %w", err)
		}
		reqToken.Header.Set("Content-Type", "application/json; charset=utf-8")

		client := &http.Client{}
		respToken, err := client.Do(reqToken)
		if err != nil {
			c.Clog.Error(err.Error(), "do refresh token request failed")
			return "", fmt.Errorf("do refresh token request failed: %w", err)
		}
		defer respToken.Body.Close()

		var tokenResp struct {
			Code         int    `json:"code"`
			Msg          string `json:"msg"`
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		}
		if err := json.NewDecoder(respToken.Body).Decode(&tokenResp); err != nil {
			c.Clog.Error(err.Error(), "decode refresh token response failed")
			return "", fmt.Errorf("decode refresh token response failed: %w", err)
		}
		if tokenResp.Code != 0 {
			// 如果 refresh token 失效，尝试继续使用 Code
			c.Clog.Warn(fmt.Sprintf("Warning: Refresh token failed (code=%d, msg=%s). Trying authorization code...", tokenResp.Code, tokenResp.Msg), "fetchUserAccessToken")
		} else {
			if tokenResp.RefreshToken != "" {
				c.Clog.Info(fmt.Sprintf("Info: New Refresh Token obtained: %s", tokenResp.RefreshToken), "fetchUserAccessToken")
			}
			return tokenResp.AccessToken, nil
		}
	}

	// 2. 尝试使用 Authorization Code 获取初始 Token
	code := os.Getenv("FEISHU_USER_CODE")
	if code != "" {
		tokenBody := map[string]string{
			"grant_type":    "authorization_code",
			"client_id":     cfg.FeishuAppID,
			"client_secret": cfg.FeishuAppSecret,
			"code":          code,
		}
		tokenBytes, _ := json.Marshal(tokenBody)

		reqToken, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(tokenBytes))
		if err != nil {
			c.Clog.Error(err.Error(), "create code token request failed")
			return "", fmt.Errorf("create code token request failed: %w", err)
		}
		reqToken.Header.Set("Content-Type", "application/json; charset=utf-8")

		client := &http.Client{}
		respToken, err := client.Do(reqToken)
		if err != nil {
			c.Clog.Error(err.Error(), "do code token request failed")
			return "", fmt.Errorf("do code token request failed: %w", err)
		}
		defer respToken.Body.Close()

		var tokenResp struct {
			Code         int    `json:"code"`
			Msg          string `json:"msg"`
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		}
		if err := json.NewDecoder(respToken.Body).Decode(&tokenResp); err != nil {
			c.Clog.Error(err.Error(), "decode code token response failed")
			return "", fmt.Errorf("decode code token response failed: %w", err)
		}
		if tokenResp.Code != 0 {
			c.Clog.Error(fmt.Sprintf("Warning: Authorization code failed (code=%d, msg=%s).", tokenResp.Code, tokenResp.Msg), "fetchUserAccessToken")
			return "", fmt.Errorf("get token by code failed: code=%d, msg=%s", tokenResp.Code, tokenResp.Msg)
		}

		c.Clog.Info(fmt.Sprintf("Info: Initial Refresh Token: %s", tokenResp.RefreshToken), "fetchUserAccessToken")
		c.Clog.Info("Action: Please set FEISHU_REFRESH_TOKEN for future runs.", "fetchUserAccessToken")
		return tokenResp.AccessToken, nil
	}

	return "", fmt.Errorf("no user credentials found (FEISHU_REFRESH_TOKEN or FEISHU_USER_CODE)")
}

// GetUserIDByUsernameOrEmpty 尝试使用原生 HTTP 请求调用 search/v1/user 接口
func (c *Client) GetUserIDByUsernameOrEmpty(ctx context.Context, username string) (*SearchUserRespBodyUser, error) {
	// 对 username 进行 URL 编码
	encodedUsername := url.QueryEscape(username)
	url := fmt.Sprintf("https://open.feishu.cn/open-apis/search/v1/user?query=%s", encodedUsername)

	// 0. 优先尝试从环境变量获取 User Access Token (用于调试)
	// Postman 测试成功是因为使用了 u- 开头的 User Token，而机器人默认只有 t- 开头的 Tenant Token
	userToken := os.Getenv("FEISHU_USER_ACCESS_TOKEN")
	var finalToken string

	if userToken != "" {
		finalToken = userToken
	} else {
		// 1. 尝试通过 Refresh Token 或 Code 获取 Owner 的 User Access Token
		// 这是最优先的，因为用户明确要求 "先去获取所有者得u token"
		ownerUserToken, err := c.GetAndRefreshUserToken(ctx)
		if err == nil {
			finalToken = ownerUserToken
		} else {
			c.Clog.Info(fmt.Sprintf("Info: Failed to get Owner User Token: %v. Falling back to Tenant/App Token.\n", err), "getUserIDByUsernameOrEmpty")
			c.Clog.Info("Tip: To use Search API properly, perform a one-time login to get a code/refresh_token.", "getUserIDByUsernameOrEmpty")
			c.Clog.Info("     Set FEISHU_REFRESH_TOKEN env var to enable automatic User Token retrieval.", "getUserIDByUsernameOrEmpty")

			// 2. 尝试获取 App Access Token (尝试作为 Owner Token 替代方案)
			appToken, err := c.GetAndRefreshUserToken(ctx)
			if err == nil {
				finalToken = appToken
			} else {
				// 3. 获取 Tenant Access Token
				tenantToken, err := c.GetTenantAccessToken(ctx)
				if err != nil {
					c.Clog.Error(fmt.Sprintf("Error: Failed to get tenant token: %v", err), "getUserIDByUsernameOrEmpty")
					return nil, fmt.Errorf("failed to get tenant token: %w", err)
				}
				finalToken = tenantToken
			}
		}
	}

	client := &http.Client{}

	// 2. 调用搜索接口
	reqSearch, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		c.Clog.Error(err.Error(), "create search request failed")
		return nil, fmt.Errorf("create search request failed: %w", err)
	}
	// 注意：此处使用获取到的 Token (User Token 或 App/Tenant Token)
	// 如果报错 99991663，说明机器人身份(Tenant Token)无法调用此接口。
	reqSearch.Header.Set("Authorization", "Bearer "+finalToken)

	respSearch, err := client.Do(reqSearch)
	if err != nil {
		c.Clog.Error(err.Error(), "do search request failed")
		return nil, fmt.Errorf("do search request failed: %w", err)
	}
	defer respSearch.Body.Close()

	bodyBytes, _ := io.ReadAll(respSearch.Body)
	// fmt.Printf("Debug Search API Response: %s\n", string(bodyBytes))

	var searchResp struct {
		Code int                 `json:"code"`
		Msg  string              `json:"msg"`
		Data *SearchUserRespBody `json:"data"`
	}

	if err := json.Unmarshal(bodyBytes, &searchResp); err != nil {
		c.Clog.Error(err.Error(), "decode search response failed")
		return nil, fmt.Errorf("decode search response failed: %w", err)
	}

	// 如果 Search API 报错 99991663 (不支持的 Token 类型) 或者 99991668 (权限不足)，尝试回退到 Contact List API
	if searchResp.Code != 0 {
		if searchResp.Code == 99991663 || searchResp.Code == 99991668 {
			c.Clog.Warn(fmt.Sprintf("Warning: Search API not supported for bot (code %d). Falling back to Contact List API.\n", searchResp.Code), "getUserIDByUsernameOrEmpty")

			// Fallback: 使用 Contact.V3.User.List 接口
			client := c.Client

			// 构造请求
			req := larkcontact.NewListUserReqBuilder().
				DepartmentIdType("open_department_id").
				UserIdType("user_id").
				PageSize(50). // 获取更多用户以增加匹配几率
				Build()

			resp, err := client.Contact.V3.User.List(ctx, req)
			if err != nil {
				c.Clog.Error(err.Error(), "fallback to list api failed")
				return nil, fmt.Errorf("fallback to list api failed: %w", err)
			}

			if !resp.Success() {
				c.Clog.Error(fmt.Sprintf("Error: Fallback list api failed: code=%d, msg=%s", resp.Code, resp.Msg), "getUserIDByUsernameOrEmpty")
				return nil, fmt.Errorf("fallback list api failed: code=%d, msg=%s", resp.Code, resp.Msg)
			}

			if resp.Data == nil || resp.Data.Items == nil {
				c.Clog.Error("Error: Fallback list api returned no data", "getUserIDByUsernameOrEmpty")
				return nil, fmt.Errorf("fallback list api returned no data")
			}

			c.Clog.Info(fmt.Sprintf("Debug: Found %d users in the list. Scanning for match...\n", len(resp.Data.Items)), "getUserIDByUsernameOrEmpty")

			var visibleUsers []string
			for _, user := range resp.Data.Items {
				userName := ""
				if user.Name != nil {
					userName = *user.Name
				}
				userID := ""
				if user.UserId != nil {
					userID = *user.UserId
				}
				visibleUsers = append(visibleUsers, fmt.Sprintf("%s(%s)", userName, userID))

				// 打印每个用户的详细信息进行调试
				c.Clog.Info(fmt.Sprintf("Debug: Scanning user: %s (ID: %s)\n", userName, userID), "getUserIDByUsernameOrEmpty")

				if userName == username {
					// 构造 SearchUserRespBodyUser 对象返回
					// 注意：Contact API 返回的 User 结构与 SearchUserRespBodyUser 不完全一致，需要转换
					return &SearchUserRespBodyUser{
						Name:   userName,
						UserID: userID,
						OpenID: *user.OpenId,
						// Avatar 和 DepartmentIDs 可能需要额外的 API 调用获取，这里简化处理
					}, nil
				}
			}

			// 如果没找到，返回详细的错误信息
			return nil, fmt.Errorf("user not found: %s. \nPossible reasons:\n1. User is not in the App's Availability Scope (Visible Users: %v).\n2. 'Access user name' permission is not enabled/published.\nPlease add user '%s' to the App's visibility in Feishu Admin.", username, visibleUsers, username)
		}
		c.Clog.Error(fmt.Sprintf("Error: Search API failed: code=%d, msg=%s", searchResp.Code, searchResp.Msg), "getUserIDByUsernameOrEmpty")
		return nil, fmt.Errorf("search api failed: code=%d, msg=%s", searchResp.Code, searchResp.Msg)
	}

	if searchResp.Data == nil || searchResp.Data.Users == nil || len(*searchResp.Data.Users) == 0 {
		c.Clog.Error(fmt.Sprintf("Error: User not found via search api: %s", username), "getUserIDByUsernameOrEmpty")
		return nil, fmt.Errorf("user not found via search api: %s", username)
	}

	for _, user := range *searchResp.Data.Users {
		// 精确匹配
		if user.Name == username {
			return &user, nil
		}
	}

	c.Clog.Error(fmt.Sprintf("Error: User found but name mismatch: %s", username), "getUserIDByUsernameOrEmpty")
	return nil, fmt.Errorf("user found but name mismatch: %s", username)
}

// GetGroupChatMembers 拉取群聊成员
func (c *Client) GetGroupChatMembers(ctx context.Context, chatID string) (interface{}, error) {
	// 使用 Client 中的 Feishu 客户端
	client := c.Client

	// 尝试获取 User Access Token
	userToken, err := c.GetAndRefreshUserToken(ctx)
	if err != nil {
		c.Clog.Info(fmt.Sprintf("Info: Failed to get User Token for GetGroupChatMembers: %v. Using Tenant Token.\n", err), "getGroupChatMembers")
	}

	// 创建请求对象
	req := larkim.NewGetChatMembersReqBuilder().
		ChatId(chatID).
		MemberIdType("open_id").
		Build()

	// 选项
	var opts []larkcore.RequestOptionFunc
	if userToken != "" {
		opts = append(opts, larkcore.WithUserAccessToken(userToken))
	}

	// 发起请求
	resp, err := client.Im.V1.ChatMembers.Get(ctx, req, opts...)
	if err != nil {
		c.Clog.Error(fmt.Sprintf("Error: Get chat members failed: %v", err), "getGroupChatMembers")
		return nil, fmt.Errorf("get chat members failed: %w", err)
	}

	// 服务端错误处理
	if !resp.Success() {
		c.Clog.Error(fmt.Sprintf("Error: Get chat members failed: code=%d, msg=%s, logId=%s", resp.Code, resp.Msg, resp.RequestId()), "getGroupChatMembers")
		return nil, fmt.Errorf("get chat members failed: code=%d, msg=%s, logId=%s", resp.Code, resp.Msg, resp.RequestId())
	}

	// 返回成员列表
	if resp.Data == nil || resp.Data.Items == nil {
		return nil, nil
	}
	return resp.Data.Items, nil
}

// AddGroupChatMembers 拉人进群
// chatID: 群 ID
// userIDs: 要邀请的用户 ID 列表
// 返回值: invalidUserIDs (无效的用户 ID 列表), error
func (c *Client) AddGroupChatMembers(ctx context.Context, chatID string, userIDs []string) ([]string, error) {
	// 使用 Client 中的 Feishu 客户端
	client := c.Client

	// 尝试获取 User Access Token
	userToken, err := c.GetAndRefreshUserToken(ctx)
	if err != nil {
		c.Clog.Info(fmt.Sprintf("Info: Failed to get User Token for AddGroupChatMembers: %v. Using Tenant Token.\n", err), "addGroupChatMembers")
	}

	// 构造请求体
	body := larkim.NewCreateChatMembersReqBodyBuilder().
		IdList(userIDs).
		Build()

	// 构造请求对象
	req := larkim.NewCreateChatMembersReqBuilder().
		ChatId(chatID).
		MemberIdType("user_id"). // 默认使用 user_id，也可以是 open_id
		Body(body).
		Build()

	// 选项
	var opts []larkcore.RequestOptionFunc
	if userToken != "" {
		opts = append(opts, larkcore.WithUserAccessToken(userToken))
	}

	// 发起请求
	resp, err := client.Im.V1.ChatMembers.Create(ctx, req, opts...)
	if err != nil {
		c.Clog.Error(fmt.Sprintf("Error: Add chat members failed: %v", err), "addGroupChatMembers")
		return nil, fmt.Errorf("add chat members failed: %w", err)
	}

	// 服务端错误处理
	if !resp.Success() {
		// 特定错误处理
		if resp.Code == 99991672 {
			c.Clog.Error(fmt.Sprintf("Error: Permission denied (User Token): The App lacks required User Scopes. \nPlease add 'im:chat' or 'im:chat:members' in 'User Scopes'.\nOriginal error: %s", resp.Msg), "addGroupChatMembers")
			return nil, fmt.Errorf("permission denied (User Token): The App lacks required User Scopes. \nPlease add 'im:chat' or 'im:chat:members' in 'User Scopes'.\nOriginal error: %s", resp.Msg)
		}
		c.Clog.Error(fmt.Sprintf("Error: Add chat members failed: code=%d, msg=%s, logId=%s", resp.Code, resp.Msg, resp.RequestId()), "addGroupChatMembers")
		return nil, fmt.Errorf("add chat members failed: code=%d, msg=%s, logId=%s", resp.Code, resp.Msg, resp.RequestId())
	}

	// 返回无效的 ID 列表 (如果有)
	if resp.Data != nil && resp.Data.InvalidIdList != nil {
		return resp.Data.InvalidIdList, nil
	}

	return nil, nil
}

// GetCronAndRefreshUserToken 获取 Token 并启动后台定时刷新任务 (Non-blocking)
// 返回当前最新的 Token，并启动一个 goroutine 每 30 分钟刷新一次
func (c *Client) GetCronAndRefreshUserToken(ctx context.Context) (*Token, error) {
	// 1. 立即获取一次 Token
	tenantToken, err := c.fetchTenantAccessToken(ctx)
	if err != nil {
		c.Clog.Error(fmt.Sprintf("Error: Failed to get tenant access token: %v", err), "getCronAndRefreshUserToken")
		return nil, fmt.Errorf("failed to get tenant access token: %w", err)
	}
	userToken, err := c.fetchUserAccessToken(ctx)
	if err != nil {
		c.Clog.Error(fmt.Sprintf("Error: Failed to get user access token: %v", err), "getCronAndRefreshUserToken")
		return nil, fmt.Errorf("failed to get user access token: %w", err)
	}

	initialToken := &Token{
		TenantAccessToken: tenantToken,
		UserAccessToken:   userToken,
	}

	// 更新缓存
	c.mu.Lock()
	c.tokenCache = initialToken
	c.mu.Unlock()

	// 2. 启动后台协程定时刷新
	go func() {
		// 创建一个新的 context，避免因传入的 ctx 取消而导致后台任务意外终止
		// 除非用户明确希望后台任务随请求结束
		// 这里假设后台任务应该持续运行直到程序退出或 Client 销毁
		// 但为了安全，我们还是监听传入的 ctx，或者使用 TODO/Background

		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// 刷新 Tenant Token
				tToken, err := c.fetchTenantAccessToken(ctx)
				if err != nil {
					c.Clog.Error(fmt.Sprintf("Error refreshing tenant token: %v", err), "getCronAndRefreshUserToken")
					continue
				}
				// 刷新 User Token
				uToken, err := c.fetchUserAccessToken(ctx)
				if err != nil {
					c.Clog.Error(fmt.Sprintf("Error refreshing user token: %v", err), "getCronAndRefreshUserToken")
					continue
				}

				// 更新缓存
				c.mu.Lock()
				c.tokenCache = &Token{
					TenantAccessToken: tToken,
					UserAccessToken:   uToken,
				}
				c.mu.Unlock()
				c.Clog.Info(fmt.Sprintf("Info: Tokens refreshed successfully in background."), "getCronAndRefreshUserToken")
			}
		}
	}()

	return initialToken, nil
}

// GetCachedToken 获取当前缓存的 Token (如果未初始化则返回 nil)
func (c *Client) GetCachedToken() *Token {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.tokenCache
}
