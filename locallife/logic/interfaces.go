package logic

import (
	"context"
	"time"

	aggregatecontracts "github.com/merrydance/locallife/baofu/aggregatepay/contracts"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/maps"
	"github.com/merrydance/locallife/rules"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
)

type OrderCommandService interface {
	CreateOrder(ctx context.Context, input CreateOrderCommandInput) (CreateOrderCommandResult, error)
	CancelOrder(ctx context.Context, input CancelOrderInput) (CancelOrderResult, error)
	UrgeOrder(ctx context.Context, input UrgeOrderInput) (UrgeOrderResult, error)
	ReplaceOrder(ctx context.Context, input ReplaceOrderInput) (ReplaceOrderResult, error)
	ConfirmOrder(ctx context.Context, input ConfirmOrderInput) (ConfirmOrderResult, error)

	AcceptMerchantOrder(ctx context.Context, input MerchantOrderUpdateInput) (MerchantOrderUpdateResult, error)
	RejectMerchantOrder(ctx context.Context, input MerchantOrderUpdateInput) (MerchantOrderUpdateResult, error)
	MarkMerchantOrderReady(ctx context.Context, input MerchantOrderUpdateInput) (MerchantOrderUpdateResult, error)
	CompleteMerchantOrder(ctx context.Context, input MerchantOrderUpdateInput) (MerchantOrderUpdateResult, error)
	PrintMerchantOrder(ctx context.Context, input MerchantOrderPrintInput) (MerchantOrderPrintResult, error)
}

type OrderQueryService interface {
	GetUserOrder(ctx context.Context, input GetUserOrderQueryInput) (GetUserOrderQueryResult, error)
	ListUserOrders(ctx context.Context, input ListUserOrdersQueryInput) (ListUserOrdersQueryResult, error)
	GetMerchantOrder(ctx context.Context, input GetMerchantOrderQueryInput) (GetMerchantOrderQueryResult, error)
	ListMerchantOrders(ctx context.Context, input ListMerchantOrdersQueryInput) (ListMerchantOrdersQueryResult, error)
	GetMerchantOrderStats(ctx context.Context, input GetMerchantOrderStatsQueryInput) (GetMerchantOrderStatsQueryResult, error)
	CalculateOrderPreview(ctx context.Context, input CalculateOrderPreviewInput) (OrderCalculationResult, error)
}

type PaymentFacade interface {
	CreatePaymentOrder(ctx context.Context, input CreatePaymentOrderInput) (CreatePaymentOrderResult, error)
	CreateCombinedPaymentOrder(ctx context.Context, input CreateCombinedPaymentOrderInput) (CreateCombinedPaymentOrderResult, error)
	GetCombinedPaymentOrder(ctx context.Context, input GetCombinedPaymentOrderInput) (GetCombinedPaymentOrderResult, error)
	QueryCombinedPaymentOrder(ctx context.Context, input QueryCombinedPaymentOrderInput) (QueryCombinedPaymentOrderResult, error)
	CloseCombinedPaymentOrder(ctx context.Context, input CloseCombinedPaymentOrderInput) (CloseCombinedPaymentOrderResult, error)
	GetPaymentOrder(ctx context.Context, input GetPaymentOrderInput) (GetPaymentOrderResult, error)
	QueryPaymentOrder(ctx context.Context, input QueryPaymentOrderInput) (QueryPaymentOrderResult, error)
	ListPaymentOrders(ctx context.Context, input ListPaymentOrdersInput) (ListPaymentOrdersResult, error)
	ListPaymentLedger(ctx context.Context, input ListPaymentLedgerInput) (ListPaymentLedgerResult, error)
	ClosePaymentOrder(ctx context.Context, input ClosePaymentOrderInput) (ClosePaymentOrderResult, error)

	CreateRefund(ctx context.Context, req *wechatcontracts.DirectRefundRequest) (*wechatcontracts.DirectRefundResponse, error)
	CreateBaofuRefund(ctx context.Context, req aggregatecontracts.RefundBeforeShareRequest) (*aggregatecontracts.RefundResult, error)
	BaofuRefundNotifyURL() string
}

