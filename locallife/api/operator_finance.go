package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/wechat"
)

const (
	operatorWithdrawMinAmount = int64(100) // 1元
	operatorWithdrawMaxAmount = int64(500000000)
	operatorWithdrawChannel   = "wechat_ecommerce_fund_operator"
)

type operatorAccountBalanceResponse struct {
	SubMchID           string `json:"sub_mch_id,omitempty"`
	AvailableAmount    int64  `json:"available_amount,omitempty"`
	PendingAmount      int64  `json:"pending_amount,omitempty"`
	WithdrawableAmount int64  `json:"withdrawable_amount,omitempty"`
	AccountStatus      string `json:"account_status,omitempty"`
	StatusDesc         string `json:"status_desc,omitempty"`
}

type operatorWithdrawalsResponse struct {
	Withdrawals   []operatorWithdrawItem `json:"withdrawals"`
	Total         int64                  `json:"total"`
	Page          int32                  `json:"page"`
	Limit         int32                  `json:"limit"`
	TotalPages    int64                  `json:"total_pages"`
	AccountStatus string                 `json:"account_status,omitempty"`
	StatusDesc    string                 `json:"status_desc,omitempty"`
}

type withdrawOperatorRequest struct {
	Amount       int64  `json:"amount" binding:"required,min=100"`
	Remark       string `json:"remark" binding:"omitempty,max=128"`
	OutRequestNo string `json:"out_request_no" binding:"omitempty,max=64"`
}

