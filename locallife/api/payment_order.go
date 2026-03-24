package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/worker"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rs/zerolog/log"
)

// ==================== 支付订单管理 ====================

// 支付类型常量
const (
	PaymentTypeNative      = "native"      // 扫码支付
	PaymentTypeMiniProgram = "miniprogram" // 小程序支付
	PaymentTypeProfitShare = "profit_sharing"
)

// 业务类型常量
const (
	BusinessTypeOrder              = "order"               // 订单支付
	BusinessTypeReservation        = "reservation"         // 预定押金
	BusinessTypeReservationAddon   = "reservation_addon"   // 预定加菜补差
	BusinessTypeMembershipRecharge = "membership_recharge" // 会员充值
	BusinessTypeRiderDeposit       = "rider_deposit"       // 骑手押金
)

// 支付状态常量
const (
	PaymentStatusPending  = "pending"  // 待支付
	PaymentStatusPaid     = "paid"     // 已支付
	PaymentStatusFailed   = "failed"   // 支付失败
	PaymentStatusRefunded = "refunded" // 已退款
	PaymentStatusClosed   = "closed"   // 已关闭
)

// ==================== 请求/响应结构体 ====================

type createPaymentOrderRequest struct {
	OrderID      int64  `json:"order_id" binding:"required,min=1"`
	PaymentType  string `json:"payment_type" binding:"omitempty,oneof=native miniprogram"`
	BusinessType string `json:"business_type" binding:"required,oneof=order reservation"`
}

type paymentOrderResponse struct {
	ID           int64                 `json:"id"`
	OrderID      *int64                `json:"order_id,omitempty"`
	UserID       int64                 `json:"user_id"`
	PaymentType  string                `json:"payment_type"`
	BusinessType string                `json:"business_type"`
	Amount       int64                 `json:"amount"`
	OutTradeNo   string                `json:"out_trade_no"`
	Status       string                `json:"status"`
	PrepayID     *string               `json:"prepay_id,omitempty"`
	PayParams    *miniProgramPayParams `json:"pay_params,omitempty"` // 小程序调起支付参数
	PaidAt       *time.Time            `json:"paid_at,omitempty"`
	CreatedAt    time.Time             `json:"created_at"`
}

type createCombinedPaymentOrderRequest struct {
	OrderIDs []int64 `json:"order_ids" binding:"required,min=1,max=10"`
}

type combinedPaymentSubOrderResponse struct {
	OrderID             int64  `json:"order_id"`
	PaymentOrderID      int64  `json:"payment_order_id"`
	MerchantID          int64  `json:"merchant_id"`
	SubMchID            string `json:"sub_mch_id"`
	Amount              int64  `json:"amount"`
	OutTradeNo          string `json:"out_trade_no"`
	Description         string `json:"description"`
	ProfitSharingStatus string `json:"profit_sharing_status,omitempty"`
	MerchantName        string `json:"merchant_name,omitempty"`
	MerchantLogo        string `json:"merchant_logo,omitempty"`
	OrderNo             string `json:"order_no,omitempty"`
}

type combinedPaymentOrderResponse struct {
	ID                int64                             `json:"id"`
	CombineOutTradeNo string                            `json:"combine_out_trade_no"`
	TotalAmount       int64                             `json:"total_amount"`
	Status            string                            `json:"status"`
	PrepayID          *string                           `json:"prepay_id,omitempty"`
	PayParams         *miniProgramPayParams             `json:"pay_params,omitempty"`
	ExpiresAt         *time.Time                        `json:"expires_at,omitempty"`
	SubOrders         []combinedPaymentSubOrderResponse `json:"sub_orders"`
}

// miniProgramPayParams 小程序调起支付所需参数
type miniProgramPayParams struct {
	TimeStamp string `json:"timeStamp"`
	NonceStr  string `json:"nonceStr"`
	Package   string `json:"package"`
	SignType  string `json:"signType"`
	PaySign   string `json:"paySign"`
}

func newPaymentOrderResponse(p db.PaymentOrder) paymentOrderResponse {
	resp := paymentOrderResponse{
		ID:           p.ID,
		UserID:       p.UserID,
		PaymentType:  p.PaymentType,
		BusinessType: p.BusinessType,
		Amount:       p.Amount,
		OutTradeNo:   p.OutTradeNo,
		Status:       p.Status,
		CreatedAt:    p.CreatedAt,
	}

	if p.OrderID.Valid {
		resp.OrderID = &p.OrderID.Int64
	}
	if p.PrepayID.Valid {
		resp.PrepayID = &p.PrepayID.String
	}
	if p.PaidAt.Valid {
		resp.PaidAt = &p.PaidAt.Time
	}

	return resp
}

