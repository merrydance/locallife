package api

import "strings"

// normalizeUploadURLForClient converts stored upload paths (e.g. "uploads/..." or "/uploads/...")
// into a URL path that can be used directly by browsers.
//
// - For local uploads stored as "uploads/...", it returns "/uploads/...".
// - For already-normalized "/uploads/...", it returns as-is.
// - For external URLs (http/https), it returns as-is.
func normalizeUploadURLForClient(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		return p
	}
	if strings.HasPrefix(p, "/uploads/") {
		return p
	}
	if strings.HasPrefix(p, "uploads/") {
		return "/" + p
	}
	return p
}

// normalizeImageURLForStorage 规范化图片URL用于存储。
// 它会将完整URL（带域名和签名）或带前导斜杠的路径转换为相对路径（如 "uploads/..."）。
func normalizeImageURLForStorage(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}

	// 1. 处理完整 URL 或带查询参数的情况
	// 寻找 /uploads/ 的位置
	if idx := strings.Index(p, "/uploads/"); idx != -1 {
		p = p[idx+1:] // 保留 "uploads/..."
	}

	// 2. 去除查询参数（如 ?expires=...&sig=...）
	if idx := strings.Index(p, "?"); idx != -1 {
		p = p[:idx]
	}

	// 3. 规范化路径前缀
	p = strings.TrimPrefix(p, "/")

	// 如果不以 uploads/ 开头，但属于已知目录，则补全
	if !strings.HasPrefix(p, "uploads/") {
		if strings.HasPrefix(p, "merchants/") || strings.HasPrefix(p, "riders/") ||
			strings.HasPrefix(p, "operators/") || strings.HasPrefix(p, "reviews/") ||
			strings.HasPrefix(p, "public/") {
			p = "uploads/" + p
		}
	}

	return p
}
