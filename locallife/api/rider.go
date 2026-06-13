package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	"github.com/rs/zerolog/log"
)

const riderWithdrawProcessingStatus = "processing"
const riderDepositPaymentOrderObjectType = "payment_order"

func isRiderOnlineEligibleStatus(status string) bool {
	return status == db.RiderStatusActive
}

func (server *Server) getRiderDepositThreshold(ctx context.Context, rider db.Rider) (int64, error) {
	return db.GetEffectiveRiderDepositThreshold(ctx, server.store, rider.RegionID)
}

func recordRiderDepositRechargeCommandAccepted(ctx context.Context, store db.Store, paymentOrder db.PaymentOrder, prepayID string) {
	paymentCommandSvc := logic.NewPaymentCommandService(store)
	secondaryKey := prepayID
	if _, err := paymentCommandSvc.RecordExternalPaymentCommand(ctx, dbRiderDepositRechargeCommandInput(paymentOrder, db.ExternalPaymentCommandStatusAccepted, &secondaryKey, nil)); err != nil {
		log.Warn().Err(err).Int64("payment_order_id", paymentOrder.ID).Msg("record rider deposit recharge command accepted failed")
	}
}

func recordRiderDepositRechargeCommandRejected(ctx context.Context, store db.Store, paymentOrder db.PaymentOrder, submitErr error) {
	paymentCommandSvc := logic.NewPaymentCommandService(store)
	lastErrorCode, lastErrorMessage := directPaymentCommandErrorFields(submitErr)
	if _, err := paymentCommandSvc.RecordExternalPaymentCommand(ctx, dbRiderDepositRechargeCommandInput(paymentOrder, db.ExternalPaymentCommandStatusRejected, nil, map[string]string{
		"error_code":    stringValue(lastErrorCode),
		"error_message": stringValue(lastErrorMessage),
	})); err != nil {
		log.Warn().Err(err).Int64("payment_order_id", paymentOrder.ID).Msg("record rider deposit recharge command rejected failed")
	}
}

func dbRiderDepositRechargeCommandInput(paymentOrder db.PaymentOrder, status string, secondaryKey *string, snapshot map[string]string) logic.RecordExternalPaymentCommandInput {
	businessObjectID := paymentOrder.ID
	responseSnapshot := []byte(`{}`)
	if len(snapshot) > 0 {
		if encoded, err := json.Marshal(snapshot); err == nil {
			responseSnapshot = encoded
		}
	}
	if secondaryKey != nil && len(snapshot) == 0 {
		responseSnapshot, _ = json.Marshal(map[string]string{"prepay_id": *secondaryKey})
	}

	return logic.RecordExternalPaymentCommandInput{
		Provider:             db.ExternalPaymentProviderWechat,
		Channel:              db.PaymentChannelDirect,
		Capability:           db.ExternalPaymentCapabilityDirectJSAPIPayment,
		CommandType:          db.ExternalPaymentCommandTypeCreatePayment,
		BusinessOwner:        db.ExternalPaymentBusinessOwnerRiderDeposit,
		BusinessObjectType:   stringPtrIfNotEmpty(riderDepositPaymentOrderObjectType),
		BusinessObjectID:     &businessObjectID,
		ExternalObjectType:   db.ExternalPaymentObjectPayment,
		ExternalObjectKey:    paymentOrder.OutTradeNo,
		ExternalSecondaryKey: secondaryKey,
		CommandStatus:        status,
		LastErrorCode:        stringPtrIfNotEmpty(stringValueFromSnapshot(snapshot, "error_code")),
		LastErrorMessage:     stringPtrIfNotEmpty(stringValueFromSnapshot(snapshot, "error_message")),
		ResponseSnapshot:     responseSnapshot,
	}
}

func directPaymentCommandErrorFields(err error) (*string, *string) {
	if err == nil {
		return nil, nil
	}
	var wxErr *wechat.WechatPayError
	if errors.As(err, &wxErr) {
		return stringPtrIfNotEmpty(wxErr.Code), stringPtrIfNotEmpty(wxErr.Message)
	}
	return nil, stringPtrIfNotEmpty(err.Error())
}

