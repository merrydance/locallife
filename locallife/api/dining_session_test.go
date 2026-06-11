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
	"github.com/merrydance/locallife/logic"
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
		GetTableReservation(gomock.Any(), reservation.ID).
		Times(1).
		Return(reservation, nil)
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

func TestGetDiningSessionEntryAPI_ResumeCurrentSession(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.Status = "approved"
	merchant.IsOpen = true
	table := randomTable(merchant.ID)
	table.Status = "available"
	session := randomDiningSession(merchant.ID, table.ID, user.ID)
	billingGroup := db.BillingGroup{
		ID:              util.RandomInt(1, 1000),
		DiningSessionID: session.ID,
		Status:          "open",
		IsDefault:       true,
		CreatedAt:       time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchant.ID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		GetTableByMerchantAndNo(gomock.Any(), db.GetTableByMerchantAndNoParams{MerchantID: merchant.ID, TableNo: table.TableNo}).
		Times(1).
		Return(table, nil)
	store.EXPECT().
		GetTable(gomock.Any(), table.ID).
		Times(1).
		Return(table, nil)
	store.EXPECT().
		ListReservationsByTableAndDate(gomock.Any(), gomock.Any()).
		Times(1).
		Return([]db.TableReservation{}, nil)
	store.EXPECT().
		GetActiveDiningSessionByTable(gomock.Any(), table.ID).
		Times(1).
		Return(session, nil)
	store.EXPECT().
		GetDefaultBillingGroupBySession(gomock.Any(), session.ID).
		Times(1).
		Return(billingGroup, nil)
	store.EXPECT().
		GetBillingGroupAmounts(gomock.Any(), billingGroup.ID).
		Times(1).
		Return(db.GetBillingGroupAmountsRow{TotalAmount: 1200, PaidAmount: 300}, nil)

	server := newTestServer(t, store)
	url := fmt.Sprintf("/v1/dining-sessions/entry?merchant_id=%d&table_no=%s", merchant.ID, table.TableNo)
	request, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp diningSessionEntryResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, logic.DiningSessionEntryActionResume, resp.Action)
	require.NotNil(t, resp.ActiveSession)
	require.Equal(t, session.ID, resp.ActiveSession.Session.ID)
	require.Equal(t, billingGroup.ID, resp.ActiveSession.BillingGroup.ID)
	require.Equal(t, table.TableNo, resp.ActiveSession.TableNo)
	require.True(t, resp.Capabilities.CanOrder)
	require.False(t, resp.Precheck.Reserved)
}