type RefundOrchestrator interface {
	CreateRefundOrder(ctx context.Context, input CreateRefundOrderInput) (CreateRefundOrderResult, error)
	GetRefundOrder(ctx context.Context, input GetRefundOrderInput) (GetRefundOrderResult, error)
	ListRefundOrdersByPayment(ctx context.Context, input ListRefundOrdersByPaymentInput) (ListRefundOrdersByPaymentResult, error)
	ListProfitSharingReturnsByRefund(ctx context.Context, input ListProfitSharingReturnsByRefundInput) (ListProfitSharingReturnsByRefundResult, error)
}

type NotificationInput struct {
	UserID             int64
	Type               string
	Title              string
	Content            string
	RelatedType        string
	RelatedID          int64
	IgnorePreferences  bool
	ExpiresAt          *time.Time
	PushToRider        bool
	PushToMerchant     bool
	RiderID            int64
	MerchantID         int64
	OrderNo            string
	OrderStatus        string
	NotificationReason string
}

type MerchantOrderPrintInput struct {
	MerchantID int64
	OrderID    int64
	OperatorID int64
}

type MerchantOrderPrintResult struct {
	Order db.Order
}

type NotificationPublisher interface {
	Send(ctx context.Context, input NotificationInput) error
}

type AuditLogInput struct {
	ActorUserID int64
	ActorRole   string
	Action      string
	TargetType  string
	TargetID    *int64
	RegionID    *int64
	Metadata    map[string]interface{}
}

type AuditLogger interface {
	Write(ctx context.Context, input AuditLogInput)
}

type MerchantUserRiskAlert struct {
	UserID     int64
	OrderID    int64
	OrderNo    string
	Message    string
	ReasonCode string
}

type OrderEventPublisher interface {
	PublishMerchantOrderSnapshot(ctx context.Context, merchantID int64, order db.Order, messageType string)
	PublishMerchantUserRiskAlert(ctx context.Context, merchantID int64, alert MerchantUserRiskAlert)
	PublishTakeoutOrderPooled(ctx context.Context, order db.Order, poolItem db.DeliveryPool)
}

type ProcessRefundTaskInput struct {
	PaymentOrderID int64
	OrderID        int64
	ReservationID  int64
	RefundAmount   int64
	Reason         string
	OutRefundNo    string
}

type ProfitSharingReturnResultTaskInput struct {
	ProfitSharingReturnID int64
	OutReturnNo           string
	OutOrderNo            string
	SubMchID              string
	RefundOrderID         int64
	RetryCount            int
	Delay                 time.Duration
}

type OrderPrintTaskInput struct {
	OrderID int64
	Trigger string
}

type TaskScheduler interface {
	ScheduleOrderPaymentTimeout(ctx context.Context, orderID int64, at time.Time) error
	SchedulePaymentOrderTimeout(ctx context.Context, paymentOrderNo string, at time.Time) error
	ScheduleCombinedPaymentOrderTimeout(ctx context.Context, combineOutTradeNo string, at time.Time) error
	ScheduleProcessRefund(ctx context.Context, input ProcessRefundTaskInput) error
	ScheduleProfitSharing(ctx context.Context, paymentOrderID, orderID int64) error
	ScheduleProfitSharingReturnResult(ctx context.Context, input ProfitSharingReturnResultTaskInput) error
	ScheduleOrderPrint(ctx context.Context, input OrderPrintTaskInput) error
}

type DishCustomizationNormalizer interface {
	Normalize(ctx context.Context, dishID int64, customizations map[string]interface{}) ([]byte, int64, error)
}

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	OrderNo(now time.Time) (string, error)
	PickupCode(now time.Time) (string, error)
	OutTradeNo(prefix string, now time.Time) (string, error)
	OutRefundNo(now time.Time) (string, error)
}