func stringPtrIfNotEmpty(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func stringValue(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

func stringValueFromSnapshot(snapshot map[string]string, key string) string {
	if snapshot == nil {
		return ""
	}
	return snapshot[key]
}

type riderWithdrawRefundItemResponse struct {
	RefundOrderID  int64  `json:"refund_order_id"`
	PaymentOrderID int64  `json:"payment_order_id"`
	OutRefundNo    string `json:"out_refund_no"`
	Amount         int64  `json:"amount"`
	Status         string `json:"status"`
}

type riderWithdrawResponse struct {
	Status          string                            `json:"status"`
	RequestedAmount int64                             `json:"requested_amount"`
	AcceptedAmount  int64                             `json:"accepted_amount"`
	Refunds         []riderWithdrawRefundItemResponse `json:"refunds"`
}

// ==================== 骑手申请 ====================

type riderResponse struct {
	ID                int64      `json:"id"`
	UserID            int64      `json:"user_id"`
	RegionID          int64      `json:"region_id"`
	RealName          string     `json:"real_name"`
	Phone             string     `json:"phone"`
	DepositAmount     int64      `json:"deposit_amount"`
	FrozenDeposit     int64      `json:"frozen_deposit"`
	Status            string     `json:"status"`
	IsOnline          bool       `json:"is_online"`
	CreditScore       int16      `json:"credit_score"`
	TotalOrders       int32      `json:"total_orders"`
	TotalEarnings     int64      `json:"total_earnings"`
	OnlineDuration    int32      `json:"online_duration"`
	CurrentLongitude  *float64   `json:"current_longitude,omitempty"`
	CurrentLatitude   *float64   `json:"current_latitude,omitempty"`
	LocationUpdatedAt *time.Time `json:"location_updated_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

func newRiderResponse(rider db.Rider) riderResponse {
	resp := riderResponse{
		ID:             rider.ID,
		UserID:         rider.UserID,
		RegionID:       rider.RegionID.Int64,
		RealName:       rider.RealName,
		Phone:          rider.Phone,
		DepositAmount:  rider.DepositAmount,
		FrozenDeposit:  rider.FrozenDeposit,
		Status:         rider.Status,
		IsOnline:       rider.IsOnline,
		CreditScore:    rider.CreditScore,
		TotalOrders:    rider.TotalOrders,
		TotalEarnings:  rider.TotalEarnings,
		OnlineDuration: rider.OnlineDuration,
		CreatedAt:      rider.CreatedAt,
	}

	if rider.CurrentLongitude.Valid {
		lng, _ := rider.CurrentLongitude.Float64Value()
		resp.CurrentLongitude = &lng.Float64
	}
	if rider.CurrentLatitude.Valid {
		lat, _ := rider.CurrentLatitude.Float64Value()
		resp.CurrentLatitude = &lat.Float64
	}
	if rider.LocationUpdatedAt.Valid {
		resp.LocationUpdatedAt = &rider.LocationUpdatedAt.Time
	}

	return resp
}

type syncRiderCurrentRegionRequest struct {
	RegionID int64 `json:"region_id" binding:"required,gt=0"`
}

func (server *Server) syncRiderCurrentRegion(ctx context.Context, rider db.Rider, regionID int64) (db.Rider, error) {
	if _, err := server.store.GetRegion(ctx, regionID); err != nil {
		return db.Rider{}, fmt.Errorf("get current rider region %d: %w", regionID, err)
	}

	updated := rider
	if !rider.RegionID.Valid || rider.RegionID.Int64 != regionID {
		var err error
		updated, err = server.store.UpdateRiderRegion(ctx, db.UpdateRiderRegionParams{
			ID:       rider.ID,
			RegionID: pgtype.Int8{Int64: regionID, Valid: true},
		})
		if err != nil {
			return db.Rider{}, fmt.Errorf("update rider current region: %w", err)
		}
	}

	switch updated.Status {
	case db.RiderStatusApproved, db.RiderStatusActive, db.RiderStatusSuspended:
		var err error
		updated, err = db.ReconcileRiderOperationalStatus(ctx, server.store, updated)
		if err != nil {
			return db.Rider{}, fmt.Errorf("reconcile rider operational status: %w", err)
		}
	}

	return updated, nil
}

// syncCurrentRiderRegion godoc
// @Summary 同步骑手当前运营区域
// @Description 将骑手当前运营区域更新为小程序定位匹配出的 region_id，并按该区域押金规则刷新运营状态
// @Tags 骑手
// @Accept json
// @Produce json
// @Param request body syncRiderCurrentRegionRequest true "当前运营区域"
// @Success 200 {object} riderResponse "骑手信息"
// @Failure 400 {object} ErrorResponse "参数错误或区域不存在"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "未注册骑手"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/current-region [patch]
// @Security BearerAuth
func (server *Server) syncCurrentRiderRegion(ctx *gin.Context) {
	var req syncRiderCurrentRegionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrRiderNotRegistered))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	updated, err := server.syncRiderCurrentRegion(ctx, rider, req.RegionID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrRegionNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newRiderResponse(updated))
}

// getRiderMe godoc
// @Summary 获取当前骑手信息
// @Description 获取当前登录用户的骑手信息，包括状态、押金、统计数据等
// @Tags 骑手
// @Accept json
// @Produce json
// @Success 200 {object} riderResponse "骑手信息"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "未注册骑手"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/me [get]
// @Security BearerAuth
func (server *Server) getRiderMe(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrRiderNotRegistered))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newRiderResponse(rider))
}

// ==================== 押金管理 ====================

// 押金常量定义
const (
	// MinDepositAmount 最小充值金额：100分 = 1元
	MinDepositAmount = 1 * fenPerYuan
	// MaxDepositAmount 最大充值金额：1000000分 = 10000元
	MaxDepositAmount = 10000 * fenPerYuan
	// MinWithdrawAmount 最小提现金额：100分 = 1元
	MinWithdrawAmount = 1 * fenPerYuan
	// MaxWithdrawAmount 单次最大提现金额：5000000分 = 50000元
	MaxWithdrawAmount = 50000 * fenPerYuan
)

type depositRequest struct {
	Amount int64  `json:"amount" binding:"required,min=100,max=1000000"`
	Remark string `json:"remark" binding:"max=200"`
}

type withdrawRequest struct {
	Amount int64  `json:"amount" binding:"required,min=100,max=5000000"`
	Remark string `json:"remark" binding:"max=200"`
}

type depositResponse struct {
	ID           int64     `json:"id"`
	RiderID      int64     `json:"rider_id"`
	Amount       int64     `json:"amount"`
	Type         string    `json:"type"`
	BalanceAfter int64     `json:"balance_after"`
	Remark       string    `json:"remark,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// depositBalanceResponse 押金余额响应
type depositBalanceResponse struct {
	CurrentRegionID            int64 `json:"current_region_id"`            // 当前运营区域
	RequiredDeposit            int64 `json:"required_deposit"`             // 当前区域上线所需押金
	TotalDeposit               int64 `json:"total_deposit"`                // 总押金
	FrozenDeposit              int64 `json:"frozen_deposit"`               // 冻结押金（兼容字段，等于代取冻结+提现处理中）
	DeliveryFrozenDeposit      int64 `json:"delivery_frozen_deposit"`      // 代取冻结
	WithdrawalProcessingAmount int64 `json:"withdrawal_processing_amount"` // 提现处理中
	AvailableDeposit           int64 `json:"available_deposit"`            // 可用押金
}

type paginationRequest struct {
	Page  int32 `form:"page" binding:"min=0"`
	Limit int32 `form:"limit" binding:"min=0,max=100"`
}

func newDepositResponse(d db.RiderDeposit) depositResponse {
	resp := depositResponse{
		ID:           d.ID,
		RiderID:      d.RiderID,
		Amount:       d.Amount,
		Type:         d.Type,
		BalanceAfter: d.BalanceAfter,
		CreatedAt:    d.CreatedAt,
	}
	if d.Remark.Valid {
		resp.Remark = d.Remark.String
	}
	return resp
}

// depositRider godoc
// @Summary 骑手押金充值
// @Description 创建骑手押金充值支付订单，返回微信支付参数
// @Tags 骑手
// @Accept json
// @Produce json
// @Param request body depositRequest true "充值金额（单位：分）"
// @Success 200 {object} object "微信支付参数"
// @Failure 400 {object} ErrorResponse "参数错误或骑手未激活"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "未注册骑手"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/deposit [post]
// @Security BearerAuth
func (server *Server) depositRider(ctx *gin.Context) {
	var req depositRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrRiderNotRegistered))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if rider.Status != "approved" && rider.Status != "active" {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrRiderNotActivated))
		return
	}
	if server.directPaymentClient == nil {
		err := errors.New("payment service not configured")
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "payment service not configured", "rider deposit payment service not configured"))
		return
	}

	// 创建支付订单
	// 骑手押金充值使用单独的 business_type，回调时根据 user_id 找到对应骑手
	expiresAt := time.Now().Add(30 * time.Minute)

	var outTradeNo string
	var paymentOrder db.PaymentOrder
	createdPaymentOrder := false

	// 幂等保护：检查是否已有未过期的 pending 支付单（同用户、同业务类型、同金额）
	existingPayment, findErr := server.store.GetPendingPaymentOrderByUserAndBusinessType(ctx, db.GetPendingPaymentOrderByUserAndBusinessTypeParams{
		UserID:       authPayload.UserID,
		BusinessType: BusinessTypeRiderDeposit,
		Amount:       req.Amount,
	})
	if findErr == nil {
		// 已存在未过期的 pending 单，复用
		paymentOrder = existingPayment
		outTradeNo = existingPayment.OutTradeNo
		if paymentOrder.PrepayID.Valid && strings.TrimSpace(paymentOrder.PrepayID.String) != "" {
			payParams, signErr := server.directPaymentClient.GenerateJSAPIPayParams(paymentOrder.PrepayID.String)
			if signErr != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("generate rider deposit pay params: %w", signErr)))
				return
			}
			resp := map[string]interface{}{
				"payment_order_id": paymentOrder.ID,
				"out_trade_no":     outTradeNo,
				"amount":           paymentOrder.Amount,
				"expires_at":       paymentOrder.ExpiresAt.Time,
				"pay_params": map[string]string{
					"timeStamp": payParams.TimeStamp,
					"nonceStr":  payParams.NonceStr,
					"package":   payParams.Package,
					"signType":  payParams.SignType,
					"paySign":   payParams.PaySign,
				},
			}
			ctx.JSON(http.StatusOK, resp)
			return
		}
	} else {
		for attempt := 1; attempt <= outTradeNoMaxRetry; attempt++ {
			var genErr error
			outTradeNo, genErr = generateOutTradeNo()
			if genErr != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, genErr))
				return
			}
			paymentOrder, err = server.store.CreatePaymentOrder(ctx, db.CreatePaymentOrderParams{
				UserID:                authPayload.UserID,
				PaymentType:           PaymentTypeMiniProgram,
				PaymentChannel:        db.PaymentChannelDirect,
				RequiresProfitSharing: false,
				BusinessType:          BusinessTypeRiderDeposit, // 骑手押金充值
				Amount:                req.Amount,
				OutTradeNo:            outTradeNo,
				ExpiresAt:             pgtype.Timestamptz{Time: expiresAt, Valid: true},
			})
			if err == nil {
				createdPaymentOrder = true
				break
			}
			if isOutTradeNoConflict(err) && attempt < outTradeNoMaxRetry {
				if !sleepWithContext(ctx.Request.Context(), outTradeNoRetryBaseBack*time.Duration(attempt)) {
					ctx.JSON(http.StatusRequestTimeout, errorResponse(errors.New("request canceled")))
					return
				}
				continue
			}
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}

	// 构建响应
	resp := map[string]interface{}{
		"payment_order_id": paymentOrder.ID,
		"out_trade_no":     outTradeNo,
		"amount":           req.Amount,
		"expires_at":       expiresAt,
	}

	// 调用微信支付 API 创建预支付订单
	user, err := server.store.GetUser(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get user: %w", err)))
		return
	}

	wxResp, payParams, err := server.directPaymentClient.CreateJSAPIOrder(ctx, &wechatcontracts.DirectJSAPIOrderRequest{
		OutTradeNo:    outTradeNo,
		Description:   "骑手押金充值",
		TotalAmount:   req.Amount,
		PayerOpenID:   user.WechatOpenid,
		ExpireTime:    expiresAt,
		Attach:        fmt.Sprintf("deposit:rider_%d", authPayload.UserID),
		PayerClientIP: ctx.ClientIP(),
	})
	if err != nil {
		if createdPaymentOrder {
			if _, closeErr := server.store.UpdatePaymentOrderToClosed(ctx, paymentOrder.ID); closeErr != nil {
				ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("close rider deposit payment order after jsapi create failure: %w", closeErr)))
				return
			}
		}
		recordRiderDepositRechargeCommandRejected(ctx, server.store, paymentOrder, err)
		if writeLogicRequestError(ctx, logic.MapDirectJSAPIOrderCreateError(err)) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("wechat pay: %w", err)))
		return
	}

	// 更新 prepay_id（非关键字段：为空不影响支付回调或退款，仅影响审计对账）
	if _, err := server.store.UpdatePaymentOrderPrepayId(ctx, db.UpdatePaymentOrderPrepayIdParams{
		ID:       paymentOrder.ID,
		PrepayID: pgtype.Text{String: wxResp.PrepayID, Valid: true},
	}); err != nil {
		log.Warn().Err(err).Int64("payment_order_id", paymentOrder.ID).Msg("failed to update prepay_id, record will lack prepay_id for audit")
	}
	recordRiderDepositRechargeCommandAccepted(ctx, server.store, paymentOrder, wxResp.PrepayID)

	// 返回支付参数
	resp["pay_params"] = map[string]string{
		"timeStamp": payParams.TimeStamp,
		"nonceStr":  payParams.NonceStr,
		"package":   payParams.Package,
		"signType":  payParams.SignType,
		"paySign":   payParams.PaySign,
	}

	ctx.JSON(http.StatusOK, resp)
}

