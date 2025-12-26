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
	// Dish transactions
	CreateDishTx(ctx context.Context, arg CreateDishTxParams) (CreateDishTxResult, error)
	UpdateDishTx(ctx context.Context, arg UpdateDishTxParams) (UpdateDishTxResult, error)
	SetDishCustomizationsTx(ctx context.Context, arg SetDishCustomizationsTxParams) (SetDishCustomizationsTxResult, error)
	// Merchant transactions
	SetBusinessHoursTx(ctx context.Context, arg SetBusinessHoursTxParams) (SetBusinessHoursTxResult, error)
	// Merchant application approval transaction
	ApproveMerchantApplicationTx(ctx context.Context, arg ApproveMerchantApplicationTxParams) (ApproveMerchantApplicationTxResult, error)
	ResetMerchantApplicationTx(ctx context.Context, arg ResetMerchantApplicationTxParams) (ResetMerchantApplicationTxResult, error)
	// Combo transactions
	CreateComboSetTx(ctx context.Context, arg CreateComboSetTxParams) (CreateComboSetTxResult, error)
	CreateOrderTx(ctx context.Context, arg CreateOrderTxParams) (CreateOrderTxResult, error)
	ProcessOrderPaymentTx(ctx context.Context, arg ProcessOrderPaymentTxParams) (ProcessOrderPaymentTxResult, error)
	// M10: Membership transactions
	JoinMembershipTx(ctx context.Context, arg JoinMembershipTxParams) (JoinMembershipTxResult, error)
	RechargeTx(ctx context.Context, arg RechargeTxParams) (RechargeTxResult, error)
	ConsumeTx(ctx context.Context, arg ConsumeTxParams) (ConsumeTxResult, error)
	RefundTx(ctx context.Context, arg RefundTxParams) (RefundTxResult, error)
	// M10: Voucher transactions
	ClaimVoucherTx(ctx context.Context, arg ClaimVoucherTxParams) (ClaimVoucherTxResult, error)
	UseVoucherTx(ctx context.Context, arg UseVoucherTxParams) (UseVoucherTxResult, error)
	// M15: Delivery transactions
	GrabOrderTx(ctx context.Context, arg GrabOrderTxParams) (GrabOrderTxResult, error)
	CompleteDeliveryTx(ctx context.Context, arg CompleteDeliveryTxParams) (CompleteDeliveryTxResult, error)
	// M15: Rider transactions
	WithdrawDepositTx(ctx context.Context, arg WithdrawDepositTxParams) (WithdrawDepositTxResult, error)
	RollbackWithdrawTx(ctx context.Context, arg RollbackWithdrawTxParams) error
	DeductRiderDepositTx(ctx context.Context, arg DeductRiderDepositTxParams) (DeductRiderDepositTxResult, error)
	// M15: Reservation transactions
	CancelReservationTx(ctx context.Context, arg CancelReservationTxParams) (CancelReservationTxResult, error)
	MarkNoShowTx(ctx context.Context, arg MarkNoShowTxParams) (MarkNoShowTxResult, error)
	ConfirmReservationTx(ctx context.Context, arg ConfirmReservationTxParams) (ConfirmReservationTxResult, error)
	CompleteReservationTx(ctx context.Context, arg CompleteReservationTxParams) (CompleteReservationTxResult, error)
	CreateReservationTx(ctx context.Context, arg CreateReservationTxParams) (CreateReservationTxResult, error)
	// M15: Order status transactions
	UpdateOrderStatusTx(ctx context.Context, arg UpdateOrderStatusTxParams) (UpdateOrderStatusTxResult, error)
	CompleteOrderTx(ctx context.Context, arg CompleteOrderTxParams) (CompleteOrderTxResult, error)
	CancelOrderTx(ctx context.Context, arg CancelOrderTxParams) (CancelOrderTxResult, error)
	// M5: Table transactions
	DeleteTableTx(ctx context.Context, arg DeleteTableParams) (DeleteTableResult, error)
	// Claim refund transactions（索赔退款）
	ClaimRefundTx(ctx context.Context, arg ClaimRefundTxParams) (ClaimRefundTxResult, error)
	DeductRiderDepositAndRefundTx(ctx context.Context, arg DeductRiderDepositAndRefundTxParams) (DeductRiderDepositAndRefundTxResult, error)
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
