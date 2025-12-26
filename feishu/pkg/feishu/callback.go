package feishu

import (
	"context"

	"devops/feishu/config"
	log "devops/tools/logger"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

var cardActionHandler func(context.Context, *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error)

// SetCardActionHandler 设置卡片交互回调处理器
func SetCardActionHandler(h func(context.Context, *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error)) {
	cardActionHandler = h
}

func RegisterCallback(cfg *config.Config) {
	// 初始化日志
	logger := log.NewLogger(cfg.LogLevel)

	// 注册回调
	eventHandler := dispatcher.NewEventDispatcher("", "").

		// 监听「卡片回传交互 card.action.trigger」
		OnP2CardActionTrigger(func(ctx context.Context, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
			if cardActionHandler != nil {
				return cardActionHandler(ctx, event)
			}
			//fmt.Printf("[ OnP2CardActionTrigger access ], data: %s\n", larkcore.Prettify(event))
			logger.Info("Card action trigger received: %s", larkcore.Prettify(event))
			return nil, nil
		}).
		// 监听「拉取链接预览数据 url.preview.get」
		OnP2CardURLPreviewGet(func(ctx context.Context, event *callback.URLPreviewGetEvent) (*callback.URLPreviewGetResponse, error) {
			//fmt.Printf("[ OnP2URLPreviewAction access ], data: %s\n", larkcore.Prettify(event))
			logger.Info("URL preview request received for URL: %s", larkcore.Prettify(event))
			return nil, nil
		}).
		// 监听「用户与机器人会话进房 im.chat.access_event.bot_p2p_chat_entered_v1」
		OnCustomizedEvent("im.chat.access_event.bot_p2p_chat_entered_v1", func(ctx context.Context, event *larkevent.EventReq) error {
			//fmt.Printf("[ OnP2P2PChatEnteredV1 access ], data: %s\n", string(event.Body))
			logger.Info("User entered P2P chat with bot: %s", string(event.Body))
			return nil
		}).
		// 监听「接收消息 im.message.receive_v1」
		OnCustomizedEvent("im.message.receive_v1", func(ctx context.Context, event *larkevent.EventReq) error {
			logger.Info("Message received: %s", string(event.Body))
			return nil
		})
	// 创建Client
	cli := larkws.NewClient(cfg.FeishuAppID, cfg.FeishuAppSecret,
		larkws.WithEventHandler(eventHandler),
		larkws.WithLogLevel(larkcore.LogLevelDebug),
	)
	// 建立长连接
	err := cli.Start(context.Background())
	if err != nil {
		logger.Error("Failed to start Feishu WebSocket client: %v", err)
	}
}
