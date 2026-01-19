package db

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func createOpenDiningSession(t *testing.T, merchantID, tableID, userID int64, reservationID pgtype.Int8) DiningSession {
	session, err := testStore.CreateDiningSession(context.Background(), CreateDiningSessionParams{
		MerchantID:    merchantID,
		TableID:       tableID,
		ReservationID: reservationID,
		UserID:        userID,
		ActiveOrderID: pgtype.Int8{Valid: false},
		Status:        "open",
	})
	require.NoError(t, err)
	return session
}

func ensureTableTransferLogsTable(t *testing.T) {
	_, err := testStore.(*SQLStore).connPool.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS table_transfer_logs (
			id BIGSERIAL PRIMARY KEY,
			merchant_id BIGINT NOT NULL REFERENCES merchants(id),
			dining_session_id BIGINT NOT NULL REFERENCES dining_sessions(id),
			reservation_id BIGINT REFERENCES table_reservations(id),
			from_table_id BIGINT NOT NULL REFERENCES tables(id),
			to_table_id BIGINT NOT NULL REFERENCES tables(id),
			operator_user_id BIGINT NOT NULL REFERENCES users(id),
			reason TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`)
	require.NoError(t, err)
}

func setTableStatus(t *testing.T, tableID int64, status string, reservationID pgtype.Int8) Table {
	table, err := testStore.UpdateTableStatus(context.Background(), UpdateTableStatusParams{
		ID:                   tableID,
		Status:               status,
		CurrentReservationID: reservationID,
	})
	require.NoError(t, err)
	return table
}

func TestTransferDiningSessionTableTx_NoReservationSuccess(t *testing.T) {
	ensureTableTransferLogsTable(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	fromTable := createRandomTable(t, merchant.ID)
	toTable := createRandomTable(t, merchant.ID)
	setTableStatus(t, fromTable.ID, "occupied", pgtype.Int8{})

	session := createOpenDiningSession(t, merchant.ID, fromTable.ID, user.ID, pgtype.Int8{Valid: false})

	result, err := testStore.TransferDiningSessionTableTx(context.Background(), TransferDiningSessionTableTxParams{
		SessionID:      session.ID,
		ToTableID:      toTable.ID,
		OperatorUserID: user.ID,
		Reason:         pgtype.Text{String: "test transfer", Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, toTable.ID, result.Session.TableID)
	require.Equal(t, "available", result.FromTable.Status)
	require.Equal(t, "occupied", result.ToTable.Status)
}

func TestTransferDiningSessionTableTx_WithReservationSuccess(t *testing.T) {
	ensureTableTransferLogsTable(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	fromTable := createRandomTable(t, merchant.ID)
	toTable := createRandomTable(t, merchant.ID)

	reservation := createRandomReservation(t, user.ID, merchant.ID, fromTable.ID, "confirmed")
	setTableStatus(t, fromTable.ID, "occupied", pgtype.Int8{Int64: reservation.ID, Valid: true})
	setTableStatus(t, toTable.ID, "reserved", pgtype.Int8{Int64: reservation.ID, Valid: true})

	session := createOpenDiningSession(t, merchant.ID, fromTable.ID, user.ID, pgtype.Int8{Int64: reservation.ID, Valid: true})

	result, err := testStore.TransferDiningSessionTableTx(context.Background(), TransferDiningSessionTableTxParams{
		SessionID:      session.ID,
		ToTableID:      toTable.ID,
		OperatorUserID: user.ID,
		Reason:         pgtype.Text{String: "reservation transfer", Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, toTable.ID, result.Session.TableID)

	updatedReservation, err := testStore.GetTableReservation(context.Background(), reservation.ID)
	require.NoError(t, err)
	require.Equal(t, toTable.ID, updatedReservation.TableID)
}

func TestTransferDiningSessionTableTx_UpdatesOrderTable(t *testing.T) {
	ensureTableTransferLogsTable(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	fromTable := createRandomTable(t, merchant.ID)
	toTable := createRandomTable(t, merchant.ID)

	session := createOpenDiningSession(t, merchant.ID, fromTable.ID, user.ID, pgtype.Int8{Valid: false})
	setTableStatus(t, fromTable.ID, "occupied", pgtype.Int8{})

	billingGroup, err := testStore.CreateBillingGroup(context.Background(), CreateBillingGroupParams{
		DiningSessionID: session.ID,
		Status:          "open",
		IsDefault:       true,
		TotalAmount:     0,
		PaidAmount:      0,
	})
	require.NoError(t, err)

	order := createRandomOrderWithUserAndMerchant(t, user.ID, merchant.ID)
	_, err = testStore.(*SQLStore).connPool.Exec(context.Background(), `UPDATE orders SET table_id = $1 WHERE id = $2`, fromTable.ID, order.ID)
	require.NoError(t, err)

	_, err = testStore.CreateBillingGroupOrder(context.Background(), CreateBillingGroupOrderParams{
		BillingGroupID: billingGroup.ID,
		OrderID:        order.ID,
		Amount:         1000,
		Status:         "linked",
	})
	require.NoError(t, err)

	_, err = testStore.TransferDiningSessionTableTx(context.Background(), TransferDiningSessionTableTxParams{
		SessionID:      session.ID,
		ToTableID:      toTable.ID,
		OperatorUserID: user.ID,
		Reason:         pgtype.Text{String: "order sync", Valid: true},
	})
	require.NoError(t, err)

	updatedOrder, err := testStore.GetOrder(context.Background(), order.ID)
	require.NoError(t, err)
	require.True(t, updatedOrder.TableID.Valid)
	require.Equal(t, toTable.ID, updatedOrder.TableID.Int64)
}

func TestTransferDiningSessionTableTx_TargetTableDisabled(t *testing.T) {
	ensureTableTransferLogsTable(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	fromTable := createRandomTable(t, merchant.ID)
	toTable := createRandomTable(t, merchant.ID)
	setTableStatus(t, toTable.ID, "disabled", pgtype.Int8{})

	session := createOpenDiningSession(t, merchant.ID, fromTable.ID, user.ID, pgtype.Int8{Valid: false})

	_, err := testStore.TransferDiningSessionTableTx(context.Background(), TransferDiningSessionTableTxParams{
		SessionID:      session.ID,
		ToTableID:      toTable.ID,
		OperatorUserID: user.ID,
		Reason:         pgtype.Text{String: "disabled target", Valid: true},
	})
	require.ErrorIs(t, err, ErrTargetTableDisabled)
}

func TestTransferDiningSessionTableTx_TargetTableReservedWithoutReservation(t *testing.T) {
	ensureTableTransferLogsTable(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	fromTable := createRandomTable(t, merchant.ID)
	toTable := createRandomTable(t, merchant.ID)

	otherReservation := createRandomReservation(t, user.ID, merchant.ID, toTable.ID, "confirmed")
	setTableStatus(t, toTable.ID, "reserved", pgtype.Int8{Int64: otherReservation.ID, Valid: true})

	session := createOpenDiningSession(t, merchant.ID, fromTable.ID, user.ID, pgtype.Int8{Valid: false})

	_, err := testStore.TransferDiningSessionTableTx(context.Background(), TransferDiningSessionTableTxParams{
		SessionID:      session.ID,
		ToTableID:      toTable.ID,
		OperatorUserID: user.ID,
		Reason:         pgtype.Text{String: "reserved target", Valid: true},
	})
	require.ErrorIs(t, err, ErrTargetTableReserved)
}

func TestTransferDiningSessionTableTx_TargetTableOccupied(t *testing.T) {
	ensureTableTransferLogsTable(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	fromTable := createRandomTable(t, merchant.ID)
	toTable := createRandomTable(t, merchant.ID)

	session := createOpenDiningSession(t, merchant.ID, fromTable.ID, user.ID, pgtype.Int8{Valid: false})
	_ = session
	createOpenDiningSession(t, merchant.ID, toTable.ID, user.ID, pgtype.Int8{Valid: false})

	_, err := testStore.TransferDiningSessionTableTx(context.Background(), TransferDiningSessionTableTxParams{
		SessionID:      session.ID,
		ToTableID:      toTable.ID,
		OperatorUserID: user.ID,
		Reason:         pgtype.Text{String: "occupied target", Valid: true},
	})
	require.ErrorIs(t, err, ErrTargetTableOccupied)
}

func TestTransferDiningSessionTableTx_SessionNotOpen(t *testing.T) {
	ensureTableTransferLogsTable(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	fromTable := createRandomTable(t, merchant.ID)
	toTable := createRandomTable(t, merchant.ID)

	session, err := testStore.CreateDiningSession(context.Background(), CreateDiningSessionParams{
		MerchantID:    merchant.ID,
		TableID:       fromTable.ID,
		ReservationID: pgtype.Int8{Valid: false},
		UserID:        user.ID,
		ActiveOrderID: pgtype.Int8{Valid: false},
		Status:        "closed",
	})
	require.NoError(t, err)

	_, err = testStore.TransferDiningSessionTableTx(context.Background(), TransferDiningSessionTableTxParams{
		SessionID:      session.ID,
		ToTableID:      toTable.ID,
		OperatorUserID: user.ID,
		Reason:         pgtype.Text{String: "closed session", Valid: true},
	})
	require.ErrorIs(t, err, ErrDiningSessionNotOpen)
}

func TestTransferDiningSessionTableTx_ConcurrentSameTarget(t *testing.T) {
	ensureTableTransferLogsTable(t)
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	user := createRandomUser(t)

	fromTable := createRandomTable(t, merchant.ID)
	toTable := createRandomTable(t, merchant.ID)
	setTableStatus(t, fromTable.ID, "occupied", pgtype.Int8{})

	session := createOpenDiningSession(t, merchant.ID, fromTable.ID, user.ID, pgtype.Int8{Valid: false})

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := testStore.TransferDiningSessionTableTx(context.Background(), TransferDiningSessionTableTxParams{
				SessionID:      session.ID,
				ToTableID:      toTable.ID,
				OperatorUserID: user.ID,
				Reason:         pgtype.Text{String: "concurrent", Valid: true},
			})
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	successCount := 0
	sameTargetCount := 0
	for _, err := range errs {
		if err == nil {
			successCount++
			continue
		}
		if errors.Is(err, ErrDiningSessionTableSame) {
			sameTargetCount++
		}
	}
	require.Equal(t, 1, successCount)
	require.Equal(t, 1, sameTargetCount)
}
