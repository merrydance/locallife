package api

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

// ==================== 代金券模板管理 ====================

// createVoucherRequest 创建代金券请求
type createVoucherRequest struct {
	Code              string    `json:"code" binding:"required,min=1,max=50"`
	Name              string    `json:"name" binding:"required,min=1,max=100"`
	Description       string    `json:"description" binding:"max=500"`
	Amount            int64     `json:"amount" binding:"required,min=1,max=100000000"`       // 最大100万元(分)
	MinOrderAmount    int64     `json:"min_order_amount" binding:"min=0,max=100000000"`      // 最大100万元(分)
	TotalQuantity     int32     `json:"total_quantity" binding:"required,min=1,max=1000000"` // 最大100万张
	ValidFrom         time.Time `json:"valid_from" binding:"required"`
	ValidUntil        time.Time `json:"valid_until" binding:"required"`
	AllowedOrderTypes []string  `json:"allowed_order_types"` // 允许的订单类型: takeout, dine_in, takeaway, reservation
}

// voucherResponse 代金券响应
type voucherResponse struct {
	ID                int64     `json:"id"`
	MerchantID        int64     `json:"merchant_id"`
	Code              string    `json:"code"`
	Name              string    `json:"name"`
	Description       string    `json:"description,omitempty"`
	Amount            int64     `json:"amount"`
	MinOrderAmount    int64     `json:"min_order_amount"`
	TotalQuantity     int32     `json:"total_quantity"`
	ClaimedQuantity   int32     `json:"claimed_quantity"`
	UsedQuantity      int32     `json:"used_quantity"`
	ValidFrom         time.Time `json:"valid_from"`
	ValidUntil        time.Time `json:"valid_until"`
	IsActive          bool      `json:"is_active"`
	AllowedOrderTypes []string  `json:"allowed_order_types"` // 允许的订单类型
	CreatedAt         time.Time `json:"created_at"`
}

// createVoucherURIRequest 创建代金券URI参数
type createVoucherURIRequest struct {
	MerchantID int64 `uri:"id" binding:"required,min=1"`
}

