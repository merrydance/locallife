package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestValidateOrderSessionAndBilling_Reservation(t *testing.T) {
	userID := int64(10)
	merchantID := int64(20)
	reservationID := int64(30)
	tableID := int64(40)
	billingGroupID := int64(50)

	baseReservation := db.TableReservation{
		ID:          reservationID,
		UserID:      userID,
		MerchantID:  merchantID,
		TableID:     tableID,
		PaymentMode: paymentModeFull,
		Status:      "paid",
	}

	testCases := []struct {
		name       string
		input      OrderSessionInput
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, result OrderSessionResult, err error)
	}{
		{
			name:  "MissingReservationID",
			input: OrderSessionInput{UserID: userID, MerchantID: merchantID, OrderType: "reservation"},
			check: func(t *testing.T, _ OrderSessionResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "reservation_id is required", reqErr.Err.Error())
			},
		},
		{
			name:  "DepositReservationPendingRequiresPayment",
			input: OrderSessionInput{UserID: userID, MerchantID: merchantID, OrderType: "reservation", ReservationID: &reservationID},
			buildStubs: func(store *mockdb.MockStore) {
				reservation := baseReservation
				reservation.PaymentMode = paymentModeDeposit
				reservation.Status = reservationStatusPending
				store.EXPECT().
					GetTableReservation(gomock.Any(), reservationID).
					Times(1).
					Return(reservation, nil)
			},
			check: func(t *testing.T, _ OrderSessionResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 409, reqErr.Status)
				require.Equal(t, "reservation deposit is not paid", reqErr.Err.Error())
			},
		},
		{
			name:  "ReservationNotFound",
			input: OrderSessionInput{UserID: userID, MerchantID: merchantID, OrderType: "reservation", ReservationID: &reservationID},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTableReservation(gomock.Any(), reservationID).
					Times(1).
					Return(db.TableReservation{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, _ OrderSessionResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 404, reqErr.Status)
				require.Equal(t, "reservation not found", reqErr.Err.Error())
			},
		},
		{
			name:  "ReservationMismatchUser",
			input: OrderSessionInput{UserID: userID + 1, MerchantID: merchantID, OrderType: "reservation", ReservationID: &reservationID},
			buildStubs: func(store *mockdb.MockStore) {
				reservation := baseReservation
				store.EXPECT().
					GetTableReservation(gomock.Any(), reservationID).
					Times(1).
					Return(reservation, nil)
			},
			check: func(t *testing.T, _ OrderSessionResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 403, reqErr.Status)
				require.Equal(t, "reservation does not belong to you", reqErr.Err.Error())
			},
		},
		{
			name:  "ReservationInvalidStatus",
			input: OrderSessionInput{UserID: userID, MerchantID: merchantID, OrderType: "reservation", ReservationID: &reservationID},
			buildStubs: func(store *mockdb.MockStore) {
				reservation := baseReservation
				reservation.Status = "cancelled"
				store.EXPECT().
					GetTableReservation(gomock.Any(), reservationID).
					Times(1).
					Return(reservation, nil)
			},
			check: func(t *testing.T, _ OrderSessionResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 409, reqErr.Status)
				require.Equal(t, "reservation is in an invalid state", reqErr.Err.Error())
			},
		},
		{
			name:  "TableMismatch",
			input: OrderSessionInput{UserID: userID, MerchantID: merchantID, OrderType: "reservation", ReservationID: &reservationID, TableID: &tableID},
			buildStubs: func(store *mockdb.MockStore) {
				reservation := baseReservation
				reservation.TableID = tableID + 1
				store.EXPECT().
					GetTableReservation(gomock.Any(), reservationID).
					Times(1).
					Return(reservation, nil)
			},
			check: func(t *testing.T, _ OrderSessionResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "table does not match reservation", reqErr.Err.Error())
			},
		},
		{
			name:  "DepositReservationPaidAllowed",
			input: OrderSessionInput{UserID: userID, MerchantID: merchantID, OrderType: "reservation", ReservationID: &reservationID},
			buildStubs: func(store *mockdb.MockStore) {
				reservation := baseReservation
				reservation.PaymentMode = paymentModeDeposit
				store.EXPECT().
					GetTableReservation(gomock.Any(), reservationID).
					Times(1).
					Return(reservation, nil)
				store.EXPECT().
					GetActiveDiningSessionByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).
					Times(1).
					Return(db.DiningSession{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, result OrderSessionResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result.Reservation)
				require.Equal(t, paymentModeDeposit, result.Reservation.PaymentMode)
				require.Nil(t, result.DiningSession)
			},
		},
		{
			name:  "SessionTableMismatch",
			input: OrderSessionInput{UserID: userID, MerchantID: merchantID, OrderType: "reservation", ReservationID: &reservationID},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTableReservation(gomock.Any(), reservationID).
					Times(1).
					Return(baseReservation, nil)
				session := db.DiningSession{ID: 70, MerchantID: merchantID, TableID: tableID + 1}
				store.EXPECT().
					GetActiveDiningSessionByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).
					Times(1).
					Return(session, nil)
			},
			check: func(t *testing.T, _ OrderSessionResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 409, reqErr.Status)
				require.Equal(t, "dining session table mismatch", reqErr.Err.Error())
			},
		},
		{
			name: "BillingGroupWithoutSession",
			input: OrderSessionInput{
				UserID: userID, MerchantID: merchantID, OrderType: "reservation", ReservationID: &reservationID, BillingGroupID: &billingGroupID,
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTableReservation(gomock.Any(), reservationID).
					Times(1).
					Return(baseReservation, nil)
				store.EXPECT().
					GetActiveDiningSessionByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).
					Times(1).
					Return(db.DiningSession{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, _ OrderSessionResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "billing group requires active session", reqErr.Err.Error())
			},
		},
		{
			name:  "SuccessWithDefaultBillingGroup",
			input: OrderSessionInput{UserID: userID, MerchantID: merchantID, OrderType: "reservation", ReservationID: &reservationID},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTableReservation(gomock.Any(), reservationID).
					Times(1).
					Return(baseReservation, nil)
				session := db.DiningSession{ID: 88, MerchantID: merchantID, TableID: tableID}
				store.EXPECT().
					GetActiveDiningSessionByReservation(gomock.Any(), pgtype.Int8{Int64: reservationID, Valid: true}).
					Times(1).
					Return(session, nil)
				bg := db.BillingGroup{ID: 99, DiningSessionID: session.ID, Status: "open"}
				store.EXPECT().
					GetDefaultBillingGroupBySession(gomock.Any(), session.ID).
					Times(1).
					Return(bg, nil)
				store.EXPECT().
					GetActiveBillingGroupMember(gomock.Any(), db.GetActiveBillingGroupMemberParams{
						BillingGroupID: bg.ID,
						UserID:         userID,
					}).
					Times(1).
					Return(db.BillingGroupMember{}, nil)
			},
			check: func(t *testing.T, result OrderSessionResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result.DiningSession)
				require.NotNil(t, result.BillingGroupID)
				require.Equal(t, int64(99), *result.BillingGroupID)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			if tc.buildStubs != nil {
				tc.buildStubs(store)
			}

			result, err := ValidateOrderSessionAndBilling(context.Background(), store, tc.input)
			tc.check(t, result, err)
		})
	}
}

