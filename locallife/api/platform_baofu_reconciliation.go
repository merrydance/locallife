package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
)

type platformBaofuDailyReconciliationRow struct {
	Date                     string `json:"date"`
	Provider                 string `json:"provider"`
	Channel                  string `json:"channel"`
	PaidAmount               int64  `json:"paid_amount"`
	PaymentFee               int64  `json:"payment_fee"`
	MerchantAmount           int64  `json:"merchant_amount"`
	RiderAmount              int64  `json:"rider_amount"`
	PlatformCommission       int64  `json:"platform_commission"`
	OperatorCommission       int64  `json:"operator_commission"`
	WithdrawSucceededAmount  int64  `json:"withdraw_succeeded_amount"`
	WithdrawProcessingAmount int64  `json:"withdraw_processing_amount"`
	UnappliedFactCount       int64  `json:"unapplied_fact_count"`
	UnknownCommandCount      int64  `json:"unknown_command_count"`
	FeeLedgerMismatchCount   int64  `json:"fee_ledger_mismatch_count"`
}

// getPlatformBaofuDailyReconciliation 获取宝付每日对账汇总
// @Summary 获取宝付每日对账汇总
// @Description 管理员查看宝付支付、分账、提现与异常事实每日汇总；响应只返回聚合金额和计数，不返回分账接收方、合同号或上游原始数据
// @Tags Platform
// @Accept json
// @Produce json
// @Param start_date query string true "开始日期 (格式: 2025-01-01)"
// @Param end_date query string true "结束日期 (格式: 2025-01-31)"
// @Security BearerAuth
// @Success 200 {array} platformBaofuDailyReconciliationRow "宝付每日对账汇总"
// @Failure 400 {object} errorRes "请求参数错误"
// @Failure 401 {object} errorRes "未授权"
// @Failure 403 {object} errorRes "权限不足"
// @Failure 500 {object} errorRes "服务器内部错误"
// @Router /v1/platform/stats/baofu/reconciliation/daily [get]
func (server *Server) getPlatformBaofuDailyReconciliation(ctx *gin.Context) {
	var req getPlatformOverviewRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	startDate, endDate, err := parseDateRange(req.StartDate, req.EndDate, 365)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	endDate = endDate.Add(24*time.Hour - time.Nanosecond)

	rows, err := server.store.GetBaofuDailyReconciliation(ctx, db.GetBaofuDailyReconciliationParams{
		StartAt: pgtype.Timestamptz{Time: startDate, Valid: true},
		EndAt:   pgtype.Timestamptz{Time: endDate, Valid: true},
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	server.writeAuditLog(ctx, AuditLogInput{
		ActorUserID: authPayload.UserID,
		ActorRole:   "platform",
		Action:      "platform_baofu_daily_reconciliation_viewed",
		TargetType:  "baofu_reconciliation",
		RegionID:    nil,
		Metadata: map[string]any{
			"start_date": req.StartDate,
			"end_date":   req.EndDate,
		},
	})

	result := make([]platformBaofuDailyReconciliationRow, len(rows))
	for i, row := range rows {
		result[i] = platformBaofuDailyReconciliationRow{
			Date:                     row.Date.Time.Format("2006-01-02"),
			Provider:                 row.Provider,
			Channel:                  row.Channel,
			PaidAmount:               row.PaidAmount,
			PaymentFee:               row.PaymentFee,
			MerchantAmount:           row.MerchantAmount,
			RiderAmount:              row.RiderAmount,
			PlatformCommission:       row.PlatformCommission,
			OperatorCommission:       row.OperatorCommission,
			WithdrawSucceededAmount:  row.WithdrawSucceededAmount,
			WithdrawProcessingAmount: row.WithdrawProcessingAmount,
			UnappliedFactCount:       row.UnappliedFactCount,
			UnknownCommandCount:      row.UnknownCommandCount,
			FeeLedgerMismatchCount:   row.FeeLedgerMismatchCount,
		}
	}

	ctx.JSON(http.StatusOK, result)
}
