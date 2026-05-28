package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/merrydance/locallife/logic"
	"github.com/merrydance/locallife/token"
)

type riderWorkbenchSummaryResponse struct {
	RiderStatus       riderWorkbenchRiderStatusResponse       `json:"rider_status"`
	CurrentDeliveries riderWorkbenchCurrentDeliveriesResponse `json:"current_deliveries"`
	OrderPool         riderWorkbenchOrderPoolResponse         `json:"order_pool"`
	Today             riderWorkbenchTodayResponse             `json:"today"`
	Income            riderWorkbenchIncomeResponse            `json:"income"`
	Deposit           riderWorkbenchDepositResponse           `json:"deposit"`
	Claims            riderWorkbenchClaimsResponse            `json:"claims"`
	Notifications     riderWorkbenchNotificationsResponse     `json:"notifications"`
	Sections          []riderWorkbenchSectionStatusResponse   `json:"sections"`
}

type riderWorkbenchSectionStatusResponse struct {
	Section   string `json:"section"`
	Available bool   `json:"available"`
	Message   string `json:"message,omitempty"`
}

type riderWorkbenchRiderStatusResponse struct {
	Status            string     `json:"status"`
	IsOnline          bool       `json:"is_online"`
	OnlineStatus      string     `json:"online_status"`
	ActiveDeliveries  int        `json:"active_deliveries"`
	CurrentLongitude  *float64   `json:"current_longitude,omitempty"`
	CurrentLatitude   *float64   `json:"current_latitude,omitempty"`
	LocationUpdatedAt *time.Time `json:"location_updated_at,omitempty"`
	CanGoOnline       bool       `json:"can_go_online"`
	CanGoOffline      bool       `json:"can_go_offline"`
	OnlineBlockReason string     `json:"online_block_reason,omitempty"`
}

type riderWorkbenchCurrentDeliveriesResponse struct {
	ActiveCount int                                  `json:"active_count"`
	Items       []riderWorkbenchDeliveryItemResponse `json:"items"`
}

