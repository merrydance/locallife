package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
)

const (
	riderWithdrawAcceptedStatus        = "accepted"
	riderWithdrawSuccessStatus         = "success"
	riderWithdrawFailedStatus          = "failed"
	riderWithdrawPartialFailedStatus   = "partial_failed"
	riderWithdrawSyncingStatus         = "syncing"
	riderWithdrawalRefundOrderIDsLimit = 20
)

type riderWithdrawalStatusRefundItemResponse struct {
	RefundOrderID       int64      `json:"refund_order_id"`
	PaymentOrderID      int64      `json:"payment_order_id"`
	OutRefundNo         string     `json:"out_refund_no"`
	RefundID            *string    `json:"refund_id,omitempty"`
	Amount              int64      `json:"amount"`
	Status              string     `json:"status"`
	StatusText          string     `json:"status_text"`
	CreatedAt           time.Time  `json:"created_at"`
	RefundedAt          *time.Time `json:"refunded_at,omitempty"`
	SourcePaymentAmount int64      `json:"source_payment_amount"`
	OutTradeNo          string     `json:"out_trade_no"`
}

type riderWithdrawalStatusResponse struct {
	Status           string                                    `json:"status"`
	StatusText       string                                    `json:"status_text"`
	Message          string                                    `json:"message"`
	AcceptedAmount   int64                                     `json:"accepted_amount"`
	ProcessingAmount int64                                     `json:"processing_amount"`
	SuccessAmount    int64                                     `json:"success_amount"`
	FailedAmount     int64                                     `json:"failed_amount"`
	Refunds          []riderWithdrawalStatusRefundItemResponse `json:"refunds"`
}

