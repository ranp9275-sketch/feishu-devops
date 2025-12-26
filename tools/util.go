package tools

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type AuthResponse struct {
	AccessToken  string `json:"access"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"` // 秒
	RefreshToken string `json:"refresh"`
	IssuedAt     time.Time
}

var mu sync.Mutex

var Clientarchery *http.Client
var defaultClient = &http.Client{Timeout: 30 * time.Second}

func init() {
	// 测试连接
	fmt.Println("")

}

func CreateClient() *http.Client {
	return defaultClient
}

func RequestC(ctx context.Context, client *http.Client, method, url string, body io.Reader, headers map[string]string) ([]byte, int, error) {
	if client == nil {
		client = http.DefaultClient
	}

	// 创建带上下文的请求
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %v", err)
	}

	// 设置请求头
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response body: %v", err)
	}

	return responseBody, resp.StatusCode, nil
}

// Request sends an HTTP request and returns the response body or an error.
func Request(client *http.Client, method, url string, payload io.Reader) ([]byte, error) {
	req, err := http.NewRequest(method, url, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	log.Printf("Response Status: %s\n", resp.Status)
	if os.Getenv("DEBUG") == "true" {
		if len(body) > 100 {
			fmt.Printf("Response Body (first 100 bytes): %s\n", string(body[:100]))
		} else {
			fmt.Printf("Response Body: %s\n", string(body))
		}
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("request failed with status: %d, body: %s", resp.StatusCode, string(body))
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		return nil, fmt.Errorf("response content type is not JSON: %s", contentType)
	}

	return body, nil
}

func Get(client *http.Client, url string) ([]byte, error) {
	return Request(client, "GET", url, nil)
}

func Post(client *http.Client, url string, jsonPayload []byte) ([]byte, error) {
	return sendRequest(client, "POST", url, bytes.NewBuffer(jsonPayload))
}

func sendRequest(client *http.Client, method, url string, payload io.Reader) ([]byte, error) {
	req, err := http.NewRequest(method, url, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	log.Printf("Response Status: %s\n", resp.Status)
	// if len(body) > 100 {
	// 	fmt.Printf("Response Body (first 100 bytes): %s\n", string(body[:100]))
	// } else {
	// 	fmt.Printf("Response Body: %s\n", string(body))
	// }

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("request failed with status: %d, body: %s", resp.StatusCode, string(body))
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		return nil, fmt.Errorf("response content type is not JSON: %s", contentType)
	}

	return body, nil
}

// GenerateRandomString generates a random string of length n using URL-safe characters.
func GenerateRandomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" // URL-safe characters
	buf := make([]byte, n)
	_, err := rand.Read(buf)
	if err != nil {
		return ""
	}

	for i := range buf {
		buf[i] = letters[int(buf[i])%len(letters)]
	}

	return string(buf)
}

// Alternatively, if you want to generate exactly 16 bytes and encode them in base64
// which will result in a slightly longer string but it's URL-safe and cryptographically secure.
func GenerateBase64String() string {
	bytes := make([]byte, 12) // 12 bytes will give us 16 characters when encoded in base64
	_, err := rand.Read(bytes)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(bytes)
}

func StringSliceToIntSlice(strs string) ([]int, error) {
	var intSlice []int
	for _, str := range strings.Fields(strs) {
		integer, err := strconv.Atoi(str)
		if err != nil {
			return nil, fmt.Errorf("conversion failed for '%s': %v", str, err)
		}
		intSlice = append(intSlice, integer)
	}
	return intSlice, nil
}
