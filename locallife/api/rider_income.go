package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/token"
)

const riderIncomeDateRangeMaxDays = 90

type riderIncomeDateRangeRequest struct {
	StartDate string `form:"start_date" binding:"omitempty,datetime=2006-01-02"`
	EndDate   string `form:"end_date" binding:"omitempty,datetime=2006-01-02"`
}

type listRiderIncomeLedgerRequest struct {
	StartDate string `form:"start_date" binding:"omitempty,datetime=2006-01-02"`
	EndDate   string `form:"end_date" binding:"omitempty,datetime=2006-01-02"`
	Status    string `form:"status" binding:"omitempty,oneof=pending processing finished failed"`
	PageID    int32  `form:"page_id" binding:"omitempty,min=1"`
	PageSize  int32  `form:"page_size" binding:"omitempty,min=1,max=50"`
}

type riderIncomeSummaryResponse struct {
	TotalDeliveries       int64                              `json:"total_deliveries"`
	TotalRiderIncome      int64                              `json:"total_rider_income"`
	TotalDeliveryFee      int64                              `json:"total_delivery_fee"`
	TotalRiderGrossAmount int64                              `json:"total_rider_gross_amount"`
	TotalRiderPaymentFee  int64                              `json:"total_rider_payment_fee"`
	StatusSummary         []riderIncomeStatusSummaryResponse `json:"status_summary"`
}

type riderIncomeStatusSummaryResponse struct {
	Status           string `json:"status"`
	OrderCount       int64  `json:"order_count"`
	RiderAmount      int64  `json:"rider_amount"`
	DeliveryFee      int64  `json:"delivery_fee"`
	RiderGrossAmount int64  `json:"rider_gross_amount"`
	RiderPaymentFee  int64  `json:"rider_payment_fee"`
}

type riderIncomeLedgerResponse struct {
	Items    []riderIncomeLedgerItemResponse `json:"items"`
	Total    int64                           `json:"total"`
	PageID   int32                           `json:"page_id"`
	PageSize int32                           `json:"page_size"`
	HasMore  bool                            `json:"has_more"`
}

type riderIncomeLedgerItemResponse struct {
	ID                  int64      `json:"id"`
	PaymentOrderID      int64      `json:"payment_order_id"`
	MerchantID          int64      `json:"merchant_id"`
	OrderID             int64      `json:"order_id"`
	OrderNo             string     `json:"order_no"`
	MerchantName        string     `json:"merchant_name"`
	Status              string     `json:"status"`
	TotalAmount         int64      `json:"total_amount"`
	DeliveryFee         int64      `json:"delivery_fee"`
	RiderGrossAmount    int64      `json:"rider_gross_amount"`
	RiderPaymentFee     int64      `json:"rider_payment_fee"`
	RiderAmount         int64      `json:"rider_amount"`
	DistributableAmount int64      `json:"distributable_amount"`
	OutOrderNo          string     `json:"out_order_no"`
	SharingOrderID      string     `json:"sharing_order_id"`
	FinishedAt          *time.Time `json:"finished_at"`
	CreatedAt           time.Time  `json:"created_at"`
}

type riderIncomeDailyResponse struct {
	Items []riderIncomeDailyItemResponse `json:"items"`
}

type riderIncomeDailyItemResponse struct {
	Date             string `json:"date"`
	DeliveryCount    int64  `json:"delivery_count"`
	DailyIncome      int64  `json:"daily_income"`
	RiderGrossAmount int64  `json:"rider_gross_amount"`
	RiderPaymentFee  int64  `json:"rider_payment_fee"`
}

func newRiderIncomeSummaryResponse(result logic.RiderIncomeSummary) riderIncomeSummaryResponse {
	statusSummary := make([]riderIncomeStatusSummaryResponse, 0, len(result.StatusSummary))
	for _, item := range result.StatusSummary {
		statusSummary = append(statusSummary, riderIncomeStatusSummaryResponse{
			Status:           item.Status,
			OrderCount:       item.OrderCount,
			RiderAmount:      item.RiderAmount,
			DeliveryFee:      item.DeliveryFee,
			RiderGrossAmount: item.RiderGrossAmount,
			RiderPaymentFee:  item.RiderPaymentFee,
		})
	}

	return riderIncomeSummaryResponse{
		TotalDeliveries:       result.TotalDeliveries,
		TotalRiderIncome:      result.TotalRiderIncome,
		TotalDeliveryFee:      result.TotalDeliveryFee,
		TotalRiderGrossAmount: result.TotalRiderGrossAmount,
		TotalRiderPaymentFee:  result.TotalRiderPaymentFee,
		StatusSummary:         statusSummary,
	}
}