func TestValidateOrderSessionAndBilling_DineIn(t *testing.T) {
	userID := int64(11)
	merchantID := int64(22)
	reservationID := int64(33)
	tableID := int64(44)

	baseSession := db.DiningSession{
		ID:            55,
		MerchantID:    merchantID,
		TableID:       tableID,
		ReservationID: pgtype.Int8{Int64: reservationID, Valid: true},
	}
	baseReservation := db.TableReservation{
		ID:          reservationID,
		UserID:      userID,
		MerchantID:  merchantID,
		TableID:     tableID,
		PaymentMode: "deposit",
		Status:      "paid",
	}

	testCases := []struct {
		name       string
		input      OrderSessionInput
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, result OrderSessionResult, err error)
	}{
		{
			name:  "MissingTableID",
			input: OrderSessionInput{UserID: userID, MerchantID: merchantID, OrderType: "dine_in"},
			check: func(t *testing.T, _ OrderSessionResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "table_id is required", reqErr.Err.Error())
			},
		},
		{
			name:  "SessionNotFound",
			input: OrderSessionInput{UserID: userID, MerchantID: merchantID, OrderType: "dine_in", TableID: &tableID},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetActiveDiningSessionByTable(gomock.Any(), tableID).
					Times(1).
					Return(db.DiningSession{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, _ OrderSessionResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 409, reqErr.Status)
				require.Equal(t, "no active dining session", reqErr.Err.Error())
			},
		},
		{
			name:  "SessionMerchantMismatch",
			input: OrderSessionInput{UserID: userID, MerchantID: merchantID, OrderType: "dine_in", TableID: &tableID},
			buildStubs: func(store *mockdb.MockStore) {
				session := baseSession
				session.MerchantID = merchantID + 1
				store.EXPECT().
					GetActiveDiningSessionByTable(gomock.Any(), tableID).
					Times(1).
					Return(session, nil)
			},
			check: func(t *testing.T, _ OrderSessionResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "dining session merchant mismatch", reqErr.Err.Error())
			},
		},
		{
			name:  "ReservationMismatch",
			input: OrderSessionInput{UserID: userID, MerchantID: merchantID, OrderType: "dine_in", TableID: &tableID, ReservationID: &reservationID},
			buildStubs: func(store *mockdb.MockStore) {
				session := baseSession
				session.ReservationID = pgtype.Int8{Int64: reservationID + 1, Valid: true}
				store.EXPECT().
					GetActiveDiningSessionByTable(gomock.Any(), tableID).
					Times(1).
					Return(session, nil)
			},
			check: func(t *testing.T, _ OrderSessionResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 409, reqErr.Status)
				require.Equal(t, "dining session reservation mismatch", reqErr.Err.Error())
			},
		},
		{
			name:  "ReservationNotDeposit",
			input: OrderSessionInput{UserID: userID, MerchantID: merchantID, OrderType: "dine_in", TableID: &tableID, ReservationID: &reservationID},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetActiveDiningSessionByTable(gomock.Any(), tableID).
					Times(1).
					Return(baseSession, nil)
				reservation := baseReservation
				reservation.PaymentMode = "full"
				store.EXPECT().
					GetTableReservation(gomock.Any(), reservationID).
					Times(1).
					Return(reservation, nil)
			},
			check: func(t *testing.T, _ OrderSessionResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 409, reqErr.Status)
				require.Equal(t, "reservation is not in deposit mode", reqErr.Err.Error())
			},
		},
		{
			name:  "ReservationNotReady",
			input: OrderSessionInput{UserID: userID, MerchantID: merchantID, OrderType: "dine_in", TableID: &tableID, ReservationID: &reservationID},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetActiveDiningSessionByTable(gomock.Any(), tableID).
					Times(1).
					Return(baseSession, nil)
				reservation := baseReservation
				reservation.Status = "pending"
				store.EXPECT().
					GetTableReservation(gomock.Any(), reservationID).
					Times(1).
					Return(reservation, nil)
			},
			check: func(t *testing.T, _ OrderSessionResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 409, reqErr.Status)
				require.Equal(t, "reservation is not ready for dining", reqErr.Err.Error())
			},
		},
		{
			name:  "BillingGroupClosed",
			input: OrderSessionInput{UserID: userID, MerchantID: merchantID, OrderType: "dine_in", TableID: &tableID},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetActiveDiningSessionByTable(gomock.Any(), tableID).
					Times(1).
					Return(baseSession, nil)
				bg := db.BillingGroup{ID: 77, DiningSessionID: baseSession.ID, Status: "closed"}
				store.EXPECT().
					GetDefaultBillingGroupBySession(gomock.Any(), baseSession.ID).
					Times(1).
					Return(bg, nil)
			},
			check: func(t *testing.T, _ OrderSessionResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 409, reqErr.Status)
				require.Equal(t, "billing group is closed", reqErr.Err.Error())
			},
		},
		{
			name:  "SuccessDineIn",
			input: OrderSessionInput{UserID: userID, MerchantID: merchantID, OrderType: "dine_in", TableID: &tableID},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetActiveDiningSessionByTable(gomock.Any(), tableID).
					Times(1).
					Return(baseSession, nil)
				bg := db.BillingGroup{ID: 88, DiningSessionID: baseSession.ID, Status: "open"}
				store.EXPECT().
					GetDefaultBillingGroupBySession(gomock.Any(), baseSession.ID).
					Times(1).
					Return(bg, nil)
				store.EXPECT().
					GetActiveBillingGroupMember(gomock.Any(), db.GetActiveBillingGroupMemberParams{
						BillingGroupID: bg.ID,
						UserID:         userID,
					}).
					Times(1).
					Return(db.BillingGroupMember{}, nil)
			},
			check: func(t *testing.T, result OrderSessionResult, err error) {
				require.NoError(t, err)
				require.NotNil(t, result.DiningSession)
				require.NotNil(t, result.BillingGroupID)
				require.Equal(t, int64(88), *result.BillingGroupID)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			if tc.buildStubs != nil {
				tc.buildStubs(store)
			}

			result, err := ValidateOrderSessionAndBilling(context.Background(), store, tc.input)
			tc.check(t, result, err)
		})
	}
}