type riderWorkbenchDeliveryItemResponse struct {
	ID                   int64      `json:"id"`
	OrderID              int64      `json:"order_id"`
	OrderStatus          string     `json:"order_status"`
	FulfillmentStatus    string     `json:"fulfillment_status"`
	Status               string     `json:"status"`
	CanConfirmPickup     bool       `json:"can_confirm_pickup"`
	PickupBlockReason    string     `json:"pickup_block_reason,omitempty"`
	PickupActionLabel    string     `json:"pickup_action_label,omitempty"`
	DeliveryFee          int64      `json:"delivery_fee"`
	RiderEarnings        int64      `json:"rider_earnings"`
	RiderGrossAmount     int64      `json:"rider_gross_amount,omitempty"`
	RiderPaymentFee      int64      `json:"rider_payment_fee,omitempty"`
	RiderNetEarnings     int64      `json:"rider_net_earnings,omitempty"`
	ProfitSharingOrderID int64      `json:"profit_sharing_order_id,omitempty"`
	ProfitSharingStatus  string     `json:"profit_sharing_status,omitempty"`
	PickupAddress        string     `json:"pickup_address"`
	DeliveryAddress      string     `json:"delivery_address"`
	EstimatedPickupAt    *time.Time `json:"estimated_pickup_at,omitempty"`
	EstimatedDeliveryAt  *time.Time `json:"estimated_delivery_at,omitempty"`
	PickedAt             *time.Time `json:"picked_at,omitempty"`
	DeliveredAt          *time.Time `json:"delivered_at,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
}

type riderWorkbenchOrderPoolResponse struct {
	AvailableCount int64 `json:"available_count"`
}

type riderWorkbenchTodayResponse struct {
	Date                string `json:"date"`
	CompletedDeliveries int64  `json:"completed_deliveries"`
}

type riderWorkbenchIncomeResponse struct {
	TotalDeliveries       int64 `json:"total_deliveries"`
	TotalRiderIncome      int64 `json:"total_rider_income"`
	TotalDeliveryFee      int64 `json:"total_delivery_fee"`
	TotalRiderGrossAmount int64 `json:"total_rider_gross_amount"`
	TotalRiderPaymentFee  int64 `json:"total_rider_payment_fee"`
	PendingRiderAmount    int64 `json:"pending_rider_amount"`
	ProcessingRiderAmount int64 `json:"processing_rider_amount"`
	FailedCount           int64 `json:"failed_count"`
}

type riderWorkbenchDepositResponse struct {
	TotalDeposit                  int64 `json:"total_deposit"`
	FrozenDeposit                 int64 `json:"frozen_deposit"`
	DeliveryFrozenDeposit         int64 `json:"delivery_frozen_deposit"`
	DepositRefundProcessingAmount int64 `json:"deposit_refund_processing_amount"`
	AvailableDeposit              int64 `json:"available_deposit"`
	ThresholdAmount               int64 `json:"threshold_amount"`
}

type riderWorkbenchClaimsResponse struct {
	PendingActionCount int64 `json:"pending_action_count"`
}

type riderWorkbenchNotificationsResponse struct {
	UnreadCount int64 `json:"unread_count"`
}

func newRiderWorkbenchSummaryResponse(result logic.RiderWorkbenchSummary) riderWorkbenchSummaryResponse {
	sections := make([]riderWorkbenchSectionStatusResponse, 0, len(result.Sections))
	for _, section := range result.Sections {
		sections = append(sections, riderWorkbenchSectionStatusResponse{
			Section:   section.Section,
			Available: section.Available,
			Message:   section.Message,
		})
	}

	items := make([]riderWorkbenchDeliveryItemResponse, 0, len(result.CurrentDeliveries.Items))
	for _, item := range result.CurrentDeliveries.Items {
		items = append(items, riderWorkbenchDeliveryItemResponse{
			ID:                   item.ID,
			OrderID:              item.OrderID,
			OrderStatus:          item.OrderStatus,
			FulfillmentStatus:    item.FulfillmentStatus,
			Status:               item.Status,
			CanConfirmPickup:     item.CanConfirmPickup,
			PickupBlockReason:    item.PickupBlockReason,
			PickupActionLabel:    item.PickupActionLabel,
			DeliveryFee:          item.DeliveryFee,
			RiderEarnings:        item.RiderEarnings,
			RiderGrossAmount:     item.RiderGrossAmount,
			RiderPaymentFee:      item.RiderPaymentFee,
			RiderNetEarnings:     item.RiderNetEarnings,
			ProfitSharingOrderID: item.ProfitSharingOrderID,
			ProfitSharingStatus:  item.ProfitSharingStatus,
			PickupAddress:        item.PickupAddress,
			DeliveryAddress:      item.DeliveryAddress,
			EstimatedPickupAt:    item.EstimatedPickupAt,
			EstimatedDeliveryAt:  item.EstimatedDeliveryAt,
			PickedAt:             item.PickedAt,
			DeliveredAt:          item.DeliveredAt,
			CreatedAt:            item.CreatedAt,
		})
	}

	return riderWorkbenchSummaryResponse{
		RiderStatus: riderWorkbenchRiderStatusResponse{
			Status:            result.RiderStatus.Status,
			IsOnline:          result.RiderStatus.IsOnline,
			OnlineStatus:      result.RiderStatus.OnlineStatus,
			ActiveDeliveries:  result.RiderStatus.ActiveDeliveries,
			CurrentLongitude:  result.RiderStatus.CurrentLongitude,
			CurrentLatitude:   result.RiderStatus.CurrentLatitude,
			LocationUpdatedAt: result.RiderStatus.LocationUpdatedAt,
			CanGoOnline:       result.RiderStatus.CanGoOnline,
			CanGoOffline:      result.RiderStatus.CanGoOffline,
			OnlineBlockReason: result.RiderStatus.OnlineBlockReason,
		},
		CurrentDeliveries: riderWorkbenchCurrentDeliveriesResponse{
			ActiveCount: result.CurrentDeliveries.ActiveCount,
			Items:       items,
		},
		OrderPool: riderWorkbenchOrderPoolResponse{
			AvailableCount: result.OrderPool.AvailableCount,
		},
		Today: riderWorkbenchTodayResponse{
			Date:                result.Today.Date,
			CompletedDeliveries: result.Today.CompletedDeliveries,
		},
		Income: riderWorkbenchIncomeResponse{
			TotalDeliveries:       result.Income.TotalDeliveries,
			TotalRiderIncome:      result.Income.TotalRiderIncome,
			TotalDeliveryFee:      result.Income.TotalDeliveryFee,
			TotalRiderGrossAmount: result.Income.TotalRiderGrossAmount,
			TotalRiderPaymentFee:  result.Income.TotalRiderPaymentFee,
			PendingRiderAmount:    result.Income.PendingRiderAmount,
			ProcessingRiderAmount: result.Income.ProcessingRiderAmount,
			FailedCount:           result.Income.FailedCount,
		},
		Deposit: riderWorkbenchDepositResponse{
			TotalDeposit:                  result.Deposit.TotalDeposit,
			FrozenDeposit:                 result.Deposit.FrozenDeposit,
			DeliveryFrozenDeposit:         result.Deposit.DeliveryFrozenDeposit,
			DepositRefundProcessingAmount: result.Deposit.DepositRefundProcessingAmount,
			AvailableDeposit:              result.Deposit.AvailableDeposit,
			ThresholdAmount:               result.Deposit.ThresholdAmount,
		},
		Claims: riderWorkbenchClaimsResponse{
			PendingActionCount: result.Claims.PendingActionCount,
		},
		Notifications: riderWorkbenchNotificationsResponse{
			UnreadCount: result.Notifications.UnreadCount,
		},
		Sections: sections,
	}
}

// getRiderWorkbenchSummary godoc
// @Summary 获取骑手工作台摘要
// @Description 获取当前骑手首屏经营摘要，聚合状态、当前任务、订单池、代取费结算、押金、追偿和通知摘要
// @Tags 骑手
// @Accept json
// @Produce json
// @Success 200 {object} riderWorkbenchSummaryResponse "骑手工作台摘要"
// @Failure 401 {object} ErrorResponse "未授权"
// @Failure 404 {object} ErrorResponse "骑手资料不存在"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/rider/workbench/summary [get]
// @Security BearerAuth
func (server *Server) getRiderWorkbenchSummary(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	service := logic.NewRiderWorkbenchService(server.store)
	result, err := service.GetSummary(ctx, authPayload.UserID)
	if err != nil {
		if writeLogicRequestError(ctx, err) {
			return
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	ctx.JSON(http.StatusOK, newRiderWorkbenchSummaryResponse(result))
}
