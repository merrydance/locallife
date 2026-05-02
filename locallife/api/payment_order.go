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
	"github.com/merrydance/locallife/wechat"
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
	BusinessTypeOrder            = "order"             // 订单支付
	BusinessTypeReservation      = "reservation"       // 预定押金
	BusinessTypeReservationAddon = "reservation_addon" // 预定加菜补差
	BusinessTypeRiderDeposit     = "rider_deposit"     // 骑手押金
	BusinessTypeClaimRecovery    = "claim_recovery"    // 索赔追偿支付
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
	PaymentType  string `json:"payment_type" binding:"omitempty,oneof=miniprogram"`
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

type paymentOrderQueryResponse struct {
	ID           int64                          `json:"id"`
	OrderID      *int64                         `json:"order_id,omitempty"`
	UserID       int64                          `json:"user_id"`
	PaymentType  string                         `json:"payment_type"`
	BusinessType string                         `json:"business_type"`
	Amount       int64                          `json:"amount"`
	OutTradeNo   string                         `json:"out_trade_no"`
	Status       string                         `json:"status"`
	PrepayID     *string                        `json:"prepay_id,omitempty"`
	PayParams    *miniProgramPayParams          `json:"pay_params,omitempty"`
	WechatQuery  *paymentOrderWechatQueryResult `json:"wechat_query,omitempty"`
	PaidAt       *time.Time                     `json:"paid_at,omitempty"`
	CreatedAt    time.Time                      `json:"created_at"`
}

type createCombinedPaymentOrderRequest struct {
	OrderIDs []int64 `json:"order_ids" binding:"required,min=1,max=50"`
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
	WechatQuery       *combinedPaymentWechatQueryResult `json:"wechat_query,omitempty"`
	SubOrders         []combinedPaymentSubOrderResponse `json:"sub_orders"`
}

type combinedPaymentWechatQueryResult struct {
	CombineOutTradeNo   string                                `json:"combine_out_trade_no"`
	AggregateTradeState string                                `json:"aggregate_trade_state"`
	SubOrders           []combinedPaymentWechatSubOrderResult `json:"sub_orders"`
}

type combinedPaymentWechatSubOrderResult struct {
	OutTradeNo      string                              `json:"out_trade_no"`
	TradeState      string                              `json:"trade_state"`
	SuccessTime     string                              `json:"success_time,omitempty"`
	PromotionDetail []paymentOrderWechatPromotionDetail `json:"promotion_detail,omitempty"`
	Amount          combinedPaymentWechatAmountResult   `json:"amount"`
}

