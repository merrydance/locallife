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
	SetDishFeaturedTagsTx(ctx context.Context, arg SetDishFeaturedTagsTxParams) (SetDishFeaturedTagsTxResult, error)
	RenameMerchantDishCategoryTx(ctx context.Context, arg RenameMerchantDishCategoryTxParams) (RenameMerchantDishCategoryTxResult, error)
	UnlinkUnusedMerchantDishCategoryTx(ctx context.Context, arg UnlinkUnusedMerchantDishCategoryParams) (MerchantDishCategory, error)
	// Merchant transactions
	SetBusinessHoursTx(ctx context.Context, arg SetBusinessHoursTxParams) (SetBusinessHoursTxResult, error)
	SetMerchantTagsTx(ctx context.Context, arg SetMerchantTagsTxParams) (SetMerchantTagsTxResult, error)
	UpdateMerchantCapabilitiesTx(ctx context.Context, arg UpdateMerchantCapabilitiesTxParams) (UpdateMerchantCapabilitiesTxResult, error)
	AddMerchantStaffTx(ctx context.Context, arg AddMerchantStaffTxParams) (AddMerchantStaffTxResult, error)
	AssignMerchantStaffRoleTx(ctx context.Context, arg AssignMerchantStaffRoleTxParams) (AssignMerchantStaffRoleTxResult, error)
	RemoveMerchantStaffTx(ctx context.Context, arg RemoveMerchantStaffTxParams) (RemoveMerchantStaffTxResult, error)
	// Merchant application approval transaction
	ApproveMerchantApplicationTx(ctx context.Context, arg ApproveMerchantApplicationTxParams) (ApproveMerchantApplicationTxResult, error)
	ResetMerchantApplicationTx(ctx context.Context, arg ResetMerchantApplicationTxParams) (ResetMerchantApplicationTxResult, error)
	VoteWantedMerchantTx(ctx context.Context, arg WantedMerchantVoteTxParams) (WantedMerchantVoteTxResult, error)
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
	CreateMerchantReservationTx(ctx context.Context, arg CreateMerchantReservationTxParams) (TableReservation, error)
	UpdateReservationTx(ctx context.Context, arg UpdateReservationTxParams) (TableReservation, error)
	ReplaceReservationItemsTx(ctx context.Context, arg ReplaceReservationItemsTxParams) (ReplaceReservationItemsTxResult, error)
	ReplaceReservationItemsWithRefundOrdersTx(ctx context.Context, arg ReplaceReservationItemsWithRefundOrdersTxParams) (ReplaceReservationItemsWithRefundOrdersTxResult, error)
	CreateReservationPositiveAdjustmentPaymentTx(ctx context.Context, arg CreateReservationPositiveAdjustmentPaymentTxParams) (CreateReservationPositiveAdjustmentPaymentTxResult, error)
	ApplyPaidReservationAdjustmentTx(ctx context.Context, arg ApplyPaidReservationAdjustmentTxParams) (ApplyPaidReservationAdjustmentTxResult, error)
	CloseReservationAdjustmentForPaymentTx(ctx context.Context, arg CloseReservationAdjustmentForPaymentTxParams) (CloseReservationAdjustmentForPaymentTxResult, error)
	// Payment transactions
	MarkBaofuAccountBindingActiveWithFeeLedgerTx(ctx context.Context, arg MarkBaofuAccountBindingActiveWithFeeLedgerTxParams) (MarkBaofuAccountBindingActiveWithFeeLedgerTxResult, error)
	MarkMerchantBaofuAccountOpeningReadyTx(ctx context.Context, arg MarkMerchantBaofuAccountOpeningReadyTxParams) (MarkMerchantBaofuAccountOpeningReadyTxResult, error)
	CreateBaofuWithdrawalOrderWithSubmittedCommandTx(ctx context.Context, arg CreateBaofuWithdrawalOrderWithSubmittedCommandTxParams) (CreateBaofuWithdrawalOrderWithSubmittedCommandTxResult, error)
	CreatePartnerPaymentTx(ctx context.Context, arg CreatePartnerPaymentTxParams) (CreatePartnerPaymentTxResult, error)
	CloseCombinedPaymentOrderTx(ctx context.Context, arg CloseCombinedPaymentOrderTxParams) (CloseCombinedPaymentOrderTxResult, error)
	CreateBaofuProfitSharingOrderTx(ctx context.Context, arg CreateBaofuProfitSharingOrderTxParams) (CreateBaofuProfitSharingOrderTxResult, error)
	EnsureBaofuProfitSharingBillTx(ctx context.Context, arg CreateBaofuProfitSharingOrderTxParams) (CreateBaofuProfitSharingOrderTxResult, error)
	PrepareBaofuProfitSharingCommandTx(ctx context.Context, arg PrepareBaofuProfitSharingCommandTxParams) (PrepareBaofuProfitSharingCommandTxResult, error)
	// Notification idempotency transactions
	TryClaimWechatNotification(ctx context.Context, arg CreateWechatNotificationParams) (bool, error)
	ReleaseWechatNotificationClaim(ctx context.Context, id string) error
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
	CreateTableTx(ctx context.Context, arg CreateTableTxParams) (CreateTableTxResult, error)
	UpdateTableTx(ctx context.Context, arg UpdateTableTxParams) (UpdateTableTxResult, error)
	UpdateTableStatusTx(ctx context.Context, arg UpdateTableStatusTxParams) (Table, error)
	DeleteTableTx(ctx context.Context, arg DeleteTableParams) (DeleteTableResult, error)
	SetTableImagePrimaryTx(ctx context.Context, arg SetTableImagePrimaryTxParams) (TableImage, error)
	// Dining session transactions
	OpenDiningSessionTx(ctx context.Context, arg OpenDiningSessionTxParams) (OpenDiningSessionTxResult, error)
	TransferDiningSessionTableTx(ctx context.Context, arg TransferDiningSessionTableTxParams) (TransferDiningSessionTableTxResult, error)
	CloseDiningSessionTx(ctx context.Context, arg CloseDiningSessionTxParams) (CloseDiningSessionTxResult, error)
	// Behavior trace transactions
	BackfillAbnormalStatsDaily(ctx context.Context, arg BackfillAbnormalStatsDailyParams) error
	CreateClaimWithBehaviorTx(ctx context.Context, arg CreateClaimWithBehaviorTxParams) (CreateClaimWithBehaviorTxResult, error)
	CreateClaimCompensationTx(ctx context.Context, arg CreateClaimCompensationTxParams) (CreateClaimCompensationTxResult, error)
	FinalizeClaimCompensationAfterPayoutTx(ctx context.Context, arg FinalizeClaimCompensationAfterPayoutTxParams) (FinalizeClaimCompensationAfterPayoutTxResult, error)
	CreateRecoveryDisputeWithRecoveryTx(ctx context.Context, arg CreateRecoveryDisputeWithRecoveryTxParams) (CreateRecoveryDisputeWithRecoveryTxResult, error)
	ReviewRecoveryDisputeWithCompensationTx(ctx context.Context, arg ReviewRecoveryDisputeWithCompensationTxParams) (ReviewRecoveryDisputeWithCompensationTxResult, error)
	MarkClaimRecoveryOverdueWithActionTx(ctx context.Context, arg MarkClaimRecoveryOverdueWithActionTxParams) (MarkClaimRecoveryOverdueWithActionTxResult, error)
	// Group multi-store transactions
	ApproveGroupApplicationTx(ctx context.Context, arg ApproveGroupApplicationTxParams) (ApproveGroupApplicationTxResult, error)
	CreateGroupJoinRequestTx(ctx context.Context, arg CreateGroupJoinRequestTxParams) (CreateGroupJoinRequestTxResult, error)
	ApproveGroupJoinRequestTx(ctx context.Context, arg ApproveGroupJoinRequestTxParams) (ApproveGroupJoinRequestTxResult, error)
	RejectGroupJoinRequestTx(ctx context.Context, arg RejectGroupJoinRequestTxParams) (RejectGroupJoinRequestTxResult, error)
	CancelGroupJoinRequestTx(ctx context.Context, arg CancelGroupJoinRequestTxParams) (CancelGroupJoinRequestTxResult, error)
	// Review transactions
	UpdateReviewTx(ctx context.Context, arg UpdateReviewTxParams) (UpdateReviewTxResult, error)
	// Profit sharing config transactions
	CreateProfitSharingConfigTx(ctx context.Context, arg CreateProfitSharingConfigTxParams) (CreateProfitSharingConfigTxResult, error)
	UpdateProfitSharingConfigTx(ctx context.Context, arg UpdateProfitSharingConfigTxParams) (UpdateProfitSharingConfigTxResult, error)
	UpdateProfitSharingConfigStatusTx(ctx context.Context, arg UpdateProfitSharingConfigStatusTxParams) (UpdateProfitSharingConfigStatusTxResult, error)
	// Session transactions
	RefreshSessionTx(ctx context.Context, arg RefreshSessionTxParams) (RefreshSessionTxResult, error)
	// Merchant app device transactions
	RegisterMerchantAppDeviceTx(ctx context.Context, arg RegisterMerchantAppDeviceParams) (MerchantAppDevice, error)
	UpdateMerchantAppDeviceHeartbeatTx(ctx context.Context, arg UpdateMerchantAppDeviceHeartbeatParams) (MerchantAppDevice, error)
	// Cloud printer authorization transactions
	AuthorizeYilianyunCloudPrinterTx(ctx context.Context, arg AuthorizeYilianyunCloudPrinterTxParams) (AuthorizeYilianyunCloudPrinterTxResult, error)
	AuthorizeYilianyunCloudPrinterWithDeviceTx(ctx context.Context, arg AuthorizeYilianyunCloudPrinterWithDeviceTxParams) (AuthorizeYilianyunCloudPrinterWithDeviceTxResult, error)
	CreateAuthorizedYilianyunCloudPrinterTx(ctx context.Context, arg CreateAuthorizedYilianyunCloudPrinterTxParams) (CreateAuthorizedYilianyunCloudPrinterTxResult, error)
	// Order replacement transaction
	ReplaceOrderTx(ctx context.Context, arg ReplaceOrderTxParams) (ReplaceOrderTxResult, error)
	ReplaceOrderWithRefundOrdersTx(ctx context.Context, arg ReplaceOrderWithRefundOrdersTxParams) (ReplaceOrderWithRefundOrdersTxResult, error)
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
