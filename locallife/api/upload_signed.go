package api

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func (server *Server) serveDevUploadFile(ctx *gin.Context) {
	filepathParam := ctx.Param("filepath")
	trimmed := strings.TrimPrefix(filepathParam, "/")
	trimmed = filepath.Clean(trimmed)
	if trimmed == "." || strings.HasPrefix(trimmed, "..") || strings.Contains(trimmed, ".."+string(filepath.Separator)) {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrIllegalPath))
		return
	}

	normalized := normalizeStoredUploadPath("uploads/" + trimmed)
	if isPubliclyAccessibleUploadPath(normalized) {
		ctx.Header("Cache-Control", "public, max-age=300")
	} else {
		// local 文件存储仅用于开发环境；私有媒体访问已迁移为
		// media_asset_id -> /v1/media/private-access，再由 dev-only 文件路由返回下载地址。
		ctx.Header("Cache-Control", "private, max-age=60")
	}
	ctx.File(normalized)
}

func isPubliclyAccessibleUploadPath(normalized string) bool {
	if normalized == "" {
		return false
	}
	// 对外展示的公共素材目录（菜品/桌台/包间等）
	if strings.HasPrefix(normalized, "uploads/public/") {
		return true
	}
	// 商户 logo/门头照/环境照/菜品图 属于对外展示素材
	parts := strings.Split(normalized, "/")
	if len(parts) >= 5 && parts[0] == "uploads" && parts[1] == "merchants" {
		category := parts[3]
		if category == "logo" || category == "storefront" || category == "environment" || category == "dishes" {
			return true
		}
	}
	// 评价图片对外展示
	if strings.HasPrefix(normalized, "uploads/reviews/") {
		return true
	}
	return false
}

func normalizeStoredUploadPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	// 允许传入完整URL（此处不支持签名下载，直接拒绝）
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		return ""
	}
	p = strings.TrimPrefix(p, "/")
	if strings.HasPrefix(p, "uploads/") {
		return p
	}
	if strings.HasPrefix(p, "merchants/") || strings.HasPrefix(p, "riders/") || strings.HasPrefix(p, "operators/") || strings.HasPrefix(p, "reviews/") {
		return "uploads/" + p
	}
	return p
}

func (server *Server) hasActiveRole(ctx *gin.Context, uid int64, role string) (bool, error) {
	userRoles, err := server.store.ListUserRoles(ctx, uid)
	if err != nil {
		return false, err
	}
	for _, userRole := range userRoles {
		if userRole.Status != "active" {
			continue
		}
		if userRole.Role == role {
			return true, nil
		}
	}
	return false, nil
}
