package api

import (
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

func randomPrinterReconciliationJob(merchantID int64) db.CloudPrinterReconciliationJob {
	now := time.Now()
	return db.CloudPrinterReconciliationJob{
		ID:            util.RandomInt(1, 1000),
		MerchantID:    merchantID,
		PrinterID:     pgtype.Int8{Int64: util.RandomInt(1, 1000), Valid: true},
		PrinterName:   "前台打印机",
		PrinterSn:     "SN-RECON-001",
		PrinterKey:    pgtype.Text{String: "KEY-RECON-001", Valid: true},
		PrinterType:   printerTypeFeieyun,
		DesiredAction: db.CloudPrinterReconciliationActionRegister,
		SourceAction:  db.CloudPrinterReconciliationSourceDelete,
		Status:        db.CloudPrinterReconciliationStatusPending,
		FailureReason: "local change failed: delete failed; remote compensation failed: add failed",
		LastError:     "add failed",
		RetryCount:    1,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func TestListPrinterReconciliationJobsAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	job := randomPrinterReconciliationJob(merchant.ID)

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OKDefaultPending",
			query: "",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).Times(1).Return(merchant, nil)
				store.EXPECT().ListCloudPrinterReconciliationJobsByMerchant(gomock.Any(), gomock.Eq(db.ListCloudPrinterReconciliationJobsByMerchantParams{
					MerchantID: merchant.ID,
					Status:     pgtype.Text{String: db.CloudPrinterReconciliationStatusPending, Valid: true},
				})).Times(1).Return([]db.CloudPrinterReconciliationJob{job}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp printerReconciliationJobListResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Len(t, resp.Jobs, 1)
				require.True(t, resp.Jobs[0].CanRetry)
				require.Equal(t, db.CloudPrinterReconciliationActionRegister, resp.Jobs[0].DesiredAction)
			},
		},
		{
			name:  "ResolvedFilter",
			query: "?status=resolved",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				resolvedJob := job
				resolvedAt := time.Now()
				resolvedJob.Status = db.CloudPrinterReconciliationStatusResolved
				resolvedJob.ResolvedAt = pgtype.Timestamptz{Time: resolvedAt, Valid: true}
				store.EXPECT().GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).Times(1).Return(merchant, nil)
				store.EXPECT().ListCloudPrinterReconciliationJobsByMerchant(gomock.Any(), gomock.Eq(db.ListCloudPrinterReconciliationJobsByMerchantParams{
					MerchantID: merchant.ID,
					Status:     pgtype.Text{String: db.CloudPrinterReconciliationStatusResolved, Valid: true},
				})).Times(1).Return([]db.CloudPrinterReconciliationJob{resolvedJob}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp printerReconciliationJobListResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Len(t, resp.Jobs, 1)
				require.False(t, resp.Jobs[0].CanRetry)
				require.NotNil(t, resp.Jobs[0].ResolvedAt)
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

			request, err := http.NewRequest(http.MethodGet, "/v1/merchant/devices/reconciliation-jobs"+tc.query, nil)
			require.NoError(t, err)
			tc.setupAuth(t, request, server.tokenMaker)

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestRetryPrinterReconciliationJobAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	job := randomPrinterReconciliationJob(merchant.ID)

	testCases := []struct {
		name          string
		job           db.CloudPrinterReconciliationJob
		buildClient   func() *printerClientStub
		buildStubs    func(store *mockdb.MockStore, job db.CloudPrinterReconciliationJob)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder, client *printerClientStub)
	}{
		{
			name: "OK",
			job:  job,
			buildClient: func() *printerClientStub {
				return &printerClientStub{}
			},
			buildStubs: func(store *mockdb.MockStore, job db.CloudPrinterReconciliationJob) {
				resolvedJob := job
				resolvedJob.Status = db.CloudPrinterReconciliationStatusResolved
				resolvedJob.RetryCount = job.RetryCount + 1
				resolvedJob.ResolvedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
				store.EXPECT().GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).Times(1).Return(merchant, nil)
				store.EXPECT().GetCloudPrinterReconciliationJob(gomock.Any(), gomock.Eq(job.ID)).Times(1).Return(job, nil)
				store.EXPECT().ResolveCloudPrinterReconciliationJob(gomock.Any(), gomock.Eq(job.ID)).Times(1).Return(resolvedJob, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder, client *printerClientStub) {
				require.Equal(t, http.StatusOK, recorder.Code)
				require.Len(t, client.addInputs, 1)
				require.Equal(t, job.PrinterSn, client.addInputs[0].SN)
				var resp printerReconciliationJobResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, db.CloudPrinterReconciliationStatusResolved, resp.Status)
			},
		},
		{
			name: "RetryFailure",
			job:  job,
			buildClient: func() *printerClientStub {
				return &printerClientStub{addErr: fmt.Errorf("provider unavailable")}
			},
			buildStubs: func(store *mockdb.MockStore, job db.CloudPrinterReconciliationJob) {
				failedJob := job
				failedJob.RetryCount = job.RetryCount + 1
				failedJob.LastError = "provider unavailable"
				store.EXPECT().GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).Times(1).Return(merchant, nil)
				store.EXPECT().GetCloudPrinterReconciliationJob(gomock.Any(), gomock.Eq(job.ID)).Times(1).Return(job, nil)
				store.EXPECT().FailCloudPrinterReconciliationJobRetry(gomock.Any(), gomock.Eq(db.FailCloudPrinterReconciliationJobRetryParams{
					ID:        job.ID,
					LastError: "provider unavailable",
				})).Times(1).Return(failedJob, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder, client *printerClientStub) {
				require.Equal(t, http.StatusBadGateway, recorder.Code)
				require.Len(t, client.addInputs, 1)
				var resp APIResponse
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
				require.Equal(t, CodeBadGateway, resp.Code)
				require.Equal(t, "internal server error", resp.Message)
			},
		},
		{
			name: "AlreadyResolved",
			job: func() db.CloudPrinterReconciliationJob {
				resolvedJob := job
				resolvedJob.Status = db.CloudPrinterReconciliationStatusResolved
				resolvedJob.ResolvedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
				return resolvedJob
			}(),
			buildClient: func() *printerClientStub {
				return &printerClientStub{}
			},
			buildStubs: func(store *mockdb.MockStore, job db.CloudPrinterReconciliationJob) {
				store.EXPECT().GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).Times(1).Return(merchant, nil)
				store.EXPECT().GetCloudPrinterReconciliationJob(gomock.Any(), gomock.Eq(job.ID)).Times(1).Return(job, nil)
				store.EXPECT().ResolveCloudPrinterReconciliationJob(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder, client *printerClientStub) {
				require.Equal(t, http.StatusOK, recorder.Code)
				require.Empty(t, client.addInputs)
				var resp printerReconciliationJobResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, db.CloudPrinterReconciliationStatusResolved, resp.Status)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store, tc.job)

			server := newTestServer(t, store)
			client := tc.buildClient()
			server.SetPrinterClientForTest(client)
			recorder := httptest.NewRecorder()

			request, err := http.NewRequest(http.MethodPost, fmt.Sprintf("/v1/merchant/devices/reconciliation-jobs/%d/retry", tc.job.ID), nil)
			require.NoError(t, err)
			addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder, client)
		})
	}
}
