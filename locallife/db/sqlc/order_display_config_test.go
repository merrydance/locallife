package db

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestOrderDisplayConfigRejectsAutoAcceptWhenPrintDisabled(t *testing.T) {
	ctx := context.Background()
	merchant := createRandomMerchantForTest(t)
	merchantID := merchant.ID

	_, err := testStore.CreateOrderDisplayConfig(ctx, CreateOrderDisplayConfigParams{
		MerchantID:           merchantID,
		EnablePrint:          false,
		PrintTakeout:         true,
		PrintDineIn:          true,
		PrintReservation:     true,
		PrintDispatchMode:    "single_full",
		PrintTriggerMode:     "accepted",
		AutoAcceptPaidOrders: true,
		EnableVoice:          false,
		VoiceTakeout:         true,
		VoiceDineIn:          true,
		EnableKds:            false,
	})
	requireOrderDisplayConfigPrintAutoAcceptConstraintError(t, err)

	config, err := testStore.CreateOrderDisplayConfig(ctx, CreateOrderDisplayConfigParams{
		MerchantID:           merchantID,
		EnablePrint:          false,
		PrintTakeout:         true,
		PrintDineIn:          true,
		PrintReservation:     true,
		PrintDispatchMode:    "single_full",
		PrintTriggerMode:     "accepted",
		AutoAcceptPaidOrders: false,
		EnableVoice:          false,
		VoiceTakeout:         true,
		VoiceDineIn:          true,
		EnableKds:            false,
	})
	require.NoError(t, err)
	require.False(t, config.EnablePrint)
	require.False(t, config.AutoAcceptPaidOrders)

	_, err = testStore.UpdateOrderDisplayConfig(ctx, UpdateOrderDisplayConfigParams{
		MerchantID:           merchantID,
		AutoAcceptPaidOrders: pgtype.Bool{Bool: true, Valid: true},
	})
	requireOrderDisplayConfigPrintAutoAcceptConstraintError(t, err)
}

func requireOrderDisplayConfigPrintAutoAcceptConstraintError(t *testing.T, err error) {
	t.Helper()

	require.Error(t, err)
	var pgErr *pgconn.PgError
	require.True(t, errors.As(err, &pgErr), "expected PostgreSQL error, got %T: %v", err, err)
	require.Equal(t, "23514", pgErr.Code)
	require.Equal(t, "order_display_configs_print_auto_accept_check", pgErr.ConstraintName)
}