// withdrawRider godoc
// @Summary 骑手押金提现
// @Description 从骑手账户提取押金到微信零钱，需要确保没有进行中的代取订单。最小提现金额1元，单次最大提现金额50000元。
// @Tags 骑手
// @Accept json
// @Produce json
// @Param request body withdrawRequest true "提现金额（单位：分）"
// @Success 200 {object} riderWithdrawResponse "提现已完成对账"
// @Success 202 {object} riderWithdrawResponse "提现请求已受理，等待异步终态"
// @Failure 400 {object} ErrorResponse "余额不足、有进行中订单或账号未激活"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "未注册骑手"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/withdraw [post]
// @Security BearerAuth
func (server *Server) withdrawRider(ctx *gin.Context) {
	var req withdrawRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	service := server.buildRiderDepositRefundService()
	result, err := service.SubmitWithdrawal(ctx, logic.SubmitRiderDepositWithdrawalInput{
		UserID: authPayload.UserID,
		Amount: req.Amount,
		Remark: req.Remark,
	})
	if err != nil {
		var reqErr *logic.RequestError
		if errors.As(err, &reqErr) {
			switch {
			case errors.Is(reqErr.Err, logic.ErrRiderProfileNotFound):
				ctx.JSON(http.StatusNotFound, errorResponse(ErrRiderNotRegistered))
			case errors.Is(reqErr.Err, logic.ErrRiderAccountNotActivated):
				ctx.JSON(http.StatusBadRequest, errorResponse(ErrRiderNotActivated))
			case errors.Is(reqErr.Err, logic.ErrRiderDepositFrozen):
				ctx.JSON(http.StatusConflict, errorResponse(ErrRiderDepositFrozen))
			case errors.Is(reqErr.Err, logic.ErrRiderAvailableDepositInsufficient):
				ctx.JSON(http.StatusBadRequest, errorResponse(ErrRiderInsufficientBalance))
			case errors.Is(reqErr.Err, logic.ErrRiderHasActiveDeliveries):
				ctx.JSON(http.StatusBadRequest, errorResponse(ErrRiderHasActiveOrders))
			default:
				ctx.JSON(reqErr.Status, errorResponse(reqErr.Err))
			}
			return
		}

		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	response := riderWithdrawResponse{
		Status:          result.Status,
		RequestedAmount: result.RequestedAmount,
		AcceptedAmount:  result.AcceptedAmount,
		Refunds:         make([]riderWithdrawRefundItemResponse, 0, len(result.Refunds)),
	}
	for _, item := range result.Refunds {
		response.Refunds = append(response.Refunds, riderWithdrawRefundItemResponse{
			RefundOrderID:  item.RefundOrder.ID,
			PaymentOrderID: item.PaymentOrder.ID,
			OutRefundNo:    item.RefundOrder.OutRefundNo,
			Amount:         item.RefundOrder.RefundAmount,
			Status:         item.Status,
		})
	}

	if response.Status == "success" {
		ctx.JSON(http.StatusOK, response)
		return
	}

	ctx.JSON(http.StatusAccepted, response)
}

// getRiderDepositBalance godoc
// @Summary 获取押金余额
// @Description 获取当前骑手的押金余额信息，包括总押金、冻结押金和可用押金
// @Tags 骑手
// @Accept json
// @Produce json
// @Success 200 {object} depositBalanceResponse "押金余额信息"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "未注册骑手"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/deposit [get]
// @Security BearerAuth
func (server *Server) getRiderDepositBalance(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrRiderNotRegistered))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if !rider.RegionID.Valid || rider.RegionID.Int64 <= 0 {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrRiderNoRegionAssigned))
		return
	}

	withdrawalProcessingAmount, err := server.store.GetPendingRiderDepositRefundAmountByUserID(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	threshold, err := server.getRiderDepositThreshold(ctx, rider)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get rider deposit threshold: %w", err)))
		return
	}

	availability := db.CalculateRiderDepositAvailability(rider, withdrawalProcessingAmount)

	response := depositBalanceResponse{
		CurrentRegionID:            rider.RegionID.Int64,
		RequiredDeposit:            threshold,
		TotalDeposit:               rider.DepositAmount,
		FrozenDeposit:              rider.FrozenDeposit,
		DeliveryFrozenDeposit:      availability.DeliveryFrozenDeposit,
		WithdrawalProcessingAmount: availability.WithdrawalProcessingAmount,
		AvailableDeposit:           availability.AvailableDeposit,
	}

	ctx.JSON(http.StatusOK, response)
}

