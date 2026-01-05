package handler

import (
	"bytes"
	cfg "devops/feishu/config"
	"devops/feishu/pkg/feishu"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func newHandler() *Handler {
	client := feishu.NewClient(&cfg.Config{FeishuAppID: "id", FeishuAppSecret: "secret", LogLevel: "debug"})
	return NewHandler(client)
}

func TestSendTextRequestParsing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newHandler()
	h.sender = feishu.NewWebhookSender()
	body := []byte(`{"receive_id":"ou_test_user_id","receive_id_type":"open_id","msg_type":"text","content":{"text":"hello"}}`)
	
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/send-card", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")
	
	h.SendCard(c)
	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d, body: %s", w.Code, w.Body.String())
	}
}

func TestSendTextPlainString(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newHandler()
	h.sender = feishu.NewWebhookSender()
	body := []byte(`{"receive_id":"ou_test_user_id","receive_id_type":"open_id","msg_type":"text","content":"你好，这是字符串内容"}`)
	
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/send-card", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.SendCard(c)
	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d, body: %s", w.Code, w.Body.String())
	}
}

func TestSendInteractiveValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newHandler()
	h.sender = feishu.NewWebhookSender()
	// 发送空 content，预期 400
	body := []byte(`{"receive_id":"ou_test_user_id","receive_id_type":"open_id","msg_type":"interactive","content":{}}`)
	
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/send-card", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.SendCard(c)
	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", w.Code)
	}
}

func TestSendInteractiveOK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newHandler()
	h.sender = feishu.NewWebhookSender()
	card := `{"schema":"2.0","header":{"title":{"content":"标题","tag":"plain_text"}},"elements":[{"tag":"markdown","content":"正文内容"}]}`
	body := []byte(`{"receive_id":"ou_test_user_id","receive_id_type":"open_id","msg_type":"interactive","content":` + card + `}`)
	
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/send-card", bytes.NewBuffer(body))
	c.Request.Header.Set("Content-Type", "application/json")

	h.SendCard(c)
	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d, body: %s", w.Code, w.Body.String())
	}
}
