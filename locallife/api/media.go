package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/merrydance/locallife/media"
	"github.com/merrydance/locallife/token"
)

// ==================== 请求/响应结构 ====================

type createUploadSessionRequest struct {
	BusinessType   string `json:"business_type" binding:"required"`
	MediaCategory  string `json:"media_category" binding:"required"`
	ContentType    string `json:"content_type" binding:"required"`
	ContentLength  int64  `json:"content_length" binding:"required,min=1"`
	ChecksumSha256 string `json:"checksum_sha256" binding:"required,len=64"`
}

type uploadSessionResponse struct {
	UploadID   string            `json:"upload_id"`
	ObjectKey  string            `json:"object_key"`
	Visibility string            `json:"visibility"`
	UploadHost string            `json:"upload_host"`
	Form       map[string]string `json:"form"`
	ExpireAt   time.Time         `json:"expire_at"`
}

type completeUploadRequest struct {
	UploadID  string `json:"upload_id" binding:"required"`
	ObjectKey string `json:"object_key" binding:"required"`
	ETag      string `json:"etag"`
}

type completeUploadResponse struct {
	MediaID  int64             `json:"media_id"`
	Variants map[string]string `json:"urls"`
	Status   string            `json:"status"`
}

type privateAccessRequest struct {
	MediaID int64  `json:"media_id" binding:"required,min=1"`
	Reason  string `json:"reason"`
}

type privateAccessResponse struct {
	DownloadURL string    `json:"download_url"`
	ExpireAt    time.Time `json:"expire_at"`
}

type mediaAssetResponse struct {
	ID               int64             `json:"id"`
	Visibility       string            `json:"visibility"`
	MediaCategory    string            `json:"media_category"`
	MimeType         string            `json:"mime_type"`
	FileSize         int64             `json:"file_size"`
	UploadStatus     string            `json:"upload_status"`
	ModerationStatus string            `json:"moderation_status"`
	Variants         map[string]string `json:"urls,omitempty"`
}

// ==================== Handlers ====================

