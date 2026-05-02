package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	mockwk "github.com/merrydance/locallife/worker/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type profitSharingReceiverLifecycleRequestTestCase struct {
	admin       db.User
	auditWriter *auditSpyWriter
	method      string
	url         string
	body        any
	buildStubs  func(store *mockdb.MockStore, distributor *mockwk.MockTaskDistributor, ecommerceClient *mockwechat.MockEcommerceClientInterface)
}

func performProfitSharingReceiverLifecycleRequest(t *testing.T, tc profitSharingReceiverLifecycleRequestTestCase) *httptest.ResponseRecorder {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	tc.buildStubs(store, distributor, ecommerceClient)

	server := newTestServerWithTaskDistributor(t, store, distributor)
	server.SetEcommerceClientForTest(ecommerceClient)
	if tc.auditWriter != nil {
		server.auditWriter = tc.auditWriter
	}

	bodyBytes := []byte(nil)
	if tc.body != nil {
		var err error
		bodyBytes, err = json.Marshal(tc.body)
		require.NoError(t, err)
	}

	request, err := http.NewRequest(tc.method, tc.url, bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	if tc.body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, tc.admin.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)
	return recorder
}

func TestListProfitSharingReceiverLifecycleTargetsAPI_FiltersAndSanitizes(t *testing.T) {
	admin, _ := randomUser(t)
	target := profitSharingReceiverLifecycleRepairTarget(3101, db.ProfitSharingReceiverOwnerTypeRider, db.ProfitSharingReceiverDesiredStatePresent)
	target.SyncStatus = db.ProfitSharingReceiverSyncStatusFailed
	target.AccountHash = "hash-openid-should-not-leak"
	target.DisplayNameHash = pgtype.Text{String: "hash-name-should-not-leak", Valid: true}
	target.LastErrorCode = pgtype.Text{String: "wechat_param_error", Valid: true}
	target.LastErrorMessage = pgtype.Text{String: "sanitized receiver rejected", Valid: true}
	auditWriter := &auditSpyWriter{}

	recorder := performProfitSharingReceiverLifecycleRequest(t, profitSharingReceiverLifecycleRequestTestCase{
		admin:       admin,
		auditWriter: auditWriter,
		method:      http.MethodGet,
		url:         "/v1/platform/profit-sharing/receiver-lifecycle/targets?owner_type=rider&owner_id=3101&sync_status=failed&page=2&limit=10",
		buildStubs: func(store *mockdb.MockStore, _ *mockwk.MockTaskDistributor, _ *mockwechat.MockEcommerceClientInterface) {
			expectAdminRole(store, admin.ID)
			store.EXPECT().ListProfitSharingReceiverTargets(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ListProfitSharingReceiverTargetsParams) ([]db.ProfitSharingReceiverTarget, error) {
				require.Equal(t, "rider", arg.OwnerType.String)
				require.True(t, arg.OwnerType.Valid)
				require.Equal(t, int64(3101), arg.OwnerID.Int64)
				require.True(t, arg.OwnerID.Valid)
				require.Equal(t, "failed", arg.SyncStatus.String)
				require.Equal(t, int32(10), arg.OffsetCount)
				require.Equal(t, int32(10), arg.LimitCount)
				return []db.ProfitSharingReceiverTarget{target}, nil
			})
			store.EXPECT().CountProfitSharingReceiverTargets(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CountProfitSharingReceiverTargetsParams) (int64, error) {
				require.Equal(t, "rider", arg.OwnerType.String)
				require.Equal(t, int64(3101), arg.OwnerID.Int64)
				require.Equal(t, "failed", arg.SyncStatus.String)
				return 21, nil
			})
		},
	})

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "hash-openid-should-not-leak")
	require.NotContains(t, recorder.Body.String(), "hash-name-should-not-leak")

	var resp listProfitSharingReceiverLifecycleTargetsResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, int64(21), resp.Total)
	require.Equal(t, int32(2), resp.Page)
	require.Equal(t, int32(10), resp.Limit)
	require.Equal(t, int64(3), resp.TotalPages)
	require.True(t, resp.HasMore)
	require.Len(t, resp.Items, 1)
	require.Equal(t, target.ID, resp.Items[0].ID)
	require.Equal(t, "sanitized receiver rejected", *resp.Items[0].LastErrorMessage)

	entries := auditWriter.Entries()
	require.Len(t, entries, 1)
	require.Equal(t, "profit_sharing_receiver_lifecycle_targets_viewed", entries[0].Action)
	require.Equal(t, "profit_sharing_receiver_targets", entries[0].TargetType)
	require.Equal(t, "rider", entries[0].Metadata["owner_type"])
	require.Equal(t, int64(3101), entries[0].Metadata["owner_id"])
	require.Equal(t, "failed", entries[0].Metadata["sync_status"])
}

