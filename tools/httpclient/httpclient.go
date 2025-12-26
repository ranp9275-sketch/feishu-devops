package httpclient

import (
	"context"
	"io"
	"net/http"
	"time"
)

var defaultClient = &http.Client{Timeout: 30 * time.Second}

func CreateClient() *http.Client { return defaultClient }

func RequestC(ctx context.Context, client *http.Client, method, url string, body io.Reader, headers map[string]string) ([]byte, int, error) {
	if client == nil {
		client = defaultClient
	}

	if headers == nil {
		headers = map[string]string{
			"Content-Type": "application/json",
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, 0, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}

	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return b, resp.StatusCode, nil
}