// createMediaUploadSession godoc
// @Summary 申请媒体直传会话
// @Tags media
// @Accept json
// @Produce json
// @Param body body createUploadSessionRequest true "申请参数"
// @Success 201 {object} uploadSessionResponse
// @Router /v1/media/upload-sessions [post]
func (server *Server) createMediaUploadSession(ctx *gin.Context) {
	var req createUploadSessionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	sourceClient := ctx.GetHeader("X-Source-Client")
	if sourceClient == "" {
		sourceClient = "unknown"
	}

	expireIn := server.config.MediaDirectUploadExpire
	if expireIn <= 0 {
		expireIn = 30 * time.Minute
	}

	result, err := server.mediaRegistry.CreateUploadSession(ctx, media.UploadSessionRequest{
		UserID:         authPayload.UserID,
		BusinessType:   req.BusinessType,
		Category:       media.Category(req.MediaCategory),
		ContentType:    req.ContentType,
		ContentLength:  req.ContentLength,
		ChecksumSha256: req.ChecksumSha256,
		SourceClient:   sourceClient,
		ExpireIn:       expireIn,
		MaxFileBytes:   server.config.MediaMaxUploadBytes,
	})
	if err != nil {
		if errors.Is(err, media.ErrInvalidCategory) || errors.Is(err, media.ErrUnsupportedContentType) || errors.Is(err, media.ErrFileTooLarge) {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusCreated, uploadSessionResponse{
		UploadID:   result.Session.ID,
		ObjectKey:  result.Session.ObjectKey,
		Visibility: result.Session.Visibility,
		UploadHost: result.UploadResult.UploadHost,
		Form:       result.UploadResult.FormFields,
		ExpireAt:   result.Session.ExpireAt,
	})
}

// completeMediaUpload godoc
// @Summary 确认媒体上传完成
// @Tags media
// @Accept json
// @Produce json
// @Param body body completeUploadRequest true "确认参数"
// @Success 200 {object} completeUploadResponse
// @Router /v1/media/complete [post]
func (server *Server) completeMediaUpload(ctx *gin.Context) {
	var req completeUploadRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	result, err := server.mediaRegistry.CompleteUpload(ctx, media.CompleteRequest{
		UploadID:  req.UploadID,
		ObjectKey: req.ObjectKey,
		ETag:      req.ETag,
		UserID:    authPayload.UserID,
	})
	if err != nil {
		switch {
		case errors.Is(err, media.ErrUploadSessionNotFound):
			ctx.JSON(http.StatusNotFound, errorResponse(err))
		case errors.Is(err, media.ErrUploadSessionExpired):
			ctx.JSON(http.StatusGone, errorResponse(err))
		case errors.Is(err, media.ErrUnauthorized):
			ctx.JSON(http.StatusForbidden, errorResponse(err))
		case errors.Is(err, media.ErrUploadNotConfirmed):
			ctx.JSON(http.StatusUnprocessableEntity, errorResponse(err))
		default:
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		}
		return
	}

	asset := result.Asset
	if err := server.triggerMediaModeration(ctx, &asset, authPayload.UserID); err != nil {
		ctx.JSON(http.StatusBadGateway, internalError(ctx, err))
		return
	}
	variants := server.publicVariantsForAsset(asset)
	if variants == nil {
		variants = map[string]string{}
	}

	ctx.JSON(http.StatusOK, completeUploadResponse{
		MediaID:  asset.ID,
		Variants: variants,
		Status:   asset.ModerationStatus,
	})
}

// getMediaPrivateAccess godoc
// @Summary 获取私有媒体短期访问地址
// @Tags media
// @Accept json
// @Produce json
// @Param body body privateAccessRequest true "请求参数"
// @Success 200 {object} privateAccessResponse
// @Router /v1/media/private-access [post]
func (server *Server) getMediaPrivateAccess(ctx *gin.Context) {
	var req privateAccessRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 查询资产、归属校验
	asset, err := server.mediaRegistry.GetAsset(ctx, req.MediaID)
	if err != nil {
		if errors.Is(err, media.ErrAssetNotFound) || errors.Is(err, media.ErrAssetDeleted) {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if asset.UploadedBy != authPayload.UserID {
		if isOwnerOnlyPrivateMedia(asset.MediaCategory) {
			ctx.JSON(http.StatusForbidden, errorResponse(media.ErrUnauthorized))
			return
		}

		// 平台管理员可以访问任意私有资产（用于审核场景）
		isAdmin, err := server.hasActiveRole(ctx, authPayload.UserID, RoleAdmin)
		if err != nil || !isAdmin {
			ctx.JSON(http.StatusForbidden, errorResponse(media.ErrUnauthorized))
			return
		}
	}

	ttl := server.config.PrivateDownloadURLTTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	downloadURL, err := server.mediaRegistry.CreatePrivateAccessURL(ctx, req.MediaID, ttl)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, privateAccessResponse{
		DownloadURL: downloadURL,
		ExpireAt:    time.Now().Add(ttl),
	})
}

func isOwnerOnlyPrivateMedia(category string) bool {
	switch category {
	case string(media.CategoryIDCardFront), string(media.CategoryIDCardBack):
		return true
	default:
		return false
	}
}

// deleteMediaAsset godoc
// @Summary 软删除媒体资产
// @Tags media
// @Produce json
// @Param id path int true "media_asset_id"
// @Success 204
// @Router /v1/media/{id} [delete]
func (server *Server) deleteMediaAsset(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid media id")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	if err := server.mediaRegistry.SoftDelete(ctx, id, authPayload.UserID); err != nil {
		switch {
		case errors.Is(err, media.ErrAssetNotFound):
			ctx.JSON(http.StatusNotFound, errorResponse(err))
		case errors.Is(err, media.ErrUnauthorized):
			ctx.JSON(http.StatusForbidden, errorResponse(err))
		default:
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		}
		return
	}

	ctx.Status(http.StatusNoContent)
}

// getMediaAsset godoc
// @Summary 获取媒体资产元数据
// @Tags media
// @Produce json
// @Param id path int true "media_asset_id"
// @Success 200 {object} mediaAssetResponse
// @Router /v1/media/{id} [get]
func (server *Server) getMediaAsset(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("invalid media id")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	asset, err := server.mediaRegistry.GetAsset(ctx, id)
	if err != nil {
		if errors.Is(err, media.ErrAssetNotFound) || errors.Is(err, media.ErrAssetDeleted) {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 私有资产需归属校验
	if asset.Visibility == string(media.VisibilityPrivate) && asset.UploadedBy != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(media.ErrUnauthorized))
		return
	}

	resp := mediaAssetResponse{
		ID:               asset.ID,
		Visibility:       asset.Visibility,
		MediaCategory:    asset.MediaCategory,
		MimeType:         asset.MimeType,
		FileSize:         asset.FileSize,
		UploadStatus:     asset.UploadStatus,
		ModerationStatus: asset.ModerationStatus,
	}
	resp.Variants = server.publicVariantsForAsset(asset)

	ctx.JSON(http.StatusOK, resp)
}