func TestGetDiningSessionEntryAPI_TransferSession(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.Status = "approved"
	merchant.IsOpen = true
	table := randomTable(merchant.ID)
	table.Status = "available"
	sourceTable := randomTable(merchant.ID)
	sourceTable.TableNo = "B02"
	transferSession := randomDiningSession(merchant.ID, sourceTable.ID, user.ID)
	billingGroup := db.BillingGroup{
		ID:              util.RandomInt(1, 1000),
		DiningSessionID: transferSession.ID,
		Status:          "open",
		IsDefault:       true,
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
		GetMerchant(gomock.Any(), merchant.ID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		GetTable(gomock.Any(), table.ID).
		Times(1).
		Return(table, nil)
	store.EXPECT().
		ListReservationsByTableAndDate(gomock.Any(), gomock.Any()).
		Times(1).
		Return([]db.TableReservation{}, nil)
	store.EXPECT().
		GetActiveDiningSessionByTable(gomock.Any(), table.ID).
		Times(1).
		Return(db.DiningSession{}, db.ErrRecordNotFound)
	store.EXPECT().
		ListDiningSessionsByUser(gomock.Any(), db.ListDiningSessionsByUserParams{UserID: user.ID, Limit: 20, Offset: 0}).
		Times(1).
		Return([]db.DiningSession{transferSession}, nil)
	store.EXPECT().
		GetDefaultBillingGroupBySession(gomock.Any(), transferSession.ID).
		Times(1).
		Return(billingGroup, nil)
	store.EXPECT().
		GetActiveBillingGroupMember(gomock.Any(), db.GetActiveBillingGroupMemberParams{BillingGroupID: billingGroup.ID, UserID: user.ID}).
		Times(1).
		Return(db.BillingGroupMember{BillingGroupID: billingGroup.ID, UserID: user.ID, Role: "owner"}, nil)
	store.EXPECT().
		GetTable(gomock.Any(), sourceTable.ID).
		Times(1).
		Return(sourceTable, nil)
	store.EXPECT().
		GetBillingGroupAmounts(gomock.Any(), billingGroup.ID).
		Times(1).
		Return(db.GetBillingGroupAmountsRow{TotalAmount: 8800, PaidAmount: 2000}, nil)

	server := newTestServer(t, store)
	url := fmt.Sprintf("/v1/dining-sessions/entry?table_id=%d", table.ID)
	request, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp diningSessionEntryResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, logic.DiningSessionEntryActionTransfer, resp.Action)
	require.Equal(t, table.ID, resp.Precheck.TableID)
	require.NotNil(t, resp.TransferSession)
	require.Equal(t, transferSession.ID, resp.TransferSession.Session.ID)
	require.Equal(t, sourceTable.TableNo, resp.TransferSession.TableNo)
	require.True(t, resp.Capabilities.CanTransfer)
	require.True(t, resp.Capabilities.CanOrder)
	require.True(t, resp.Capabilities.TransferRequiresTableCode)
}

func TestGetDiningSessionEntryAPI_DoesNotOfferOfflineReservationOperatorTransfer(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.Status = "approved"
	merchant.IsOpen = true
	table := randomTable(merchant.ID)
	table.Status = "available"
	sourceTable := randomTable(merchant.ID)
	sourceTable.TableNo = "B02"
	transferSession := randomDiningSession(merchant.ID, sourceTable.ID, user.ID)
	transferSession.ReservationID = pgtype.Int8{Int64: util.RandomInt(1000, 2000), Valid: true}
	offlineReservation := db.TableReservation{
		ID:                transferSession.ReservationID.Int64,
		UserID:            user.ID,
		MerchantID:        merchant.ID,
		TableID:           sourceTable.ID,
		Status:            "checked_in",
		Source:            pgtype.Text{String: db.ReservationSourceMerchant, Valid: true},
		OfflineCustomerID: pgtype.Int8{Int64: 3002, Valid: true},
		CreatedByUserID:   pgtype.Int8{Int64: user.ID, Valid: true},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetTable(gomock.Any(), table.ID).
		Times(1).
		Return(table, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchant.ID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		GetTable(gomock.Any(), table.ID).
		Times(1).
		Return(table, nil)
	store.EXPECT().
		ListReservationsByTableAndDate(gomock.Any(), gomock.Any()).
		Times(1).
		Return([]db.TableReservation{}, nil)
	store.EXPECT().
		GetActiveDiningSessionByTable(gomock.Any(), table.ID).
		Times(1).
		Return(db.DiningSession{}, db.ErrRecordNotFound)
	store.EXPECT().
		ListDiningSessionsByUser(gomock.Any(), db.ListDiningSessionsByUserParams{UserID: user.ID, Limit: 20, Offset: 0}).
		Times(1).
		Return([]db.DiningSession{transferSession}, nil)
	store.EXPECT().
		GetTableReservation(gomock.Any(), transferSession.ReservationID.Int64).
		Times(1).
		Return(offlineReservation, nil)

	server := newTestServer(t, store)
	url := fmt.Sprintf("/v1/dining-sessions/entry?table_id=%d", table.ID)
	request, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp diningSessionEntryResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, logic.DiningSessionEntryActionOpen, resp.Action)
	require.Nil(t, resp.TransferSession)
	require.False(t, resp.Capabilities.CanTransfer)
}

func TestGetDiningSessionEntryAPI_BlockedStillReturnsTableContext(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.Status = "pending"
	merchant.IsOpen = false
	table := randomTable(merchant.ID)
	table.Status = "available"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetTable(gomock.Any(), table.ID).
		Times(1).
		Return(table, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchant.ID).
		Times(1).
		Return(merchant, nil)

	server := newTestServer(t, store)
	url := fmt.Sprintf("/v1/dining-sessions/entry?table_id=%d", table.ID)
	request, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp diningSessionEntryResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, logic.DiningSessionEntryActionBlocked, resp.Action)
	require.Equal(t, table.ID, resp.Table.ID)
	require.Equal(t, table.ID, resp.Precheck.TableID)
	require.NotNil(t, resp.BlockedReason)
	require.False(t, resp.Capabilities.CanOrder)
	require.False(t, resp.Capabilities.CanTransfer)
}

func TestGetDiningSessionMenuAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.Status = "approved"
	merchant.IsOpen = true
	table := randomTable(merchant.ID)
	session := randomDiningSession(merchant.ID, table.ID, user.ID)
	billingGroup := db.BillingGroup{
		ID:              util.RandomInt(1, 1000),
		DiningSessionID: session.ID,
		Status:          "open",
		IsDefault:       true,
		CreatedAt:       time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetDiningSession(gomock.Any(), session.ID).
		Times(1).
		Return(session, nil)
	store.EXPECT().
		GetDefaultBillingGroupBySession(gomock.Any(), session.ID).
		Times(1).
		Return(billingGroup, nil)
	store.EXPECT().
		GetTable(gomock.Any(), session.TableID).
		Times(1).
		Return(table, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), session.MerchantID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		GetBillingGroupAmounts(gomock.Any(), billingGroup.ID).
		Times(1).
		Return(db.GetBillingGroupAmountsRow{TotalAmount: 9900, PaidAmount: 1200}, nil)
	store.EXPECT().
		ListDishCategories(gomock.Any(), merchant.ID).
		Times(1).
		Return([]db.ListDishCategoriesRow{}, nil)
	store.EXPECT().
		ListDishesForMenu(gomock.Any(), merchant.ID).
		Times(1).
		Return([]db.ListDishesForMenuRow{}, nil)
	store.EXPECT().
		ListOnlineCombosByMerchant(gomock.Any(), merchant.ID).
		Times(1).
		Return([]db.ListOnlineCombosByMerchantRow{}, nil)
	store.EXPECT().
		ListActiveDeliveryPromotionsByMerchant(gomock.Any(), merchant.ID).
		Times(1).
		Return([]db.MerchantDeliveryPromotion{}, nil)
	store.EXPECT().
		ListActiveDiscountRules(gomock.Any(), merchant.ID).
		Times(1).
		Return([]db.DiscountRule{}, nil)

	server := newTestServer(t, store)
	url := fmt.Sprintf("/v1/dining-sessions/%d/menu", session.ID)
	request, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp diningSessionMenuResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, session.ID, resp.Session.ID)
	require.Equal(t, billingGroup.ID, resp.BillingGroup.ID)
	require.Equal(t, merchant.ID, resp.Merchant.ID)
	require.Equal(t, table.ID, resp.Table.ID)
	require.Empty(t, resp.Categories)
	require.Empty(t, resp.Combos)
	require.Empty(t, resp.Promotions)
}

