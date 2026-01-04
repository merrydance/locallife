package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/merrydance/locallife/token"
)

type signUploadURLRequest struct {
	Path string `json:"path" binding:"required,min=1,max=2048"`
}

type signUploadURLResponse struct {
	URL     string `json:"url"`
	Expires int64  `json:"expires"`
}

// @Summary 生成上传文件的签名下载URL
// @Description 用于访问 uploads 下的敏感图片（证照/身份证/健康证等）。
// @Description
// @Description 说明：
// @Description - 公共展示素材通常可直接访问 /uploads/...（无需签名）。
// @Description - 私有/敏感图片必须先调用本接口获取短期签名URL，再使用该URL下载。
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body signUploadURLRequest true "签名请求（path 为 uploads 相对路径，如 uploads/merchants/1/licenses/xxx.jpg 或 merchants/1/licenses/xxx.jpg）"
// @Success 200 {object} signUploadURLResponse "签名URL与过期时间（Unix秒）"
// @Router /uploads/sign [post]
func (server *Server) signUploadURL(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	var req signUploadURLRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	normalized := normalizeStoredUploadPath(req.Path)
	if normalized == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("path参数必须是本地uploads相对路径")))
		return
	}
	if !isUploadPathOwnedByUser(normalized, authPayload.UserID) {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("无权访问该文件")))
		return
	}

	ttl := server.config.UploadURLTTL
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	expires := time.Now().Add(ttl).Unix()

	sig := server.signUploadAccess(authPayload.UserID, expires, normalized)

	// 对外访问路径保持为 /uploads/...（与历史一致）
	publicPath := strings.TrimPrefix(normalized, "uploads/")
	url := fmt.Sprintf("%s/uploads/%s?expires=%d&uid=%d&sig=%s",
		externalBaseURL(ctx),
		strings.TrimPrefix(publicPath, "/"),
		expires,
		authPayload.UserID,
		sig,
	)

	ctx.JSON(http.StatusOK, signUploadURLResponse{URL: url, Expires: expires})
}

func (server *Server) getSignedUpload(ctx *gin.Context) {
	filepathParam := ctx.Param("filepath")
	trimmed := strings.TrimPrefix(filepathParam, "/")
	trimmed = filepath.Clean(trimmed)
	if trimmed == "." || strings.HasPrefix(trimmed, "..") || strings.Contains(trimmed, ".."+string(filepath.Separator)) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("非法路径")))
		return
	}

	normalized := normalizeStoredUploadPath("uploads/" + trimmed)
	if isPubliclyAccessibleUploadPath(normalized) {
		ctx.Header("Cache-Control", "public, max-age=300")
		ctx.File(normalized)
		return
	}

	expiresStr := ctx.Query("expires")
	uidStr := ctx.Query("uid")
	sig := ctx.Query("sig")
	if expiresStr == "" || uidStr == "" || sig == "" {
		ctx.JSON(http.StatusUnauthorized, errorResponse(errors.New("缺少签名参数")))
		return
	}

	expires, err := strconv.ParseInt(expiresStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("expires参数错误")))
		return
	}
	uid, err := strconv.ParseInt(uidStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("uid参数错误")))
		return
	}
	if time.Now().Unix() > expires {
		ctx.JSON(http.StatusUnauthorized, errorResponse(errors.New("签名已过期")))
		return
	}

	if !isUploadPathOwnedByUser(normalized, uid) {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("无权访问该文件")))
		return
	}

	expected := server.signUploadAccess(uid, expires, normalized)
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		ctx.JSON(http.StatusUnauthorized, errorResponse(errors.New("签名无效")))
		return
	}

	ctx.Header("Cache-Control", "private, max-age=60")
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
	// 商户 logo/门头照/环境照 属于对外展示素材
	parts := strings.Split(normalized, "/")
	if len(parts) >= 5 && parts[0] == "uploads" && parts[1] == "merchants" {
		category := parts[3]
		if category == "logo" || category == "storefront" || category == "environment" {
			return true
		}
	}
	// 评价图片一般需要对外展示
	if strings.HasPrefix(normalized, "uploads/reviews/") {
		return true
	}

	return false
}

func (server *Server) signUploadAccess(uid int64, expires int64, normalizedPath string) string {
	key := server.config.UploadURLSigningKey
	if key == "" {
		key = server.config.TokenSymmetricKey
	}
	msg := fmt.Sprintf("%d|%d|%s", uid, expires, normalizedPath)
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(msg))
	sum := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(sum)
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

func isUploadPathOwnedByUser(normalized string, uid int64) bool {
	if normalized == "" {
		return false
	}
	// 对外展示的公共素材：允许任意已登录用户签名访问（仍然需要短期签名）
	if strings.HasPrefix(normalized, "uploads/public/") {
		return true
	}
	// 商户证照对所有登录用户公开可见（营业执照、食品经营许可证）
	// 路径格式: uploads/merchants/{id}/business_license/... 或 uploads/merchants/{id}/food_permit/...
	if strings.Contains(normalized, "/merchants/") &&
		(strings.Contains(normalized, "/business_license/") || strings.Contains(normalized, "/food_permit/")) {
		return true
	}
	uidStr := fmt.Sprintf("/%d/", uid)
	return strings.Contains(normalized, "/merchants"+uidStr) ||
		strings.Contains(normalized, "/riders"+uidStr) ||
		strings.Contains(normalized, "/operators"+uidStr) ||
		strings.Contains(normalized, "/reviews"+uidStr)
}