func newRiderIncomeLedgerResponse(result logic.RiderIncomeLedger) riderIncomeLedgerResponse {
	items := make([]riderIncomeLedgerItemResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, riderIncomeLedgerItemResponse{
			ID:                  item.ID,
			PaymentOrderID:      item.PaymentOrderID,
			MerchantID:          item.MerchantID,
			OrderID:             item.OrderID,
			OrderNo:             item.OrderNo,
			MerchantName:        item.MerchantName,
			Status:              item.Status,
			TotalAmount:         item.TotalAmount,
			DeliveryFee:         item.DeliveryFee,
			RiderGrossAmount:    item.RiderGrossAmount,
			RiderPaymentFee:     item.RiderPaymentFee,
			RiderAmount:         item.RiderAmount,
			DistributableAmount: item.DistributableAmount,
			OutOrderNo:          item.OutOrderNo,
			SharingOrderID:      item.SharingOrderID,
			FinishedAt:          item.FinishedAt,
			CreatedAt:           item.CreatedAt,
		})
	}

	return riderIncomeLedgerResponse{
		Items:    items,
		Total:    result.Total,
		PageID:   result.PageID,
		PageSize: result.PageSize,
		HasMore:  result.HasMore,
	}
}

func newRiderIncomeDailyResponse(items []logic.RiderIncomeDailyItem) riderIncomeDailyResponse {
	responses := make([]riderIncomeDailyItemResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, riderIncomeDailyItemResponse{
			Date:             item.Date.Format("2006-01-02"),
			DeliveryCount:    item.DeliveryCount,
			DailyIncome:      item.DailyIncome,
			RiderGrossAmount: item.RiderGrossAmount,
			RiderPaymentFee:  item.RiderPaymentFee,
		})
	}
	return riderIncomeDailyResponse{Items: responses}
}

// getRiderIncomeSummary godoc
// @Summary 获取骑手分账收入汇总
// @Description 按日期范围获取当前骑手基于分账账本的已完成收入和状态汇总
// @Tags 骑手
// @Accept json
// @Produce json
// @Param start_date query string false "开始日期，格式 YYYY-MM-DD"
// @Param end_date query string false "结束日期，格式 YYYY-MM-DD"
// @Success 200 {object} riderIncomeSummaryResponse "骑手分账收入汇总"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "骑手资料不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/rider/income/summary [get]
// @Security BearerAuth
func (server *Server) getRiderIncomeSummary(ctx *gin.Context) {
	var req riderIncomeDateRangeRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	startAt, endAt, err := parseRiderIncomeDateRange(req.StartDate, req.EndDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	service := logic.NewRiderIncomeService(server.store)
	result, err := service.GetSummary(ctx, authPayload.UserID, startAt, endAt)
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newRiderIncomeSummaryResponse(result))
}

// listRiderIncomeLedger godoc
// @Summary 获取骑手分账收入账本
// @Description 分页获取当前骑手基于 profit sharing 的收入账本，可按分账状态筛选
// @Tags 骑手
// @Accept json
// @Produce json
// @Param start_date query string false "开始日期，格式 YYYY-MM-DD"
// @Param end_date query string false "结束日期，格式 YYYY-MM-DD"
// @Param status query string false "分账状态" Enums(pending, processing, finished, failed)
// @Param page_id query int false "页码" minimum(1)
// @Param page_size query int false "每页条数" minimum(1) maximum(50)
// @Success 200 {object} riderIncomeLedgerResponse "骑手分账收入账本"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "骑手资料不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/rider/income/ledger [get]
// @Security BearerAuth
func (server *Server) listRiderIncomeLedger(ctx *gin.Context) {
	var req listRiderIncomeLedgerRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	startAt, endAt, err := parseRiderIncomeDateRange(req.StartDate, req.EndDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	service := logic.NewRiderIncomeService(server.store)
	result, err := service.ListLedger(ctx, authPayload.UserID, req.Status, startAt, endAt, req.PageID, req.PageSize)
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newRiderIncomeLedgerResponse(result))
}

// getRiderIncomeDaily godoc
// @Summary 获取骑手每日分账收入
// @Description 按日期范围获取当前骑手已完成分账收入的日汇总
// @Tags 骑手
// @Accept json
// @Produce json
// @Param start_date query string false "开始日期，格式 YYYY-MM-DD"
// @Param end_date query string false "结束日期，格式 YYYY-MM-DD"
// @Success 200 {object} riderIncomeDailyResponse "骑手每日分账收入"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "骑手资料不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/rider/income/daily [get]
// @Security BearerAuth
func (server *Server) getRiderIncomeDaily(ctx *gin.Context) {
	var req riderIncomeDateRangeRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	startAt, endAt, err := parseRiderIncomeDateRange(req.StartDate, req.EndDate)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	service := logic.NewRiderIncomeService(server.store)
	result, err := service.GetDaily(ctx, authPayload.UserID, startAt, endAt)
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newRiderIncomeDailyResponse(result))
}

func parseRiderIncomeDateRange(startDateStr, endDateStr string) (time.Time, time.Time, error) {
	if endDateStr == "" {
		endDateStr = time.Now().UTC().Format("2006-01-02")
	}
	if startDateStr == "" {
		endDate, err := time.Parse("2006-01-02", endDateStr)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		startDateStr = endDate.AddDate(0, 0, -30).Format("2006-01-02")
	}

	startAt, endDate, err := parseDateRange(startDateStr, endDateStr, riderIncomeDateRangeMaxDays)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	endAt := endDate.AddDate(0, 0, 1).Add(-time.Nanosecond)
	return startAt, endAt, nil
}
