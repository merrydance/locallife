package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Store defines all functions to execute db queries and transactions
type Store interface {
	Querier
	// Ping checks the database connection
	Ping(ctx context.Context) error
	// User registration transaction
	CreateUserTx(ctx context.Context, arg CreateUserTxParams) (CreateUserTxResult, error)
	// User address transactions
	CreateUserAddressTx(ctx context.Context, arg CreateUserAddressTxParams) (CreateUserAddressTxResult, error)
	SetDefaultAddressTx(ctx context.Context, arg SetDefaultAddressTxParams) (SetDefaultAddressTxResult, error)
	// Dish transactions
	CreateDishTx(ctx context.Context, arg CreateDishTxParams) (CreateDishTxResult, error)
	UpdateDishTx(ctx context.Context, arg UpdateDishTxParams) (UpdateDishTxResult, error)
	SetDishCustomizationsTx(ctx context.Context, arg SetDishCustomizationsTxParams) (SetDishCustomizationsTxResult, error)
	RenameMerchantDishCategoryTx(ctx context.Context, arg RenameMerchantDishCategoryTxParams) (RenameMerchantDishCategoryTxResult, error)
	// Merchant transactions
	SetBusinessHoursTx(ctx context.Context, arg SetBusinessHoursTxParams) (SetBusinessHoursTxResult, error)
	UpdateMerchantCapabilitiesTx(ctx context.Context, arg UpdateMerchantCapabilitiesTxParams) (UpdateMerchantCapabilitiesTxResult, error)
	// Merchant application approval transaction
	ApproveMerchantApplicationTx(ctx context.Context, arg ApproveMerchantApplicationTxParams) (ApproveMerchantApplicationTxResult, error)
	ResetMerchantApplicationTx(ctx context.Context, arg ResetMerchantApplicationTxParams) (ResetMerchantApplicationTxResult, error)
	// Rider application approval transaction
	ApproveRiderApplicationTx(ctx context.Context, arg ApproveRiderApplicationTxParams) (ApproveRiderApplicationTxResult, error)
	// Combo transactions
	CreateComboSetTx(ctx context.Context, arg CreateComboSetTxParams) (CreateComboSetTxResult, error)
	UpdateComboSetTx(ctx context.Context, arg UpdateComboSetTxParams) (UpdateComboSetTxResult, error)
	CreateOrderTx(ctx context.Context, arg CreateOrderTxParams) (CreateOrderTxResult, error)
	ProcessOrderPaymentTx(ctx context.Context, arg ProcessOrderPaymentTxParams) (ProcessOrderPaymentTxResult, error)
	ProcessPaymentSuccessTx(ctx context.Context, arg ProcessPaymentSuccessTxParams) (ProcessPaymentSuccessTxResult, error)
	// M10: Membership transactions
	JoinMembershipTx(ctx context.Context, arg JoinMembershipTxParams) (JoinMembershipTxResult, error)
	RechargeTx(ctx context.Context, arg RechargeTxParams) (RechargeTxResult, error)
	ConsumeTx(ctx context.Context, arg ConsumeTxParams) (ConsumeTxResult, error)
	RefundTx(ctx context.Context, arg RefundTxParams) (RefundTxResult, error)
	AdjustMemberBalanceTx(ctx context.Context, arg AdjustMemberBalanceTxParams) (AdjustMemberBalanceTxResult, error)
	// M10: Voucher transactions
	ClaimVoucherTx(ctx context.Context, arg ClaimVoucherTxParams) (ClaimVoucherTxResult, error)
	UseVoucherTx(ctx context.Context, arg UseVoucherTxParams) (UseVoucherTxResult, error)
	// M15: Delivery transactions
	GrabOrderTx(ctx context.Context, arg GrabOrderTxParams) (GrabOrderTxResult, error)
	UpdateDeliveryToPickupTx(ctx context.Context, arg UpdateDeliveryToPickupTxParams) (UpdateDeliveryToPickupTxResult, error)
	UpdateDeliveryToPickedTx(ctx context.Context, arg UpdateDeliveryToPickedTxParams) (UpdateDeliveryToPickedTxResult, error)
	UpdateDeliveryToDeliveringTx(ctx context.Context, arg UpdateDeliveryToDeliveringTxParams) (UpdateDeliveryToDeliveringTxResult, error)
	CompleteDeliveryTx(ctx context.Context, arg CompleteDeliveryTxParams) (CompleteDeliveryTxResult, error)
	// M15: Rider transactions
	PrepareRiderDepositRefundTx(ctx context.Context, arg PrepareRiderDepositRefundTxParams) (PrepareRiderDepositRefundTxResult, error)
	ResolveRiderDepositRefundTx(ctx context.Context, arg ResolveRiderDepositRefundTxParams) (ResolveRiderDepositRefundTxResult, error)
	// M15: Reservation transactions
	CancelReservationTx(ctx context.Context, arg CancelReservationTxParams) (CancelReservationTxResult, error)
	MarkNoShowTx(ctx context.Context, arg MarkNoShowTxParams) (MarkNoShowTxResult, error)
	ConfirmReservationTx(ctx context.Context, arg ConfirmReservationTxParams) (ConfirmReservationTxResult, error)
	CompleteReservationTx(ctx context.Context, arg CompleteReservationTxParams) (CompleteReservationTxResult, error)
	CreateReservationTx(ctx context.Context, arg CreateReservationTxParams) (CreateReservationTxResult, error)
	ReplaceReservationItemsTx(ctx context.Context, arg ReplaceReservationItemsTxParams) (ReplaceReservationItemsTxResult, error)
	// Payment transactions
	CreateCombinedPaymentTx(ctx context.Context, arg CreateCombinedPaymentTxParams) (CreateCombinedPaymentTxResult, error)
	CreateEcommercePaymentTx(ctx context.Context, arg CreateEcommercePaymentTxParams) (CreateEcommercePaymentTxResult, error)
	CreatePartnerPaymentTx(ctx context.Context, arg CreatePartnerPaymentTxParams) (CreatePartnerPaymentTxResult, error)
	CloseCombinedPaymentOrderTx(ctx context.Context, arg CloseCombinedPaymentOrderTxParams) (CloseCombinedPaymentOrderTxResult, error)
	// Notification idempotency transactions
	TryClaimWechatNotification(ctx context.Context, arg CreateWechatNotificationParams) (bool, error)
	ReleaseWechatNotificationClaim(ctx context.Context, id string) error
	// Applyment activation transaction (CB-4)
	ApplymentSubMchActivationTx(ctx context.Context, arg ApplymentSubMchActivationTxParams) error
	ListEcommerceApplymentsPendingFollowUp(ctx context.Context, arg ListEcommerceApplymentsPendingFollowUpParams) ([]EcommerceApplymentPendingFollowUp, error)
	MarkEcommerceApplymentResultProcessed(ctx context.Context, arg MarkEcommerceApplymentResultProcessedParams) error
	// Refund transactions
	CreateRefundOrderTx(ctx context.Context, arg CreateRefundOrderTxParams) (CreateRefundOrderTxResult, error)
	// CreateAnomalyRefundRecord 为已关闭/失败状态的支付单创建异常退款记录（跳过 status='paid' 校验）
	CreateAnomalyRefundRecord(ctx context.Context, arg CreateAnomalyRefundRecordParams) (RefundOrder, error)
	SyncReservationInventoryTx(ctx context.Context, arg SyncReservationInventoryTxParams) (SyncReservationInventoryTxResult, error)
	ReleaseReservationInventoryTx(ctx context.Context, arg ReleaseReservationInventoryTxParams) error
	// M15: Order status transactions
	UpdateOrderStatusTx(ctx context.Context, arg UpdateOrderStatusTxParams) (UpdateOrderStatusTxResult, error)
	AcceptTakeoutOrderTx(ctx context.Context, arg AcceptTakeoutOrderTxParams) (AcceptTakeoutOrderTxResult, error)
	MarkTakeoutOrderReadyTx(ctx context.Context, arg MarkTakeoutOrderReadyTxParams) (MarkTakeoutOrderReadyTxResult, error)
	CompleteOrderTx(ctx context.Context, arg CompleteOrderTxParams) (CompleteOrderTxResult, error)
	CancelOrderTx(ctx context.Context, arg CancelOrderTxParams) (CancelOrderTxResult, error)
	// M5: Table transactions
	DeleteTableTx(ctx context.Context, arg DeleteTableParams) (DeleteTableResult, error)
	// Dining session transactions
	OpenDiningSessionTx(ctx context.Context, arg OpenDiningSessionTxParams) (OpenDiningSessionTxResult, error)
	TransferDiningSessionTableTx(ctx context.Context, arg TransferDiningSessionTableTxParams) (TransferDiningSessionTableTxResult, error)
	CloseDiningSessionTx(ctx context.Context, arg CloseDiningSessionTxParams) (CloseDiningSessionTxResult, error)
	// Behavior trace transactions
	CreateClaimWithBehaviorTx(ctx context.Context, arg CreateClaimWithBehaviorTxParams) (CreateClaimWithBehaviorTxResult, error)
	CreateClaimCompensationTx(ctx context.Context, arg CreateClaimCompensationTxParams) (CreateClaimCompensationTxResult, error)
	FinalizeClaimCompensationAfterPayoutTx(ctx context.Context, arg FinalizeClaimCompensationAfterPayoutTxParams) (FinalizeClaimCompensationAfterPayoutTxResult, error)
	CreateRecoveryDisputeWithRecoveryTx(ctx context.Context, arg CreateRecoveryDisputeWithRecoveryTxParams) (CreateRecoveryDisputeWithRecoveryTxResult, error)
	ReviewRecoveryDisputeWithCompensationTx(ctx context.Context, arg ReviewRecoveryDisputeWithCompensationTxParams) (ReviewRecoveryDisputeWithCompensationTxResult, error)
	MarkClaimRecoveryOverdueWithActionTx(ctx context.Context, arg MarkClaimRecoveryOverdueWithActionTxParams) (MarkClaimRecoveryOverdueWithActionTxResult, error)
	// Group multi-store transactions
	ApproveGroupApplicationTx(ctx context.Context, arg ApproveGroupApplicationTxParams) (ApproveGroupApplicationTxResult, error)
	ApproveGroupJoinRequestTx(ctx context.Context, arg ApproveGroupJoinRequestTxParams) (ApproveGroupJoinRequestTxResult, error)
	// Profit sharing config transactions
	CreateProfitSharingConfigTx(ctx context.Context, arg CreateProfitSharingConfigTxParams) (CreateProfitSharingConfigTxResult, error)
	UpdateProfitSharingConfigTx(ctx context.Context, arg UpdateProfitSharingConfigTxParams) (UpdateProfitSharingConfigTxResult, error)
	UpdateProfitSharingConfigStatusTx(ctx context.Context, arg UpdateProfitSharingConfigStatusTxParams) (UpdateProfitSharingConfigStatusTxResult, error)
	// Session transactions
	RefreshSessionTx(ctx context.Context, arg RefreshSessionTxParams) (RefreshSessionTxResult, error)
	// Merchant app device transactions
	RegisterMerchantAppDeviceTx(ctx context.Context, arg RegisterMerchantAppDeviceParams) (MerchantAppDevice, error)
	UpdateMerchantAppDeviceHeartbeatTx(ctx context.Context, arg UpdateMerchantAppDeviceHeartbeatParams) (MerchantAppDevice, error)
	// Order replacement transaction
	ReplaceOrderTx(ctx context.Context, arg ReplaceOrderTxParams) (ReplaceOrderTxResult, error)
	// Food safety transactions
	ReportFoodSafetyIncidentTx(ctx context.Context, arg ReportFoodSafetyIncidentTxParams) (ReportFoodSafetyIncidentTxResult, error)
	ResolveFoodSafetyCaseTx(ctx context.Context, arg ResolveFoodSafetyCaseTxParams) (ResolveFoodSafetyCaseTxResult, error)
	ApproveOperatorRegionApplicationTx(ctx context.Context, applicationID int64) (ApproveOperatorRegionApplicationTxResult, error)
}

// SQLStore provides all functions to execute SQL queries and transactions
type SQLStore struct {
	connPool *pgxpool.Pool
	*Queries
}

// NewStore creates a new store
func NewStore(connPool *pgxpool.Pool) Store {
	return &SQLStore{
		connPool: connPool,
		Queries:  New(connPool),
	}
}

// Ping checks the database connection
func (store *SQLStore) Ping(ctx context.Context) error {
	return store.connPool.Ping(ctx)
}
