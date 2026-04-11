package util

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

// HashToken computes a deterministic HMAC-SHA256 for tokens.
func HashToken(token string, secret string) (string, error) {
	if token == "" {
		return "", errors.New("token is empty")
	}
	if secret == "" {
		return "", errors.New("secret is empty")
	}

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(token))
	return hex.EncodeToString(mac.Sum(nil)), nil
}