type combinedPaymentWechatAmountResult struct {
	TotalAmount   int64  `json:"total_amount"`
	PayerAmount   int64  `json:"payer_amount"`
	Currency      string `json:"currency"`
	PayerCurrency string `json:"payer_currency,omitempty"`
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

func newMiniProgramPayParams(payParams *wechat.JSAPIPayParams) *miniProgramPayParams {
	if payParams == nil {
		return nil
	}
	return &miniProgramPayParams{
		TimeStamp: payParams.TimeStamp,
		NonceStr:  payParams.NonceStr,
		Package:   payParams.Package,
		SignType:  payParams.SignType,
		PaySign:   payParams.PaySign,
	}
}

func newPaymentOrderQueryResponse(paymentOrder db.PaymentOrder, payParams *wechat.JSAPIPayParams, query *logic.QueryPaymentOrderWechatOrder) paymentOrderQueryResponse {
	base := newPaymentOrderResponse(paymentOrder)
	return paymentOrderQueryResponse{
		ID:           base.ID,
		OrderID:      base.OrderID,
		UserID:       base.UserID,
		PaymentType:  base.PaymentType,
		BusinessType: base.BusinessType,
		Amount:       base.Amount,
		OutTradeNo:   base.OutTradeNo,
		Status:       base.Status,
		PrepayID:     base.PrepayID,
		PayParams:    newMiniProgramPayParams(payParams),
		WechatQuery:  newPaymentOrderWechatQueryResult(query),
		PaidAt:       base.PaidAt,
		CreatedAt:    base.CreatedAt,
	}
}

func buildCombinedPaymentOrderResponse(combinedRow db.GetCombinedPaymentOrderWithSubOrdersRow, payParams *wechat.JSAPIPayParams) (combinedPaymentOrderResponse, error) {
	var subOrders []combinedPaymentSubOrderResponse
	if err := json.Unmarshal(combinedRow.SubOrders, &subOrders); err != nil {
		return combinedPaymentOrderResponse{}, err
	}

	resp := combinedPaymentOrderResponse{
		ID:                combinedRow.ID,
		CombineOutTradeNo: combinedRow.CombineOutTradeNo,
		TotalAmount:       combinedRow.TotalAmount,
		Status:            combinedRow.Status,
		SubOrders:         subOrders,
		PayParams:         newMiniProgramPayParams(payParams),
	}
	if combinedRow.PrepayID.Valid {
		resp.PrepayID = &combinedRow.PrepayID.String
	}
	if combinedRow.ExpiresAt.Valid {
		expires := combinedRow.ExpiresAt.Time
		resp.ExpiresAt = &expires
	}
	return resp, nil
}

func newCombinedPaymentWechatQueryResult(query *logic.QueryCombinedPaymentWechatOrder) *combinedPaymentWechatQueryResult {
	if query == nil {
		return nil
	}

	subOrders := make([]combinedPaymentWechatSubOrderResult, 0, len(query.SubOrders))
	for _, subOrder := range query.SubOrders {
		subOrders = append(subOrders, combinedPaymentWechatSubOrderResult{
			OutTradeNo:      subOrder.OutTradeNo,
			TradeState:      subOrder.TradeState,
			SuccessTime:     subOrder.SuccessTime,
			PromotionDetail: newWechatPromotionDetails(subOrder.PromotionDetail),
			Amount: combinedPaymentWechatAmountResult{
				TotalAmount:   subOrder.Amount.TotalAmount,
				PayerAmount:   subOrder.Amount.PayerAmount,
				Currency:      subOrder.Amount.Currency,
				PayerCurrency: subOrder.Amount.PayerCurrency,
			},
		})
	}

	return &combinedPaymentWechatQueryResult{
		CombineOutTradeNo:   query.CombineOutTradeNo,
		AggregateTradeState: query.AggregateTradeState,
		SubOrders:           subOrders,
	}
}

// generateOutTradeNo 生成商户订单号。
func generateOutTradeNo() (string, error) {
	return util.GenerateOutTradeNo("P")
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
// @Description `payment_type` 为兼容字段，可不传；当前 `order` 和 `reservation` 主支付统一走普通服务商小程序支付。
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
// @Success 201 {object} paymentOrderResponse "支付订单(含小程序支付参数)"
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
		respondPaymentRequestError(ctx, "create_payment_order_bind_request", err, "支付请求参数格式无效，请选择订单并确认支付类型后重试")
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	server.normalizeCreatePaymentOrderRequest(&req)

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
		if isEcommerceClientNotConfigured(err) {
			ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "商户支付能力未完成配置，请联系平台处理", "merchant payment client not configured"))
			return
		}
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

// queryPaymentOrder godoc
// @Summary 查询支付订单远端状态
// @Description 查询本地普通支付订单详情，并按支付通道拉取微信普通服务商、平台收付通冷备或直连支付最新状态，供小程序恢复支付或判断后续动作
// @Tags 支付管理
// @Accept json
// @Produce json
// @Param id path int true "支付订单ID"
// @Success 200 {object} paymentOrderQueryResponse "支付订单详情(含微信远端状态)"
// @Failure 400 {object} ErrorResponse "请求参数错误或支付单不支持该查询"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "支付订单不属于当前用户"
// @Failure 404 {object} ErrorResponse "支付订单不存在"
// @Failure 503 {object} ErrorResponse "支付服务不可用"
// @Router /v1/payments/{id}/query [get]
// @Security BearerAuth
func (server *Server) queryPaymentOrder(ctx *gin.Context) {
	var req getPaymentOrderRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		respondPaymentRequestError(ctx, "query_payment_order_bind_uri", err, "支付单编号无效，请刷新页面后重试")
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	facade := server.paymentFacade
	if facade == nil {
		facade = server.buildPaymentFacade()
	}

	result, err := facade.QueryPaymentOrder(ctx, logic.QueryPaymentOrderInput{
		UserID:         authPayload.UserID,
		PaymentOrderID: req.ID,
	})
	if err != nil {
		if isEcommerceClientNotConfigured(err) {
			ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "商户支付能力未完成配置，当前无法确认支付状态，请联系平台处理", "query payment client not configured"))
			return
		}
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newPaymentOrderQueryResponse(result.PaymentOrder, result.PayParams, result.WechatOrder))
}

