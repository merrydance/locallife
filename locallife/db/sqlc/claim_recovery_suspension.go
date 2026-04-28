package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

type claimRecoverySuspensionStore interface {
	GetOrder(ctx context.Context, id int64) (Order, error)
	GetDeliveryByOrderID(ctx context.Context, orderID int64) (Delivery, error)
	HasBlockingClaimRecoveryForMerchant(ctx context.Context, merchantID int64) (bool, error)
	HasBlockingClaimRecoveryForRider(ctx context.Context, riderID pgtype.Int8) (bool, error)
	UnsuspendMerchantTakeout(ctx context.Context, merchantID int64) error
	UnsuspendRider(ctx context.Context, riderID int64) error
}

// ReleaseClaimRecoverySuspensionIfClear only clears service suspension when no blocking recovery remains.
func ReleaseClaimRecoverySuspensionIfClear(ctx context.Context, store claimRecoverySuspensionStore, recovery ClaimRecovery) error {
	if !recovery.RecoveryTarget.Valid {
		return nil
	}

	switch recovery.RecoveryTarget.String {
	case "merchant":
		order, err := store.GetOrder(ctx, recovery.OrderID)
		if err != nil {
			return err
		}
		hasBlocking, err := store.HasBlockingClaimRecoveryForMerchant(ctx, order.MerchantID)
		if err != nil {
			return err
		}
		if hasBlocking {
			return nil
		}
		return store.UnsuspendMerchantTakeout(ctx, order.MerchantID)
	case "rider":
		delivery, err := store.GetDeliveryByOrderID(ctx, recovery.OrderID)
		if err != nil {
			return err
		}
		if !delivery.RiderID.Valid {
			return nil
		}
		hasBlocking, err := store.HasBlockingClaimRecoveryForRider(ctx, delivery.RiderID)
		if err != nil {
			return err
		}
		if hasBlocking {
			return nil
		}
		return store.UnsuspendRider(ctx, delivery.RiderID.Int64)
	default:
		return nil
	}
}