type operatorWithdrawItem struct {
	ID           int64  `json:"id"`
	Amount       int64  `json:"amount"`
	Status       string `json:"status"`
	Channel      string `json:"channel"`
	OutRequestNo string `json:"out_request_no,omitempty"`
	WithdrawID   string `json:"withdraw_id,omitempty"`
	SubMchID     string `json:"sub_mch_id,omitempty"`
	Reason       string `json:"reason,omitempty"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type operatorWithdrawCreateResponse struct {
	Withdrawal operatorWithdrawItem `json:"withdrawal"`
	Wechat     interface{}          `json:"wechat"`
}

type getOperatorWithdrawalResponse struct {
	Withdrawal operatorWithdrawItem `json:"withdrawal"`
}

type listOperatorWithdrawalsRequest struct {
	Page  int32 `form:"page" binding:"omitempty,min=1"`
	Limit int32 `form:"limit" binding:"omitempty,min=1,max=100"`
}

type getOperatorWithdrawalRequest struct {
	ID int64 `uri:"id" binding:"required,min=1"`
}

type operatorWithdrawAccountInfo struct {
	OperatorID   int64  `json:"operator_id"`
	SubMchID     string `json:"sub_mch_id"`
	OutRequestNo string `json:"out_request_no"`
	WithdrawID   string `json:"withdraw_id,omitempty"`
	Remark       string `json:"remark,omitempty"`
}

func parseOperatorWithdrawAccountInfo(raw []byte) operatorWithdrawAccountInfo {
	if len(raw) == 0 {
		return operatorWithdrawAccountInfo{}
	}

	var info operatorWithdrawAccountInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return operatorWithdrawAccountInfo{}
	}

	return info
}

func toOperatorWithdrawItem(record db.WithdrawalRecord) operatorWithdrawItem {
	info := parseOperatorWithdrawAccountInfo(record.AccountInfo)
	reason := ""
	if record.Reason.Valid {
		reason = record.Reason.String
	}

	return operatorWithdrawItem{
		ID:           record.ID,
		Amount:       record.Amount,
		Status:       record.Status,
		Channel:      record.Channel,
		OutRequestNo: info.OutRequestNo,
		WithdrawID:   info.WithdrawID,
		SubMchID:     info.SubMchID,
		Reason:       reason,
		CreatedAt:    record.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    record.UpdatedAt.Format(time.RFC3339),
	}
}

func normalizeOperatorWithdrawRemark(remark string) string {
	if remark == "" {
		return "运营商提现"
	}
	return remark
}

func (server *Server) getActiveOperatorForFinance(ctx *gin.Context, userID int64) (db.Operator, error) {
	operator, err := server.getOperatorFromUserID(ctx, userID)
	if err != nil {
		return db.Operator{}, err
	}

	if operator.Status != "active" {
		return db.Operator{}, errors.New("operator is not active")
	}

	if !operator.SubMchID.Valid || operator.SubMchID.String == "" {
		return db.Operator{}, errors.New("operator payment config is not active")
	}

	return operator, nil
}

// getOperatorFinanceAccountStatus 从运营商记录判断财务账户状态
// 返回 (accountStatus, statusDesc)，不占用 HTTP 层。
func getOperatorFinanceAccountStatus(operator db.Operator) (string, string) {
	if !operator.SubMchID.Valid || operator.SubMchID.String == "" {
		return "not_configured", "收付通账户尚未开通，请先完成进件流程"
	}
	return "active", ""
}

// getOperatorAccountBalance 查询运营商收付通账户余额
func (server *Server) getOperatorAccountBalance(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(errors.New("ecommerce client not configured")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	operator, err := server.getOperatorFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}
	if operator.Status != "active" {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("operator is not active")))
		return
	}
	if accountStatus, statusDesc := getOperatorFinanceAccountStatus(operator); accountStatus != "active" {
		ctx.JSON(http.StatusOK, operatorAccountBalanceResponse{
			AccountStatus: accountStatus,
			StatusDesc:    statusDesc,
		})
		return
	}

	balance, err := server.ecommerceClient.QueryEcommerceFundBalance(ctx, operator.SubMchID.String)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, internalError(ctx, fmt.Errorf("query ecommerce fund balance: %w", err)))
		return
	}

	ctx.JSON(http.StatusOK, operatorAccountBalanceResponse{
		SubMchID:           operator.SubMchID.String,
		AvailableAmount:    balance.AvailableAmount,
		PendingAmount:      balance.PendingAmount,
		WithdrawableAmount: balance.WithdrawableAmount,
		AccountStatus:      "active",
	})
}

// withdrawOperator 运营商提现
// @Summary 运营商提现
// @Description 运营商发起收付通提现到绑定银行卡
// @Tags 运营商财务
// @Accept json
// @Produce json
// @Param request body withdrawOperatorRequest true "提现请求"
// @Success 200 {object} map[string]interface{} "提现申请提交成功"
// @Failure 400 {object} ErrorResponse "参数错误或余额不足"
// @Failure 403 {object} ErrorResponse "无权限"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Security BearerAuth
// @Router /v1/operators/me/finance/withdraw [post]
func (server *Server) withdrawOperator(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(errors.New("ecommerce client not configured")))
		return
	}

	var req withdrawOperatorRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	if req.Amount < operatorWithdrawMinAmount || req.Amount > operatorWithdrawMaxAmount {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("withdraw amount out of range")))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	operator, err := server.getActiveOperatorForFinance(ctx, authPayload.UserID)
	if err != nil {
		switch err.Error() {
		case "operator is not active":
			ctx.JSON(http.StatusForbidden, errorResponse(err))
		case "operator payment config is not active":
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
		default:
			return
		}
		return
	}

	balance, err := server.ecommerceClient.QueryEcommerceFundBalance(ctx, operator.SubMchID.String)
	if err != nil {
		ctx.JSON(http.StatusBadGateway, internalError(ctx, fmt.Errorf("query ecommerce fund balance: %w", err)))
		return
	}
	if req.Amount > balance.WithdrawableAmount {
		ctx.JSON(http.StatusBadRequest, errorResponse(errors.New("insufficient withdrawable balance")))
		return
	}

	outRequestNo := req.OutRequestNo
	if outRequestNo == "" {
		outRequestNo = fmt.Sprintf("OW%d%d", operator.ID, time.Now().UnixNano()/1e6)
	}
	remark := normalizeOperatorWithdrawRemark(req.Remark)

	_, lookupErr := server.store.GetWithdrawalRecordByOutRequestNo(ctx, pgtype.Text{String: outRequestNo, Valid: true})
	if lookupErr == nil {
		ctx.JSON(http.StatusConflict, errorResponse(errors.New("duplicate out_request_no: withdrawal already exists")))
		return
	}
	if !isNotFoundError(lookupErr) {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, lookupErr))
		return
	}

	accountInfoBytes, _ := json.Marshal(operatorWithdrawAccountInfo{
		OperatorID:   operator.ID,
		SubMchID:     operator.SubMchID.String,
		OutRequestNo: outRequestNo,
		Remark:       remark,
	})

	record, err := server.store.CreateWithdrawalRecord(ctx, db.CreateWithdrawalRecordParams{
		UserID:       authPayload.UserID,
		Amount:       req.Amount,
		Status:       "pending",
		Channel:      operatorWithdrawChannel,
		AccountInfo:  accountInfoBytes,
		OutRequestNo: pgtype.Text{String: outRequestNo, Valid: true},
	})
	if err != nil {
		if db.ErrorCode(err) == db.UniqueViolation {
			ctx.JSON(http.StatusConflict, errorResponse(errors.New("duplicate out_request_no: withdrawal already exists")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	withdrawResp, err := server.ecommerceClient.CreateEcommerceWithdraw(ctx, &wechat.EcommerceWithdrawRequest{
		SubMchID:     operator.SubMchID.String,
		OutRequestNo: outRequestNo,
		Amount:       req.Amount,
		Remark:       remark,
	})
	if err != nil {
		queryResp, queryErr := server.ecommerceClient.QueryEcommerceWithdrawByOutRequestNo(ctx, operator.SubMchID.String, outRequestNo)
		if queryErr != nil {
			record = server.updateWithdrawalRecordStatus(ctx, record, "pending", fmt.Sprintf("withdraw request submitted, awaiting confirmation: %v", err))
			server.enqueueWithdrawalResultPolling(ctx, record)
			ctx.JSON(http.StatusAccepted, operatorWithdrawCreateResponse{Withdrawal: toOperatorWithdrawItem(record), Wechat: nil})
			return
		}
		withdrawResp = queryResp
	}

	accountInfoBytes, _ = json.Marshal(operatorWithdrawAccountInfo{
		OperatorID:   operator.ID,
		SubMchID:     operator.SubMchID.String,
		OutRequestNo: outRequestNo,
		WithdrawID:   withdrawResp.WithdrawID,
		Remark:       remark,
	})
	record = server.updateWithdrawalRecordAccountInfo(ctx, record, accountInfoBytes)

	status := mapWechatWithdrawStatus(withdrawResp.Status)
	record = server.updateWithdrawalRecordStatus(ctx, record, status, withdrawResp.FailReason)
	server.enqueueWithdrawalResultPolling(ctx, record)

	ctx.JSON(http.StatusOK, operatorWithdrawCreateResponse{Withdrawal: toOperatorWithdrawItem(record), Wechat: withdrawResp})
}

// listOperatorWithdrawals 查询运营商提现记录
func (server *Server) listOperatorWithdrawals(ctx *gin.Context) {
	var req listOperatorWithdrawalsRequest
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

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	operator, err := server.getOperatorFromUserID(ctx, authPayload.UserID)
	if err != nil {
		return
	}
	if operator.Status != "active" {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("operator is not active")))
		return
	}
	if accountStatus, statusDesc := getOperatorFinanceAccountStatus(operator); accountStatus != "active" {
		ctx.JSON(http.StatusOK, operatorWithdrawalsResponse{
			Withdrawals:   []operatorWithdrawItem{},
			AccountStatus: accountStatus,
			StatusDesc:    statusDesc,
		})
		return
	}

	offset := pageOffset(req.Page, req.Limit)
	rows, err := server.store.ListWithdrawalRecords(ctx, db.ListWithdrawalRecordsParams{
		UserID:  authPayload.UserID,
		Channel: operatorWithdrawChannel,
		Limit:   req.Limit,
		Offset:  offset,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	totalCount, err := server.store.CountWithdrawalRecords(ctx, db.CountWithdrawalRecordsParams{
		UserID:  authPayload.UserID,
		Channel: operatorWithdrawChannel,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	items := make([]operatorWithdrawItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, toOperatorWithdrawItem(row))
	}

	ctx.JSON(http.StatusOK, operatorWithdrawalsResponse{
		Withdrawals: items,
		Total:       totalCount,
		Page:        req.Page,
		Limit:       req.Limit,
		TotalPages:  (totalCount + int64(req.Limit) - 1) / int64(req.Limit),
	})
}

// getOperatorWithdrawal 查询单笔提现并同步微信状态
func (server *Server) getOperatorWithdrawal(ctx *gin.Context) {
	if server.ecommerceClient == nil {
		ctx.JSON(http.StatusServiceUnavailable, errorResponse(errors.New("ecommerce client not configured")))
		return
	}

	var req getOperatorWithdrawalRequest
	if err := ctx.ShouldBindUri(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	if _, err := server.getActiveOperatorForFinance(ctx, authPayload.UserID); err != nil {
		switch err.Error() {
		case "operator is not active":
			ctx.JSON(http.StatusForbidden, errorResponse(err))
		case "operator payment config is not active":
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
		default:
			return
		}
		return
	}

	record, err := server.store.GetWithdrawalRecord(ctx, req.ID)
	if err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(errors.New("withdrawal not found")))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	if record.UserID != authPayload.UserID || record.Channel != operatorWithdrawChannel {
		ctx.JSON(http.StatusForbidden, errorResponse(errors.New("no permission to access this withdrawal")))
		return
	}

	info := parseOperatorWithdrawAccountInfo(record.AccountInfo)
	if info.SubMchID != "" && info.OutRequestNo != "" {
		wxResp, queryErr := server.ecommerceClient.QueryEcommerceWithdrawByOutRequestNo(ctx, info.SubMchID, info.OutRequestNo)
		if queryErr == nil {
			newStatus := mapWechatWithdrawStatus(wxResp.Status)
			reasonText := ""
			if wxResp.FailReason != "" {
				reasonText = wxResp.FailReason
			}

			if newStatus != record.Status || reasonText != "" {
				updated, updateErr := server.store.UpdateWithdrawalStatus(ctx, db.UpdateWithdrawalStatusParams{
					ID:     record.ID,
					Status: newStatus,
					Reason: pgtype.Text{String: reasonText, Valid: reasonText != ""},
				})
				if updateErr == nil {
					record = updated
				}
			}

			if wxResp.WithdrawID != "" && wxResp.WithdrawID != info.WithdrawID {
				info.WithdrawID = wxResp.WithdrawID
				if raw, marshalErr := json.Marshal(info); marshalErr == nil {
					record = server.updateWithdrawalRecordAccountInfo(ctx, record, raw)
				}
			}
		}
	}

	ctx.JSON(http.StatusOK, getOperatorWithdrawalResponse{Withdrawal: toOperatorWithdrawItem(record)})
}