type listRiderDepositsResponse struct {
	Deposits []depositResponse `json:"deposits"`
	Total    int64             `json:"total"`
	PageID   int32             `json:"page_id"`
	PageSize int32             `json:"page_size"`
}

// listRiderDeposits godoc
// @Summary 查询押金流水
// @Description 分页查询当前骑手的押金变动流水记录，包括充值、提现、代取冻结、解冻、扣款等；提现冻结中间流水不作为账单明细返回
// @Tags 骑手
// @Accept json
// @Produce json
// @Param page query int false "页码" minimum(1) default(1)
// @Param limit query int false "每页数量" minimum(1) maximum(100) default(20)
// @Success 200 {object} listRiderDepositsResponse "押金流水列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "未注册骑手"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/deposits [get]
// @Security BearerAuth
func (server *Server) listRiderDeposits(ctx *gin.Context) {
	var req paginationRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 处理分页默认值
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Limit <= 0 {
		req.Limit = 20
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrRiderNotRegistered))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	deposits, err := server.store.ListRiderDeposits(ctx, db.ListRiderDepositsParams{
		RiderID: rider.ID,
		Limit:   req.Limit,
		Offset:  pageOffset(req.Page, req.Limit),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	totalCount, err := server.store.CountRiderDeposits(ctx, rider.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 确保返回空数组而非 null
	response := make([]depositResponse, 0, len(deposits))
	for _, d := range deposits {
		response = append(response, newDepositResponse(d))
	}

	ctx.JSON(http.StatusOK, listRiderDepositsResponse{
		Deposits: response,
		Total:    totalCount,
		PageID:   req.Page,
		PageSize: req.Limit,
	})
}

// ==================== 上下线管理 ====================

type goOnlineRequest struct {
	RegionID int64 `json:"region_id" binding:"required,gt=0"`
}

// riderStatusResponse 骑手状态响应
type riderStatusResponse struct {
	Status            string                            `json:"status"`            // 账号状态：approved/active/suspended
	IsOnline          bool                              `json:"is_online"`         // 是否在线
	OnlineStatus      string                            `json:"online_status"`     // 在线状态描述：offline/online/delivering
	ActiveDeliveries  int                               `json:"active_deliveries"` // 当前代取中订单数量
	CurrentRegionID   int64                             `json:"current_region_id"`
	RequiredDeposit   int64                             `json:"required_deposit"`
	CurrentLongitude  *float64                          `json:"current_longitude,omitempty"`
	CurrentLatitude   *float64                          `json:"current_latitude,omitempty"`
	LocationUpdatedAt *time.Time                        `json:"location_updated_at,omitempty"`
	CanGoOnline       bool                              `json:"can_go_online"`                 // 是否可以上线
	CanGoOffline      bool                              `json:"can_go_offline"`                // 是否可以下线
	OnlineBlockReason string                            `json:"online_block_reason,omitempty"` // 不能上线的原因
	SettlementAccount *baofuSettlementReadinessResponse `json:"settlement_account,omitempty"`
}

type baofuSettlementReadinessResponse struct {
	State        string `json:"state"`
	Label        string `json:"label"`
	PaymentReady bool   `json:"payment_ready"`
}

// getRiderStatus godoc
// @Summary 获取骑手当前状态
// @Description 获取骑手当前在线状态、位置信息、代取状态等
// @Tags 骑手
// @Accept json
// @Produce json
// @Success 200 {object} riderStatusResponse "骑手状态"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "未注册骑手"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/status [get]
// @Security BearerAuth
func (server *Server) getRiderStatus(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrRiderNotRegistered))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if !rider.RegionID.Valid || rider.RegionID.Int64 <= 0 {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrRiderNoRegionAssigned))
		return
	}

	// 获取活跃代取数量
	activeDeliveries, err := server.store.ListRiderActiveDeliveries(ctx, pgtype.Int8{Int64: rider.ID, Valid: true})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	threshold, err := server.getRiderDepositThreshold(ctx, rider)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get rider deposit threshold: %w", err)))
		return
	}

	resp := riderStatusResponse{
		Status:           rider.Status,
		IsOnline:         rider.IsOnline,
		ActiveDeliveries: len(activeDeliveries),
		CurrentRegionID:  rider.RegionID.Int64,
		RequiredDeposit:  threshold,
	}
	readiness, err := server.getRiderBaofuSettlementReadiness(ctx, rider)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	resp.SettlementAccount = newBaofuSettlementReadinessResponse(readiness)

	// 确定在线状态描述
	if !rider.IsOnline {
		resp.OnlineStatus = "offline"
	} else if len(activeDeliveries) > 0 {
		resp.OnlineStatus = "delivering"
	} else {
		resp.OnlineStatus = "online"
	}

	// 位置信息
	if rider.CurrentLongitude.Valid {
		lng, _ := rider.CurrentLongitude.Float64Value()
		resp.CurrentLongitude = &lng.Float64
	}
	if rider.CurrentLatitude.Valid {
		lat, _ := rider.CurrentLatitude.Float64Value()
		resp.CurrentLatitude = &lat.Float64
	}
	if rider.LocationUpdatedAt.Valid {
		resp.LocationUpdatedAt = &rider.LocationUpdatedAt.Time
	}

	// 判断是否可以上线/下线
	availableDeposit := rider.DepositAmount - rider.FrozenDeposit
	if rider.Status == db.RiderStatusSuspended {
		resp.CanGoOnline = false
		resp.OnlineBlockReason = "账号已停用"
	} else if !isRiderOnlineEligibleStatus(rider.Status) {
		resp.CanGoOnline = false
		resp.OnlineBlockReason = "账号尚未激活，暂不可上线"
	} else if availableDeposit < threshold {
		resp.CanGoOnline = false
		resp.OnlineBlockReason = fmt.Sprintf("可用押金不足，需要至少%s元", fenToYuanString(threshold, 0))
	} else if readiness.PaymentReady == false {
		resp.CanGoOnline = false
		resp.OnlineBlockReason = ErrRiderBaofuAccountMissing.Error()
	} else {
		resp.CanGoOnline = true
	}

	// 有活跃代取时不能下线
	resp.CanGoOffline = rider.IsOnline && len(activeDeliveries) == 0

	ctx.JSON(http.StatusOK, resp)
}

