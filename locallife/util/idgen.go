package util

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// GenerateOutTradeNo 生成商户订单号。
// 格式：prefix + yyyyMMddHHmmss(14位) + hex随机(8位)，合计 len(prefix)+22 位。
// prefix 为空时默认使用 "P"。
// 使用 crypto/rand 生成 4 字节（32位）随机部分，失败时返回 error。
func GenerateOutTradeNo(prefix string) (string, error) {
	if prefix == "" {
		prefix = "P"
	}
	dateStr := time.Now().Format("20060102150405")
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto/rand.Read failed: %w", err)
	}
	return prefix + dateStr + hex.EncodeToString(b), nil
}

// GenerateOutRefundNo 生成退款单号。
// 格式：R + yyyyMMddHHmmss(14位) + hex随机(8位) = 23位。
// 使用 crypto/rand 生成 4 字节（32位）随机部分，失败时返回 error。
func GenerateOutRefundNo() (string, error) {
	dateStr := time.Now().Format("20060102150405")
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto/rand.Read failed: %w", err)
	}
	return "R" + dateStr + hex.EncodeToString(b), nil
}