func parseRiderWithdrawalRefundOrderIDs(rawValues []string) ([]int64, error) {
	if len(rawValues) == 0 {
		return nil, errors.New("请提供提现退款单ID")
	}

	ids := make([]int64, 0, len(rawValues))
	seen := make(map[int64]struct{})
	for _, rawValue := range rawValues {
		for _, part := range strings.Split(rawValue, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" {
				continue
			}
			id, err := strconv.ParseInt(trimmed, 10, 64)
			if err != nil || id <= 0 {
				return nil, errors.New("提现退款单ID格式不正确")
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}

	if len(ids) == 0 {
		return nil, errors.New("请提供提现退款单ID")
	}
	if len(ids) > riderWithdrawalRefundOrderIDsLimit {
		return nil, fmt.Errorf("一次最多查询%d笔提现退款单", riderWithdrawalRefundOrderIDsLimit)
	}
	return ids, nil
}

func newRiderWithdrawalStatusResponse(rows []db.ListRiderDepositWithdrawalRefundOrdersByIDsRow) riderWithdrawalStatusResponse {
	response := riderWithdrawalStatusResponse{
		Status:  riderWithdrawSyncingStatus,
		Refunds: make([]riderWithdrawalStatusRefundItemResponse, 0, len(rows)),
	}

	allPending := len(rows) > 0
	for _, row := range rows {
		itemStatus, itemStatusText := riderWithdrawalRefundStatusView(row.Status)
		if row.Status != "pending" {
			allPending = false
		}

		response.AcceptedAmount += row.RefundAmount
		item := riderWithdrawalStatusRefundItemResponse{
			RefundOrderID:       row.RefundOrderID,
			PaymentOrderID:      row.PaymentOrderID,
			OutRefundNo:         row.OutRefundNo,
			Amount:              row.RefundAmount,
			Status:              itemStatus,
			StatusText:          itemStatusText,
			CreatedAt:           row.CreatedAt,
			SourcePaymentAmount: row.SourcePaymentAmount,
			OutTradeNo:          row.OutTradeNo,
		}
		if row.RefundID.Valid {
			refundID := row.RefundID.String
			item.RefundID = &refundID
		}
		if row.RefundedAt.Valid {
			refundedAt := row.RefundedAt.Time
			item.RefundedAt = &refundedAt
		}

		switch itemStatus {
		case riderWithdrawSuccessStatus:
			response.SuccessAmount += row.RefundAmount
		case riderWithdrawFailedStatus:
			response.FailedAmount += row.RefundAmount
		default:
			response.ProcessingAmount += row.RefundAmount
		}

		response.Refunds = append(response.Refunds, item)
	}

	response.Status, response.StatusText, response.Message = riderWithdrawalStatusSummary(response.AcceptedAmount, response.ProcessingAmount, response.SuccessAmount, response.FailedAmount, allPending)
	return response
}

func riderWithdrawalRefundStatusView(status string) (string, string) {
	switch status {
	case "pending":
		return riderWithdrawAcceptedStatus, "已受理"
	case "processing":
		return riderWithdrawProcessingStatus, "处理中"
	case "success":
		return riderWithdrawSuccessStatus, "已到账"
	case "failed", "closed":
		return riderWithdrawFailedStatus, "已退回"
	default:
		return riderWithdrawSyncingStatus, "状态同步中"
	}
}

func riderWithdrawalStatusSummary(acceptedAmount, processingAmount, successAmount, failedAmount int64, allPending bool) (string, string, string) {
	switch {
	case acceptedAmount == 0:
		return riderWithdrawSyncingStatus, "状态同步中", "暂未查询到本次提现记录，请刷新后重试。"
	case processingAmount > 0 && allPending:
		return riderWithdrawAcceptedStatus, "提现已受理", "提现请求已受理，微信退款结果确认后会同步到账单。"
	case processingAmount > 0:
		return riderWithdrawProcessingStatus, "提现处理中", "提现正在处理中，到账结果会在微信确认后同步。"
	case successAmount == acceptedAmount:
		return riderWithdrawSuccessStatus, "提现已完成", "提现已完成，押金余额和流水已同步。"
	case failedAmount == acceptedAmount:
		return riderWithdrawFailedStatus, "提现未完成", "提现未完成，已退回可用押金，请刷新余额后重试。"
	case successAmount > 0 && failedAmount > 0:
		return riderWithdrawPartialFailedStatus, "部分提现未完成", "部分提现已完成，未完成部分已退回可用押金。"
	default:
		return riderWithdrawSyncingStatus, "状态同步中", "提现状态正在同步，请稍后刷新。"
	}
}

// getRiderWithdrawalStatus godoc
// @Summary 查询骑手押金提现状态
// @Description 按提现提交返回的 refund_order_ids 查询本次提现处理状态和金额汇总。
// @Tags 骑手
// @Accept json
// @Produce json
// @Param refund_order_ids query []int true "提现退款单ID，支持逗号或重复参数" collectionFormat(multi)
// @Success 200 {object} riderWithdrawalStatusResponse "提现状态"
// @Failure 400 {object} ErrorResponse "参数错误"
// @Failure 401 {object} ErrorResponse "未登录"
// @Failure 404 {object} ErrorResponse "未注册骑手或提现记录不存在"
// @Failure 500 {object} ErrorResponse "服务器错误"
// @Router /v1/rider/withdrawals/status [get]
// @Security BearerAuth
func (server *Server) getRiderWithdrawalStatus(ctx *gin.Context) {
	refundOrderIDs, err := parseRiderWithdrawalRefundOrderIDs(ctx.QueryArray("refund_order_ids"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	if _, err := server.store.GetRiderByUserID(ctx, authPayload.UserID); err != nil {
		if isNotFoundError(err) {
			ctx.JSON(http.StatusNotFound, errorResponse(ErrRiderNotRegistered))
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	rows, err := server.store.ListRiderDepositWithdrawalRefundOrdersByIDs(ctx, db.ListRiderDepositWithdrawalRefundOrdersByIDsParams{
		UserID:         authPayload.UserID,
		RefundOrderIds: refundOrderIDs,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if len(rows) == 0 {
		ctx.JSON(http.StatusNotFound, errorResponse(errors.New("未找到提现记录")))
		return
	}

	ctx.JSON(http.StatusOK, newRiderWithdrawalStatusResponse(rows))
}
