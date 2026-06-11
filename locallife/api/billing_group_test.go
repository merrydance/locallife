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
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func offlineReservationForBillingGroupTest(tableID, operatorUserID, merchantID int64) db.TableReservation {
	return db.TableReservation{
		ID:                time.Now().UnixNano(),
		TableID:           tableID,
		UserID:            operatorUserID,
		MerchantID:        merchantID,
		Status:            ReservationStatusCheckedIn,
		Source:            pgtype.Text{String: db.ReservationSourcePhone, Valid: true},
		OfflineCustomerID: pgtype.Int8{Int64: 7001, Valid: true},
		CreatedByUserID:   pgtype.Int8{Int64: operatorUserID, Valid: true},
		CreatedAt:         time.Now(),
	}
}

func TestCreateBillingGroupAPI_UsesAggregatedAmounts(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	table := randomTable(merchant.ID)
	session := randomDiningSession(merchant.ID, table.ID, user.ID)
	billingGroup := db.BillingGroup{
		ID:              901,
		DiningSessionID: session.ID,
		Status:          "open",
		IsDefault:       false,
		TotalAmount:     9999,
		PaidAmount:      8888,
		CreatedAt:       time.Now(),
	}

	testCases := []struct {
		name          string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetDiningSession(gomock.Any(), session.ID).
					Times(1).
					Return(session, nil)
				store.EXPECT().
					CreateBillingGroup(gomock.Any(), gomock.Any()).
					Times(1).
					Return(billingGroup, nil)
				store.EXPECT().
					CreateBillingGroupMember(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.BillingGroupMember{BillingGroupID: billingGroup.ID, UserID: user.ID, Role: "owner"}, nil)
				store.EXPECT().
					GetBillingGroupAmounts(gomock.Any(), billingGroup.ID).
					Times(1).
					Return(db.GetBillingGroupAmountsRow{TotalAmount: 0, PaidAmount: 0}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
				var resp struct {
					Data billingGroupResponse `json:"data"`
				}
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Equal(t, int64(0), resp.Data.TotalAmount)
				require.Equal(t, int64(0), resp.Data.PaidAmount)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			body, err := json.Marshal(map[string]int64{"dining_session_id": session.ID})
			require.NoError(t, err)

			request, err := http.NewRequest(http.MethodPost, "/v1/billing-groups", bytes.NewReader(body))
			require.NoError(t, err)
			addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

			recorder := httptest.NewRecorder()
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestCreateBillingGroupAPI_OfflineReservationOperatorDenied(t *testing.T) {
	operator, _ := randomUser(t)
	merchant := randomMerchant(operator.ID)
	table := randomTable(merchant.ID)
	reservation := offlineReservationForBillingGroupTest(table.ID, operator.ID, merchant.ID)
	session := randomDiningSession(merchant.ID, table.ID, operator.ID)
	session.ReservationID = pgtype.Int8{Int64: reservation.ID, Valid: true}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetDiningSession(gomock.Any(), session.ID).
		Times(1).
		Return(session, nil)
	store.EXPECT().
		GetTableReservation(gomock.Any(), reservation.ID).
		Times(1).
		Return(reservation, nil)

	server := newTestServer(t, store)
	body, err := json.Marshal(map[string]int64{"dining_session_id": session.ID})
	require.NoError(t, err)
	request, err := http.NewRequest(http.MethodPost, "/v1/billing-groups", bytes.NewReader(body))
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, operator.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestListBillingGroupsAPI_UsesAggregatedAmounts(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	table := randomTable(merchant.ID)
	session := randomDiningSession(merchant.ID, table.ID, user.ID)
	groupA := db.BillingGroup{ID: 1001, DiningSessionID: session.ID, Status: "open", IsDefault: true, TotalAmount: 777, PaidAmount: 666, CreatedAt: time.Now()}
	groupB := db.BillingGroup{ID: 1002, DiningSessionID: session.ID, Status: "open", IsDefault: false, TotalAmount: 555, PaidAmount: 444, CreatedAt: time.Now()}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetDiningSession(gomock.Any(), session.ID).
		Times(1).
		Return(session, nil)
	store.EXPECT().
		ListBillingGroupsBySession(gomock.Any(), session.ID).
		Times(1).
		Return([]db.BillingGroup{groupA, groupB}, nil)
	store.EXPECT().
		GetBillingGroupAmounts(gomock.Any(), groupA.ID).
		Times(1).
		Return(db.GetBillingGroupAmountsRow{TotalAmount: 1200, PaidAmount: 800}, nil)
	store.EXPECT().
		GetBillingGroupAmounts(gomock.Any(), groupB.ID).
		Times(1).
		Return(db.GetBillingGroupAmountsRow{TotalAmount: 500, PaidAmount: 0}, nil)

	server := newTestServer(t, store)
	url := fmt.Sprintf("/v1/billing-groups?dining_session_id=%d", session.ID)
	request, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Data billingGroupListResponse `json:"data"`
	}
	err = json.Unmarshal(recorder.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Len(t, resp.Data.BillingGroups, 2)
	require.Equal(t, int64(1200), resp.Data.BillingGroups[0].TotalAmount)
	require.Equal(t, int64(800), resp.Data.BillingGroups[0].PaidAmount)
	require.Equal(t, int64(500), resp.Data.BillingGroups[1].TotalAmount)
	require.Equal(t, int64(0), resp.Data.BillingGroups[1].PaidAmount)
}

func TestListBillingGroupsAPI_OfflineReservationOperatorDenied(t *testing.T) {
	operator, _ := randomUser(t)
	merchant := randomMerchant(operator.ID)
	table := randomTable(merchant.ID)
	reservation := offlineReservationForBillingGroupTest(table.ID, operator.ID, merchant.ID)
	session := randomDiningSession(merchant.ID, table.ID, operator.ID)
	session.ReservationID = pgtype.Int8{Int64: reservation.ID, Valid: true}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetDiningSession(gomock.Any(), session.ID).
		Times(1).
		Return(session, nil)
	store.EXPECT().
		GetTableReservation(gomock.Any(), reservation.ID).
		Times(1).
		Return(reservation, nil)

	server := newTestServer(t, store)
	url := fmt.Sprintf("/v1/billing-groups?dining_session_id=%d", session.ID)
	request, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, operator.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestListBillingGroupsAPI_OfflineReservationJoinedMemberAllowed(t *testing.T) {
	operator, _ := randomUser(t)
	member, _ := randomUser(t)
	merchant := randomMerchant(operator.ID)
	table := randomTable(merchant.ID)
	reservation := offlineReservationForBillingGroupTest(table.ID, operator.ID, merchant.ID)
	session := randomDiningSession(merchant.ID, table.ID, operator.ID)
	session.ReservationID = pgtype.Int8{Int64: reservation.ID, Valid: true}
	defaultGroup := db.BillingGroup{ID: 1302, DiningSessionID: session.ID, Status: "open", IsDefault: true, CreatedAt: time.Now()}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetDiningSession(gomock.Any(), session.ID).
		Times(1).
		Return(session, nil)
	store.EXPECT().
		GetTableReservation(gomock.Any(), reservation.ID).
		Times(1).
		Return(reservation, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchant.ID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		CheckUserHasMerchantAccess(gomock.Any(), db.CheckUserHasMerchantAccessParams{
			MerchantID: merchant.ID,
			UserID:     member.ID,
		}).
		Times(1).
		Return(false, nil)
	store.EXPECT().
		GetDefaultBillingGroupBySession(gomock.Any(), session.ID).
		Times(1).
		Return(defaultGroup, nil)
	store.EXPECT().
		GetActiveBillingGroupMember(gomock.Any(), db.GetActiveBillingGroupMemberParams{
			BillingGroupID: defaultGroup.ID,
			UserID:         member.ID,
		}).
		Times(1).
		Return(db.BillingGroupMember{BillingGroupID: defaultGroup.ID, UserID: member.ID, Role: "member"}, nil)
	store.EXPECT().
		ListBillingGroupsBySession(gomock.Any(), session.ID).
		Times(1).
		Return([]db.BillingGroup{defaultGroup}, nil)
	store.EXPECT().
		GetBillingGroupAmounts(gomock.Any(), defaultGroup.ID).
		Times(1).
		Return(db.GetBillingGroupAmountsRow{}, nil)

	server := newTestServer(t, store)
	url := fmt.Sprintf("/v1/billing-groups?dining_session_id=%d", session.ID)
	request, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, member.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestListBillingGroupsAPI_OfflineReservationMerchantOwnerDeniedEvenWithMembership(t *testing.T) {
	customer, _ := randomUser(t)
	owner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	table := randomTable(merchant.ID)
	reservation := offlineReservationForBillingGroupTest(table.ID, customer.ID, merchant.ID)
	session := randomDiningSession(merchant.ID, table.ID, customer.ID)
	session.ReservationID = pgtype.Int8{Int64: reservation.ID, Valid: true}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetDiningSession(gomock.Any(), session.ID).
		Times(1).
		Return(session, nil)
	store.EXPECT().
		GetTableReservation(gomock.Any(), reservation.ID).
		Times(1).
		Return(reservation, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchant.ID).
		Times(1).
		Return(merchant, nil)

	server := newTestServer(t, store)
	url := fmt.Sprintf("/v1/billing-groups?dining_session_id=%d", session.ID)
	request, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, owner.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestListBillingGroupsAPI_LegacyOfflineReservationUserDeniedEvenWhenSessionUserDiffers(t *testing.T) {
	operator, _ := randomUser(t)
	otherStaff, _ := randomUser(t)
	merchant := randomMerchant(operator.ID)
	table := randomTable(merchant.ID)
	reservation := offlineReservationForBillingGroupTest(table.ID, operator.ID, merchant.ID)
	reservation.CreatedByUserID = pgtype.Int8{Valid: false}
	session := randomDiningSession(merchant.ID, table.ID, otherStaff.ID)
	session.ReservationID = pgtype.Int8{Int64: reservation.ID, Valid: true}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetDiningSession(gomock.Any(), session.ID).
		Times(1).
		Return(session, nil)
	store.EXPECT().
		GetTableReservation(gomock.Any(), reservation.ID).
		Times(1).
		Return(reservation, nil)

	server := newTestServer(t, store)
	url := fmt.Sprintf("/v1/billing-groups?dining_session_id=%d", session.ID)
	request, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, operator.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestJoinBillingGroupAPI_UsesAggregatedAmounts(t *testing.T) {
	owner, _ := randomUser(t)
	joiner, _ := randomUser(t)
	merchant := randomMerchant(owner.ID)
	table := randomTable(merchant.ID)
	session := randomDiningSession(merchant.ID, table.ID, owner.ID)
	targetGroup := db.BillingGroup{
		ID:              1101,
		DiningSessionID: session.ID,
		Status:          "open",
		IsDefault:       false,
		TotalAmount:     777,
		PaidAmount:      666,
		CreatedAt:       time.Now(),
	}
	defaultGroup := db.BillingGroup{
		ID:              1102,
		DiningSessionID: session.ID,
		Status:          "open",
		IsDefault:       true,
		TotalAmount:     333,
		PaidAmount:      222,
		CreatedAt:       time.Now(),
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetBillingGroup(gomock.Any(), targetGroup.ID).
		Times(1).
		Return(targetGroup, nil)
	store.EXPECT().
		GetDiningSession(gomock.Any(), session.ID).
		Times(1).
		Return(session, nil)
	store.EXPECT().
		GetDefaultBillingGroupBySession(gomock.Any(), session.ID).
		Times(1).
		Return(defaultGroup, nil)
	store.EXPECT().
		GetActiveBillingGroupMember(gomock.Any(), db.GetActiveBillingGroupMemberParams{BillingGroupID: defaultGroup.ID, UserID: joiner.ID}).
		Times(1).
		Return(db.BillingGroupMember{BillingGroupID: defaultGroup.ID, UserID: joiner.ID, Role: "member"}, nil)
	store.EXPECT().
		CreateBillingGroupMember(gomock.Any(), db.CreateBillingGroupMemberParams{BillingGroupID: targetGroup.ID, UserID: joiner.ID, Role: "member"}).
		Times(1).
		Return(db.BillingGroupMember{BillingGroupID: targetGroup.ID, UserID: joiner.ID, Role: "member"}, nil)
	store.EXPECT().
		GetBillingGroupAmounts(gomock.Any(), targetGroup.ID).
		Times(1).
		Return(db.GetBillingGroupAmountsRow{TotalAmount: 1680, PaidAmount: 920}, nil)

	server := newTestServer(t, store)
	url := fmt.Sprintf("/v1/billing-groups/%d/join", targetGroup.ID)
	request, err := http.NewRequest(http.MethodPost, url, nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, joiner.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp struct {
		Data billingGroupResponse `json:"data"`
	}
	err = json.Unmarshal(recorder.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Equal(t, targetGroup.ID, resp.Data.ID)
	require.Equal(t, int64(1680), resp.Data.TotalAmount)
	require.Equal(t, int64(920), resp.Data.PaidAmount)
}

func TestJoinBillingGroupAPI_OfflineReservationOperatorDenied(t *testing.T) {
	operator, _ := randomUser(t)
	merchant := randomMerchant(operator.ID)
	table := randomTable(merchant.ID)
	reservation := offlineReservationForBillingGroupTest(table.ID, operator.ID, merchant.ID)
	session := randomDiningSession(merchant.ID, table.ID, operator.ID)
	session.ReservationID = pgtype.Int8{Int64: reservation.ID, Valid: true}
	targetGroup := db.BillingGroup{ID: 1401, DiningSessionID: session.ID, Status: "open", IsDefault: false, CreatedAt: time.Now()}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetBillingGroup(gomock.Any(), targetGroup.ID).
		Times(1).
		Return(targetGroup, nil)
	store.EXPECT().
		GetDiningSession(gomock.Any(), session.ID).
		Times(1).
		Return(session, nil)
	store.EXPECT().
		GetTableReservation(gomock.Any(), reservation.ID).
		Times(1).
		Return(reservation, nil)

	server := newTestServer(t, store)
	url := fmt.Sprintf("/v1/billing-groups/%d/join", targetGroup.ID)
	request, err := http.NewRequest(http.MethodPost, url, nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, operator.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestListBillingGroupOrdersAPI_OfflineReservationOperatorDenied(t *testing.T) {
	operator, _ := randomUser(t)
	merchant := randomMerchant(operator.ID)
	table := randomTable(merchant.ID)
	reservation := offlineReservationForBillingGroupTest(table.ID, operator.ID, merchant.ID)
	session := randomDiningSession(merchant.ID, table.ID, operator.ID)
	session.ReservationID = pgtype.Int8{Int64: reservation.ID, Valid: true}
	targetGroup := db.BillingGroup{ID: 1501, DiningSessionID: session.ID, Status: "open", IsDefault: false, CreatedAt: time.Now()}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetBillingGroup(gomock.Any(), targetGroup.ID).
		Times(1).
		Return(targetGroup, nil)
	store.EXPECT().
		GetDiningSession(gomock.Any(), session.ID).
		Times(1).
		Return(session, nil)
	store.EXPECT().
		GetTableReservation(gomock.Any(), reservation.ID).
		Times(1).
		Return(reservation, nil)

	server := newTestServer(t, store)
	url := fmt.Sprintf("/v1/billing-groups/%d/orders", targetGroup.ID)
	request, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, operator.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)
}
