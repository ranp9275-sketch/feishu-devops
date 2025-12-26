package randutil

import (
	"crypto/rand"
	"encoding/base64"
)

// GenerateRandomString generates a random string of length n
func GenerateRandomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
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

// GenerateBase64String generates a base64 URL-encoded strings
func GenerateBase64String() string {
	bytes := make([]byte, 12)
	_, err := rand.Read(bytes)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(bytes)
}
