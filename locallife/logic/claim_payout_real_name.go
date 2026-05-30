package logic

import (
	"strings"
	"unicode/utf8"
)

func ClaimPayoutRealNameReady(fullName string) bool {
	name := strings.TrimSpace(fullName)
	if name == "" {
		return false
	}
	if name == "微信用户" {
		return false
	}
	if strings.HasPrefix(strings.ToLower(name), "user ") {
		return false
	}
	return utf8.RuneCountInString(name) >= 2
}
