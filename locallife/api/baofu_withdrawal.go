package api

import (
	"errors"
	"fmt"
	"hash/fnv"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
)

const (
	baofuWithdrawalMinAmountFen            = int64(100)
	baofuWithdrawalMaxAmountFen            = int64(500000000)
	baofuWithdrawalIdempotencyHeader       = "Idempotency-Key"
	baofuWithdrawalMaxIdempotencyKeyLength = 256
	baofuWithdrawalReturnedSyncMessage     = "资金已退回至宝付结算账户，请刷新可提现余额后按需重新申请"
)

type baofuWithdrawalOwnerScope struct {
	OwnerType string
	OwnerID   int64
	Role      string
	CanCreate bool
}

type baofuWithdrawalBalanceResponse struct {
	AccountStatus     string `json:"account_status"`
	StatusDesc        string `json:"status_desc"`
	AvailableAmount   int64  `json:"available_amount"`
	PendingAmount     int64  `json:"pending_amount"`
	LedgerAmount      int64  `json:"ledger_amount"`
	FrozenAmount      int64  `json:"frozen_amount"`
	MinWithdrawAmount int64  `json:"min_withdraw_amount"`
	MaxWithdrawAmount int64  `json:"max_withdraw_amount"`
	CanWithdraw       bool   `json:"can_withdraw"`
	DisabledReason    string `json:"disabled_reason"`
}

type createBaofuWithdrawalRequest struct {
	Amount int64  `json:"amount" binding:"required,min=100"`
	Remark string `json:"remark" binding:"omitempty,max=128"`
}

type listBaofuWithdrawalsRequest struct {
	Page  int32 `form:"page" binding:"omitempty,min=1"`
	Limit int32 `form:"limit" binding:"omitempty,min=1,max=100"`
}

type getBaofuWithdrawalURIRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