type OrderPolicy interface {
	ValidateCreateInput(input CreateOrderCommandInput) error
}

type CreateOrderCommandInput struct {
	UserID         int64
	MerchantID     int64
	OrderType      string
	AddressID      *int64
	TableID        *int64
	ReservationID  *int64
	BillingGroupID *int64
	Items          []OrderItemInput
	Notes          string
	UserVoucherID  *int64
	UseBalance     bool

	RulesEngine        rules.Engine
	RulesEngineEnabled bool
	OnRuleDecision     func(input rules.Context, decision rules.Decision, actorRole string)

	MapClient maps.TencentMapClientInterface

	DeliveryFeeCalculator DeliveryFeeCalculator

	RiderAverageSpeed  int
	DefaultPrepareTime int
}

type CreateOrderCommandResult struct {
	Order        db.Order
	RuleDecision rules.Decision
	HasRule      bool
}

type CreateRefundOrderInput struct {
	ActorUserID                      int64
	PaymentOrderID                   int64
	RefundType                       string
	RefundAmount                     int64
	RefundReason                     string
	ProfitSharingReturnRetryInterval time.Duration
}

type CreateRefundOrderResult struct {
	RefundOrder db.RefundOrder
}

type GetRefundOrderInput struct {
	ActorUserID int64
	RefundID    int64
}

type GetRefundOrderResult struct {
	RefundOrder db.RefundOrder
}

type ListRefundOrdersByPaymentInput struct {
	ActorUserID     int64
	PaymentOrderID  int64
	IsMerchantActor bool
}

type ListRefundOrdersByPaymentResult struct {
	RefundOrders []db.RefundOrder
}

type ListProfitSharingReturnsByRefundInput struct {
	ActorUserID int64
	RefundID    int64
}

type ListProfitSharingReturnsByRefundResult struct {
	Returns []db.ProfitSharingReturn
}

type GetUserOrderQueryInput struct {
	UserID  int64
	OrderID int64
}

type GetUserOrderQueryResult struct {
	Order               db.GetOrderWithDetailsRow
	Items               []db.ListOrderItemsWithDishByOrderRow
	DeliveryEtaMinutes  *int32
	EstimatedDeliveryAt *time.Time
	WechatTransactionID *string
}

type ListUserOrdersQueryInput struct {
	UserID        int64
	OrderType     *string
	Status        *string
	ReservationID *int64
	MerchantID    *int64
	PageID        int32
	PageSize      int32
	SortBy        string
	SortOrder     string
	Keyword       *string
	DateFrom      *time.Time
	DateTo        *time.Time
	MinAmount     *int64
	MaxAmount     *int64
	Source        *string
	DeliveryETA   bool
}

type ListUserOrdersQueryResult struct {
	Orders     []db.ListOrdersByUserWithFiltersRow
	TotalCount int64
}

type GetMerchantOrderQueryInput struct {
	MerchantID int64
	OrderID    int64
}

type GetMerchantOrderQueryResult struct {
	Order db.Order
	Items []db.ListOrderItemsWithDishByOrderRow
}

type ListMerchantOrdersQueryInput struct {
	MerchantID int64
	Status     *string
	OrderType  *string
	PageID     int32
	PageSize   int32
}

type ListMerchantOrdersQueryResult struct {
	Orders         []db.Order
	ItemsByOrderID map[int64][]db.ListOrderItemsWithDishByOrderIDsRow
	TotalCount     int64
}

type GetMerchantOrderStatsQueryInput struct {
	MerchantID int64
	StartDate  time.Time
	EndDate    time.Time
}

type GetMerchantOrderStatsQueryResult struct {
	Stats db.GetOrderStatsRow
}

type CalculateOrderPreviewInput struct {
	OrderCalculationInput
	Normalize      NormalizeDishCustomizationsFunc
	CalculateFee   DeliveryFeeCalculator
	MapClient      maps.TencentMapClientInterface
	Store          db.Store
	RequireMapCalc bool
}
