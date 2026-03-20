package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/media"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/wechat"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

// ==================== 评价管理 ====================

// ==================== 请求/响应结构体 ====================

type createReviewRequest struct {
	OrderID int64    `json:"order_id" binding:"required,min=1"`
	Content string   `json:"content" binding:"required,min=1,max=1000"`
	Images  []string `json:"images,omitempty" binding:"omitempty,max=9,dive,min=1,max=500"` // 最多9张图片（本地 uploads 相对路径）
}

type reviewResponse struct {
	ID                  int64   `json:"id"`
	OrderID             int64   `json:"order_id"`
	UserID              int64   `json:"user_id"`
	MerchantID          int64   `json:"merchant_id"`
	MerchantName        string  `json:"merchant_name,omitempty"`
	MerchantLogoAssetID *int64  `json:"-"`
	MerchantLogoURL     string  `json:"merchant_logo_url,omitempty"`
	Content             string  `json:"content"`
	IsVisible           bool    `json:"is_visible"`
	MerchantReply       *string `json:"merchant_reply,omitempty"`
	RepliedAt           *string `json:"replied_at,omitempty"`
	CreatedAt           string  `json:"created_at"`
}

type reviewListResponse struct {
	Reviews  []reviewResponse `json:"reviews"`
	Total    int64            `json:"total"`
	PageID   int32            `json:"page_id"`
	PageSize int32            `json:"page_size"`
}

type replyReviewRequest struct {
	Reply string `json:"reply" binding:"required,min=1,max=500"`
}

type listReviewsRequest struct {
	PageID   int32 `form:"page_id" binding:"required,min=1"`
	PageSize int32 `form:"page_size" binding:"required,min=5,max=50"`
}

// ==================== Handler实现 ====================

// createReview 创建评价
// @Summary 创建评价
// @Description 用户为已完成的订单创建评价。
// @Tags 评价管理
// @Accept json
// @Produce json
// @Param request body createReviewRequest true "评价信息"
// @Success 200 {object} reviewResponse "评价创建成功"
// @Failure 400 {object} ErrorResponse "参数错误或订单状态不允许评价"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "订单不属于当前用户"
// @Failure 404 {object} ErrorResponse "订单不存在"
// @Failure 409 {object} ErrorResponse "订单已评价"
// @Failure 500 {object} ErrorResponse "内部错误"
// @Router /v1/reviews [post]
// @Security BearerAuth
func (server *Server) createReview(ctx *gin.Context) {
	var req createReviewRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取当前用户
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 1. 验证订单存在且属于该用户
	order, err := server.store.GetOrder(ctx, req.OrderID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("order not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证订单所有者
	if order.UserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("order does not belong to user")))
		return
	}

	// 2. 验证订单已完成
	if order.Status != OrderStatusCompleted {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("only completed orders can be reviewed")))
		return
	}

	// 3. 检查是否已评价
	_, err = server.store.GetReviewByOrderID(ctx, req.OrderID)
	if err == nil {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("order already reviewed")))
		return
	}
	if !isNotFoundError(err) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 4. 默认可见
	isVisible := true

	// 4.1 评价文本内容安全检测：先审后存
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

	// TODO(media-service): images now stored separately in review_images table
	// if len(req.Images) > 0 { ... }

	// 5. 创建评价
	review, err := server.store.CreateReview(ctx, db.CreateReviewParams{
		OrderID:    req.OrderID,
		UserID:     authPayload.UserID,
		MerchantID: order.MerchantID,
		Content:    req.Content,
		IsVisible:  isVisible,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusCreated, newReviewResponse(review))
}