// goOnline godoc
// @Summary 骑手上线
// @Description 设置骑手状态为在线，开始接受订单。需要骑手处于 active 且可用押金达到平台阈值
// @Tags 骑手
// @Accept json
// @Produce json
// @Param request body goOnlineRequest true "当前运营区域"
// @Success 200 {object} riderResponse "上线成功"
// @Failure 400 {object} ErrorResponse "账号未激活或押金不足"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "未注册骑手"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/online [post]
// @Security BearerAuth
func (server *Server) goOnline(ctx *gin.Context) {
	var req goOnlineRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrRiderNotRegistered))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	rider, err = server.syncRiderCurrentRegion(ctx, rider, req.RegionID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrRegionNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if !isRiderOnlineEligibleStatus(rider.Status) {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrRiderNotActivated))
		return
	}

	threshold, err := server.getRiderDepositThreshold(ctx, rider)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("get rider deposit threshold: %w", err)))
		return
	}

	// 检查押金余额
	if rider.DepositAmount-rider.FrozenDeposit < threshold {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrRiderDepositInsufficient))
		return
	}

	if err := server.ensureRiderBaofuSettlementReady(ctx, rider); err != nil {
		if errors.Is(err, ErrRiderBaofuAccountMissing) {
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrRiderBaofuAccountMissing))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if rider.IsOnline {
		ctx.JSON(http.StatusOK, newRiderResponse(rider))
		return
	}

	updated, err := server.store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{
		ID:       rider.ID,
		IsOnline: true,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newRiderResponse(updated))
}

