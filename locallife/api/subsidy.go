package api

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	"github.com/rs/zerolog/log"
)

// ===========================================================================
// 补差（Subsidy）API
// 2026-04-18 备注：Finding 4 已明确 deferred，当前仍保留 legacy operator 补差路由语义，
// 本轮不推进对象级授权整改本体。
// 路由（均需 Operator 权限）：
//   POST /v1/operator/payment-orders/:id/subsidies        createSubsidy
//   POST /v1/operator/payment-orders/:id/subsidies/return returnSubsidy
//   POST /v1/operator/payment-orders/:id/subsidies/cancel cancelSubsidy
// ===========================================================================

// subsidyOrderResponse HTTP 补差订单响应体
type subsidyOrderResponse struct {
	ID             int64     `json:"id"`
	PaymentOrderID int64     `json:"payment_order_id"`
	SubMchID       string    `json:"sub_mch_id"`
	TransactionID  *string   `json:"transaction_id,omitempty"`
	OutSubsidyNo   string    `json:"out_subsidy_no"`
	PayerAmount    int64     `json:"payer_amount"`
	Amount         int64     `json:"amount"`
	Description    string    `json:"description"`
	Status         string    `json:"status"`
	WxpaySubsidyID *string   `json:"wxpay_subsidy_id,omitempty"`
	FailReason     *string   `json:"fail_reason,omitempty"`
	OutReturnNo    *string   `json:"out_return_no,omitempty"`
	ReturnAmount   *int64    `json:"return_amount,omitempty"`
	ReturnStatus   *string   `json:"return_status,omitempty"`
	ReturnWxpayID  *string   `json:"return_wxpay_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func toSubsidyOrderResponse(s db.SubsidyOrder) subsidyOrderResponse {
	resp := subsidyOrderResponse{
		ID:             s.ID,
		PaymentOrderID: s.PaymentOrderID,
		SubMchID:       s.SubMchID,
		OutSubsidyNo:   s.OutSubsidyNo,
		PayerAmount:    s.PayerAmount,
		Amount:         s.Amount,
		Description:    s.Description,
		Status:         s.Status,
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt.Time,
	}
	if s.TransactionID.Valid {
		resp.TransactionID = &s.TransactionID.String
	}
	if s.WxpaySubsidyID.Valid {
		resp.WxpaySubsidyID = &s.WxpaySubsidyID.String
	}
	if s.FailReason.Valid {
		resp.FailReason = &s.FailReason.String
	}
	if s.OutReturnNo.Valid {
		resp.OutReturnNo = &s.OutReturnNo.String
	}
	if s.ReturnAmount.Valid {
		amt := s.ReturnAmount.Int64
		resp.ReturnAmount = &amt
	}
	if s.ReturnStatus.Valid {
		resp.ReturnStatus = &s.ReturnStatus.String
	}
	if s.ReturnWxpayID.Valid {
		resp.ReturnWxpayID = &s.ReturnWxpayID.String
	}
	return resp
}

// ========================= 创建补差 ==========================================

type createSubsidyRequest struct {
	MerchantID  int64  `json:"merchant_id" binding:"required,min=1"`
	PayerAmount int64  `json:"payer_amount" binding:"required,min=1"`
	Amount      int64  `json:"amount" binding:"required,min=1"`
	Description string `json:"description" binding:"required,min=1,max=80"`
}

// createSubsidy 为收付通支付订单发起补差（平台向二级商户补贴，不影响用户侧）
// POST /v1/operator/payment-orders/:id/subsidies
//
// 幂等保证：out_subsidy_no = "S-{payment_order_id}-{merchant_id}"，同一订单对同一
// 商户只能发起一次补差；失败后仅允许用相同参数和原补差单号重入。
func (server *Server) createSubsidy(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		err := errors.New("payment service not configured")
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "payment service not configured", "create subsidy payment service not configured"))
		return
	}

	paymentOrderID, err := parseIDParam(ctx, "id")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req createSubsidyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 查询支付订单（校验存在且已支付）
	paymentOrder, err := server.store.GetPaymentOrder(ctx, paymentOrderID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("payment order not found")))
		} else {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		}
		return
	}
	if paymentOrder.Status != PaymentStatusPaid {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("payment order is not in paid status")))
		return
	}
	if !paymentOrder.TransactionID.Valid {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("payment order has no transaction_id yet")))
		return
	}

	// 查询商户支付配置获取 sub_mch_id
	merchantPayConfig, err := server.store.GetMerchantPaymentConfig(ctx, req.MerchantID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("merchant has no payment config (not onboarded to WeChat)")))
		} else {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		}
		return
	}

	// 幂等：同一支付订单 + 同一商户只允许一笔补差
	outSubsidyNo := fmt.Sprintf("S-%d-%d", paymentOrderID, req.MerchantID)

	existingSubsidy, err := server.store.GetSubsidyOrderByOutSubsidyNo(ctx, outSubsidyNo)
	if err != nil && !isNotFoundError(err) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if err == nil {
		// 已存在：若已成功或处理中，幂等返回
		if existingSubsidy.Status == "success" || existingSubsidy.Status == "pending" {
			ctx.JSON(http.StatusOK, toSubsidyOrderResponse(existingSubsidy))
			return
		}
		if existingSubsidy.Status == "canceled" {
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("subsidy already canceled for this payment order")))
			return
		}
		if existingSubsidy.Status != "failed" {
			ctx.JSON(http.StatusConflict, errorResponse(fmt.Errorf("cannot create subsidy from status %q", existingSubsidy.Status)))
			return
		}
		if err := validateSubsidyRetryMatch(existingSubsidy, req, merchantPayConfig.SubMchID, paymentOrder.TransactionID.String); err != nil {
			ctx.JSON(http.StatusConflict, errorResponse(err))
			return
		}
	}

	subsidyOrder := existingSubsidy
	if isNotFoundError(err) {
		// 在 DB 中先写入 pending 状态（防止重复调用微信 API）
		subsidyOrder, err = server.store.CreateSubsidyOrder(ctx, db.CreateSubsidyOrderParams{
			PaymentOrderID: paymentOrderID,
			SubMchID:       merchantPayConfig.SubMchID,
			TransactionID:  paymentOrder.TransactionID,
			OutSubsidyNo:   outSubsidyNo,
			PayerAmount:    req.PayerAmount,
			Amount:         req.Amount,
			Description:    req.Description,
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}

	// 调用微信补差 API
	wxResp, wxErr := server.ecommerceClient.CreateSubsidy(ctx, wechatcontracts.SubsidyRequest{
		SubMchID:      merchantPayConfig.SubMchID,
		TransactionID: paymentOrder.TransactionID.String,
		Amount:        req.Amount,
		Description:   req.Description,
		OutSubsidyNo:  outSubsidyNo,
	})

	if wxErr != nil {
		// 标记失败，让运营商知晓
		if _, err := server.store.UpdateSubsidyOrderToFailed(ctx, db.UpdateSubsidyOrderToFailedParams{
			ID:         subsidyOrder.ID,
			FailReason: pgtype.Text{String: wxErr.Error(), Valid: true},
		}); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("mark subsidy order failed after wxpay error: %w", err)))
			return
		}
		ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, wxErr, "subsidy api unavailable", "create subsidy: wxpay api failed"))
		return
	}

	result := ""
	if wxResp != nil {
		result = wxResp.Result
	}
	if wxResp == nil || result != wechatcontracts.SubsidyResultSuccess {
		failReason := "subsidy result is not SUCCESS"
		if result != "" {
			failReason = fmt.Sprintf("subsidy result is %s", result)
		}
		log.Error().Str("out_subsidy_no", outSubsidyNo).Str("result", result).Msg("create subsidy: wxpay returned non-success result")
		if _, err := server.store.UpdateSubsidyOrderToFailed(ctx, db.UpdateSubsidyOrderToFailedParams{
			ID:         subsidyOrder.ID,
			FailReason: pgtype.Text{String: failReason, Valid: true},
		}); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("mark subsidy order failed after non-success wxpay result: %w", err)))
			return
		}
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("subsidy request did not succeed")))
		return
	}

	// 标记成功
	wxpaySubsidyID := pgtype.Text{}
	if wxResp.SubsidyID != "" {
		wxpaySubsidyID = pgtype.Text{String: wxResp.SubsidyID, Valid: true}
	}
	updated, err := server.store.UpdateSubsidyOrderToSuccess(ctx, db.UpdateSubsidyOrderToSuccessParams{
		ID:             subsidyOrder.ID,
		WxpaySubsidyID: wxpaySubsidyID,
		TransactionID:  paymentOrder.TransactionID,
	})
	if err != nil {
		log.Error().Err(err).Int64("subsidy_order_id", subsidyOrder.ID).Msg("create subsidy: db update to success failed")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusCreated, toSubsidyOrderResponse(updated))
}

// ========================= 退回补差 ==========================================

type returnSubsidyRequest struct {
	RefundID    string `json:"refund_id"`
	Amount      int64  `json:"amount" binding:"required,min=1"`
	Description string `json:"description" binding:"required,min=1,max=80"`
}

// returnSubsidy 退回补差（退款时将平台已补贴的款项回收）
// POST /v1/operator/payment-orders/:id/subsidies/return
func (server *Server) returnSubsidy(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		err := errors.New("payment service not configured")
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "payment service not configured", "return subsidy payment service not configured"))
		return
	}

	paymentOrderID, err := parseIDParam(ctx, "id")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	var req returnSubsidyRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 查询已成功的补差订单
	subsidyOrder, err := server.store.GetSubsidyOrderByPaymentOrderID(ctx, paymentOrderID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("no successful subsidy found for this payment order")))
		} else {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		}
		return
	}
	if subsidyOrder.Status != "success" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("subsidy is not in success status, cannot return")))
		return
	}

	// 校验退回金额不超过原补差金额
	if req.Amount > subsidyOrder.Amount {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("return amount exceeds original subsidy amount")))
		return
	}

	// 幂等：out_return_no = "SR-{payment_order_id}"
	outReturnNo := fmt.Sprintf("SR-%d", paymentOrderID)

	shouldInitiateReturn := true
	if subsidyOrder.OutReturnNo.Valid {
		if subsidyOrder.OutReturnNo.String != outReturnNo {
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("unexpected subsidy return order number")))
			return
		}
		if subsidyOrder.ReturnAmount.Valid && subsidyOrder.ReturnAmount.Int64 != req.Amount {
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("subsidy return retry must use the original return amount")))
			return
		}
		if subsidyOrder.ReturnStatus.Valid {
			switch subsidyOrder.ReturnStatus.String {
			case "pending_return", "return_success":
				ctx.JSON(http.StatusOK, toSubsidyOrderResponse(subsidyOrder))
				return
			case "return_failed":
				shouldInitiateReturn = false
			default:
				ctx.JSON(http.StatusConflict, errorResponse(fmt.Errorf("cannot retry subsidy return from status %q", subsidyOrder.ReturnStatus.String)))
				return
			}
		}
	}

	if shouldInitiateReturn {
		_, err = server.store.InitiateSubsidyReturn(ctx, db.InitiateSubsidyReturnParams{
			ID:           subsidyOrder.ID,
			OutReturnNo:  pgtype.Text{String: outReturnNo, Valid: true},
			ReturnAmount: pgtype.Int8{Int64: req.Amount, Valid: true},
		})
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
	}

	// 调用微信退回补差 API
	subsidyID := ""
	if subsidyOrder.WxpaySubsidyID.Valid {
		subsidyID = subsidyOrder.WxpaySubsidyID.String
	}

	wxResp, wxErr := server.ecommerceClient.ReturnSubsidy(ctx, wechatcontracts.SubsidyReturnRequest{
		SubMchID:      subsidyOrder.SubMchID,
		OutOrderNo:    outReturnNo,
		TransactionID: subsidyOrder.TransactionID.String,
		RefundID:      req.RefundID,
		Amount:        req.Amount,
		Description:   req.Description,
		SubsidyID:     subsidyID,
	})

	if wxErr != nil {
		if _, err := server.store.UpdateSubsidyReturnToFailed(ctx, db.UpdateSubsidyReturnToFailedParams{
			OutReturnNo:      pgtype.Text{String: outReturnNo, Valid: true},
			ReturnFailReason: pgtype.Text{String: wxErr.Error(), Valid: true},
		}); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("mark subsidy return failed after wxpay error: %w", err)))
			return
		}
		ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, wxErr, "subsidy return api unavailable", "return subsidy: wxpay api failed"))
		return
	}

	result := ""
	if wxResp != nil {
		result = wxResp.Result
	}
	if wxResp == nil || result != wechatcontracts.SubsidyResultSuccess {
		failReason := "subsidy return result is not SUCCESS"
		if result != "" {
			failReason = fmt.Sprintf("subsidy return result is %s", result)
		}
		log.Error().Str("out_return_no", outReturnNo).Str("result", result).Msg("return subsidy: wxpay returned non-success result")
		if _, err := server.store.UpdateSubsidyReturnToFailed(ctx, db.UpdateSubsidyReturnToFailedParams{
			OutReturnNo:      pgtype.Text{String: outReturnNo, Valid: true},
			ReturnFailReason: pgtype.Text{String: failReason, Valid: true},
		}); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("mark subsidy return failed after non-success wxpay result: %w", err)))
			return
		}
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("subsidy return did not succeed")))
		return
	}

	// 标记退回成功
	returnWxpayID := pgtype.Text{}
	if wxResp.SubsidyRefundID != "" {
		returnWxpayID = pgtype.Text{String: wxResp.SubsidyRefundID, Valid: true}
	}
	updated, err := server.store.UpdateSubsidyReturnToSuccess(ctx, db.UpdateSubsidyReturnToSuccessParams{
		OutReturnNo:   pgtype.Text{String: outReturnNo, Valid: true},
		ReturnWxpayID: returnWxpayID,
	})
	if err != nil {
		log.Error().Err(err).Str("out_return_no", outReturnNo).Msg("return subsidy: db update to success failed")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, toSubsidyOrderResponse(updated))
}

// ========================= 取消补差 ==========================================

// cancelSubsidy 取消补差（仅在分账前可调用，取消后补差款不再支付）
// POST /v1/operator/payment-orders/:id/subsidies/cancel
func (server *Server) cancelSubsidy(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		err := errors.New("payment service not configured")
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "payment service not configured", "cancel subsidy payment service not configured"))
		return
	}

	paymentOrderID, err := parseIDParam(ctx, "id")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 查询补差订单
	subsidyOrder, err := server.store.GetSubsidyOrderByPaymentOrderID(ctx, paymentOrderID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("no subsidy found for this payment order")))
		} else {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		}
		return
	}

	// 幂等：已取消则直接返回
	if subsidyOrder.Status == "canceled" {
		ctx.JSON(http.StatusOK, toSubsidyOrderResponse(subsidyOrder))
		return
	}

	// 只有 pending 或 success 状态的补差可以取消
	if subsidyOrder.Status != "pending" && subsidyOrder.Status != "success" {
		ctx.JSON(http.StatusBadRequest, errorResponse(
			fmt.Errorf("cannot cancel subsidy in status %q", subsidyOrder.Status)))
		return
	}

	// 调用微信取消补差 API
	wxResp, wxErr := server.ecommerceClient.CancelSubsidy(ctx, wechatcontracts.SubsidyCancelRequest{
		SubMchID:      subsidyOrder.SubMchID,
		TransactionID: subsidyOrder.TransactionID.String,
		Description:   "operator cancel",
	})
	if wxErr != nil {
		ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, wxErr, "cancel subsidy api unavailable", "cancel subsidy: wxpay api failed"))
		return
	}
	result := ""
	if wxResp != nil {
		result = wxResp.Result
	}
	if wxResp == nil || result != wechatcontracts.SubsidyResultSuccess {
		log.Error().Str("out_subsidy_no", subsidyOrder.OutSubsidyNo).Str("result", result).Msg("cancel subsidy: wxpay returned non-success result")
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("cancel subsidy did not succeed")))
		return
	}

	// 本地标记取消
	updated, err := server.store.UpdateSubsidyOrderToCanceled(ctx, subsidyOrder.ID)
	if err != nil {
		log.Error().Err(err).Int64("subsidy_order_id", subsidyOrder.ID).Msg("cancel subsidy: db update failed")
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, toSubsidyOrderResponse(updated))
}

func validateSubsidyRetryMatch(existing db.SubsidyOrder, req createSubsidyRequest, subMchID, transactionID string) error {
	if existing.SubMchID != subMchID {
		return errors.New("existing subsidy order belongs to a different sub merchant")
	}
	if existing.Amount != req.Amount || existing.PayerAmount != req.PayerAmount || existing.Description != req.Description {
		return errors.New("subsidy retry must use the original amount, payer amount, and description")
	}
	if existing.TransactionID.Valid && existing.TransactionID.String != transactionID {
		return errors.New("existing subsidy order belongs to a different transaction")
	}
	return nil
}
