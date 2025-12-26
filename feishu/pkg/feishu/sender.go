package feishu

import (
    "bytes"
    "context"
    "encoding/json"
    "net/http"
    "os"
)

type Sender interface {
    Send(ctx context.Context, receiveID, receiveIDType, msgType, content string) error
}

type APISender struct{
    client *Client
}

func NewAPISender(client *Client) *APISender { return &APISender{client: client} }

func (s *APISender) Send(ctx context.Context, receiveID, receiveIDType, msgType, content string) error {
    return s.client.SendMessage(ctx, receiveID, receiveIDType, msgType, content)
}

type WebhookSender struct{
    httpClient *http.Client
    url        string
}

func NewWebhookSender() *WebhookSender {
    return &WebhookSender{httpClient: &http.Client{}, url: os.Getenv("FEISHU_WEBHOOK_URL")}
}

func (s *WebhookSender) Send(ctx context.Context, receiveID, receiveIDType, msgType, content string) error {
    if s.url == "" { return nil }
    payload := map[string]interface{}{"msg_type": msgType}
    if msgType == "text" {
        var m map[string]string
        _ = json.Unmarshal([]byte(content), &m)
        payload["content"] = m
    } else {
        var card map[string]interface{}
        _ = json.Unmarshal([]byte(content), &card)
        payload["card"] = card
    }
    data, _ := json.Marshal(payload)
    req, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewBuffer(data))
    req.Header.Set("Content-Type", "application/json")
    _, err := s.httpClient.Do(req)
    if err != nil { return err }
    return nil
}

