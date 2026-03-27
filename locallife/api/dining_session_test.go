package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func reservationTimeForAPITest(t time.Time) pgtype.Time {
	seconds := int64(t.Hour()*3600+t.Minute()*60+t.Second()) * 1000000
	return pgtype.Time{Microseconds: seconds, Valid: true}
}

func TestPrecheckDiningSessionAPI(t *testing.T) {
	reservationStart := time.Now().Add(10 * time.Minute)
	dateOnly := time.Date(reservationStart.Year(), reservationStart.Month(), reservationStart.Day(), 0, 0, 0, 0, reservationStart.Location())
	reservationDate := pgtype.Date{Time: dateOnly, Valid: true}
	reservationTime := reservationTimeForAPITest(reservationStart)

	reservationUser, _ := randomUser(t)
	merchantOwner, _ := randomUser(t)
	merchant := randomMerchant(merchantOwner.ID)
	table := randomTable(merchant.ID)
	reservation := db.TableReservation{
		ID:              util.RandomInt(1, 1000),
		UserID:          reservationUser.ID,
		MerchantID:      merchant.ID,
		TableID:         table.ID,
		Status:          "paid",
		PaymentMode:     PaymentModeDeposit,
		DepositAmount:   600,
		PrepaidAmount:   900,
		ReservationDate: reservationDate,
		ReservationTime: reservationTime,
	}
	order := db.Order{ID: util.RandomInt(1, 1000), Status: db.OrderStatusPaid, FulfillmentStatus: db.FulfillmentStatusScheduled}

	testCases := []struct {
		name          string
		authUserID    int64
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:       "NoActiveReservation",
			authUserID: reservationUser.ID,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)
				store.EXPECT().
					ListReservationsByTableAndDate(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.TableReservation{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp diningSessionPrecheckResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, table.ID, resp.TableID)
				require.False(t, resp.Reserved)
				require.Nil(t, resp.ReservationID)
				require.False(t, resp.IsReservationOwner)
			},
		},
		{
			name:       "ReservationOwner",
			authUserID: reservationUser.ID,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)
				store.EXPECT().
					ListReservationsByTableAndDate(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.TableReservation{reservation}, nil)
				store.EXPECT().
					GetMerchant(gomock.Any(), table.MerchantID).
					Times(1).
					Return(merchant, nil)
				store.EXPECT().
					CheckUserHasMerchantAccess(gomock.Any(), db.CheckUserHasMerchantAccessParams{MerchantID: table.MerchantID, UserID: reservationUser.ID}).
					Times(1).
					Return(false, nil)
				store.EXPECT().
					GetLatestOrderByReservation(gomock.Any(), pgtype.Int8{Int64: reservation.ID, Valid: true}).
					Times(1).
					Return(order, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp diningSessionPrecheckResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.True(t, resp.Reserved)
				require.NotNil(t, resp.ReservationID)
				require.Equal(t, reservation.ID, *resp.ReservationID)
				require.True(t, resp.IsReservationOwner)
				require.NotNil(t, resp.PaymentMode)
				require.Equal(t, PaymentModeDeposit, *resp.PaymentMode)
				require.NotNil(t, resp.PaidAmount)
				require.Equal(t, reservation.DepositAmount, *resp.PaidAmount)
				require.NotNil(t, resp.OrderID)
				require.Equal(t, order.ID, *resp.OrderID)
				require.NotNil(t, resp.OrderStatus)
				require.Equal(t, order.Status, *resp.OrderStatus)
				require.NotNil(t, resp.OrderFulfillmentStatus)
				require.Equal(t, order.FulfillmentStatus, *resp.OrderFulfillmentStatus)
			},
		},
		{
			name:       "MerchantViewerIsNotReservationOwner",
			authUserID: merchant.OwnerUserID,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)
				store.EXPECT().
					ListReservationsByTableAndDate(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.TableReservation{reservation}, nil)
				store.EXPECT().
					GetMerchant(gomock.Any(), table.MerchantID).
					Times(1).
					Return(merchant, nil)
				store.EXPECT().
					GetLatestOrderByReservation(gomock.Any(), pgtype.Int8{Int64: reservation.ID, Valid: true}).
					Times(1).
					Return(db.Order{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp diningSessionPrecheckResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.True(t, resp.Reserved)
				require.NotNil(t, resp.ReservationID)
				require.Equal(t, reservation.ID, *resp.ReservationID)
				require.False(t, resp.IsReservationOwner)
				require.Nil(t, resp.OrderID)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			url := fmt.Sprintf("/v1/dining-sessions/precheck?table_id=%d", table.ID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)
			addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, tc.authUserID, time.Minute)

			recorder := httptest.NewRecorder()
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestOpenDiningSessionAPI_UsesAggregatedBillingGroupAmounts(t *testing.T) {
	reservationStart := time.Now().Add(10 * time.Minute)
	dateOnly := time.Date(reservationStart.Year(), reservationStart.Month(), reservationStart.Day(), 0, 0, 0, 0, reservationStart.Location())
	reservationDate := pgtype.Date{Time: dateOnly, Valid: true}
	reservationTime := reservationTimeForAPITest(reservationStart)

	user, _ := randomUser(t)
	merchantOwner, _ := randomUser(t)
	merchant := randomMerchant(merchantOwner.ID)
	table := randomTable(merchant.ID)
	reservation := db.TableReservation{
		ID:              util.RandomInt(1, 1000),
		UserID:          user.ID,
		MerchantID:      merchant.ID,
		TableID:         table.ID,
		Status:          "confirmed",
		ReservationDate: reservationDate,
		ReservationTime: reservationTime,
	}
	session := randomDiningSession(merchant.ID, table.ID, user.ID)
	session.ReservationID = pgtype.Int8{Int64: reservation.ID, Valid: true}
	billingGroup := db.BillingGroup{
		ID:              util.RandomInt(1, 1000),
		DiningSessionID: session.ID,
		Status:          "open",
		IsDefault:       true,
		TotalAmount:     9999,
		PaidAmount:      8888,
		CreatedAt:       time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetTable(gomock.Any(), table.ID).
		Times(1).
		Return(table, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), table.MerchantID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		CheckUserHasMerchantAccess(gomock.Any(), db.CheckUserHasMerchantAccessParams{MerchantID: table.MerchantID, UserID: user.ID}).
		Times(1).
		Return(false, nil)
	store.EXPECT().
		ListReservationsByTableAndDate(gomock.Any(), gomock.Any()).
		Times(1).
		Return([]db.TableReservation{reservation}, nil)
	store.EXPECT().
		GetActiveDiningSessionByReservation(gomock.Any(), pgtype.Int8{Int64: reservation.ID, Valid: true}).
		Times(1).
		Return(session, nil)
	store.EXPECT().
		GetDefaultBillingGroupBySession(gomock.Any(), session.ID).
		Times(1).
		Return(billingGroup, nil)
	store.EXPECT().
		GetActiveBillingGroupMember(gomock.Any(), db.GetActiveBillingGroupMemberParams{BillingGroupID: billingGroup.ID, UserID: user.ID}).
		Times(1).
		Return(db.BillingGroupMember{BillingGroupID: billingGroup.ID, UserID: user.ID, Role: "owner"}, nil)
	store.EXPECT().
		GetBillingGroupAmounts(gomock.Any(), billingGroup.ID).
		Times(1).
		Return(db.GetBillingGroupAmountsRow{TotalAmount: 1234, PaidAmount: 567}, nil)

	server := newTestServer(t, store)
	body, err := json.Marshal(openDiningSessionRequest{TableID: table.ID})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/dining-sessions/open", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp openDiningSessionResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, session.ID, resp.Session.ID)
	require.NotNil(t, resp.Session.ReservationID)
	require.Equal(t, reservation.ID, *resp.Session.ReservationID)
	require.Equal(t, billingGroup.ID, resp.BillingGroup.ID)
	require.Equal(t, int64(1234), resp.BillingGroup.TotalAmount)
	require.Equal(t, int64(567), resp.BillingGroup.PaidAmount)
}

func TestCheckoutDiningSessionAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	// randomDiningSession(merchantID, tableID, userID)
	diningSession := randomDiningSession(merchant.ID, util.RandomInt(1, 100), user.ID)

	testCases := []struct {
		name          string
		sessionID     int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:      "OK",
			sessionID: diningSession.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				closedSession := diningSession
				closedSession.Status = "closed"
				closedSession.ClosedAt.Time = time.Now()
				closedSession.ClosedAt.Valid = true

				store.EXPECT().
					CloseDiningSessionTx(gomock.Any(), gomock.Eq(db.CloseDiningSessionTxParams{
						ID:         diningSession.ID,
						MerchantID: merchant.ID,
					})).
					Times(1).
					Return(db.CloseDiningSessionTxResult{
						Session: closedSession,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:      "NotMerchant",
			sessionID: diningSession.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.Merchant{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:      "SessionNotFound",
			sessionID: diningSession.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					CloseDiningSessionTx(gomock.Any(), gomock.Eq(db.CloseDiningSessionTxParams{
						ID:         diningSession.ID,
						MerchantID: merchant.ID,
					})).
					Times(1).
					Return(db.CloseDiningSessionTxResult{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:      "InternalServerError",
			sessionID: diningSession.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					CloseDiningSessionTx(gomock.Any(), gomock.Eq(db.CloseDiningSessionTxParams{
						ID:         diningSession.ID,
						MerchantID: merchant.ID,
					})).
					Times(1).
					Return(db.CloseDiningSessionTxResult{}, fmt.Errorf("some internal error"))
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/dining-sessions/%d/checkout", tc.sessionID)
			request, err := http.NewRequest(http.MethodPost, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