func TestGetProfitSharingReceiverLifecycleTargetAPI_ReturnsDetail(t *testing.T) {
	admin, _ := randomUser(t)
	target := profitSharingReceiverLifecycleRepairTarget(3201, db.ProfitSharingReceiverOwnerTypeOperator, db.ProfitSharingReceiverDesiredStateAbsent)
	target.AccountHash = "hash-detail-should-not-leak"
	auditWriter := &auditSpyWriter{}

	recorder := performProfitSharingReceiverLifecycleRequest(t, profitSharingReceiverLifecycleRequestTestCase{
		admin:       admin,
		auditWriter: auditWriter,
		method:      http.MethodGet,
		url:         "/v1/platform/profit-sharing/receiver-lifecycle/targets/" + fmt.Sprint(target.ID),
		buildStubs: func(store *mockdb.MockStore, _ *mockwk.MockTaskDistributor, _ *mockwechat.MockEcommerceClientInterface) {
			expectAdminRole(store, admin.ID)
			store.EXPECT().GetProfitSharingReceiverTarget(gomock.Any(), target.ID).Return(target, nil)
		},
	})

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "hash-detail-should-not-leak")
	var resp getProfitSharingReceiverLifecycleTargetResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, target.ID, resp.Target.ID)
	require.Equal(t, db.ProfitSharingReceiverOwnerTypeOperator, resp.Target.OwnerType)

	entries := auditWriter.Entries()
	require.Len(t, entries, 1)
	require.Equal(t, "profit_sharing_receiver_lifecycle_target_viewed", entries[0].Action)
	require.Equal(t, target.ID, *entries[0].TargetID)
}

func TestListProfitSharingReceiverLifecycleAttemptsAPI_ReturnsPaginatedAttempts(t *testing.T) {
	admin, _ := randomUser(t)
	target := profitSharingReceiverLifecycleRepairTarget(3301, db.ProfitSharingReceiverOwnerTypeRider, db.ProfitSharingReceiverDesiredStatePresent)
	attempt := db.ProfitSharingReceiverAttempt{
		ID:                4401,
		TargetID:          target.ID,
		Action:            db.ProfitSharingReceiverAttemptActionEnsure,
		Status:            db.ProfitSharingReceiverAttemptStatusFailed,
		IdempotentSuccess: false,
		ErrorCode:         pgtype.Text{String: "wechat_5xx", Valid: true},
		ErrorMessage:      pgtype.Text{String: "sanitized upstream unavailable", Valid: true},
		StartedAt:         time.Date(2026, 4, 25, 10, 5, 0, 0, time.UTC),
		FinishedAt:        pgtype.Timestamptz{Time: time.Date(2026, 4, 25, 10, 5, 2, 0, time.UTC), Valid: true},
		CreatedAt:         time.Date(2026, 4, 25, 10, 5, 0, 0, time.UTC),
	}
	auditWriter := &auditSpyWriter{}

	recorder := performProfitSharingReceiverLifecycleRequest(t, profitSharingReceiverLifecycleRequestTestCase{
		admin:       admin,
		auditWriter: auditWriter,
		method:      http.MethodGet,
		url:         "/v1/platform/profit-sharing/receiver-lifecycle/targets/" + fmt.Sprint(target.ID) + "/attempts?page=1&limit=5",
		buildStubs: func(store *mockdb.MockStore, _ *mockwk.MockTaskDistributor, _ *mockwechat.MockEcommerceClientInterface) {
			expectAdminRole(store, admin.ID)
			store.EXPECT().GetProfitSharingReceiverTarget(gomock.Any(), target.ID).Return(target, nil)
			store.EXPECT().ListProfitSharingReceiverAttemptsByTargetPaginated(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.ListProfitSharingReceiverAttemptsByTargetPaginatedParams) ([]db.ProfitSharingReceiverAttempt, error) {
				require.Equal(t, target.ID, arg.TargetID)
				require.Equal(t, int32(0), arg.OffsetCount)
				require.Equal(t, int32(5), arg.LimitCount)
				return []db.ProfitSharingReceiverAttempt{attempt}, nil
			})
			store.EXPECT().CountProfitSharingReceiverAttemptsByTarget(gomock.Any(), target.ID).Return(int64(6), nil)
		},
	})

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp listProfitSharingReceiverLifecycleAttemptsResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, target.ID, resp.Target.ID)
	require.Equal(t, int64(6), resp.Total)
	require.Equal(t, int64(2), resp.TotalPages)
	require.True(t, resp.HasMore)
	require.Len(t, resp.Items, 1)
	require.Equal(t, attempt.ID, resp.Items[0].ID)
	require.Equal(t, "sanitized upstream unavailable", *resp.Items[0].ErrorMessage)

	entries := auditWriter.Entries()
	require.Len(t, entries, 1)
	require.Equal(t, "profit_sharing_receiver_lifecycle_attempts_viewed", entries[0].Action)
	require.Equal(t, target.ID, *entries[0].TargetID)
}

