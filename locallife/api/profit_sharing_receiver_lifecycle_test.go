package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/merrydance/locallife/worker"
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
	operatorUser := db.User{ID: 2101, WechatOpenid: "operator-openid-repair"}
	operator := randomOperator(operatorUser.ID)
	target := profitSharingReceiverLifecycleRepairTarget(operator.ID, db.ProfitSharingReceiverOwnerTypeOperator, db.ProfitSharingReceiverDesiredStatePresent)
	auditWriter := &auditSpyWriter{}

	recorder := performProfitSharingReceiverLifecycleRepairRequest(t, profitSharingReceiverLifecycleRepairRequestTestCase{
		admin:       admin,
		auditWriter: auditWriter,
		body: map[string]any{
			"owner_type":    db.ProfitSharingReceiverOwnerTypeOperator,
			"owner_id":      operator.ID,
			"desired_state": db.ProfitSharingReceiverDesiredStatePresent,
		},
		buildStubs: func(store *mockdb.MockStore, distributor *mockwk.MockTaskDistributor, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
			expectAdminRole(store, admin.ID)
			store.EXPECT().GetOperator(gomock.Any(), operator.ID).Return(operator, nil)
			store.EXPECT().GetUser(gomock.Any(), operator.UserID).Return(operatorUser, nil)
			ecommerceClient.EXPECT().GetSpAppID().Return("wx_sp_app_repair")
			store.EXPECT().UpsertProfitSharingReceiverTarget(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.UpsertProfitSharingReceiverTargetParams) (db.ProfitSharingReceiverTarget, error) {
				require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
				require.Equal(t, db.PaymentChannelEcommerce, arg.Channel)
				require.Equal(t, db.ProfitSharingReceiverOwnerTypeOperator, arg.OwnerType)
				require.Equal(t, operator.ID, arg.OwnerID)
				require.Equal(t, db.ProfitSharingReceiverTypePersonalOpenID, arg.ReceiverType)
				require.Equal(t, "wx_sp_app_repair", arg.Appid)
				require.Equal(t, db.ProfitSharingReceiverDesiredStatePresent, arg.DesiredState)
				require.NotEmpty(t, arg.AccountHash)
				require.NotContains(t, arg.AccountHash, operatorUser.WechatOpenid)
				return target, nil
			})
			expectReceiverTargetEnqueued(t, distributor, target.ID, nil)
		},
	})

	require.Equal(t, http.StatusAccepted, recorder.Code)
	var resp repairProfitSharingReceiverLifecycleResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.True(t, resp.Enqueued)
	require.Equal(t, target.ID, resp.Target.ID)
	require.Equal(t, db.ProfitSharingReceiverOwnerTypeOperator, resp.Target.OwnerType)
	require.NotContains(t, recorder.Body.String(), operatorUser.WechatOpenid)

	entries := auditWriter.Entries()
	require.Len(t, entries, 1)
	require.Equal(t, admin.ID, entries[0].ActorUserID)
	require.Equal(t, "platform", entries[0].ActorRole)
	require.Equal(t, "profit_sharing_receiver_lifecycle_repair_requested", entries[0].Action)
	require.Equal(t, "profit_sharing_receiver_target", entries[0].TargetType)
	require.NotNil(t, entries[0].TargetID)
	require.Equal(t, target.ID, *entries[0].TargetID)
	require.Equal(t, db.ProfitSharingReceiverOwnerTypeOperator, entries[0].Metadata["owner_type"])
	require.Equal(t, operator.ID, entries[0].Metadata["owner_id"])
	require.Equal(t, db.ProfitSharingReceiverDesiredStatePresent, entries[0].Metadata["desired_state"])
	require.Equal(t, true, entries[0].Metadata["enqueued"])
}