// getReview 获取评价详情
// @Summary 获取评价详情
// @Description 获取指定评价的详细信息
// @Tags 评价管理
// @Accept json
// @Produce json
// @Param id path int true "评价ID"
// @Success 201 {object} reviewResponse "评价详情"
// @Failure 400 {object} ErrorResponse "无效的评价ID"
// @Failure 404 {object} ErrorResponse "评价不存在"
// @Failure 500 {object} ErrorResponse "内部错误"
// @Router /v1/reviews/{id} [get]
// @Security BearerAuth
func (server *Server) getReview(ctx *gin.Context) {
	var uri struct {
		ID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	review, err := server.store.GetReview(ctx, uri.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("review not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newReviewResponse(review))
}

// getReviewByOrder 根据订单ID获取评价
func (server *Server) getReviewByOrder(ctx *gin.Context) {
	var uri struct {
		ID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	review, err := server.store.GetReviewByOrderID(ctx, uri.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("no review found for this order")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newReviewResponse(review))
}

// listMerchantReviews 获取商户评价列表（顾客视角）
// @Summary 获取商户评价列表（公开）
// @Description 获取商户的可见评价列表，顾客视角，仅展示高信用用户的评价
// @Tags 评价管理
// @Accept json
// @Produce json
// @Param id path int true "商户ID"
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(5) maximum(50)
// @Success 200 {object} reviewListResponse "评价列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 500 {object} ErrorResponse "内部错误"
// @Router /v1/reviews/merchants/{id} [get]
// @Security BearerAuth
func (server *Server) listMerchantReviews(ctx *gin.Context) {
	var uri struct {
		MerchantID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req listReviewsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 只返回可见的评价
	reviews, err := server.store.ListReviewsByMerchant(ctx, db.ListReviewsByMerchantParams{
		MerchantID: uri.MerchantID,
		Limit:      req.PageSize,
		Offset:     pageOffset(req.PageID, req.PageSize),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取总数
	count, err := server.store.CountReviewsByMerchant(ctx, uri.MerchantID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	response := reviewListResponse{
		Reviews:  newReviewListResponse(reviews),
		Total:    count,
		PageID:   req.PageID,
		PageSize: req.PageSize,
	}
	ctx.JSON(http.StatusOK, response)
}

// listMerchantAllReviews 获取商户所有评价（商户视角）
// @Summary 获取商户所有评价（商户专用）
// @Description 商户查看自己店铺的所有评价，包括低信用用户的隐藏评价
// @Tags 评价管理-商户
// @Accept json
// @Produce json
// @Param id path int true "商户ID"
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(5) maximum(50)
// @Success 200 {object} reviewListResponse "评价列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "无权访问该商户"
// @Failure 500 {object} ErrorResponse "内部错误"
// @Router /v1/reviews/merchants/{id}/all [get]
// @Security BearerAuth
func (server *Server) listMerchantAllReviews(ctx *gin.Context) {
	var uri struct {
		MerchantID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req listReviewsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("merchant not loaded, ensure MerchantStaffMiddleware is applied")))
		return
	}

	// 验证请求的商户ID是否匹配
	if merchant.ID != uri.MerchantID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not authorized to access this merchant")))
		return
	}

	// 返回所有评价（包含不可见的）
	reviews, err := server.store.ListAllReviewsByMerchant(ctx, db.ListAllReviewsByMerchantParams{
		MerchantID: uri.MerchantID,
		Limit:      req.PageSize,
		Offset:     pageOffset(req.PageID, req.PageSize),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取总数
	count, err := server.store.CountAllReviewsByMerchant(ctx, uri.MerchantID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	response := reviewListResponse{
		Reviews:  newReviewListResponse(reviews),
		Total:    count,
		PageID:   req.PageID,
		PageSize: req.PageSize,
	}
	ctx.JSON(http.StatusOK, response)
}

// listUserReviews 获取用户的评价列表
// @Summary 获取我的评价列表
// @Description 获取当前用户创建的所有评价
// @Tags 评价管理
// @Accept json
// @Produce json
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(5) maximum(50)
// @Success 200 {object} reviewListResponse "评价列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 500 {object} ErrorResponse "内部错误"
// @Router /v1/reviews/me [get]
// @Security BearerAuth
func (server *Server) listUserReviews(ctx *gin.Context) {
	var req listReviewsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取当前用户
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	reviews, err := server.store.ListReviewsByUser(ctx, db.ListReviewsByUserParams{
		UserID: authPayload.UserID,
		Limit:  req.PageSize,
		Offset: pageOffset(req.PageID, req.PageSize),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取总数
	count, err := server.store.CountReviewsByUser(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	response := reviewListResponse{
		Reviews:  newListReviewByUserResponse(reviews),
		Total:    count,
		PageID:   req.PageID,
		PageSize: req.PageSize,
	}
	// 批量填充商户 Logo URL
	logoAssetIDs := make([]int64, 0, len(response.Reviews))
	for _, r := range response.Reviews {
		if r.MerchantLogoAssetID != nil {
			logoAssetIDs = append(logoAssetIDs, *r.MerchantLogoAssetID)
		}
	}
	if len(logoAssetIDs) > 0 {
		logoURLs := server.batchPublicImageURLs(ctx, logoAssetIDs, media.VariantCard)
		for i := range response.Reviews {
			if response.Reviews[i].MerchantLogoAssetID != nil {
				response.Reviews[i].MerchantLogoURL = logoURLs[*response.Reviews[i].MerchantLogoAssetID]
			}
		}
	}
	ctx.JSON(http.StatusOK, response)
}

// replyReview 商户回复评价
// @Summary 回复评价（商户）
// @Description 商户回复顾客的评价
// @Tags 评价管理-商户
// @Accept json
// @Produce json
// @Param id path int true "评价ID"
// @Param request body replyReviewRequest true "回复内容"
// @Success 200 {object} reviewResponse "回复成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "无权回复此评价"
// @Failure 404 {object} ErrorResponse "评价不存在"
// @Failure 500 {object} ErrorResponse "内部错误"
// @Router /v1/reviews/{id}/reply [post]
// @Security BearerAuth
func (server *Server) replyReview(ctx *gin.Context) {
	var uri struct {
		ID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req replyReviewRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 1. 查询评价
	review, err := server.store.GetReview(ctx, uri.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("review not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("merchant not loaded, ensure MerchantStaffMiddleware is applied")))
		return
	}

	// 验证评价属于该商户
	if merchant.ID != review.MerchantID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("review does not belong to your merchant")))
		return
	}

	// 2.1 回复文本内容安全检测：先审后存
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	merchantUser, err := server.store.GetUser(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("user not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if strings.TrimSpace(merchantUser.WechatOpenid) == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("missing wechat openid")))
		return
	}
	if err := server.wechatClient.MsgSecCheck(ctx, merchantUser.WechatOpenid, 2, req.Reply); err != nil {
		if errors.Is(err, wechat.ErrRiskyTextContent) {
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrTextContentSafetyFailed))
			return
		}
		ctx.JSON(http.StatusBadGateway, internalError(ctx, fmt.Errorf("wechat msg sec check: %w", err)))
		return
	}

	// 3. 更新回复
	updatedReview, err := server.store.UpdateMerchantReply(ctx, db.UpdateMerchantReplyParams{
		ID:            uri.ID,
		MerchantReply: pgtype.Text{String: req.Reply, Valid: true},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newReviewResponse(updatedReview))
}

// deleteReview 删除评价（运营商）
// @Summary 删除评价（运营商）
// @Description 运营商删除违规评价，只能删除自己管辖区域商户的评价
// @Tags 评价管理-运营商
// @Accept json
// @Produce json
// @Param id path int true "评价ID"
// @Success 200 {object} successMessageResponse "删除成功"
// @Failure 400 {object} ErrorResponse "无效的评价ID"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "只能删除管辖区域商户的评价"
// @Failure 404 {object} ErrorResponse "评价不存在"
// @Failure 500 {object} ErrorResponse "内部错误"
// @Router /v1/reviews/{id} [delete]
// @Security BearerAuth
func (server *Server) deleteReview(ctx *gin.Context) {
	var uri struct {
		ID int64 `uri:"id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uri); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 获取评价信息
	review, err := server.store.GetReview(ctx, uri.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("review not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取商户信息以验证区域
	merchant, err := server.store.GetMerchant(ctx, review.MerchantID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证 operator 是否管理该商户的区域
	if _, err := server.checkOperatorManagesRegion(ctx, merchant.RegionID); err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you can only delete reviews for merchants in your region")))
		return
	}

	// 删除评价
	err = server.store.DeleteReview(ctx, uri.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, successMessage("review deleted successfully"))
}

// ==================== 辅助函数 ====================

func newReviewResponse(review db.Review) reviewResponse {
	resp := reviewResponse{
		ID:         review.ID,
		OrderID:    review.OrderID,
		UserID:     review.UserID,
		MerchantID: review.MerchantID,
		Content:    review.Content,
		IsVisible:  review.IsVisible,
		CreatedAt:  review.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	// 处理可选字段
	if review.MerchantReply.Valid {
		resp.MerchantReply = &review.MerchantReply.String
	}
	if review.RepliedAt.Valid {
		repliedAt := review.RepliedAt.Time.Format("2006-01-02T15:04:05Z07:00")
		resp.RepliedAt = &repliedAt
	}

	return resp
}

func newReviewListResponse(reviews []db.Review) []reviewResponse {
	responses := make([]reviewResponse, len(reviews))
	for i, review := range reviews {
		responses[i] = newReviewResponse(review)
	}
	return responses
}

func newListReviewByUserResponse(reviews []db.ListReviewsByUserRow) []reviewResponse {
	responses := make([]reviewResponse, len(reviews))
	for i, r := range reviews {
		resp := reviewResponse{
			ID:           r.ID,
			OrderID:      r.OrderID,
			UserID:       r.UserID,
			MerchantID:   r.MerchantID,
			MerchantName: r.MerchantName,
			Content:      r.Content,
			IsVisible:    r.IsVisible,
			CreatedAt:    r.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if r.MerchantLogoMediaAssetID.Valid {
			v := r.MerchantLogoMediaAssetID.Int64
			resp.MerchantLogoAssetID = &v
		}

		if r.MerchantReply.Valid {
			resp.MerchantReply = &r.MerchantReply.String
		}
		if r.RepliedAt.Valid {
			repliedAt := r.RepliedAt.Time.Format("2006-01-02T15:04:05Z07:00")
			resp.RepliedAt = &repliedAt
		}
		responses[i] = resp
	}
	return responses
}