func TestGetDiningSessionMenuAPI_ForbiddenWhenUserNotInSession(t *testing.T) {
	owner, _ := randomUser(t)
	viewer, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	table := randomTable(merchant.ID)
	session := randomDiningSession(merchant.ID, table.ID, owner.ID)
	billingGroup := db.BillingGroup{
		ID:              util.RandomInt(1, 1000),
		DiningSessionID: session.ID,
		Status:          "open",
		IsDefault:       true,
		CreatedAt:       time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetDiningSession(gomock.Any(), session.ID).
		Times(1).
		Return(session, nil)
	store.EXPECT().
		GetDefaultBillingGroupBySession(gomock.Any(), session.ID).
		Times(1).
		Return(billingGroup, nil)
	store.EXPECT().
		GetActiveBillingGroupMember(gomock.Any(), db.GetActiveBillingGroupMemberParams{BillingGroupID: billingGroup.ID, UserID: viewer.ID}).
		Times(1).
		Return(db.BillingGroupMember{}, db.ErrRecordNotFound)

	server := newTestServer(t, store)
	url := fmt.Sprintf("/v1/dining-sessions/%d/menu", session.ID)
	request, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, viewer.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestGetDiningSessionMenuAPI_ForbiddenForOfflineReservationOperator(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	table := randomTable(merchant.ID)
	session := randomDiningSession(merchant.ID, table.ID, user.ID+1)
	session.ReservationID = pgtype.Int8{Int64: util.RandomInt(1000, 2000), Valid: true}
	offlineReservation := db.TableReservation{
		ID:                session.ReservationID.Int64,
		UserID:            user.ID + 1,
		MerchantID:        merchant.ID,
		TableID:           table.ID,
		Status:            "checked_in",
		Source:            pgtype.Text{String: db.ReservationSourcePhone, Valid: true},
		OfflineCustomerID: pgtype.Int8{Int64: 3003, Valid: true},
		CreatedByUserID:   pgtype.Int8{Int64: user.ID, Valid: true},
	}
	billingGroup := db.BillingGroup{
		ID:              util.RandomInt(1, 1000),
		DiningSessionID: session.ID,
		Status:          "open",
		IsDefault:       true,
		CreatedAt:       time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetDiningSession(gomock.Any(), session.ID).
		Times(1).
		Return(session, nil)
	store.EXPECT().
		GetDefaultBillingGroupBySession(gomock.Any(), session.ID).
		Times(1).
		Return(billingGroup, nil)
	store.EXPECT().
		GetTableReservation(gomock.Any(), session.ReservationID.Int64).
		Times(1).
		Return(offlineReservation, nil)
	store.EXPECT().
		GetActiveBillingGroupMember(gomock.Any(), gomock.Any()).
		Times(0)

	server := newTestServer(t, store)
	url := fmt.Sprintf("/v1/dining-sessions/%d/menu", session.ID)
	request, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)
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
