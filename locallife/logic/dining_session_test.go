package logic

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestOpenDiningSessionSkipsDefaultBillingMemberForOfflineReservationOperator(t *testing.T) {
	now := time.Date(2026, 2, 12, 12, 0, 0, 0, time.UTC)
	dateOnly := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	operatorID := int64(10)
	merchantID := int64(20)
	tableID := int64(30)
	reservationID := int64(40)
	sessionID := int64(50)
	billingGroupID := int64(60)

	table := db.Table{ID: tableID, MerchantID: merchantID, Status: db.TableStatusReserved}
	merchant := db.Merchant{ID: merchantID, OwnerUserID: operatorID}
	reservation := db.TableReservation{
		ID:                reservationID,
		TableID:           tableID,
		UserID:            operatorID,
		MerchantID:        merchantID,
		Status:            db.ReservationStatusConfirmed,
		ReservationDate:   pgtype.Date{Time: dateOnly, Valid: true},
		ReservationTime:   reservationTimeFrom(now),
		Source:            pgtype.Text{String: db.ReservationSourcePhone, Valid: true},
		OfflineCustomerID: pgtype.Int8{Int64: 70, Valid: true},
		CreatedByUserID:   pgtype.Int8{Int64: operatorID, Valid: true},
	}
	session := db.DiningSession{
		ID:            sessionID,
		MerchantID:    merchantID,
		TableID:       tableID,
		ReservationID: pgtype.Int8{Int64: reservationID, Valid: true},
		UserID:        operatorID,
		Status:        "open",
	}
	billingGroup := db.BillingGroup{
		ID:              billingGroupID,
		DiningSessionID: sessionID,
		Status:          "open",
		IsDefault:       true,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetTable(gomock.Any(), tableID).
		Times(1).
		Return(table, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchantID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		ListReservationsByTableAndDate(gomock.Any(), gomock.Any()).
		Times(1).
		Return([]db.TableReservation{reservation}, nil)
	store.EXPECT().
		GetTableReservation(gomock.Any(), reservationID).
		Times(1).
		Return(reservation, nil)
	store.EXPECT().
		GetActiveDiningSessionByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).
		Times(1).
		Return(db.DiningSession{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetActiveDiningSessionByTable(gomock.Any(), tableID).
		Times(1).
		Return(db.DiningSession{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetLatestOrderByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).
		Times(1).
		Return(db.Order{}, db.ErrRecordNotFound)
	store.EXPECT().
		OpenDiningSessionTx(gomock.Any(), db.OpenDiningSessionTxParams{
			TableID:                       tableID,
			MerchantID:                    merchantID,
			UserID:                        operatorID,
			ReservationID:                 pgtype.Int8{Int64: reservationID, Valid: true},
			ImportReservationItems:        true,
			SkipDefaultBillingGroupMember: true,
		}).
		Times(1).
		Return(db.OpenDiningSessionTxResult{
			Session:      session,
			BillingGroup: billingGroup,
		}, nil)

	result, err := OpenDiningSession(context.Background(), store, OpenDiningSessionInput{
		UserID:        operatorID,
		TableID:       tableID,
		ReservationID: &reservationID,
		Now:           now,
	})

	require.NoError(t, err)
	require.Equal(t, session.ID, result.Session.ID)
	require.Equal(t, billingGroup.ID, result.BillingGroup.ID)
}

func TestGetOrCreateDefaultBillingGroupSkipsMembershipForOfflineReservationOperator(t *testing.T) {
	operatorID := int64(10)
	session := db.DiningSession{
		ID:            50,
		MerchantID:    20,
		TableID:       30,
		ReservationID: pgtype.Int8{Int64: 40, Valid: true},
		UserID:        operatorID,
		Status:        "open",
	}
	reservation := db.TableReservation{
		ID:                session.ReservationID.Int64,
		TableID:           session.TableID,
		UserID:            operatorID,
		MerchantID:        session.MerchantID,
		Status:            db.ReservationStatusCheckedIn,
		Source:            pgtype.Text{String: db.ReservationSourceMerchant, Valid: true},
		OfflineCustomerID: pgtype.Int8{Int64: 70, Valid: true},
		CreatedByUserID:   pgtype.Int8{Valid: false},
	}
	billingGroup := db.BillingGroup{
		ID:              60,
		DiningSessionID: session.ID,
		Status:          "open",
		IsDefault:       true,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetDefaultBillingGroupBySession(gomock.Any(), session.ID).
		Times(1).
		Return(billingGroup, nil)
	store.EXPECT().
		GetTableReservation(gomock.Any(), session.ReservationID.Int64).
		Times(1).
		Return(reservation, nil)

	result, err := GetOrCreateDefaultBillingGroup(context.Background(), store, session, operatorID)

	require.NoError(t, err)
	require.Equal(t, billingGroup.ID, result.ID)
}

func TestGetOrCreateDefaultBillingGroupSkipsMembershipForOfflineReservationMerchantOwner(t *testing.T) {
	merchantOwnerID := int64(10)
	session := db.DiningSession{
		ID:            50,
		MerchantID:    20,
		TableID:       30,
		ReservationID: pgtype.Int8{Int64: 40, Valid: true},
		UserID:        merchantOwnerID + 9,
		Status:        "open",
	}
	reservation := db.TableReservation{
		ID:                session.ReservationID.Int64,
		TableID:           session.TableID,
		UserID:            merchantOwnerID + 9,
		MerchantID:        session.MerchantID,
		Status:            db.ReservationStatusCheckedIn,
		Source:            pgtype.Text{String: db.ReservationSourceMerchant, Valid: true},
		OfflineCustomerID: pgtype.Int8{Int64: 70, Valid: true},
		CreatedByUserID:   pgtype.Int8{Int64: merchantOwnerID + 9, Valid: true},
	}
	billingGroup := db.BillingGroup{
		ID:              60,
		DiningSessionID: session.ID,
		Status:          "open",
		IsDefault:       true,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetDefaultBillingGroupBySession(gomock.Any(), session.ID).
		Times(1).
		Return(billingGroup, nil)
	store.EXPECT().
		GetTableReservation(gomock.Any(), session.ReservationID.Int64).
		Times(1).
		Return(reservation, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), session.MerchantID).
		Times(1).
		Return(db.Merchant{ID: session.MerchantID, OwnerUserID: merchantOwnerID}, nil)

	result, err := GetOrCreateDefaultBillingGroup(context.Background(), store, session, merchantOwnerID)

	require.NoError(t, err)
	require.Equal(t, billingGroup.ID, result.ID)
}

func TestResolveDiningSessionMenuRejectsLegacyOfflineReservationUser(t *testing.T) {
	operatorID := int64(10)
	session := db.DiningSession{
		ID:            50,
		MerchantID:    20,
		TableID:       30,
		ReservationID: pgtype.Int8{Int64: 40, Valid: true},
		UserID:        operatorID + 9,
		Status:        "open",
	}
	reservation := db.TableReservation{
		ID:                session.ReservationID.Int64,
		TableID:           session.TableID,
		UserID:            operatorID,
		MerchantID:        session.MerchantID,
		Status:            db.ReservationStatusCheckedIn,
		Source:            pgtype.Text{String: db.ReservationSourceMerchant, Valid: true},
		OfflineCustomerID: pgtype.Int8{Int64: 70, Valid: true},
		CreatedByUserID:   pgtype.Int8{Valid: false},
	}
	billingGroup := db.BillingGroup{
		ID:              60,
		DiningSessionID: session.ID,
		Status:          "open",
		IsDefault:       true,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetDiningSession(gomock.Any(), session.ID).
		Times(1).
		Return(session, nil)
	store.EXPECT().
		GetTableReservation(gomock.Any(), session.ReservationID.Int64).
		Times(1).
		Return(reservation, nil)
	store.EXPECT().
		GetDefaultBillingGroupBySession(gomock.Any(), session.ID).
		Times(1).
		Return(billingGroup, nil)

	_, err := ResolveDiningSessionMenu(context.Background(), store, session.ID, operatorID)

	reqErr := assertRequestError(t, err)
	require.Equal(t, 403, reqErr.Status)
	require.Equal(t, "dining session does not belong to you", reqErr.Err.Error())
}

func TestResolveDiningSessionMenuRejectsOfflineReservationMerchantOwner(t *testing.T) {
	merchantOwnerID := int64(10)
	session := db.DiningSession{
		ID:            50,
		MerchantID:    20,
		TableID:       30,
		ReservationID: pgtype.Int8{Int64: 40, Valid: true},
		UserID:        merchantOwnerID + 9,
		Status:        "open",
	}
	reservation := db.TableReservation{
		ID:                session.ReservationID.Int64,
		TableID:           session.TableID,
		UserID:            merchantOwnerID + 9,
		MerchantID:        session.MerchantID,
		Status:            db.ReservationStatusCheckedIn,
		Source:            pgtype.Text{String: db.ReservationSourceMerchant, Valid: true},
		OfflineCustomerID: pgtype.Int8{Int64: 70, Valid: true},
		CreatedByUserID:   pgtype.Int8{Int64: merchantOwnerID + 9, Valid: true},
	}
	billingGroup := db.BillingGroup{
		ID:              60,
		DiningSessionID: session.ID,
		Status:          "open",
		IsDefault:       true,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetDiningSession(gomock.Any(), session.ID).
		Times(1).
		Return(session, nil)
	store.EXPECT().
		GetTableReservation(gomock.Any(), session.ReservationID.Int64).
		Times(1).
		Return(reservation, nil)
	store.EXPECT().
		GetDefaultBillingGroupBySession(gomock.Any(), session.ID).
		Times(1).
		Return(billingGroup, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), session.MerchantID).
		Times(1).
		Return(db.Merchant{ID: session.MerchantID, OwnerUserID: merchantOwnerID}, nil)

	_, err := ResolveDiningSessionMenu(context.Background(), store, session.ID, merchantOwnerID)

	reqErr := assertRequestError(t, err)
	require.Equal(t, 403, reqErr.Status)
	require.Equal(t, "dining session does not belong to you", reqErr.Err.Error())
}

func TestResolveDiningSessionMenuAllowsOnlineReservationMember(t *testing.T) {
	memberID := int64(10)
	session := db.DiningSession{
		ID:            50,
		MerchantID:    20,
		TableID:       30,
		ReservationID: pgtype.Int8{Int64: 40, Valid: true},
		UserID:        memberID + 9,
		Status:        "open",
	}
	reservation := db.TableReservation{
		ID:              session.ReservationID.Int64,
		TableID:         session.TableID,
		UserID:          memberID + 100,
		MerchantID:      session.MerchantID,
		Status:          db.ReservationStatusCheckedIn,
		Source:          pgtype.Text{String: db.ReservationSourceOnline, Valid: true},
		ReservationDate: pgtype.Date{Valid: true},
	}
	billingGroup := db.BillingGroup{
		ID:              60,
		DiningSessionID: session.ID,
		Status:          "open",
		IsDefault:       true,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetDiningSession(gomock.Any(), session.ID).
		Times(1).
		Return(session, nil)
	store.EXPECT().
		GetTableReservation(gomock.Any(), session.ReservationID.Int64).
		Times(1).
		Return(reservation, nil)
	store.EXPECT().
		GetDefaultBillingGroupBySession(gomock.Any(), session.ID).
		Times(1).
		Return(billingGroup, nil)
	store.EXPECT().
		GetActiveBillingGroupMember(gomock.Any(), db.GetActiveBillingGroupMemberParams{
			BillingGroupID: billingGroup.ID,
			UserID:         memberID,
		}).
		Times(1).
		Return(db.BillingGroupMember{BillingGroupID: billingGroup.ID, UserID: memberID, Role: "member"}, nil)
	store.EXPECT().
		GetTable(gomock.Any(), session.TableID).
		Times(1).
		Return(db.Table{ID: session.TableID, MerchantID: session.MerchantID}, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), session.MerchantID).
		Times(1).
		Return(db.Merchant{ID: session.MerchantID, OwnerUserID: memberID + 200}, nil)

	result, err := ResolveDiningSessionMenu(context.Background(), store, session.ID, memberID)

	require.NoError(t, err)
	require.Equal(t, billingGroup.ID, result.BillingGroup.ID)
}