// generateOutTradeNo 生成商户订单号
// 格式：P + yyyyMMddHHmmss(14位) + hex随机(8位) = 23位
func generateOutTradeNo() (string, error) {
	return util.GenerateOutTradeNo("P")
}

func generateOutTradeNoWithPrefix(prefix string) (string, error) {
	return util.GenerateOutTradeNo(prefix)
}

const (
	outTradeNoMaxRetry      = 3
	outTradeNoRetryBaseBack = 50 * time.Millisecond
)

func isOutTradeNoConflict(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	if pgErr.Code != "23505" {
		return false
	}
	if strings.Contains(pgErr.ConstraintName, "out_trade_no") {
		return true
	}
	return strings.Contains(pgErr.Detail, "out_trade_no")
}

func sleepWithContext(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// ==================== 支付订单API ====================

// createPaymentOrder godoc
// @Summary 创建支付订单
// @Description 为订单或预定创建支付订单，当前主路径会按业务类型自动选择真实支付链路。
// @Description
// @Description **兼容字段：**
// @Description - `payment_type` 仅作为兼容保留字段，可不传。
// @Description - 对 `order` 和 `reservation` 主支付，系统已统一走平台收付通合单支付。
// @Description - 旧客户端即使传入 `native` 或 `miniprogram`，也不会再改变底层支付物理链路。
// @Description
// @Description **业务类型：**
// @Description - order: 订单支付
// @Description - reservation: 预定押金
// @Description
// @Description **幂等性：** 如果已存在待支付的支付订单，将直接返回该订单。
// @Description
// @Description **安全限制：**
// @Description - 订单必须属于当前用户
// @Description - 订单必须处于pending状态
// @Tags 支付管理
// @Accept json
// @Produce json
// @Param request body createPaymentOrderRequest true "支付订单参数"
// @Success 200 {object} paymentOrderResponse "支付订单(含小程序支付参数)"
// @Failure 400 {object} ErrorResponse "订单状态不允许支付"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "订单不属于当前用户"
// @Failure 404 {object} ErrorResponse "订单不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/payments [post]
// @Security BearerAuth
func (server *Server) createPaymentOrder(ctx *gin.Context) {
	var req createPaymentOrderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	server.normalizeCreatePaymentOrderRequest(&req, authPayload.UserID)

	facade := server.paymentFacade
	if facade == nil {
		facade = server.buildPaymentFacade()
	}

	result, err := facade.CreatePaymentOrder(ctx, logic.CreatePaymentOrderInput{
		UserID:       authPayload.UserID,
		OrderID:      req.OrderID,
		PaymentType:  req.PaymentType,
		BusinessType: req.BusinessType,
		ClientIP:     ctx.ClientIP(),
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := newPaymentOrderResponse(result.PaymentOrder)
	if result.PayParams != nil {
		resp.PayParams = &miniProgramPayParams{
			TimeStamp: result.PayParams.TimeStamp,
			NonceStr:  result.PayParams.NonceStr,
			Package:   result.PayParams.Package,
			SignType:  result.PayParams.SignType,
			PaySign:   result.PayParams.PaySign,
		}
	}

	server.scheduleTimeoutForPaymentOrder(ctx, result.PaymentOrder)

	ctx.JSON(http.StatusCreated, resp)
}

func (server *Server) normalizeCreatePaymentOrderRequest(req *createPaymentOrderRequest, userID int64) {
	if req.PaymentType == "" {
		req.PaymentType = PaymentTypeMiniProgram
		return
	}

	entry := log.Info()
	message := "legacy payment_type accepted for create payment order api"
	if req.PaymentType == PaymentTypeNative {
		entry = log.Warn()
		message = "legacy native payment_type ignored for create payment order api"
	}

	entry.
		Int64("user_id", userID).
		Int64("order_id", req.OrderID).
		Str("business_type", req.BusinessType).
		Str("payment_type", req.PaymentType).
		Bool("legacy_client_compat", true).
		Msg(message)

	req.PaymentType = PaymentTypeMiniProgram
}

func (server *Server) scheduleTimeoutForPaymentOrder(ctx context.Context, paymentOrder db.PaymentOrder) {
	if server.taskDistributor == nil || !paymentOrder.ExpiresAt.Valid {
		return
	}
	if _, ok := server.taskDistributor.(worker.NoopTaskDistributor); ok {
		return
	}

	if paymentOrder.CombinedPaymentID.Valid {
		combinedPaymentOrder, getErr := server.store.GetCombinedPaymentOrder(ctx, paymentOrder.CombinedPaymentID.Int64)
		if getErr != nil {
			log.Error().Err(getErr).Int64("combined_payment_id", paymentOrder.CombinedPaymentID.Int64).Msg("failed to load combined payment order for timeout scheduling")
			return
		}
		if enqErr := server.taskDistributor.DistributeTaskCombinedPaymentOrderTimeout(
			ctx,
			&worker.PayloadCombinedPaymentOrderTimeout{CombineOutTradeNo: combinedPaymentOrder.CombineOutTradeNo},
			asynq.ProcessAt(paymentOrder.ExpiresAt.Time),
		); enqErr != nil {
			log.Error().Err(enqErr).Str("combine_out_trade_no", combinedPaymentOrder.CombineOutTradeNo).Msg("failed to enqueue combined payment order timeout task")
		}
		return
	}

	if enqErr := server.taskDistributor.DistributeTaskPaymentOrderTimeout(
		ctx,
		&worker.PayloadPaymentOrderTimeout{PaymentOrderNo: paymentOrder.OutTradeNo},
		asynq.ProcessAt(paymentOrder.ExpiresAt.Time),
	); enqErr != nil {
		log.Error().Err(enqErr).Str("out_trade_no", paymentOrder.OutTradeNo).Msg("⚠️ failed to enqueue payment order timeout task")
	}
}

func writeLogicRequestError(ctx *gin.Context, err error) bool {
	var reqErr *logic.RequestError
	if !errors.As(err, &reqErr) {
		return false
	}
	ctx.JSON(reqErr.Status, errorResponse(reqErr.Err))
	return true
}

// createCombinedPaymentOrder 创建平台收付通合单支付订单
// @Summary 创建合单支付订单
// @Description 为多个订单创建合单支付订单（平台收付通）
// @Tags 支付管理
// @Accept json
// @Produce json
// @Param request body createCombinedPaymentOrderRequest true "合单支付参数"
// @Success 201 {object} combinedPaymentOrderResponse "合单支付订单(含小程序支付参数)"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "订单不属于当前用户"
// @Failure 404 {object} ErrorResponse "订单不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/payments/combined [post]
// @Security BearerAuth
func (server *Server) createCombinedPaymentOrder(ctx *gin.Context) {
	var req createCombinedPaymentOrderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	facade := server.paymentFacade
	if facade == nil {
		facade = server.buildPaymentFacade()
	}

	result, err := facade.CreateCombinedPaymentOrder(ctx, logic.CreateCombinedPaymentOrderInput{
		UserID:   authPayload.UserID,
		OrderIDs: req.OrderIDs,
		ClientIP: ctx.ClientIP(),
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	subOrders := make([]combinedPaymentSubOrderResponse, 0, len(result.SubOrders))
	for _, sub := range result.SubOrders {
		subOrders = append(subOrders, combinedPaymentSubOrderResponse{
			OrderID:        sub.OrderID,
			PaymentOrderID: sub.PaymentOrderID,
			MerchantID:     sub.MerchantID,
			SubMchID:       sub.SubMchID,
			Amount:         sub.Amount,
			OutTradeNo:     sub.OutTradeNo,
			Description:    sub.Description,
		})
	}

	combinedPayment := result.CombinedPayment
	resp := combinedPaymentOrderResponse{
		ID:                combinedPayment.ID,
		CombineOutTradeNo: combinedPayment.CombineOutTradeNo,
		TotalAmount:       combinedPayment.TotalAmount,
		Status:            combinedPayment.Status,
		SubOrders:         subOrders,
	}
	if combinedPayment.PrepayID.Valid {
		resp.PrepayID = &combinedPayment.PrepayID.String
	}
	if combinedPayment.ExpiresAt.Valid {
		expires := combinedPayment.ExpiresAt.Time
		resp.ExpiresAt = &expires
	}
	if result.PayParams != nil {
		resp.PayParams = &miniProgramPayParams{
			TimeStamp: result.PayParams.TimeStamp,
			NonceStr:  result.PayParams.NonceStr,
			Package:   result.PayParams.Package,
			SignType:  result.PayParams.SignType,
			PaySign:   result.PayParams.PaySign,
		}
	}

	if server.taskDistributor != nil && combinedPayment.ExpiresAt.Valid {
		if enqErr := server.taskDistributor.DistributeTaskCombinedPaymentOrderTimeout(
			ctx,
			&worker.PayloadCombinedPaymentOrderTimeout{CombineOutTradeNo: combinedPayment.CombineOutTradeNo},
			asynq.ProcessAt(combinedPayment.ExpiresAt.Time),
		); enqErr != nil {
			log.Error().Err(enqErr).Str("combine_out_trade_no", combinedPayment.CombineOutTradeNo).Msg("⚠️ failed to enqueue combined payment timeout task")
		}
	}

	ctx.JSON(http.StatusCreated, resp)
}

// getCombinedPaymentOrder 获取合单支付订单详情
type getCombinedPaymentOrderRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// getCombinedPaymentOrder godoc
// @Summary 获取合单支付订单详情
// @Description 根据ID获取合单支付订单详情
// @Tags 支付管理
// @Accept json
// @Produce json
// @Param id path int true "合单支付订单ID"
// @Success 201 {object} combinedPaymentOrderResponse "合单支付订单详情"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "合单支付订单不属于当前用户"
// @Failure 404 {object} ErrorResponse "合单支付订单不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/payments/combined/{id} [get]
// @Security BearerAuth
func (server *Server) getCombinedPaymentOrder(ctx *gin.Context) {
	var req getCombinedPaymentOrderRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	facade := server.paymentFacade
	if facade == nil {
		facade = server.buildPaymentFacade()
	}

	result, err := facade.GetCombinedPaymentOrder(ctx, logic.GetCombinedPaymentOrderInput{
		UserID:            authPayload.UserID,
		CombinedPaymentID: req.ID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	combinedRow := result.CombinedPayment

	var subOrders []combinedPaymentSubOrderResponse
	if err := json.Unmarshal(combinedRow.SubOrders, &subOrders); err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := combinedPaymentOrderResponse{
		ID:                combinedRow.ID,
		CombineOutTradeNo: combinedRow.CombineOutTradeNo,
		TotalAmount:       combinedRow.TotalAmount,
		Status:            combinedRow.Status,
		SubOrders:         subOrders,
	}
	if combinedRow.PrepayID.Valid {
		resp.PrepayID = &combinedRow.PrepayID.String
	}
	if combinedRow.ExpiresAt.Valid {
		expires := combinedRow.ExpiresAt.Time
		resp.ExpiresAt = &expires
	}

	ctx.JSON(http.StatusOK, resp)
}

// closeCombinedPaymentOrder 关闭合单支付订单
type closeCombinedPaymentOrderRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// closeCombinedPaymentOrder godoc
// @Summary 关闭合单支付订单
// @Description 关闭待支付的合单支付订单
// @Tags 支付管理
// @Accept json
// @Produce json
// @Param id path int true "合单支付订单ID"
// @Success 200 {object} combinedPaymentOrderResponse "关闭成功的合单支付订单"
// @Failure 400 {object} ErrorResponse "请求参数错误或订单状态不允许关闭"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "合单支付订单不属于当前用户"
// @Failure 404 {object} ErrorResponse "合单支付订单不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/payments/combined/{id}/close [post]
// @Security BearerAuth
func (server *Server) closeCombinedPaymentOrder(ctx *gin.Context) {
	var req closeCombinedPaymentOrderRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	facade := server.paymentFacade
	if facade == nil {
		facade = server.buildPaymentFacade()
	}

	result, err := facade.CloseCombinedPaymentOrder(ctx, logic.CloseCombinedPaymentOrderInput{
		UserID:            authPayload.UserID,
		CombinedPaymentID: req.ID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	updatedCombined := result.CombinedPayment
	subOrders := make([]combinedPaymentSubOrderResponse, 0, len(result.SubOrders))
	for _, sub := range result.SubOrders {
		subOrders = append(subOrders, combinedPaymentSubOrderResponse{
			OrderID:        sub.OrderID,
			PaymentOrderID: sub.PaymentOrderID,
			MerchantID:     sub.MerchantID,
			SubMchID:       sub.SubMchID,
			Amount:         sub.Amount,
			OutTradeNo:     sub.OutTradeNo,
			Description:    sub.Description,
		})
	}

	resp := combinedPaymentOrderResponse{
		ID:                updatedCombined.ID,
		CombineOutTradeNo: updatedCombined.CombineOutTradeNo,
		TotalAmount:       updatedCombined.TotalAmount,
		Status:            updatedCombined.Status,
		SubOrders:         subOrders,
	}
	if updatedCombined.PrepayID.Valid {
		resp.PrepayID = &updatedCombined.PrepayID.String
	}
	if updatedCombined.ExpiresAt.Valid {
		expires := updatedCombined.ExpiresAt.Time
		resp.ExpiresAt = &expires
	}

	ctx.JSON(http.StatusOK, resp)
}

// getPaymentOrder 获取支付订单详情
type getPaymentOrderRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// getPaymentOrder godoc
// @Summary 获取支付订单详情
// @Description 根据ID获取支付订单详情
// @Tags 支付管理
// @Accept json
// @Produce json
// @Param id path int true "支付订单ID"
// @Success 200 {object} paymentOrderResponse "支付订单详情"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "支付订单不属于当前用户"
// @Failure 404 {object} ErrorResponse "支付订单不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/payments/{id} [get]
// @Security BearerAuth
func (server *Server) getPaymentOrder(ctx *gin.Context) {
	var req getPaymentOrderRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	facade := server.paymentFacade
	if facade == nil {
		facade = server.buildPaymentFacade()
	}

	result, err := facade.GetPaymentOrder(ctx, logic.GetPaymentOrderInput{
		UserID:         authPayload.UserID,
		PaymentOrderID: req.ID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	paymentOrder := result.PaymentOrder

	ctx.JSON(http.StatusOK, newPaymentOrderResponse(paymentOrder))
}

// listPaymentOrders 获取用户支付订单列表
type listPaymentOrdersRequest struct {
	PageID   int32  `form:"page_id" binding:"omitempty,min=1"`
	PageSize int32  `form:"page_size" binding:"omitempty,min=1,max=20"`
	OrderID  *int64 `form:"order_id" binding:"omitempty,min=1"` // 可选：按订单ID筛选
}

type listPaymentOrdersResponse struct {
	PaymentOrders []paymentOrderResponse `json:"payment_orders"`
	Total         int64                  `json:"total"`
	PageID        int32                  `json:"page_id"`
	PageSize      int32                  `json:"page_size"`
}

// listPaymentOrders godoc
// @Summary 获取支付订单列表
// @Description 分页获取当前用户的支付订单列表，或按订单ID查询
// @Tags 支付管理
// @Accept json
// @Produce json
// @Param page_id query int false "页码" minimum(1)
// @Param page_size query int false "每页条数" minimum(1) maximum(20)
// @Param order_id query int false "订单ID（按订单查询时使用）"
// @Success 200 {object} listPaymentOrdersResponse "支付订单列表"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/payments [get]
// @Security BearerAuth
func (server *Server) listPaymentOrders(ctx *gin.Context) {
	var req listPaymentOrdersRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	pageID := req.PageID
	if pageID == 0 {
		pageID = 1
	}
	pageSize := req.PageSize
	if pageSize == 0 {
		pageSize = 10
	}

	facade := server.paymentFacade
	if facade == nil {
		facade = server.buildPaymentFacade()
	}

	result, err := facade.ListPaymentOrders(ctx, logic.ListPaymentOrdersInput{
		UserID:   authPayload.UserID,
		OrderID:  req.OrderID,
		PageID:   pageID,
		PageSize: pageSize,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	paymentOrders := result.PaymentOrders

	resp := make([]paymentOrderResponse, len(paymentOrders))
	for i, p := range paymentOrders {
		resp[i] = newPaymentOrderResponse(p)
	}

	ctx.JSON(http.StatusOK, listPaymentOrdersResponse{
		PaymentOrders: resp,
		Total:         result.TotalCount,
		PageID:        pageID,
		PageSize:      pageSize,
	})
}

// closePaymentOrder 关闭支付订单
type closePaymentOrderRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// closePaymentOrder godoc
// @Summary 关闭支付订单
// @Description 关闭待支付的支付订单
// @Tags 支付管理
// @Accept json
// @Produce json
// @Param id path int true "支付订单ID"
// @Success 200 {object} paymentOrderResponse "关闭成功的支付订单"
// @Failure 400 {object} ErrorResponse "请求参数错误或订单状态不允许关闭"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "非订单所有者"
// @Failure 404 {object} ErrorResponse "支付订单不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/payments/{id}/close [post]
// @Security BearerAuth
func (server *Server) closePaymentOrder(ctx *gin.Context) {
	var req closePaymentOrderRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	facade := server.paymentFacade
	if facade == nil {
		facade = server.buildPaymentFacade()
	}

	result, err := facade.ClosePaymentOrder(ctx, logic.ClosePaymentOrderInput{
		UserID:         authPayload.UserID,
		PaymentOrderID: req.ID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newPaymentOrderResponse(result.PaymentOrder))
}

// ==================== 退款订单API ====================

type refundOrderResponse struct {
	ID             int64      `json:"id"`
	PaymentOrderID int64      `json:"payment_order_id"`
	RefundType     string     `json:"refund_type"`
	RefundAmount   int64      `json:"refund_amount"`
	RefundReason   *string    `json:"refund_reason,omitempty"`
	OutRefundNo    string     `json:"out_refund_no"`
	Status         string     `json:"status"`
	RefundedAt     *time.Time `json:"refunded_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type profitSharingReturnResponse struct {
	ID            int64      `json:"id"`
	RefundOrderID int64      `json:"refund_order_id"`
	OutOrderNo    string     `json:"out_order_no"`
	OutReturnNo   string     `json:"out_return_no"`
	ReturnMchID   string     `json:"return_mchid"`
	Amount        int64      `json:"amount"`
	Status        string     `json:"status"`
	ReturnID      *string    `json:"return_id,omitempty"`
	FailReason    *string    `json:"fail_reason,omitempty"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func newProfitSharingReturnResponse(r db.ProfitSharingReturn) profitSharingReturnResponse {
	resp := profitSharingReturnResponse{
		ID:            r.ID,
		RefundOrderID: r.RefundOrderID,
		OutOrderNo:    r.OutOrderNo,
		OutReturnNo:   r.OutReturnNo,
		ReturnMchID:   r.ReturnMchid,
		Amount:        r.Amount,
		Status:        r.Status,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
	if r.ReturnID.Valid {
		resp.ReturnID = &r.ReturnID.String
	}
	if r.FailReason.Valid {
		resp.FailReason = &r.FailReason.String
	}
	if r.FinishedAt.Valid {
		resp.FinishedAt = &r.FinishedAt.Time
	}
	return resp
}

func newRefundOrderResponse(r db.RefundOrder) refundOrderResponse {
	resp := refundOrderResponse{
		ID:             r.ID,
		PaymentOrderID: r.PaymentOrderID,
		RefundType:     r.RefundType,
		RefundAmount:   r.RefundAmount,
		OutRefundNo:    r.OutRefundNo,
		Status:         r.Status,
		CreatedAt:      r.CreatedAt,
	}

	if r.RefundReason.Valid {
		resp.RefundReason = &r.RefundReason.String
	}
	if r.RefundedAt.Valid {
		resp.RefundedAt = &r.RefundedAt.Time
	}

	return resp
}

// createRefundOrder 创建退款订单（商户端）
type createRefundOrderRequest struct {
	PaymentOrderID int64  `json:"payment_order_id" binding:"required,min=1"`
	RefundType     string `json:"refund_type" binding:"required,oneof=full partial"`
	RefundAmount   int64  `json:"refund_amount" binding:"required,min=1"`
	RefundReason   string `json:"refund_reason,omitempty"`
}

// createRefundOrder godoc
// @Summary 创建退款订单（商户端）
// @Description 商户为已支付的支付订单创建退款
// @Tags 退款管理
// @Accept json
// @Produce json
// @Param request body createRefundOrderRequest true "退款详情"
// @Success 200 {object} refundOrderResponse "退款订单"
// @Failure 400 {object} ErrorResponse "请求参数错误或订单状态不允许退款"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "非商户用户"
// @Failure 404 {object} ErrorResponse "支付订单不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/refunds [post]
// @Security BearerAuth
func (server *Server) createRefundOrder(ctx *gin.Context) {
	var req createRefundOrderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	orchestrator := server.refundOrchestrator
	if orchestrator == nil {
		orchestrator = server.buildRefundOrchestrator()
	}

	result, err := orchestrator.CreateRefundOrder(ctx, logic.CreateRefundOrderInput{
		ActorUserID:                      authPayload.UserID,
		PaymentOrderID:                   req.PaymentOrderID,
		RefundType:                       req.RefundType,
		RefundAmount:                     req.RefundAmount,
		RefundReason:                     req.RefundReason,
		ProfitSharingReturnRetryInterval: server.config.ProfitSharingReturnRetryInterval,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusCreated, newRefundOrderResponse(result.RefundOrder))
}

// listProfitSharingReturnsByRefund 获取退款关联的分账回退记录
type listProfitSharingReturnsByRefundRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// listProfitSharingReturnsByRefund godoc
// @Summary 查询退款的分账回退记录
// @Description 查询指定退款订单的分账回退记录
// @Tags 退款管理
// @Accept json
// @Produce json
// @Param id path int true "退款订单ID"
// @Success 201 {array} profitSharingReturnResponse "分账回退记录列表"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "非商户用户"
// @Failure 404 {object} ErrorResponse "退款订单不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/refunds/{id}/returns [get]
// @Security BearerAuth
func (server *Server) listProfitSharingReturnsByRefund(ctx *gin.Context) {
	var req listProfitSharingReturnsByRefundRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	orchestrator := server.refundOrchestrator
	if orchestrator == nil {
		orchestrator = server.buildRefundOrchestrator()
	}

	result, err := orchestrator.ListProfitSharingReturnsByRefund(ctx, logic.ListProfitSharingReturnsByRefundInput{
		ActorUserID: authPayload.UserID,
		RefundID:    req.ID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := make([]profitSharingReturnResponse, 0, len(result.Returns))
	for _, r := range result.Returns {
		resp = append(resp, newProfitSharingReturnResponse(r))
	}

	ctx.JSON(http.StatusOK, resp)
}

// getRefundOrder 获取退款订单详情
type getRefundOrderRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// getRefundOrder godoc
// @Summary 获取退款订单详情
// @Description 根据ID获取退款订单详情
// @Tags 退款管理
// @Accept json
// @Produce json
// @Param id path int true "退款订单ID"
// @Success 200 {object} refundOrderResponse "退款订单详情"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "无访问权限"
// @Failure 404 {object} ErrorResponse "退款订单不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/refunds/{id} [get]
// @Security BearerAuth
func (server *Server) getRefundOrder(ctx *gin.Context) {
	var req getRefundOrderRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	orchestrator := server.refundOrchestrator
	if orchestrator == nil {
		orchestrator = server.buildRefundOrchestrator()
	}

	result, err := orchestrator.GetRefundOrder(ctx, logic.GetRefundOrderInput{
		ActorUserID: authPayload.UserID,
		RefundID:    req.ID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newRefundOrderResponse(result.RefundOrder))
}

// listRefundOrdersByPayment 获取支付订单的退款列表
type listRefundOrdersByPaymentRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

type listRefundOrdersByPaymentResponse struct {
	RefundOrders []refundOrderResponse `json:"refund_orders"`
	Total        int64                 `json:"total"`
}

// listRefundOrdersByPayment godoc
// @Summary 获取支付订单的退款列表
// @Description 获取指定支付订单的所有退款记录
// @Tags 退款管理
// @Accept json
// @Produce json
// @Param id path int true "支付订单ID"
// @Success 200 {object} listRefundOrdersByPaymentResponse "退款订单列表"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "无访问权限"
// @Failure 404 {object} ErrorResponse "支付订单不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/payments/{id}/refunds [get]
// @Security BearerAuth
func (server *Server) listRefundOrdersByPayment(ctx *gin.Context) {
	var req listRefundOrdersByPaymentRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	orchestrator := server.refundOrchestrator
	if orchestrator == nil {
		orchestrator = server.buildRefundOrchestrator()
	}

	result, err := orchestrator.ListRefundOrdersByPayment(ctx, logic.ListRefundOrdersByPaymentInput{
		ActorUserID:    authPayload.UserID,
		PaymentOrderID: req.ID,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := make([]refundOrderResponse, len(result.RefundOrders))
	for i, r := range result.RefundOrders {
		resp[i] = newRefundOrderResponse(r)
	}

	ctx.JSON(http.StatusOK, listRefundOrdersByPaymentResponse{
		RefundOrders: resp,
		Total:        int64(len(resp)),
	})
}