func TestValidateOrderSessionAndBilling_BillingGroupNotAllowed(t *testing.T) {
	userID := int64(90)
	merchantID := int64(91)
	billingGroupID := int64(92)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	result, err := ValidateOrderSessionAndBilling(context.Background(), store, OrderSessionInput{
		UserID:         userID,
		MerchantID:     merchantID,
		OrderType:      "takeout",
		BillingGroupID: &billingGroupID,
	})
	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
	require.Equal(t, "billing group is only allowed for dine-in or reservation", reqErr.Err.Error())
	require.Nil(t, result.DiningSession)
}

func TestValidateOrderSessionAndBilling_BillingGroupMemberMissing(t *testing.T) {
	userID := int64(111)
	merchantID := int64(222)
	tableID := int64(333)

	session := db.DiningSession{ID: 444, MerchantID: merchantID, TableID: tableID}
	billingGroup := db.BillingGroup{ID: 555, DiningSessionID: session.ID, Status: "open"}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetActiveDiningSessionByTable(gomock.Any(), tableID).
		Times(1).
		Return(session, nil)
	store.EXPECT().
		GetDefaultBillingGroupBySession(gomock.Any(), session.ID).
		Times(1).
		Return(billingGroup, nil)
	store.EXPECT().
		GetActiveBillingGroupMember(gomock.Any(), db.GetActiveBillingGroupMemberParams{
			BillingGroupID: billingGroup.ID,
			UserID:         userID,
		}).
		Times(1).
		Return(db.BillingGroupMember{}, db.ErrRecordNotFound)

	_, err := ValidateOrderSessionAndBilling(context.Background(), store, OrderSessionInput{
		UserID:     userID,
		MerchantID: merchantID,
		OrderType:  "dine_in",
		TableID:    &tableID,
	})
	reqErr := assertRequestError(t, err)
	require.Equal(t, 403, reqErr.Status)
	require.Equal(t, "not a member of billing group", reqErr.Err.Error())
}