// goOffline godoc
// @Summary 骑手下线
// @Description 设置骑手状态为离线，停止接单。如果有进行中的代取订单则无法下线
// @Tags 骑手
// @Accept json
// @Produce json
// @Success 200 {object} riderResponse "下线成功"
// @Failure 400 {object} ErrorResponse "有进行中订单"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "未注册骑手"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/offline [post]
// @Security BearerAuth
func (server *Server) goOffline(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrRiderNotRegistered))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 检查是否有进行中的订单
	activeDeliveries, err := server.store.ListRiderActiveDeliveries(ctx, pgtype.Int8{Int64: rider.ID, Valid: true})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if len(activeDeliveries) > 0 {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrRiderHasActiveOrders))
		return
	}

	if !rider.IsOnline {
		ctx.JSON(http.StatusOK, newRiderResponse(rider))
		return
	}

	updated, err := server.store.UpdateRiderOnlineStatus(ctx, db.UpdateRiderOnlineStatusParams{
		ID:       rider.ID,
		IsOnline: false,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newRiderResponse(updated))
}

// ==================== 位置上报 ====================

type updateLocationRequest struct {
	RegionID  int64           `json:"region_id" binding:"required,gt=0"`
	Locations []locationPoint `json:"locations" binding:"required,min=1,max=100,dive"`
}