func TestRepairProfitSharingReceiverLifecycleAPI_RiderAbsentWritesTargetAndEnqueues(t *testing.T) {
	admin, _ := randomUser(t)
	riderUser := db.User{ID: 2201, WechatOpenid: "rider-openid-repair"}
	rider := randomRider(riderUser.ID)
	target := profitSharingReceiverLifecycleRepairTarget(rider.ID, db.ProfitSharingReceiverOwnerTypeRider, db.ProfitSharingReceiverDesiredStateAbsent)

	recorder := performProfitSharingReceiverLifecycleRepairRequest(t, profitSharingReceiverLifecycleRepairRequestTestCase{
		admin: admin,
		body: map[string]any{
			"owner_type":    db.ProfitSharingReceiverOwnerTypeRider,
			"owner_id":      rider.ID,
			"desired_state": db.ProfitSharingReceiverDesiredStateAbsent,
		},
		buildStubs: func(store *mockdb.MockStore, distributor *mockwk.MockTaskDistributor, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
			expectAdminRole(store, admin.ID)
			store.EXPECT().GetRider(gomock.Any(), rider.ID).Return(rider, nil)
			store.EXPECT().GetUser(gomock.Any(), rider.UserID).Return(riderUser, nil)
			ecommerceClient.EXPECT().GetSpAppID().Return("wx_sp_app_repair")
			store.EXPECT().UpsertProfitSharingReceiverTarget(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.UpsertProfitSharingReceiverTargetParams) (db.ProfitSharingReceiverTarget, error) {
				require.Equal(t, db.ProfitSharingReceiverOwnerTypeRider, arg.OwnerType)
				require.Equal(t, rider.ID, arg.OwnerID)
				require.Equal(t, db.ProfitSharingReceiverDesiredStateAbsent, arg.DesiredState)
				require.NotEmpty(t, arg.DisplayNameHash.String)
				require.NotContains(t, arg.DisplayNameHash.String, rider.RealName)
				return target, nil
			})
			expectReceiverTargetEnqueued(t, distributor, target.ID, nil)
		},
	})

	require.Equal(t, http.StatusAccepted, recorder.Code)
	var resp repairProfitSharingReceiverLifecycleResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.True(t, resp.Enqueued)
	require.Equal(t, db.ProfitSharingReceiverOwnerTypeRider, resp.Target.OwnerType)
	require.Equal(t, db.ProfitSharingReceiverDesiredStateAbsent, resp.Target.DesiredState)
	require.NotContains(t, recorder.Body.String(), riderUser.WechatOpenid)
}

