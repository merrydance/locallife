package api

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/wechat"
	"github.com/merrydance/locallife/worker"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ==================== 支付订单管理 ====================

// 支付类型常量
const (
	PaymentTypeNative      = "native"      // 扫码支付
	PaymentTypeMiniProgram = "miniprogram" // 小程序支付
)

// 业务类型常量
const (
	BusinessTypeOrder       = "order"       // 订单支付
	BusinessTypeReservation = "reservation" // 预定押金
)

// 支付状态常量
const (
	PaymentStatusPending  = "pending"  // 待支付
	PaymentStatusPaid     = "paid"     // 已支付
	PaymentStatusRefunded = "refunded" // 已退款
	PaymentStatusClosed   = "closed"   // 已关闭
)

// ==================== 请求/响应结构体 ====================

type createPaymentOrderRequest struct {
	OrderID      int64  `json:"order_id" binding:"required,min=1"`
	PaymentType  string `json:"payment_type" binding:"required,oneof=native miniprogram"`
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
func generateOutTradeNo() string {
	now := time.Now()
	dateStr := now.Format("20060102150405")

	// 生成8位随机数
	b := make([]byte, 4)
	rand.Read(b)
	randomNum := fmt.Sprintf("%08d", int(b[0])*1000000+int(b[1])*10000+int(b[2])*100+int(b[3]))

	return "P" + dateStr + randomNum[:8]
}

// ==================== 支付订单API ====================

// createPaymentOrder godoc
// @Summary 创建支付订单
// @Description 为订单或预定创建支付订单，调用微信支付统一下单接口。
// @Description
// @Description **支付类型：**
// @Description - native: 扫码支付(商户端扫用户付款码)
// @Description - miniprogram: 小程序支付(返回调起支付所需参数)
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

	// 获取订单
	order, err := server.store.GetOrder(ctx, req.OrderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("order not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证订单属于当前用户
	if order.UserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("order does not belong to you")))
		return
	}

	// 验证订单状态
	if order.Status != OrderStatusPending {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("order is not in pending status")))
		return
	}

	// 检查是否已存在待支付的支付订单
	existingPayment, err := server.store.GetLatestPaymentOrderByOrder(ctx, pgtype.Int8{Int64: req.OrderID, Valid: true})
	if err == nil && existingPayment.Status == PaymentStatusPending {
		// 已存在待支付订单，直接返回
		ctx.JSON(http.StatusOK, newPaymentOrderResponse(existingPayment))
		return
	}

	// 生成商户订单号
	outTradeNo := generateOutTradeNo()

	// 设置支付过期时间（30分钟）
	expiresAt := time.Now().Add(30 * time.Minute)

	// 创建支付订单
	paymentOrder, err := server.store.CreatePaymentOrder(ctx, db.CreatePaymentOrderParams{
		OrderID:      pgtype.Int8{Int64: req.OrderID, Valid: true},
		UserID:       authPayload.UserID,
		PaymentType:  req.PaymentType,
		BusinessType: req.BusinessType,
		Amount:       order.TotalAmount,
		OutTradeNo:   outTradeNo,
		ExpiresAt:    pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 构建响应
	resp := newPaymentOrderResponse(paymentOrder)

	// 调用微信支付 API 创建预支付订单
	if server.paymentClient != nil && req.PaymentType == PaymentTypeMiniProgram {
		// 获取用户 OpenID
		user, err := server.store.GetUser(ctx, authPayload.UserID)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get user: %w", err)))
			return
		}

		// 获取商户名称作为商品描述
		merchantName := "订单支付"
		if order.MerchantID > 0 {
			merchant, err := server.store.GetMerchant(ctx, order.MerchantID)
			if err == nil {
				merchantName = merchant.Name + " - 订单支付"
			}
		}

		// 调用微信支付 JSAPI 下单
		wxResp, payParams, err := server.paymentClient.CreateJSAPIOrder(ctx, &wechat.JSAPIOrderRequest{
			OutTradeNo:    outTradeNo,
			Description:   merchantName,
			TotalAmount:   order.TotalAmount,
			OpenID:        user.WechatOpenid,
			ExpireTime:    expiresAt,
			Attach:        fmt.Sprintf("order_id:%d", order.ID), // 传递订单ID用于回调关联
			PayerClientIP: ctx.ClientIP(),                       // 用户终端IP（用于风控）
		})
		if err != nil {
			// 微信支付失败，关闭支付订单
			server.store.UpdatePaymentOrderToClosed(ctx, paymentOrder.ID)
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("wechat pay: %w", err)))
			return
		}

		// 更新支付订单的 prepay_id
		updatedPayment, err := server.store.UpdatePaymentOrderPrepayId(ctx, db.UpdatePaymentOrderPrepayIdParams{
			ID:       paymentOrder.ID,
			PrepayID: pgtype.Text{String: wxResp.PrepayID, Valid: true},
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		// 更新响应
		resp = newPaymentOrderResponse(updatedPayment)
		resp.PayParams = &miniProgramPayParams{
			TimeStamp: payParams.TimeStamp,
			NonceStr:  payParams.NonceStr,
			Package:   payParams.Package,
			SignType:  payParams.SignType,
			PaySign:   payParams.PaySign,
		}
	}

	// 分发支付超时任务
	if server.taskDistributor != nil {
		err = server.taskDistributor.DistributeTaskPaymentOrderTimeout(
			ctx,
			&worker.PayloadPaymentOrderTimeout{PaymentOrderNo: outTradeNo},
			asynq.ProcessAt(expiresAt),
		)
		if err != nil {
			// 任务分发失败不影响主流程，记录日志即可
			// 可以通过定时任务轮询处理超时订单作为兜底
		}
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

	paymentOrder, err := server.store.GetPaymentOrder(ctx, req.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("payment order not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证支付订单属于当前用户
	if paymentOrder.UserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("payment order does not belong to you")))
		return
	}

	ctx.JSON(http.StatusOK, newPaymentOrderResponse(paymentOrder))
}

