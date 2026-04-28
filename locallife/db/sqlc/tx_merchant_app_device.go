package db

import (
	"context"
	"fmt"
)

// RegisterMerchantAppDeviceTx registers or updates an active merchant app push device.
//
// It deactivates any other active device row using the same push token before
// upserting the current device, keeping the active-token uniqueness invariant in
// one transaction.
func (store *SQLStore) RegisterMerchantAppDeviceTx(ctx context.Context, arg RegisterMerchantAppDeviceParams) (MerchantAppDevice, error) {
	var result MerchantAppDevice

	err := store.execTx(ctx, func(q *Queries) error {
		if err := q.DeactivateMerchantAppDevicesByPushToken(ctx, DeactivateMerchantAppDevicesByPushTokenParams{
			PushToken: arg.PushToken,
			DeviceID:  arg.DeviceID,
		}); err != nil {
			return fmt.Errorf("deactivate previous merchant app devices by push token: %w", err)
		}

		device, err := q.RegisterMerchantAppDevice(ctx, arg)
		if err != nil {
			return fmt.Errorf("register merchant app device: %w", err)
		}

		result = device
		return nil
	})

	return result, err
}

// UpdateMerchantAppDeviceHeartbeatTx updates an active merchant app device heartbeat.
//
// When a fresh push token is provided, any other active device row using that
// token is deactivated inside the same transaction before the heartbeat update.
func (store *SQLStore) UpdateMerchantAppDeviceHeartbeatTx(ctx context.Context, arg UpdateMerchantAppDeviceHeartbeatParams) (MerchantAppDevice, error) {
	var result MerchantAppDevice

	err := store.execTx(ctx, func(q *Queries) error {
		current, err := q.GetActiveMerchantAppDevice(ctx, GetActiveMerchantAppDeviceParams{
			MerchantID: arg.MerchantID,
			DeviceID:   arg.DeviceID,
		})
		if err != nil {
			return fmt.Errorf("get active merchant app device: %w", err)
		}

		if arg.PushToken.Valid && arg.PushToken.String != current.PushToken {
			if err := q.DeactivateMerchantAppDevicesByPushToken(ctx, DeactivateMerchantAppDevicesByPushTokenParams{
				PushToken: arg.PushToken.String,
				DeviceID:  arg.DeviceID,
			}); err != nil {
				return fmt.Errorf("deactivate previous merchant app devices by push token: %w", err)
			}
		}

		device, err := q.UpdateMerchantAppDeviceHeartbeat(ctx, arg)
		if err != nil {
			return fmt.Errorf("update merchant app device heartbeat: %w", err)
		}

		result = device
		return nil
	})

	return result, err
}
