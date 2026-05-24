package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/media"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/wechat"
)

type updateReviewRequest struct {
	Content       string  `json:"content" binding:"required,min=1,max=1000"`
	MediaAssetIDs []int64 `json:"media_asset_ids,omitempty" binding:"omitempty,max=9,dive,min=1"`
}

var errInvalidReviewImageAsset = errors.New("invalid review image asset")

// updateOwnReview 更新本人评价
// @Summary 更新本人评价
// @Description 用户更新自己创建的评价内容和图片。内容会重新进行文本安全检测。
// @Tags 评价管理
// @Accept json
// @Produce json
// @Param id path int true "评价ID"
// @Param request body updateReviewRequest true "评价更新信息"
// @Success 200 {object} reviewResponse "评价更新成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "只能更新自己的评价"
// @Failure 404 {object} ErrorResponse "评价不存在"
// @Failure 502 {object} ErrorResponse "内容安全检测失败"
// @Failure 500 {object} ErrorResponse "内部错误"
// @Router /v1/reviews/{id} [patch]
// @Security BearerAuth
func (server *Server) updateOwnReview(ctx *gin.Context) {
	var uri struct {
		ID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req updateReviewRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	review, err := server.store.GetReview(ctx, uri.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("review not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if review.UserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you can only update your own review")))
		return
	}

	user, err := server.store.GetUser(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("user not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if strings.TrimSpace(user.WechatOpenid) == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("missing wechat openid")))
		return
	}
	if err := server.wechatClient.MsgSecCheck(ctx, user.WechatOpenid, 2, req.Content); err != nil {
		if errors.Is(err, wechat.ErrRiskyTextContent) {
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrTextContentSafetyFailed))
			return
		}
		ctx.JSON(http.StatusBadGateway, internalError(ctx, fmt.Errorf("wechat msg sec check: %w", err)))
		return
	}

	if err := server.validateReviewImageAssets(ctx, req.MediaAssetIDs, authPayload.UserID); err != nil {
		if errors.Is(err, errInvalidReviewImageAsset) {
			ctx.JSON(http.StatusBadRequest, errorResponse(errInvalidReviewImageAsset))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	result, err := server.store.UpdateReviewTx(ctx, db.UpdateReviewTxParams{
		ID:            uri.ID,
		Content:       req.Content,
		MediaAssetIDs: req.MediaAssetIDs,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := newReviewResponse(result.Review)
	resp.ImageAssetIDs = req.MediaAssetIDs
	resp.ImageURLs = server.orderedOwnerReviewImageURLs(ctx, req.MediaAssetIDs, authPayload.UserID)
	ctx.JSON(http.StatusOK, resp)
}

func (server *Server) validateReviewImageAssets(ctx *gin.Context, mediaAssetIDs []int64, uploaderID int64) error {
	if len(mediaAssetIDs) == 0 {
		return nil
	}

	seen := make(map[int64]struct{}, len(mediaAssetIDs))
	for _, assetID := range mediaAssetIDs {
		if _, ok := seen[assetID]; ok {
			return errInvalidReviewImageAsset
		}
		seen[assetID] = struct{}{}
	}

	assets, err := server.store.ListMediaAssetsByIDs(ctx, mediaAssetIDs)
	if err != nil {
		return err
	}
	if len(assets) != len(mediaAssetIDs) {
		return errInvalidReviewImageAsset
	}

	byID := make(map[int64]db.ListMediaAssetsByIDsRow, len(assets))
	for _, asset := range assets {
		byID[asset.ID] = asset
	}
	for _, assetID := range mediaAssetIDs {
		asset, ok := byID[assetID]
		if !ok {
			return errInvalidReviewImageAsset
		}
		if asset.UploadedBy != uploaderID ||
			asset.MediaCategory != string(media.CategoryReviewImage) ||
			asset.Visibility != string(media.VisibilityPublic) ||
			asset.UploadStatus != "confirmed" {
			return errInvalidReviewImageAsset
		}
		if asset.ModerationStatus != "pending" && asset.ModerationStatus != "approved" {
			return errInvalidReviewImageAsset
		}
	}

	return nil
}

func (server *Server) orderedOwnerReviewImageURLs(ctx *gin.Context, assetIDs []int64, ownerID int64) []string {
	urlMap := server.batchOwnerVisibleReviewImageURLs(ctx, assetIDs, media.VariantOriginal, ownerID)
	urls := make([]string, 0, len(assetIDs))
	for _, id := range assetIDs {
		if u, ok := urlMap[id]; ok {
			urls = append(urls, u)
		}
	}
	return urls
}

func (server *Server) currentUserHasActiveRole(ctx *gin.Context, userID int64, role string) (bool, error) {
	userRoles, ok := GetUserRolesFromContext(ctx)
	if !ok {
		var err error
		userRoles, err = server.store.ListUserRoles(ctx, userID)
		if err != nil {
			return false, err
		}
		ctx.Set(userRolesKey, userRoles)
	}
	for _, userRole := range userRoles {
		if userRole.Status == "active" && userRole.Role == role {
			return true, nil
		}
	}
	return false, nil
}