func TestRepairProfitSharingReceiverLifecycleAPI_InvalidRequest(t *testing.T) {
	admin, _ := randomUser(t)

	recorder := performProfitSharingReceiverLifecycleRepairRequest(t, profitSharingReceiverLifecycleRepairRequestTestCase{
		admin: admin,
		body: map[string]any{
			"owner_type":    "payment_order",
			"owner_id":      1,
			"desired_state": db.ProfitSharingReceiverDesiredStatePresent,
		},
		buildStubs: func(store *mockdb.MockStore, _ *mockwk.MockTaskDistributor, _ *mockwechat.MockEcommerceClientInterface) {
			expectAdminRole(store, admin.ID)
		},
	})

	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestRepairProfitSharingReceiverLifecycleAPI_OwnerNotFound(t *testing.T) {
	admin, _ := randomUser(t)
	operatorID := int64(2401)
	auditWriter := &auditSpyWriter{}

	recorder := performProfitSharingReceiverLifecycleRepairRequest(t, profitSharingReceiverLifecycleRepairRequestTestCase{
		admin:       admin,
		auditWriter: auditWriter,
		body: map[string]any{
			"owner_type":    db.ProfitSharingReceiverOwnerTypeOperator,
			"owner_id":      operatorID,
			"desired_state": db.ProfitSharingReceiverDesiredStatePresent,
		},
		buildStubs: func(store *mockdb.MockStore, _ *mockwk.MockTaskDistributor, _ *mockwechat.MockEcommerceClientInterface) {
			expectAdminRole(store, admin.ID)
			store.EXPECT().GetOperator(gomock.Any(), operatorID).Return(db.Operator{}, db.ErrRecordNotFound)
		},
	})

	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Empty(t, auditWriter.Entries())
}

func TestRepairProfitSharingReceiverLifecycleAPI_MissingOpenIDReturnsBadRequest(t *testing.T) {
	admin, _ := randomUser(t)
	riderUser := db.User{ID: 2501, WechatOpenid: " "}
	rider := randomRider(riderUser.ID)
	auditWriter := &auditSpyWriter{}

	recorder := performProfitSharingReceiverLifecycleRepairRequest(t, profitSharingReceiverLifecycleRepairRequestTestCase{
		admin:       admin,
		auditWriter: auditWriter,
		body: map[string]any{
			"owner_type":    db.ProfitSharingReceiverOwnerTypeRider,
			"owner_id":      rider.ID,
			"desired_state": db.ProfitSharingReceiverDesiredStatePresent,
		},
		buildStubs: func(store *mockdb.MockStore, _ *mockwk.MockTaskDistributor, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
			expectAdminRole(store, admin.ID)
			store.EXPECT().GetRider(gomock.Any(), rider.ID).Return(rider, nil)
			ecommerceClient.EXPECT().GetSpAppID().Return("wx_sp_app_repair")
			store.EXPECT().GetUser(gomock.Any(), rider.UserID).Return(riderUser, nil)
		},
	})

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Empty(t, auditWriter.Entries())
}

func TestRepairProfitSharingReceiverLifecycleAPI_EnqueueFailure(t *testing.T) {
	admin, _ := randomUser(t)
	riderUser := db.User{ID: 2301, WechatOpenid: "rider-openid-enqueue-failed"}
	rider := randomRider(riderUser.ID)
	target := profitSharingReceiverLifecycleRepairTarget(rider.ID, db.ProfitSharingReceiverOwnerTypeRider, db.ProfitSharingReceiverDesiredStatePresent)
	auditWriter := &auditSpyWriter{}

	recorder := performProfitSharingReceiverLifecycleRepairRequest(t, profitSharingReceiverLifecycleRepairRequestTestCase{
		admin:       admin,
		auditWriter: auditWriter,
		body: map[string]any{
			"owner_type":    db.ProfitSharingReceiverOwnerTypeRider,
			"owner_id":      rider.ID,
			"desired_state": db.ProfitSharingReceiverDesiredStatePresent,
		},
		buildStubs: func(store *mockdb.MockStore, distributor *mockwk.MockTaskDistributor, ecommerceClient *mockwechat.MockEcommerceClientInterface) {
			expectAdminRole(store, admin.ID)
			store.EXPECT().GetRider(gomock.Any(), rider.ID).Return(rider, nil)
			store.EXPECT().GetUser(gomock.Any(), rider.UserID).Return(riderUser, nil)
			ecommerceClient.EXPECT().GetSpAppID().Return("wx_sp_app_repair")
			store.EXPECT().UpsertProfitSharingReceiverTarget(gomock.Any(), gomock.Any()).Return(target, nil)
			expectReceiverTargetEnqueued(t, distributor, target.ID, errors.New("redis unavailable"))
		},
	})

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "redis unavailable")

	entries := auditWriter.Entries()
	require.Len(t, entries, 1)
	require.Equal(t, target.ID, *entries[0].TargetID)
	require.Equal(t, false, entries[0].Metadata["enqueued"])
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

func expectReceiverTargetEnqueued(t *testing.T, distributor *mockwk.MockTaskDistributor, targetID int64, enqueueErr error) {
	t.Helper()

	distributor.EXPECT().DistributeTaskProcessProfitSharingReceiverTarget(
		gomock.Any(),
		gomock.AssignableToTypeOf(&worker.ProfitSharingReceiverTargetPayload{}),
		gomock.Any(),
		gomock.Any(),
	).DoAndReturn(func(_ context.Context, payload *worker.ProfitSharingReceiverTargetPayload, _ ...asynq.Option) error {
		require.Equal(t, targetID, payload.TargetID)
		return enqueueErr
	})
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