func (server *Server) normalizeCreatePaymentOrderRequest(req *createPaymentOrderRequest) {
	if req.PaymentType == "" {
		req.PaymentType = PaymentTypeMiniProgram
	}
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
	logErr := logic.LoggableError(reqErr)
	_ = ctx.Error(logErr)
	log.Error().
		Err(logErr).
		Str("request_id", GetRequestID(ctx)).
		Str("path", ctx.Request.URL.Path).
		Str("method", ctx.Request.Method).
		Int("status", reqErr.Status).
		Msg("logic request rejected")
	ctx.JSON(reqErr.Status, errorResponse(errors.New(logicRequestErrorPublicMessage(reqErr))))
	return true
}

func logicRequestErrorPublicMessage(reqErr *logic.RequestError) string {
	if reqErr == nil || reqErr.Err == nil {
		return "操作未完成，请稍后重试；如持续失败请联系平台处理"
	}

	message := strings.TrimSpace(reqErr.Err.Error())
	if message == "" {
		return "操作未完成，请稍后重试；如持续失败请联系平台处理"
	}
	if containsNonASCII(message) {
		return message
	}

	normalized := strings.ToLower(message)
	if mapped := legacyPaymentRequestMessage(normalized); mapped != "" {
		return mapped
	}

	switch reqErr.Status {
	case http.StatusBadRequest:
		return "请求参数或当前状态不满足操作条件，请刷新页面后重试；如仍失败请联系平台处理"
	case http.StatusForbidden:
		return "当前账号无权执行该操作，请确认登录账号后重试"
	case http.StatusNotFound:
		return "未找到要操作的记录，请刷新页面后重试"
	case http.StatusConflict:
		return "当前状态已变化，请刷新页面确认后重试"
	case http.StatusRequestTimeout:
		return "请求已取消，请重新发起操作"
	default:
		return "操作未完成，请稍后重试；如持续失败请联系平台处理"
	}
}

func containsNonASCII(message string) bool {
	for _, r := range message {
		if r > 127 {
			return true
		}
	}
	return false
}