func TestValidateOrderSessionAndBilling_BillingGroupMismatch(t *testing.T) {
	userID := int64(121)
	merchantID := int64(131)
	tableID := int64(141)
	billingGroupID := int64(151)

	session := db.DiningSession{ID: 161, MerchantID: merchantID, TableID: tableID}
	billingGroup := db.BillingGroup{ID: billingGroupID, DiningSessionID: session.ID + 1, Status: "open"}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetActiveDiningSessionByTable(gomock.Any(), tableID).
		Times(1).
		Return(session, nil)
	store.EXPECT().
		GetBillingGroup(gomock.Any(), billingGroupID).
		Times(1).
		Return(billingGroup, nil)

	_, err := ValidateOrderSessionAndBilling(context.Background(), store, OrderSessionInput{
		UserID:         userID,
		MerchantID:     merchantID,
		OrderType:      "dine_in",
		TableID:        &tableID,
		BillingGroupID: &billingGroupID,
	})
	reqErr := assertRequestError(t, err)
	require.Equal(t, 409, reqErr.Status)
	require.Equal(t, "billing group does not belong to dining session", reqErr.Err.Error())
}

func TestValidateOrderSessionAndBilling_BillingGroupNotFound(t *testing.T) {
	userID := int64(171)
	merchantID := int64(181)
	tableID := int64(191)
	billingGroupID := int64(201)

	session := db.DiningSession{ID: 211, MerchantID: merchantID, TableID: tableID}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetActiveDiningSessionByTable(gomock.Any(), tableID).
		Times(1).
		Return(session, nil)
	store.EXPECT().
		GetBillingGroup(gomock.Any(), billingGroupID).
		Times(1).
		Return(db.BillingGroup{}, db.ErrRecordNotFound)

	_, err := ValidateOrderSessionAndBilling(context.Background(), store, OrderSessionInput{
		UserID:         userID,
		MerchantID:     merchantID,
		OrderType:      "dine_in",
		TableID:        &tableID,
		BillingGroupID: &billingGroupID,
	})
	reqErr := assertRequestError(t, err)
	require.Equal(t, 404, reqErr.Status)
	require.Equal(t, "billing group not found", reqErr.Err.Error())
}

func TestValidateOrderSessionAndBilling_SessionLoadError(t *testing.T) {
	userID := int64(221)
	merchantID := int64(231)
	tableID := int64(241)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetActiveDiningSessionByTable(gomock.Any(), tableID).
		Times(1).
		Return(db.DiningSession{}, errors.New("boom"))

	_, err := ValidateOrderSessionAndBilling(context.Background(), store, OrderSessionInput{
		UserID:     userID,
		MerchantID: merchantID,
		OrderType:  "dine_in",
		TableID:    &tableID,
	})
	require.Error(t, err)
}