type riderLocationUpdateResponse struct {
	Message   string  `json:"message"`
	Count     int     `json:"count"`
	Longitude float64 `json:"longitude"`
	Latitude  float64 `json:"latitude"`
}

type locationPoint struct {
	DeliveryID *int64    `json:"delivery_id,omitempty" binding:"omitempty,min=1"`
	Longitude  float64   `json:"longitude" binding:"required,gte=-180,lte=180"`
	Latitude   float64   `json:"latitude" binding:"required,gte=-90,lte=90"`
	Accuracy   *float64  `json:"accuracy,omitempty" binding:"omitempty,gte=0,lte=1000"` // GPS精度(米)，0-1000
	Speed      *float64  `json:"speed,omitempty" binding:"omitempty,gte=0,lte=200"`     // 速度(m/s)，0-200(约720km/h)
	Heading    *float64  `json:"heading,omitempty" binding:"omitempty,gte=0,lte=360"`   // 航向角(度)，0-360
	RecordedAt time.Time `json:"recorded_at" binding:"required"`
	Source     string    `json:"source,omitempty"`
}

// updateRiderLocation godoc
// @Summary 更新骑手位置
// @Description 批量上报骑手GPS位置点，仅在线状态可调用。可选传 delivery_id（必须为当前进行中代取）与 source 标识上报来源
// @Tags 骑手
// @Accept json
// @Produce json
// @Param request body updateLocationRequest true "位置点数组"
// @Success 200 {object} object{message=string,count=int,longitude=number,latitude=number} "上报成功"
// @Failure 400 {object} ErrorResponse "参数错误或不在线"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "未注册骑手"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/location [post]
// @Security BearerAuth
func (server *Server) updateRiderLocation(ctx *gin.Context) {
	var req updateLocationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 时间验证：设备时间可能漂移，未来时间统一收敛到服务端接收时间；
	// 历史时间仍拒绝，避免轨迹和围栏判断使用过期位置。
	now := time.Now()
	maxPast := now.Add(-1 * time.Hour)
	for i := range req.Locations {
		loc := &req.Locations[i]
		if loc.RecordedAt.After(now) {
			loc.RecordedAt = now
		}
		if loc.RecordedAt.Before(maxPast) {
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrLocationTimestampTooOld))
			return
		}
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrRiderNotRegistered))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if !rider.IsOnline {
		ctx.JSON(http.StatusBadRequest, errorResponse(ErrRiderOffline))
		return
	}

	// 获取当前活跃订单
	var activeDeliveryID *int64
	activeDeliveries, err := server.store.ListRiderActiveDeliveries(ctx, pgtype.Int8{Int64: rider.ID, Valid: true})
	if err == nil && len(activeDeliveries) > 0 {
		activeDeliveryID = &activeDeliveries[0].ID
	}

	// 批量插入位置记录
	var locations []db.BatchCreateRiderLocationsParams
	var latestLocation locationPoint

	for _, loc := range req.Locations {
		if loc.DeliveryID != nil {
			if activeDeliveryID == nil || *loc.DeliveryID != *activeDeliveryID {
				ctx.JSON(http.StatusBadRequest, errorResponse(ErrRiderActiveOrderOnly))
				return
			}
		}

		param := db.BatchCreateRiderLocationsParams{
			RiderID:    rider.ID,
			Longitude:  numericFromFloat(loc.Longitude),
			Latitude:   numericFromFloat(loc.Latitude),
			RecordedAt: loc.RecordedAt,
		}

		deliveryID := activeDeliveryID
		if loc.DeliveryID != nil {
			deliveryID = loc.DeliveryID
		}
		if deliveryID != nil {
			param.DeliveryID = pgtype.Int8{Int64: *deliveryID, Valid: true}
		}
		if loc.Accuracy != nil {
			param.Accuracy = numericFromFloat(*loc.Accuracy)
		}
		if loc.Speed != nil {
			param.Speed = numericFromFloat(*loc.Speed)
		}
		if loc.Heading != nil {
			param.Heading = numericFromFloat(*loc.Heading)
		}

		locations = append(locations, param)

		// 记录最新位置
		if loc.RecordedAt.After(latestLocation.RecordedAt) {
			latestLocation = loc
		}
	}

	// 批量插入
	_, err = server.store.BatchCreateRiderLocations(ctx, locations)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	// 更新骑手最新位置
	updatedRider, err := server.store.UpdateRiderLocation(ctx, db.UpdateRiderLocationParams{
		ID:               rider.ID,
		CurrentLongitude: numericFromFloat(latestLocation.Longitude),
		CurrentLatitude:  numericFromFloat(latestLocation.Latitude),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	updatedRider, err = server.syncRiderCurrentRegion(ctx, updatedRider, req.RegionID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusBadRequest, errorResponse(ErrRegionNotFound))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if activeDeliveryID != nil {
		server.processDeliveryLocationEvents(ctx, updatedRider, *activeDeliveryID, latestLocation)
	}

	ctx.JSON(http.StatusOK, riderLocationUpdateResponse{
		Message:   "位置更新成功",
		Count:     len(locations),
		Longitude: latestLocation.Longitude,
		Latitude:  latestLocation.Latitude,
	})
}

// ==================== 管理员接口 ====================

type listRidersRequest struct {
	Status string `form:"status"`
	Page   int32  `form:"page" binding:"min=1"`
	Limit  int32  `form:"limit" binding:"min=1,max=100"`
}

type listRidersResponse struct {
	Riders   []riderResponse `json:"riders"`
	Total    int64           `json:"total"`
	PageID   int32           `json:"page_id"`
	PageSize int32           `json:"page_size"`
}

// listRiders godoc
// @Summary 获取骑手列表（管理员）
// @Description 管理员或运营商分页获取骑手列表，支持状态筛选
// @Tags 骑手管理
// @Accept json
// @Produce json
// @Param status query string false "筛选状态" Enums(approved, active, suspended)
// @Param page query int false "页码" minimum(1) default(1)
// @Param limit query int false "每页数量" minimum(1) maximum(100) default(20)
// @Success 200 {object} listRidersResponse "骑手列表"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/admin/riders [get]
// @Security BearerAuth
func (server *Server) listRiders(ctx *gin.Context) {
	var req listRidersRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	if req.Page == 0 {
		req.Page = 1
	}
	if req.Limit == 0 {
		req.Limit = 20
	}

	if req.Status == "" {
		req.Status = db.RiderStatusActive
	}

	riders, err := server.store.ListRidersByStatus(ctx, db.ListRidersByStatusParams{
		Status: req.Status,
		Limit:  req.Limit,
		Offset: pageOffset(req.Page, req.Limit),
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	totalCount, err := server.store.CountRidersByStatus(ctx, req.Status)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	var response []riderResponse
	for _, r := range riders {
		response = append(response, newRiderResponse(r))
	}

	ctx.JSON(http.StatusOK, listRidersResponse{
		Riders:   response,
		Total:    totalCount,
		PageID:   req.Page,
		PageSize: req.Limit,
	})
}