func legacyPaymentRequestMessage(normalized string) string {
	switch normalized {
	case "invalid business type":
		return "支付业务类型无效，请返回订单页重新发起支付"
	case "reservation not found":
		return "未找到预订，请刷新页面后重试"
	case "reservation does not belong to you":
		return "当前预订不属于你，无法操作"
	case "reservation is not in pending status":
		return "当前预订已不在待支付状态，请刷新页面确认"
	case "order not found":
		return "未找到订单，请刷新页面后重试"
	case "order does not belong to you", "order does not belong to your merchant":
		return "当前订单不属于你，无法操作"
	case "order is not in pending status":
		return "当前订单已不在待支付状态，请刷新页面确认"
	case "payment amount must be greater than 0":
		return "支付金额必须大于 0，请刷新订单后重试"
	case "wechat openid not found":
		return "未获取到微信用户标识，请重新登录小程序后重试"
	case "payment order is being recreated, please retry", "payment order is still preparing, please retry":
		return "支付单仍在准备中，请稍后刷新后重试"
	case "payment order not found":
		return "未找到支付单，请刷新页面后重试"
	case "payment order does not belong to you":
		return "当前支付单不属于你，无法操作"
	case "payment order is not paid":
		return "支付单尚未完成支付，无法继续操作"
	case "payment order has no associated order":
		return "支付单缺少关联订单，请联系平台处理"
	case "only pending payment orders can be closed":
		return "只有待支付的支付单可以关闭，请刷新状态后重试"
	case "no sub orders available to close":
		return "未找到可关闭的合单子单，请刷新支付状态后重试"
	case "invalid order ids":
		return "请选择需要合并支付的订单后重试"
	case "orders are already preparing in another combined payment, please retry":
		return "订单正在准备合单支付，请稍后重试"
	case "combined payment order does not belong to you":
		return "当前合单支付不属于你，无法操作"
	case "combined payment order is still preparing, please retry":
		return "合单支付仍在准备中，请稍后刷新后重试"
	case "combined payment order not found":
		return "未找到合单支付单，请刷新页面后重试"
	case "only pending combined payment orders can be closed":
		return "只有待支付的合单支付单可以关闭，请刷新状态后重试"
	case "you are not a merchant":
		return "当前账号不是商户账号，无法执行该操作"
	case "refund amount exceeds payment amount":
		return "退款金额不能超过支付金额，请重新输入后重试"
	case "refund order not found":
		return "未找到退款单，请刷新页面后重试"
	case "refund order has no associated order":
		return "退款单缺少关联订单，请联系平台处理"
	case "refund order does not belong to your merchant":
		return "退款单不属于当前商户，无法操作"
	case "refund order is not in failed state":
		return "退款单未处于失败状态，请刷新后确认是否仍需异常退款"
	case "refund order has no wechat refund id":
		return "退款单缺少微信退款号，请联系平台处理"
	case "refund order is not an ecommerce refund":
		return "当前退款单不支持异常退款处理，请刷新退款状态"
	case "access denied":
		return "当前账号无权查看该记录"
	case "unsupported provider":
		return "设备推送服务商不支持，请更新商户端后重试；如仍失败请联系平台处理"
	case "merchant sub mchid not configured":
		return "商户微信支付商户号未配置，请联系平台处理"
	case "profit sharing order not found":
		return "未找到分账单，请刷新后重试"
	case "profit sharing order id missing":
		return "分账单缺少微信分账单号，请联系平台处理"
	case "operator not found for profit sharing":
		return "未找到分账运营商，请联系平台处理"
	case "operator wechat mchid not configured":
		return "运营商微信商户号未配置，请联系平台处理"
	case "request canceled":
		return "请求已取消，请重新发起操作"
	default:
		return ""
	}
}

func respondPaymentRequestError(ctx *gin.Context, operation string, err error, publicMessage string) {
	_ = ctx.Error(err)
	log.Warn().
		Err(err).
		Str("request_id", GetRequestID(ctx)).
		Str("path", ctx.Request.URL.Path).
		Str("method", ctx.Request.Method).
		Str("operation", operation).
		Msg("payment request rejected")
	ctx.JSON(http.StatusBadRequest, errorResponse(errors.New(publicMessage)))
}

func isEcommerceClientNotConfigured(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return (strings.Contains(msg, "ecommerce client") || strings.Contains(msg, "ordinary service provider client")) && strings.Contains(msg, "not configured")
}

// createCombinedPaymentOrder 创建普通服务商多订单合单支付订单
// @Summary 创建多订单合单支付订单
// @Description 为单次结算中的多个订单创建普通服务商合单支付订单，当前主要用于多商户外卖一起结算；平台收付通仅作为历史冷备通道保留
// @Tags 支付管理
// @Accept json
// @Produce json
// @Param request body createCombinedPaymentOrderRequest true "多订单合单支付参数"
// @Success 201 {object} combinedPaymentOrderResponse "多订单合单支付订单(含小程序支付参数)"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "订单不属于当前用户"
// @Failure 404 {object} ErrorResponse "订单不存在"
// @Failure 503 {object} ErrorResponse "支付服务不可用"
// @Router /v1/payments/combined [post]
// @Security BearerAuth
func (server *Server) createCombinedPaymentOrder(ctx *gin.Context) {
	var req createCombinedPaymentOrderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		respondPaymentRequestError(ctx, "create_combined_payment_order_bind_request", err, "合单支付请求参数格式无效，请选择至少一个订单后重试")
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
		if isEcommerceClientNotConfigured(err) {
			ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "合单支付能力未完成配置，请联系平台处理", "combined payment client not configured"))
			return
		}
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

