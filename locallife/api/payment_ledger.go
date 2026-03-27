package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/token"
)

type listPaymentLedgerRequest struct {
	PageID   int32 `form:"page_id" binding:"omitempty,min=1"`
	PageSize int32 `form:"page_size" binding:"omitempty,min=1,max=20"`
}

type paymentLedgerEntryResponse struct {
	ID             int64     `json:"id"`
	EntryType      string    `json:"entry_type"`
	PaymentOrderID int64     `json:"payment_order_id"`
	RefundOrderID  *int64    `json:"refund_order_id,omitempty"`
	OrderID        *int64    `json:"order_id,omitempty"`
	BusinessType   string    `json:"business_type"`
	Amount         int64     `json:"amount"`
	Status         string    `json:"status"`
	OccurredAt     time.Time `json:"occurred_at"`
	CreatedAt      time.Time `json:"created_at"`
}

type listPaymentLedgerResponse struct {
	Entries  []paymentLedgerEntryResponse `json:"entries"`
	Total    int64                        `json:"total"`
	PageID   int32                        `json:"page_id"`
	PageSize int32                        `json:"page_size"`
}

func newPaymentLedgerEntryResponse(entry logic.PaymentLedgerEntry) paymentLedgerEntryResponse {
	return paymentLedgerEntryResponse{
		ID:             entry.ID,
		EntryType:      entry.EntryType,
		PaymentOrderID: entry.PaymentOrderID,
		RefundOrderID:  entry.RefundOrderID,
		OrderID:        entry.OrderID,
		BusinessType:   entry.BusinessType,
		Amount:         entry.Amount,
		Status:         entry.Status,
		OccurredAt:     entry.OccurredAt,
		CreatedAt:      entry.CreatedAt,
	}
}

// listPaymentLedger godoc
// @Summary 获取用户支付账单流水
// @Description 分页获取当前用户的支付与退款流水
// @Tags 支付管理
// @Accept json
// @Produce json
// @Param page_id query int false "页码" minimum(1)
// @Param page_size query int false "每页条数" minimum(1) maximum(20)
// @Success 200 {object} listPaymentLedgerResponse "支付账单流水"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/payments/ledger [get]
// @Security BearerAuth
func (server *Server) listPaymentLedger(ctx *gin.Context) {
	var req listPaymentLedgerRequest
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

	result, err := facade.ListPaymentLedger(ctx, logic.ListPaymentLedgerInput{
		UserID:   authPayload.UserID,
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

	entries := make([]paymentLedgerEntryResponse, 0, len(result.Entries))
	for _, entry := range result.Entries {
		entries = append(entries, newPaymentLedgerEntryResponse(entry))
	}

	ctx.JSON(http.StatusOK, listPaymentLedgerResponse{
		Entries:  entries,
		Total:    result.TotalCount,
		PageID:   pageID,
		PageSize: pageSize,
	})
}