func TestGetProfitSharingReceiverLifecycleTargetAPI_NotFound(t *testing.T) {
	admin, _ := randomUser(t)
	targetID := int64(99901)

	recorder := performProfitSharingReceiverLifecycleRequest(t, profitSharingReceiverLifecycleRequestTestCase{
		admin:  admin,
		method: http.MethodGet,
		url:    "/v1/platform/profit-sharing/receiver-lifecycle/targets/" + fmt.Sprint(targetID),
		buildStubs: func(store *mockdb.MockStore, _ *mockwk.MockTaskDistributor, _ *mockwechat.MockEcommerceClientInterface) {
			expectAdminRole(store, admin.ID)
			store.EXPECT().GetProfitSharingReceiverTarget(gomock.Any(), targetID).Return(db.ProfitSharingReceiverTarget{}, db.ErrRecordNotFound)
		},
	})

	require.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestRepairProfitSharingReceiverLifecycleAPI_OperatorPresentWritesTargetAndEnqueues(t *testing.T) {
	admin, _ := randomUser(t)

	recorder := performProfitSharingReceiverLifecycleRepairRequest(t, profitSharingReceiverLifecycleRepairRequestTestCase{
		admin: admin,
		body: map[string]any{
			"owner_type":    db.ProfitSharingReceiverOwnerTypeOperator,
			"owner_id":      int64(2101),
			"desired_state": db.ProfitSharingReceiverDesiredStatePresent,
		},
		buildStubs: func(store *mockdb.MockStore, _ *mockwk.MockTaskDistributor, _ *mockwechat.MockEcommerceClientInterface) {
			expectAdminRole(store, admin.ID)
		},
	})

	requireOrdinaryUnsupportedReceiverLifecycleAPIResponse(t, recorder)
}
func TestRepairProfitSharingReceiverLifecycleAPI_RiderAbsentWritesTargetAndEnqueues(t *testing.T) {
	admin, _ := randomUser(t)

	recorder := performProfitSharingReceiverLifecycleRepairRequest(t, profitSharingReceiverLifecycleRepairRequestTestCase{
		admin: admin,
		body: map[string]any{
			"owner_type":    db.ProfitSharingReceiverOwnerTypeRider,
			"owner_id":      int64(2201),
			"desired_state": db.ProfitSharingReceiverDesiredStateAbsent,
		},
		buildStubs: func(store *mockdb.MockStore, _ *mockwk.MockTaskDistributor, _ *mockwechat.MockEcommerceClientInterface) {
			expectAdminRole(store, admin.ID)
		},
	})

	requireOrdinaryUnsupportedReceiverLifecycleAPIResponse(t, recorder)
}
func TestRepairProfitSharingReceiverLifecycleAPI_InvalidRequest(t *testing.T) {
	admin, _ := randomUser(t)

	recorder := performProfitSharingReceiverLifecycleRepairRequest(t, profitSharingReceiverLifecycleRepairRequestTestCase{
		admin: admin,
		body: map[string]any{
			"owner_type":    "payment_order",
			"owner_id":      int64(1),
			"desired_state": db.ProfitSharingReceiverDesiredStatePresent,
		},
		buildStubs: func(store *mockdb.MockStore, _ *mockwk.MockTaskDistributor, _ *mockwechat.MockEcommerceClientInterface) {
			expectAdminRole(store, admin.ID)
		},
	})

	requireOrdinaryUnsupportedReceiverLifecycleAPIResponse(t, recorder)
}
func TestRepairProfitSharingReceiverLifecycleAPI_OwnerNotFound(t *testing.T) {
	admin, _ := randomUser(t)

	recorder := performProfitSharingReceiverLifecycleRepairRequest(t, profitSharingReceiverLifecycleRepairRequestTestCase{
		admin: admin,
		body: map[string]any{
			"owner_type":    db.ProfitSharingReceiverOwnerTypeOperator,
			"owner_id":      int64(2401),
			"desired_state": db.ProfitSharingReceiverDesiredStatePresent,
		},
		buildStubs: func(store *mockdb.MockStore, _ *mockwk.MockTaskDistributor, _ *mockwechat.MockEcommerceClientInterface) {
			expectAdminRole(store, admin.ID)
		},
	})

	requireOrdinaryUnsupportedReceiverLifecycleAPIResponse(t, recorder)
}
func TestRepairProfitSharingReceiverLifecycleAPI_MissingOpenIDReturnsBadRequest(t *testing.T) {
	admin, _ := randomUser(t)

	recorder := performProfitSharingReceiverLifecycleRepairRequest(t, profitSharingReceiverLifecycleRepairRequestTestCase{
		admin: admin,
		body: map[string]any{
			"owner_type":    db.ProfitSharingReceiverOwnerTypeRider,
			"owner_id":      int64(2501),
			"desired_state": db.ProfitSharingReceiverDesiredStatePresent,
		},
		buildStubs: func(store *mockdb.MockStore, _ *mockwk.MockTaskDistributor, _ *mockwechat.MockEcommerceClientInterface) {
			expectAdminRole(store, admin.ID)
		},
	})

	requireOrdinaryUnsupportedReceiverLifecycleAPIResponse(t, recorder)
}
func TestRepairProfitSharingReceiverLifecycleAPI_EnqueueFailure(t *testing.T) {
	admin, _ := randomUser(t)

	recorder := performProfitSharingReceiverLifecycleRepairRequest(t, profitSharingReceiverLifecycleRepairRequestTestCase{
		admin: admin,
		body: map[string]any{
			"owner_type":    db.ProfitSharingReceiverOwnerTypeRider,
			"owner_id":      int64(2301),
			"desired_state": db.ProfitSharingReceiverDesiredStatePresent,
		},
		buildStubs: func(store *mockdb.MockStore, _ *mockwk.MockTaskDistributor, _ *mockwechat.MockEcommerceClientInterface) {
			expectAdminRole(store, admin.ID)
		},
	})

	requireOrdinaryUnsupportedReceiverLifecycleAPIResponse(t, recorder)
}

func requireOrdinaryUnsupportedReceiverLifecycleAPIResponse(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var resp APIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	require.Equal(t, CodeServiceUnavail, resp.Code)
	require.Equal(t, ordinaryServiceProviderUnsupportedReceiverLifecycleMessage, resp.Message)
}

type profitSharingReceiverLifecycleRepairRequestTestCase struct {
	admin       db.User
	auditWriter *auditSpyWriter
	body        map[string]any
	buildStubs  func(store *mockdb.MockStore, distributor *mockwk.MockTaskDistributor, ecommerceClient *mockwechat.MockEcommerceClientInterface)
}

func performProfitSharingReceiverLifecycleRepairRequest(t *testing.T, tc profitSharingReceiverLifecycleRepairRequestTestCase) *httptest.ResponseRecorder {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
	tc.buildStubs(store, distributor, ecommerceClient)

	server := newTestServerWithTaskDistributor(t, store, distributor)
	server.SetEcommerceClientForTest(ecommerceClient)
	if tc.auditWriter != nil {
		server.auditWriter = tc.auditWriter
	}

	bodyBytes, err := json.Marshal(tc.body)
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/platform/profit-sharing/receiver-lifecycle/repair", bytes.NewReader(bodyBytes))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, tc.admin.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)
	return recorder
}

func expectAdminRole(store *mockdb.MockStore, userID int64) {
	store.EXPECT().
		ListUserRoles(gomock.Any(), userID).
		Return([]db.UserRole{{UserID: userID, Role: "admin", Status: "active"}}, nil)
}

func profitSharingReceiverLifecycleRepairTarget(ownerID int64, ownerType string, desiredState string) db.ProfitSharingReceiverTarget {
	now := time.Date(2026, 4, 25, 10, 0, 0, 0, time.UTC)
	return db.ProfitSharingReceiverTarget{
		ID:           ownerID + 9000,
		Provider:     db.ExternalPaymentProviderWechat,
		Channel:      db.PaymentChannelEcommerce,
		OwnerType:    ownerType,
		OwnerID:      ownerID,
		ReceiverType: db.ProfitSharingReceiverTypePersonalOpenID,
		Appid:        "wx_sp_app_repair",
		AccountHash:  "hashed-account",
		DesiredState: desiredState,
		SyncStatus:   db.ProfitSharingReceiverSyncStatusPending,
		AttemptCount: 1,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}