type baofuWithdrawalItem struct {
	ID           int64  `json:"id"`
	OutRequestNo string `json:"out_request_no"`
	Amount       int64  `json:"amount"`
	Status       string `json:"status"`
	StatusText   string `json:"status_text"`
	SyncState    string `json:"sync_state,omitempty"`
	SyncMessage  string `json:"sync_message,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type baofuWithdrawalsResponse struct {
	Withdrawals []baofuWithdrawalItem `json:"withdrawals"`
	Total       int64                 `json:"total"`
	Page        int32                 `json:"page"`
	Limit       int32                 `json:"limit"`
	TotalPages  int64                 `json:"total_pages"`
}

type baofuWithdrawalCreateResponse struct {
	Withdrawal baofuWithdrawalItem `json:"withdrawal"`
	Message    string              `json:"message,omitempty"`
}

func (server *Server) getMerchantBaofuWithdrawalBalance(ctx *gin.Context) {
	scope, ok := server.merchantBaofuWithdrawalScope(ctx, true)
	if !ok {
		return
	}
	server.handleGetBaofuWithdrawalBalance(ctx, scope)
}

func (server *Server) listMerchantBaofuWithdrawals(ctx *gin.Context) {
	scope, ok := server.merchantBaofuWithdrawalScope(ctx, true)
	if !ok {
		return
	}
	server.handleListBaofuWithdrawals(ctx, scope)
}

func (server *Server) getMerchantBaofuWithdrawal(ctx *gin.Context) {
	scope, ok := server.merchantBaofuWithdrawalScope(ctx, true)
	if !ok {
		return
	}
	server.handleGetBaofuWithdrawal(ctx, scope)
}

func (server *Server) createMerchantBaofuWithdrawal(ctx *gin.Context) {
	scope, ok := server.merchantBaofuWithdrawalScope(ctx, true)
	if !ok {
		return
	}
	server.handleCreateBaofuWithdrawal(ctx, scope)
}

func (server *Server) getPlatformBaofuWithdrawalBalance(ctx *gin.Context) {
	server.handleGetBaofuWithdrawalBalance(ctx, platformBaofuWithdrawalScope(true))
}

func (server *Server) listPlatformBaofuWithdrawals(ctx *gin.Context) {
	server.handleListBaofuWithdrawals(ctx, platformBaofuWithdrawalScope(true))
}

func (server *Server) getPlatformBaofuWithdrawal(ctx *gin.Context) {
	server.handleGetBaofuWithdrawal(ctx, platformBaofuWithdrawalScope(true))
}

func (server *Server) createPlatformBaofuWithdrawal(ctx *gin.Context) {
	server.handleCreateBaofuWithdrawal(ctx, platformBaofuWithdrawalScope(true))
}

func (server *Server) getOperatorBaofuWithdrawalBalance(ctx *gin.Context) {
	scope, ok := operatorBaofuWithdrawalScope(ctx, true)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("operator not loaded, ensure LoadOperatorMiddleware is applied")))
		return
	}
	server.handleGetBaofuWithdrawalBalance(ctx, scope)
}

func (server *Server) listOperatorBaofuWithdrawals(ctx *gin.Context) {
	scope, ok := operatorBaofuWithdrawalScope(ctx, true)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("operator not loaded, ensure LoadOperatorMiddleware is applied")))
		return
	}
	server.handleListBaofuWithdrawals(ctx, scope)
}

func (server *Server) getOperatorBaofuWithdrawal(ctx *gin.Context) {
	scope, ok := operatorBaofuWithdrawalScope(ctx, true)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("operator not loaded, ensure LoadOperatorMiddleware is applied")))
		return
	}
	server.handleGetBaofuWithdrawal(ctx, scope)
}

func (server *Server) createOperatorBaofuWithdrawal(ctx *gin.Context) {
	scope, ok := operatorBaofuWithdrawalScope(ctx, true)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("operator not loaded, ensure LoadOperatorMiddleware is applied")))
		return
	}
	server.handleCreateBaofuWithdrawal(ctx, scope)
}

func (server *Server) getRiderBaofuIncomeWithdrawalBalance(ctx *gin.Context) {
	scope, ok := riderBaofuIncomeWithdrawalScope(ctx, true)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("rider not loaded, ensure RiderMiddleware is applied")))
		return
	}
	server.handleGetBaofuWithdrawalBalance(ctx, scope)
}

func (server *Server) listRiderBaofuIncomeWithdrawals(ctx *gin.Context) {
	scope, ok := riderBaofuIncomeWithdrawalScope(ctx, true)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("rider not loaded, ensure RiderMiddleware is applied")))
		return
	}
	server.handleListBaofuWithdrawals(ctx, scope)
}

func (server *Server) getRiderBaofuIncomeWithdrawal(ctx *gin.Context) {
	scope, ok := riderBaofuIncomeWithdrawalScope(ctx, true)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("rider not loaded, ensure RiderMiddleware is applied")))
		return
	}
	server.handleGetBaofuWithdrawal(ctx, scope)
}

func (server *Server) createRiderBaofuIncomeWithdrawal(ctx *gin.Context) {
	scope, ok := riderBaofuIncomeWithdrawalScope(ctx, true)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("rider not loaded, ensure RiderMiddleware is applied")))
		return
	}
	server.handleCreateBaofuWithdrawal(ctx, scope)
}

func (server *Server) merchantBaofuWithdrawalScope(ctx *gin.Context, canCreate bool) (baofuWithdrawalOwnerScope, bool) {
	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, errors.New("merchant not loaded, ensure MerchantStaffMiddleware is applied")))
		return baofuWithdrawalOwnerScope{}, false
	}
	return baofuWithdrawalOwnerScope{
		OwnerType: db.BaofuAccountOwnerTypeMerchant,
		OwnerID:   merchant.ID,
		Role:      "merchant",
		CanCreate: canCreate,
	}, true
}

func platformBaofuWithdrawalScope(canCreate bool) baofuWithdrawalOwnerScope {
	return baofuWithdrawalOwnerScope{
		OwnerType: db.BaofuAccountOwnerTypePlatform,
		OwnerID:   platformBaofuAccountOwnerID,
		Role:      "platform",
		CanCreate: canCreate,
	}
}

func operatorBaofuWithdrawalScope(ctx *gin.Context, canCreate bool) (baofuWithdrawalOwnerScope, bool) {
	operator, ok := GetOperatorFromContext(ctx)
	if !ok {
		return baofuWithdrawalOwnerScope{}, false
	}
	return baofuWithdrawalOwnerScope{
		OwnerType: db.BaofuAccountOwnerTypeOperator,
		OwnerID:   operator.ID,
		Role:      "operator",
		CanCreate: canCreate,
	}, true
}

func riderBaofuIncomeWithdrawalScope(ctx *gin.Context, canCreate bool) (baofuWithdrawalOwnerScope, bool) {
	rider, ok := GetRiderFromContext(ctx)
	if !ok {
		return baofuWithdrawalOwnerScope{}, false
	}
	return baofuWithdrawalOwnerScope{
		OwnerType: db.BaofuAccountOwnerTypeRider,
		OwnerID:   rider.ID,
		Role:      "rider",
		CanCreate: canCreate,
	}, true
}

func (server *Server) handleGetBaofuWithdrawalBalance(ctx *gin.Context, scope baofuWithdrawalOwnerScope) {
	if !server.ensureBaofuWithdrawService(ctx) {
		return
	}
	result, err := server.baofuWithdrawService.QueryBalance(ctx.Request.Context(), logic.BaofuBalanceQueryInput{
		OwnerType: scope.OwnerType,
		OwnerID:   scope.OwnerID,
	})
	if err != nil {
		server.respondBaofuWithdrawalError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, baofuWithdrawalBalanceResponse{
		AccountStatus:     "active",
		StatusDesc:        "结算账户已开通",
		AvailableAmount:   result.AvailableAmountFen,
		PendingAmount:     result.PendingAmountFen,
		LedgerAmount:      result.LedgerAmountFen,
		FrozenAmount:      result.FrozenAmountFen,
		MinWithdrawAmount: baofuWithdrawalMinAmountFen,
		MaxWithdrawAmount: baofuWithdrawalMaxAmountFen,
		CanWithdraw:       result.AvailableAmountFen >= baofuWithdrawalMinAmountFen,
		DisabledReason:    baofuWithdrawalDisabledReason(result.AvailableAmountFen),
	})
}

func (server *Server) handleCreateBaofuWithdrawal(ctx *gin.Context, scope baofuWithdrawalOwnerScope) {
	if !server.ensureBaofuWithdrawService(ctx) {
		return
	}
	if !scope.CanCreate {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("insufficient permissions for this operation")))
		return
	}
	var req createBaofuWithdrawalRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if req.Amount > baofuWithdrawalMaxAmountFen {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("提现金额超过单笔上限")))
		return
	}
	idempotencyKey := strings.TrimSpace(ctx.GetHeader(baofuWithdrawalIdempotencyHeader))
	if idempotencyKey == "" {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("Idempotency-Key header is required")))
		return
	}
	if len(idempotencyKey) > baofuWithdrawalMaxIdempotencyKeyLength {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("Idempotency-Key header is too long")))
		return
	}
	result, err := server.baofuWithdrawService.CreateWithdrawal(ctx.Request.Context(), logic.BaofuCreateWithdrawalInput{
		OwnerType:      scope.OwnerType,
		OwnerID:        scope.OwnerID,
		AmountFen:      req.Amount,
		OutRequestNo:   newBaofuWithdrawalOutRequestNo(ctx, scope),
		IdempotencyKey: idempotencyKey,
	})
	if err != nil {
		if server.respondBaofuWithdrawalCreateResultError(ctx, result, err) {
			return
		}
		server.respondBaofuWithdrawalCreateError(ctx, err)
		return
	}
	status := http.StatusCreated
	if result.IdempotencyReplayed {
		status = http.StatusOK
	}
	ctx.JSON(status, baofuWithdrawalCreateResponse{
		Withdrawal: newBaofuWithdrawalItem(result.WithdrawalOrder),
	})
}

func (server *Server) handleListBaofuWithdrawals(ctx *gin.Context, scope baofuWithdrawalOwnerScope) {
	if !server.ensureBaofuWithdrawService(ctx) {
		return
	}
	var req listBaofuWithdrawalsRequest
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
	total, err := server.store.CountBaofuWithdrawalOrdersByOwner(ctx.Request.Context(), db.CountBaofuWithdrawalOrdersByOwnerParams{
		OwnerType: scope.OwnerType,
		OwnerID:   scope.OwnerID,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	rows, err := server.store.ListBaofuWithdrawalOrdersByOwner(ctx.Request.Context(), db.ListBaofuWithdrawalOrdersByOwnerParams{
		OwnerType:   scope.OwnerType,
		OwnerID:     scope.OwnerID,
		OffsetCount: pageOffset(req.Page, req.Limit),
		LimitCount:  req.Limit,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	items := make([]baofuWithdrawalItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, newBaofuWithdrawalItem(row))
	}
	ctx.JSON(http.StatusOK, baofuWithdrawalsResponse{
		Withdrawals: items,
		Total:       total,
		Page:        req.Page,
		Limit:       req.Limit,
		TotalPages:  baofuWithdrawalTotalPages(total, req.Limit),
	})
}

func (server *Server) handleGetBaofuWithdrawal(ctx *gin.Context, scope baofuWithdrawalOwnerScope) {
	if !server.ensureBaofuWithdrawService(ctx) {
		return
	}
	var req getBaofuWithdrawalURIRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	order, err := server.store.GetBaofuWithdrawalOrder(ctx.Request.Context(), req.ID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("withdrawal not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if order.OwnerType != scope.OwnerType || order.OwnerID != scope.OwnerID {
		ctx.JSON(http.StatusNotFound, errorResponse(errors.New("withdrawal not found")))
		return
	}
	ctx.JSON(http.StatusOK, baofuWithdrawalCreateResponse{
		Withdrawal: newBaofuWithdrawalItem(order),
	})
}

func (server *Server) ensureBaofuWithdrawService(ctx *gin.Context) bool {
	if server.baofuWithdrawService != nil {
		return true
	}
	ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, logic.ErrBaofuWithdrawServiceNotConfigured, "提现服务暂不可用，请稍后再试", "baofu withdrawal service not configured"))
	return false
}

func (server *Server) respondBaofuWithdrawalError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, logic.ErrBaofuWithdrawServiceNotConfigured):
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "提现服务暂不可用，请稍后再试", "baofu withdrawal service not configured"))
	case errors.Is(err, logic.ErrBaofuWithdrawAccountNotReady):
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("结算账户未开通，暂不能提现")))
	case errors.Is(err, logic.ErrBaofuWithdrawContractNoRequired):
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("结算账户状态异常，请联系平台处理")))
	case errors.Is(err, logic.ErrBaofuWithdrawFeeMemberIDRequired):
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("结算账户状态异常，请联系平台处理")))
	default:
		ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, err, "提现账户余额暂不可确认，请稍后刷新", "baofu withdrawal provider request failed"))
	}
}

func (server *Server) respondBaofuWithdrawalCreateError(ctx *gin.Context, err error) {
	if writeLogicRequestError(ctx, err) {
		return
	}
	switch {
	case errors.Is(err, logic.ErrBaofuWithdrawServiceNotConfigured):
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "提现服务暂不可用，请稍后再试", "baofu withdrawal service not configured"))
	case errors.Is(err, logic.ErrBaofuWithdrawAccountNotReady):
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("结算账户未开通，暂不能提现")))
	case errors.Is(err, logic.ErrBaofuWithdrawContractNoRequired):
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("结算账户状态异常，请联系平台处理")))
	case errors.Is(err, logic.ErrBaofuWithdrawFeeMemberIDRequired):
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("结算账户状态异常，请联系平台处理")))
	case errors.Is(err, logic.ErrBaofuWithdrawInsufficientBalance):
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("可提现金额不足")))
	case errors.Is(err, logic.ErrBaofuWithdrawBalanceUnavailable):
		ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, err, "提现账户余额暂不可确认，请稍后刷新", "baofu withdrawal balance check failed before create"))
	default:
		ctx.JSON(http.StatusBadGateway, loggedServerError(ctx, err, "提现申请暂不可提交，请稍后重试", "baofu withdrawal create failed"))
	}
}

func baofuWithdrawalDisabledReason(availableAmount int64) string {
	if availableAmount >= baofuWithdrawalMinAmountFen {
		return ""
	}
	return "可提现金额不足"
}

func newBaofuWithdrawalItem(order db.BaofuWithdrawalOrder) baofuWithdrawalItem {
	return baofuWithdrawalItem{
		ID:           order.ID,
		OutRequestNo: order.OutRequestNo,
		Amount:       order.Amount,
		Status:       order.Status,
		StatusText:   baofuWithdrawalStatusText(order.Status),
		SyncMessage:  baofuWithdrawalSyncMessage(order.Status),
		CreatedAt:    order.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    order.UpdatedAt.Format(time.RFC3339),
	}
}

func baofuWithdrawalStatusText(status string) string {
	switch strings.TrimSpace(status) {
	case db.BaofuWithdrawalStatusProcessing:
		return "提现处理中"
	case db.BaofuWithdrawalStatusSucceeded:
		return "提现成功"
	case db.BaofuWithdrawalStatusFailed:
		return "提现失败"
	case db.BaofuWithdrawalStatusReturned:
		return "提现已退回"
	default:
		return "提现状态确认中"
	}
}

func baofuWithdrawalSyncMessage(status string) string {
	switch strings.TrimSpace(status) {
	case db.BaofuWithdrawalStatusReturned:
		return baofuWithdrawalReturnedSyncMessage
	default:
		return ""
	}
}

func baofuWithdrawalTotalPages(total int64, limit int32) int64 {
	if total == 0 || limit <= 0 {
		return 0
	}
	return (total + int64(limit) - 1) / int64(limit)
}

func newBaofuWithdrawalOutRequestNo(ctx *gin.Context, scope baofuWithdrawalOwnerScope) string {
	prefix := baofuWithdrawalOutRequestNoPrefix(scope.OwnerType)
	seed := fmt.Sprintf("%s:%s:%d:%d", GetRequestID(ctx), scope.OwnerType, scope.OwnerID, time.Now().UTC().UnixNano())
	return fmt.Sprintf("%s%d%010d", prefix, time.Now().UTC().UnixNano(), hashBaofuWithdrawalSeed(seed)%10000000000)
}

func baofuWithdrawalOutRequestNoPrefix(ownerType string) string {
	switch ownerType {
	case db.BaofuAccountOwnerTypeMerchant:
		return "MBW"
	case db.BaofuAccountOwnerTypePlatform:
		return "PBW"
	case db.BaofuAccountOwnerTypeOperator:
		return "OBW"
	case db.BaofuAccountOwnerTypeRider:
		return "RBW"
	default:
		return "BWF"
	}
}

func hashBaofuWithdrawalSeed(seed string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(seed))
	return h.Sum64()
}