// createVoucher godoc
// @Summary 创建代金券
// @Description 商户创建代金券模板（仅商户可操作）
// @Tags 代金券管理-商户
// @Accept json
// @Produce json
// @Param id path int true "商户ID"
// @Param request body createVoucherRequest true "代金券信息"
// @Success 200 {object} voucherResponse "创建成功"
// @Failure 400 {object} ErrorResponse "参数错误或时间范围无效"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非商户用户或无权操作此商户"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchants/{id}/vouchers [post]
// @Security BearerAuth
func (server *Server) createVoucher(ctx *gin.Context) {
	var uriReq createVoucherURIRequest
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req createVoucherRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 验证商户权限
	merchantID, err := server.getMerchantIDByUser(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("merchant role required")))
		return
	}

	// 验证URL中的商户ID与用户的商户ID一致
	if merchantID != uriReq.MerchantID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not authorized for this merchant")))
		return
	}

	// 验证时间范围
	if req.ValidUntil.Before(req.ValidFrom) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("valid_until must be after valid_from")))
		return
	}

	// 验证允许的订单类型（如果未指定，默认允许所有类型）
	allowedTypes := req.AllowedOrderTypes
	if len(allowedTypes) == 0 {
		allowedTypes = []string{"takeout", "dine_in", "takeaway", "reservation"}
	} else {
		// 验证输入的订单类型是否合法
		validTypes := map[string]bool{"takeout": true, "dine_in": true, "takeaway": true, "reservation": true}
		for _, t := range allowedTypes {
			if !validTypes[t] {
				ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("无效的订单类型: %s，允许的类型: takeout, dine_in, takeaway, reservation", t)))
				return
			}
		}
	}

	voucher, err := server.store.CreateVoucher(ctx, db.CreateVoucherParams{
		MerchantID:        merchantID,
		Code:              req.Code,
		Name:              req.Name,
		Description:       pgtype.Text{String: req.Description, Valid: req.Description != ""},
		Amount:            req.Amount,
		MinOrderAmount:    req.MinOrderAmount,
		TotalQuantity:     req.TotalQuantity,
		ValidFrom:         req.ValidFrom,
		ValidUntil:        req.ValidUntil,
		IsActive:          true,
		AllowedOrderTypes: allowedTypes,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	rsp := convertVoucherResponse(voucher)
	ctx.JSON(http.StatusOK, rsp)
}

// listMerchantVouchersRequest 获取商户代金券列表请求
type listMerchantVouchersURIRequest struct {
	MerchantID int64 `uri:"id" binding:"required,min=1"`
}

type listMerchantVouchersQueryRequest struct {
	PageID   int32 `form:"page_id" binding:"required,min=1"`
	PageSize int32 `form:"page_size" binding:"required,min=5,max=50"`
}

// listMerchantVouchers godoc
// @Summary 获取商户代金券列表
// @Description 获取指定商户的所有代金券（包含所有状态）
// @Tags 代金券管理-商户
// @Accept json
// @Produce json
// @Param id path int true "商户ID"
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(5) maximum(50)
// @Success 200 {array} voucherResponse "代金券列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchants/{id}/vouchers [get]
// @Security BearerAuth
func (server *Server) listMerchantVouchers(ctx *gin.Context) {
	var uriReq listMerchantVouchersURIRequest
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	var queryReq listMerchantVouchersQueryRequest
	if err := ctx.ShouldBindQuery(&queryReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	vouchers, err := server.store.ListMerchantVouchers(ctx, db.ListMerchantVouchersParams{
		MerchantID: uriReq.MerchantID,
		Limit:      queryReq.PageSize,
		Offset:     (queryReq.PageID - 1) * queryReq.PageSize,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	rsp := make([]voucherResponse, len(vouchers))
	for i, v := range vouchers {
		rsp[i] = convertVoucherResponse(v)
	}

	ctx.JSON(http.StatusOK, rsp)
}

// listActiveVouchersRequest 获取可领取代金券请求
type listActiveVouchersRequest struct {
	MerchantID int64 `uri:"id" binding:"required,min=1"`
	PageID     int32 `form:"page_id" binding:"required,min=1"`
	PageSize   int32 `form:"page_size" binding:"required,min=5,max=50"`
}

// listActiveVouchers godoc
// @Summary 获取可领取代金券列表
// @Description 获取商户当前可领取的代金券（已激活、在有效期内、未领完）
// @Tags 代金券管理
// @Accept json
// @Produce json
// @Param id path int true "商户ID"
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(5) maximum(50)
// @Success 200 {array} voucherResponse "可领取代金券列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchants/{id}/vouchers/active [get]
// @Security BearerAuth
func (server *Server) listActiveVouchers(ctx *gin.Context) {
	var req listActiveVouchersRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	vouchers, err := server.store.ListActiveVouchers(ctx, db.ListActiveVouchersParams{
		MerchantID: req.MerchantID,
		Limit:      req.PageSize,
		Offset:     (req.PageID - 1) * req.PageSize,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	rsp := make([]voucherResponse, len(vouchers))
	for i, v := range vouchers {
		rsp[i] = convertVoucherResponse(v)
	}

	ctx.JSON(http.StatusOK, rsp)
}

// updateVoucherURIRequest 更新代金券URI参数
type updateVoucherURIRequest struct {
	MerchantID int64 `uri:"id" binding:"required,min=1"`
	VoucherID  int64 `uri:"voucher_id" binding:"required,min=1"`
}

// updateVoucherRequest 更新代金券请求
type updateVoucherRequest struct {
	Name              *string    `json:"name" binding:"omitempty,min=1,max=100"`
	Description       *string    `json:"description" binding:"omitempty,max=500"`
	Amount            *int64     `json:"amount" binding:"omitempty,min=1,max=100000000"`
	MinOrderAmount    *int64     `json:"min_order_amount" binding:"omitempty,min=0,max=100000000"`
	TotalQuantity     *int32     `json:"total_quantity" binding:"omitempty,min=1,max=1000000"`
	ValidFrom         *time.Time `json:"valid_from"`
	ValidUntil        *time.Time `json:"valid_until"`
	IsActive          *bool      `json:"is_active"`
	AllowedOrderTypes []string   `json:"allowed_order_types"` // 允许的订单类型
}

// updateVoucher godoc
// @Summary 更新代金券
// @Description 商户更新代金券信息（仅商户可操作）
// @Tags 代金券管理-商户
// @Accept json
// @Produce json
// @Param id path int true "商户ID"
// @Param voucher_id path int true "代金券ID"
// @Param request body updateVoucherRequest true "更新字段"
// @Success 200 {object} voucherResponse "更新成功"
// @Failure 400 {object} ErrorResponse "参数错误或时间范围无效"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非代金券所有者"
// @Failure 404 {object} ErrorResponse "代金券不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchants/{id}/vouchers/{voucher_id} [patch]
// @Security BearerAuth
func (server *Server) updateVoucher(ctx *gin.Context) {
	var uriReq updateVoucherURIRequest
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req updateVoucherRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取原代金券验证权限
	voucher, err := server.store.GetVoucher(ctx, uriReq.VoucherID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("voucher not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证代金券属于指定商户
	if voucher.MerchantID != uriReq.MerchantID {
		ctx.JSON(http.StatusNotFound, errorResponse(errors.New("voucher not found")))
		return
	}

	// 验证商户权限
	merchantID, err := server.getMerchantIDByUser(ctx, authPayload.UserID)
	if err != nil || merchantID != voucher.MerchantID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not authorized")))
		return
	}

	// 构造更新参数
	arg := db.UpdateVoucherParams{
		ID: uriReq.VoucherID,
	}

	if req.Name != nil {
		arg.Name = pgtype.Text{String: *req.Name, Valid: true}
	}
	if req.Description != nil {
		arg.Description = pgtype.Text{String: *req.Description, Valid: true}
	}
	if req.Amount != nil {
		arg.Amount = pgtype.Int8{Int64: *req.Amount, Valid: true}
	}
	if req.MinOrderAmount != nil {
		arg.MinOrderAmount = pgtype.Int8{Int64: *req.MinOrderAmount, Valid: true}
	}
	if req.TotalQuantity != nil {
		arg.TotalQuantity = pgtype.Int4{Int32: *req.TotalQuantity, Valid: true}
	}
	if req.ValidFrom != nil {
		arg.ValidFrom = pgtype.Timestamptz{Time: *req.ValidFrom, Valid: true}
	}
	if req.ValidUntil != nil {
		arg.ValidUntil = pgtype.Timestamptz{Time: *req.ValidUntil, Valid: true}
	}
	if req.IsActive != nil {
		arg.IsActive = pgtype.Bool{Bool: *req.IsActive, Valid: true}
	}
	if len(req.AllowedOrderTypes) > 0 {
		// 验证输入的订单类型是否合法
		validTypes := map[string]bool{"takeout": true, "dine_in": true, "takeaway": true, "reservation": true}
		for _, t := range req.AllowedOrderTypes {
			if !validTypes[t] {
				ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("无效的订单类型: %s，允许的类型: takeout, dine_in, takeaway, reservation", t)))
				return
			}
		}
		arg.AllowedOrderTypes = req.AllowedOrderTypes
	}

	// 验证时间范围（如果两个时间都提供或部分提供）
	effectiveValidFrom := voucher.ValidFrom
	effectiveValidUntil := voucher.ValidUntil
	if req.ValidFrom != nil {
		effectiveValidFrom = *req.ValidFrom
	}
	if req.ValidUntil != nil {
		effectiveValidUntil = *req.ValidUntil
	}
	if effectiveValidUntil.Before(effectiveValidFrom) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("valid_until must be after valid_from")))
		return
	}

	updatedVoucher, err := server.store.UpdateVoucher(ctx, arg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	rsp := convertVoucherResponse(updatedVoucher)
	ctx.JSON(http.StatusOK, rsp)
}

// deleteVoucherURIRequest 删除代金券URI参数
type deleteVoucherURIRequest struct {
	MerchantID int64 `uri:"id" binding:"required,min=1"`
	VoucherID  int64 `uri:"voucher_id" binding:"required,min=1"`
}

// deleteVoucher godoc
// @Summary 删除代金券
// @Description 商户删除代金券模板（仅商户可操作，且不能有未使用的用户代金券）
// @Tags 代金券管理-商户
// @Accept json
// @Produce json
// @Param id path int true "商户ID"
// @Param voucher_id path int true "代金券ID"
// @Success 200 {object} map[string]string "删除成功"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 403 {object} ErrorResponse "非代金券所有者"
// @Failure 404 {object} ErrorResponse "代金券不存在"
// @Failure 409 {object} ErrorResponse "存在未使用的用户代金券，无法删除"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/merchants/{id}/vouchers/{voucher_id} [delete]
// @Security BearerAuth
func (server *Server) deleteVoucher(ctx *gin.Context) {
	var req deleteVoucherURIRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 获取代金券验证权限
	voucher, err := server.store.GetVoucher(ctx, req.VoucherID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("voucher not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证代金券属于指定商户
	if voucher.MerchantID != req.MerchantID {
		ctx.JSON(http.StatusNotFound, errorResponse(errors.New("voucher not found")))
		return
	}

	// 验证商户权限
	merchantID, err := server.getMerchantIDByUser(ctx, authPayload.UserID)
	if err != nil || merchantID != voucher.MerchantID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("not authorized")))
		return
	}

	// 检查是否有未使用的用户代金券
	unusedCount, err := server.store.CountUnusedVouchersByVoucherID(ctx, req.VoucherID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if unusedCount > 0 {
		ctx.JSON(http.StatusConflict, errorResponse(fmt.Errorf("cannot delete voucher: %d users have unused vouchers", unusedCount)))
		return
	}

	err = server.store.DeleteVoucher(ctx, req.VoucherID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "voucher deleted successfully"})
}

// ==================== 用户代金券管理 ====================

// claimVoucherRequest 领取代金券请求
type claimVoucherRequest struct {
	VoucherID int64 `uri:"voucher_id" binding:"required,min=1"`
}

// userVoucherResponse 用户代金券响应
type userVoucherResponse struct {
	ID             int64      `json:"id"`
	VoucherID      int64      `json:"voucher_id"`
	UserID         int64      `json:"user_id"`
	MerchantID     int64      `json:"merchant_id"`
	MerchantName   string     `json:"merchant_name,omitempty"`
	Code           string     `json:"code"`
	Name           string     `json:"name"`
	Amount         int64      `json:"amount"`
	MinOrderAmount int64      `json:"min_order_amount"`
	Status         string     `json:"status"`
	OrderID        *int64     `json:"order_id,omitempty"`
	ObtainedAt     time.Time  `json:"obtained_at"`
	ExpiresAt      time.Time  `json:"expires_at"`
	UsedAt         *time.Time `json:"used_at,omitempty"`
}

// claimVoucher godoc
// @Summary 领取代金券
// @Description 用户领取指定代金券
// @Tags 代金券管理
// @Accept json
// @Produce json
// @Param voucher_id path int true "代金券ID"
// @Success 200 {object} userVoucherResponse "领取成功"
// @Failure 400 {object} ErrorResponse "参数错误或已领取/已过期/已领完"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 404 {object} ErrorResponse "代金券不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/vouchers/{voucher_id}/claim [post]
// @Security BearerAuth
func (server *Server) claimVoucher(ctx *gin.Context) {
	var req claimVoucherRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// 使用事务领取代金券
	result, err := server.store.ClaimVoucherTx(ctx, db.ClaimVoucherTxParams{
		VoucherID: req.VoucherID,
		UserID:    authPayload.UserID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("voucher not found")))
			return
		}
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	rsp := userVoucherResponse{
		ID:             result.UserVoucher.ID,
		VoucherID:      result.UserVoucher.VoucherID,
		UserID:         result.UserVoucher.UserID,
		MerchantID:     result.Voucher.MerchantID,
		Code:           result.Voucher.Code,
		Name:           result.Voucher.Name,
		Amount:         result.Voucher.Amount,
		MinOrderAmount: result.Voucher.MinOrderAmount,
		Status:         result.UserVoucher.Status,
		ObtainedAt:     result.UserVoucher.ObtainedAt,
		ExpiresAt:      result.UserVoucher.ExpiresAt,
	}

	ctx.JSON(http.StatusOK, rsp)
}

// listUserVouchersRequest 获取用户代金券列表请求
type listUserVouchersRequest struct {
	PageID   int32 `form:"page_id" binding:"required,min=1"`
	PageSize int32 `form:"page_size" binding:"required,min=5,max=50"`
}

// listUserVouchers godoc
// @Summary 获取我的代金券列表
// @Description 获取当前用户拥有的所有代金券（包含所有状态）
// @Tags 代金券管理
// @Accept json
// @Produce json
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(5) maximum(50)
// @Success 200 {array} userVoucherResponse "代金券列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/vouchers/me [get]
// @Security BearerAuth
func (server *Server) listUserVouchers(ctx *gin.Context) {
	var req listUserVouchersRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	vouchers, err := server.store.ListUserVouchers(ctx, db.ListUserVouchersParams{
		UserID: authPayload.UserID,
		Limit:  req.PageSize,
		Offset: (req.PageID - 1) * req.PageSize,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, vouchers)
}

// listUserAvailableVouchersRequest 获取用户可用代金券请求
type listUserAvailableVouchersRequest struct {
	PageID   int32 `form:"page_id" binding:"required,min=1"`
	PageSize int32 `form:"page_size" binding:"required,min=5,max=50"`
}

// listUserAvailableVouchers godoc
// @Summary 获取我的可用代金券列表
// @Description 获取当前用户所有可用的代金券（未使用且未过期）
// @Tags 代金券管理
// @Accept json
// @Produce json
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页数量" minimum(5) maximum(50)
// @Success 200 {array} userVoucherResponse "可用代金券列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/vouchers/me/available [get]
// @Security BearerAuth
func (server *Server) listUserAvailableVouchers(ctx *gin.Context) {
	var req listUserAvailableVouchersRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	vouchers, err := server.store.ListUserAvailableVouchers(ctx, db.ListUserAvailableVouchersParams{
		UserID: authPayload.UserID,
		Limit:  req.PageSize,
		Offset: (req.PageID - 1) * req.PageSize,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, vouchers)
}

// listUserAvailableVouchersForMerchant godoc
// @Summary 获取指定商户可用代金券
// @Description 获取当前用户在指定商户下满足订单金额要求的可用代金券
// @Tags 代金券管理
// @Accept json
// @Produce json
// @Param merchant_id path int true "商户ID"
// @Param order_amount query int true "订单金额(分)，用于筛选满足最低消费要求的代金券" minimum(1)
// @Success 200 {array} userVoucherResponse "可用代金券列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未认证"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/vouchers/available/{merchant_id} [get]
// @Security BearerAuth
func (server *Server) listUserAvailableVouchersForMerchant(ctx *gin.Context) {
	var uriReq struct {
		MerchantID int64 `uri:"merchant_id" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var queryReq struct {
		OrderAmount int64 `form:"order_amount" binding:"required,min=1"`
	}
	if err := ctx.ShouldBindQuery(&queryReq); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	vouchers, err := server.store.ListUserAvailableVouchersForMerchant(ctx, db.ListUserAvailableVouchersForMerchantParams{
		UserID:         authPayload.UserID,
		MerchantID:     uriReq.MerchantID,
		MinOrderAmount: queryReq.OrderAmount,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, vouchers)
}

// ==================== 辅助函数 ====================

// convertVoucherResponse 转换代金券为响应格式
func convertVoucherResponse(v db.Voucher) voucherResponse {
	rsp := voucherResponse{
		ID:                v.ID,
		MerchantID:        v.MerchantID,
		Code:              v.Code,
		Name:              v.Name,
		Amount:            v.Amount,
		MinOrderAmount:    v.MinOrderAmount,
		TotalQuantity:     v.TotalQuantity,
		ClaimedQuantity:   v.ClaimedQuantity,
		UsedQuantity:      v.UsedQuantity,
		ValidFrom:         v.ValidFrom,
		ValidUntil:        v.ValidUntil,
		IsActive:          v.IsActive,
		AllowedOrderTypes: v.AllowedOrderTypes,
		CreatedAt:         v.CreatedAt,
	}

	if v.Description.Valid {
		rsp.Description = v.Description.String
	}

	return rsp
}