// listPaymentOrders 获取用户支付订单列表
type listPaymentOrdersRequest struct {
	PageID   int32 `form:"page_id" binding:"required,min=1"`
	PageSize int32 `form:"page_size" binding:"required,min=5,max=20"`
}

// listPaymentOrders godoc
// @Summary 获取支付订单列表
// @Description 分页获取当前用户的支付订单列表
// @Tags 支付管理
// @Accept json
// @Produce json
// @Param page_id query int true "页码" minimum(1)
// @Param page_size query int true "每页条数" minimum(5) maximum(20)
// @Success 200 {array} paymentOrderResponse "支付订单列表"
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

	offset := (req.PageID - 1) * req.PageSize

	paymentOrders, err := server.store.ListPaymentOrdersByUser(ctx, db.ListPaymentOrdersByUserParams{
		UserID: authPayload.UserID,
		Limit:  req.PageSize,
		Offset: offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := make([]paymentOrderResponse, len(paymentOrders))
	for i, p := range paymentOrders {
		resp[i] = newPaymentOrderResponse(p)
	}

	ctx.JSON(http.StatusOK, resp)
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

	paymentOrder, err := server.store.GetPaymentOrder(ctx, req.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("payment order not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证支付订单属于当前用户
	if paymentOrder.UserID != authPayload.UserID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("payment order does not belong to you")))
		return
	}

	// 验证状态
	if paymentOrder.Status != PaymentStatusPending {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("only pending payment orders can be closed")))
		return
	}

	// 关闭支付订单
	updatedPayment, err := server.store.UpdatePaymentOrderToClosed(ctx, req.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 调用微信关单 API（如果有 prepay_id）
	if server.paymentClient != nil && paymentOrder.PrepayID.Valid {
		err = server.paymentClient.CloseOrder(ctx, paymentOrder.OutTradeNo)
		if err != nil {
			// 关单失败不影响本地状态，微信支付订单会在超时后自动关闭
			// 记录日志即可
		}
	}

	ctx.JSON(http.StatusOK, newPaymentOrderResponse(updatedPayment))
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

// generateOutRefundNo 生成退款单号
func generateOutRefundNo() string {
	now := time.Now()
	dateStr := now.Format("20060102150405")

	// 生成8位随机数
	b := make([]byte, 4)
	rand.Read(b)
	randomNum := fmt.Sprintf("%08d", int(b[0])*1000000+int(b[1])*10000+int(b[2])*100+int(b[3]))

	return "R" + dateStr + randomNum[:8]
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

	// 获取当前用户的商户
	merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("you are not a merchant")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取支付订单
	paymentOrder, err := server.store.GetPaymentOrder(ctx, req.PaymentOrderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("payment order not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证支付订单已支付
	if paymentOrder.Status != PaymentStatusPaid {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("payment order is not paid")))
		return
	}

	// 获取订单验证归属
	if !paymentOrder.OrderID.Valid {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("payment order has no associated order")))
		return
	}

	order, err := server.store.GetOrder(ctx, paymentOrder.OrderID.Int64)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if order.MerchantID != merchant.ID {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("order does not belong to your merchant")))
		return
	}

	// 验证退款金额
	if req.RefundAmount > paymentOrder.Amount {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("refund amount exceeds payment amount")))
		return
	}

	// 生成退款单号
	outRefundNo := generateOutRefundNo()

	// 创建退款订单
	refundOrder, err := server.store.CreateRefundOrder(ctx, db.CreateRefundOrderParams{
		PaymentOrderID: req.PaymentOrderID,
		RefundType:     req.RefundType,
		RefundAmount:   req.RefundAmount,
		RefundReason:   pgtype.Text{String: req.RefundReason, Valid: req.RefundReason != ""},
		OutRefundNo:    outRefundNo,
		Status:         "pending",
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 调用微信退款 API
	if server.paymentClient != nil {
		wxRefund, err := server.paymentClient.CreateRefund(ctx, &wechat.RefundRequest{
			OutTradeNo:   paymentOrder.OutTradeNo,
			OutRefundNo:  outRefundNo,
			Reason:       req.RefundReason,
			RefundAmount: req.RefundAmount,
			TotalAmount:  paymentOrder.Amount,
		})
		if err != nil {
			// 退款失败，更新退款订单状态
			server.store.UpdateRefundOrderToFailed(ctx, refundOrder.ID)
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("wechat refund: %w", err)))
			return
		}

		// 根据微信返回状态更新退款订单
		switch wxRefund.Status {
		case wechat.RefundStatusSuccess:
			server.store.UpdateRefundOrderToSuccess(ctx, refundOrder.ID)
			// 更新支付订单状态为已退款
			server.store.UpdatePaymentOrderToRefunded(ctx, paymentOrder.ID)
		case wechat.RefundStatusProcessing:
			server.store.UpdateRefundOrderToProcessing(ctx, db.UpdateRefundOrderToProcessingParams{
				ID:       refundOrder.ID,
				RefundID: pgtype.Text{String: wxRefund.RefundID, Valid: true},
			})
		}
	}

	// 重新获取退款订单以返回最新状态
	refundOrder, _ = server.store.GetRefundOrder(ctx, refundOrder.ID)

	ctx.JSON(http.StatusOK, newRefundOrderResponse(refundOrder))
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

	refundOrder, err := server.store.GetRefundOrder(ctx, req.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("refund order not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 获取支付订单和订单，验证权限
	paymentOrder, err := server.store.GetPaymentOrder(ctx, refundOrder.PaymentOrderID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 用户可以查看
	if paymentOrder.UserID == authPayload.UserID {
		ctx.JSON(http.StatusOK, newRefundOrderResponse(refundOrder))
		return
	}

	// 检查是否是商户
	if paymentOrder.OrderID.Valid {
		order, err := server.store.GetOrder(ctx, paymentOrder.OrderID.Int64)
		if err == nil {
			merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
			if err == nil && order.MerchantID == merchant.ID {
				ctx.JSON(http.StatusOK, newRefundOrderResponse(refundOrder))
				return
			}
		}
	}

	ctx.JSON(http.StatusForbidden, errorResponse(errors.New("access denied")))
}

// listRefundOrdersByPayment 获取支付订单的退款列表
type listRefundOrdersByPaymentRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// listRefundOrdersByPayment godoc
// @Summary 获取支付订单的退款列表
// @Description 获取指定支付订单的所有退款记录
// @Tags 退款管理
// @Accept json
// @Produce json
// @Param id path int true "支付订单ID"
// @Success 200 {array} refundOrderResponse "退款订单列表"
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

	// 获取支付订单
	paymentOrder, err := server.store.GetPaymentOrder(ctx, req.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("payment order not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 验证权限
	if paymentOrder.UserID != authPayload.UserID {
		if paymentOrder.OrderID.Valid {
			order, err := server.store.GetOrder(ctx, paymentOrder.OrderID.Int64)
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
				return
			}
			merchant, err := server.store.GetMerchantByOwner(ctx, authPayload.UserID)
			if err != nil || order.MerchantID != merchant.ID {
				ctx.JSON(http.StatusForbidden, errorResponse(errors.New("access denied")))
				return
			}
		} else {
			ctx.JSON(http.StatusForbidden, errorResponse(errors.New("access denied")))
			return
		}
	}

	refundOrders, err := server.store.ListRefundOrdersByPaymentOrder(ctx, req.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp := make([]refundOrderResponse, len(refundOrders))
	for i, r := range refundOrders {
		resp[i] = newRefundOrderResponse(r)
	}

	ctx.JSON(http.StatusOK, resp)
}