// getCombinedPaymentOrder 获取多订单合单支付订单详情
type getCombinedPaymentOrderRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// getCombinedPaymentOrder godoc
// @Summary 获取多订单合单支付订单详情
// @Description 根据ID获取多订单合单支付订单详情
// @Tags 支付管理
// @Accept json
// @Produce json
// @Param id path int true "合单支付订单ID"
// @Success 200 {object} combinedPaymentOrderResponse "多订单合单支付订单详情"
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
		respondPaymentRequestError(ctx, "get_combined_payment_order_bind_uri", err, "合单支付单编号无效，请刷新页面后重试")
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

	resp, err := buildCombinedPaymentOrderResponse(result.CombinedPayment, result.PayParams)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, resp)
}

// queryCombinedPaymentOrder godoc
// @Summary 查询多订单合单支付远端状态
// @Description 查询本地多订单合单详情，并拉取微信侧最新支付状态，供小程序恢复支付或判断后续动作
// @Tags 支付管理
// @Accept json
// @Produce json
// @Param id path int true "合单支付订单ID"
// @Success 200 {object} combinedPaymentOrderResponse "多订单合单支付订单详情(含微信远端状态)"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "合单支付订单不属于当前用户"
// @Failure 404 {object} ErrorResponse "合单支付订单不存在"
// @Failure 503 {object} ErrorResponse "支付服务不可用"
// @Router /v1/payments/combined/{id}/query [get]
// @Security BearerAuth
func (server *Server) queryCombinedPaymentOrder(ctx *gin.Context) {
	var req getCombinedPaymentOrderRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		respondPaymentRequestError(ctx, "query_combined_payment_order_bind_uri", err, "合单支付单编号无效，请刷新页面后重试")
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	facade := server.paymentFacade
	if facade == nil {
		facade = server.buildPaymentFacade()
	}

	result, err := facade.QueryCombinedPaymentOrder(ctx, logic.QueryCombinedPaymentOrderInput{
		UserID:            authPayload.UserID,
		CombinedPaymentID: req.ID,
	})
	if err != nil {
		if isEcommerceClientNotConfigured(err) {
			ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "合单支付能力未完成配置，当前无法确认支付状态，请联系平台处理", "query combined payment client not configured"))
			return
		}
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	resp, err := buildCombinedPaymentOrderResponse(result.CombinedPayment, result.PayParams)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	resp.WechatQuery = newCombinedPaymentWechatQueryResult(result.WechatOrder)

	ctx.JSON(http.StatusOK, resp)
}

// closeCombinedPaymentOrder 关闭多订单合单支付订单
type closeCombinedPaymentOrderRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

// closeCombinedPaymentOrder godoc
// @Summary 关闭多订单合单支付订单
// @Description 关闭待支付的多订单合单支付订单
// @Tags 支付管理
// @Accept json
// @Produce json
// @Param id path int true "合单支付订单ID"
// @Success 200 {object} combinedPaymentOrderResponse "关闭成功的多订单合单支付订单"
// @Failure 400 {object} ErrorResponse "请求参数错误或订单状态不允许关闭"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "合单支付订单不属于当前用户"
// @Failure 404 {object} ErrorResponse "合单支付订单不存在"
// @Failure 503 {object} ErrorResponse "支付服务不可用"
// @Router /v1/payments/combined/{id}/close [post]
// @Security BearerAuth
func (server *Server) closeCombinedPaymentOrder(ctx *gin.Context) {
	var req closeCombinedPaymentOrderRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		respondPaymentRequestError(ctx, "close_combined_payment_order_bind_uri", err, "合单支付单编号无效，请刷新页面后重试")
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
		if isEcommerceClientNotConfigured(err) {
			ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "合单支付能力未完成配置，当前无法关闭支付单，请联系平台处理", "close combined payment client not configured"))
			return
		}
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
		respondPaymentRequestError(ctx, "get_payment_order_bind_uri", err, "支付单编号无效，请刷新页面后重试")
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
		respondPaymentRequestError(ctx, "list_payment_orders_bind_query", err, "支付记录查询条件无效，请调整分页条件后重试")
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
		respondPaymentRequestError(ctx, "close_payment_order_bind_uri", err, "支付单编号无效，请刷新页面后重试")
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
		if isEcommerceClientNotConfigured(err) {
			ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "支付关闭服务暂不可用，请稍后重试", "close payment client not configured"))
			return
		}
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

type applyAbnormalRefundURIRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

type applyAbnormalRefundBodyRequest struct {
	Type        string `json:"type" binding:"required,oneof=USER_BANK_CARD MERCHANT_BANK_CARD"`
	BankType    string `json:"bank_type,omitempty" binding:"omitempty,max=32"`
	BankAccount string `json:"bank_account,omitempty" binding:"omitempty,max=128"`
	RealName    string `json:"real_name,omitempty" binding:"omitempty,max=128"`
}

type abnormalRefundWechatResponse struct {
	RefundID string `json:"refund_id,omitempty"`
	Status   string `json:"status"`
}

type applyAbnormalRefundResponse struct {
	RefundOrder refundOrderResponse          `json:"refund_order"`
	Wechat      abnormalRefundWechatResponse `json:"wechat"`
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

// applyPlatformAbnormalRefund 平台人工发起异常退款处理
// @Summary 平台人工发起异常退款处理
// @Description 平台管理员对微信返回 ABNORMAL 的收付通退款单发起人工异常退款处理。
// @Tags 平台退款管理
// @Accept json
// @Produce json
// @Param id path int true "退款订单ID"
// @Param request body applyAbnormalRefundBodyRequest true "异常退款处理参数"
// @Success 200 {object} applyAbnormalRefundResponse "异常退款处理结果"
// @Failure 400 {object} ErrorResponse "请求参数错误或退款状态不允许处理"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 403 {object} ErrorResponse "非平台管理员"
// @Failure 404 {object} ErrorResponse "退款订单不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/platform/refunds/{id}/apply-abnormal-refund [post]
// @Security BearerAuth
func (server *Server) applyPlatformAbnormalRefund(ctx *gin.Context) {
	var uriReq applyAbnormalRefundURIRequest
	if err := ctx.ShouldBindUri(&uriReq); err != nil {
		respondPaymentRequestError(ctx, "apply_abnormal_refund_bind_uri", err, "退款单编号无效，请刷新页面后重试")
		return
	}

	var bodyReq applyAbnormalRefundBodyRequest
	if err := ctx.ShouldBindJSON(&bodyReq); err != nil {
		respondPaymentRequestError(ctx, "apply_abnormal_refund_bind_request", err, "异常退款处理参数格式无效，请核对收款账户类型和银行卡信息后重试")
		return
	}

	orchestrator := server.refundOrchestrator
	if orchestrator == nil {
		orchestrator = server.buildRefundOrchestrator()
	}

	result, err := orchestrator.ApplyAbnormalRefund(ctx, logic.ApplyAbnormalRefundInput{
		RefundID:    uriReq.ID,
		Type:        bodyReq.Type,
		BankType:    bodyReq.BankType,
		BankAccount: bodyReq.BankAccount,
		RealName:    bodyReq.RealName,
	})
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, applyAbnormalRefundResponse{
		RefundOrder: newRefundOrderResponse(result.RefundOrder),
		Wechat: abnormalRefundWechatResponse{
			RefundID: result.WechatRefund.RefundID,
			Status:   result.WechatRefund.Status,
		},
	})
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
		respondPaymentRequestError(ctx, "create_refund_order_bind_request", err, "退款申请参数格式无效，请选择支付单、退款类型和退款金额后重试")
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
		respondPaymentRequestError(ctx, "list_profit_sharing_returns_bind_uri", err, "退款单编号无效，请刷新页面后重试")
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
		respondPaymentRequestError(ctx, "get_refund_order_bind_uri", err, "退款单编号无效，请刷新页面后重试")
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
		respondPaymentRequestError(ctx, "list_refund_orders_by_payment_bind_uri", err, "支付单编号无效，请刷新页面后重试")
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
