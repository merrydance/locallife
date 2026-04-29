package api

import (
	"bytes"
	"encoding/json"
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

func TestListOperatorFoodSafetyCases_UsesOperatorRegion(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomOperator(user.ID)
	operator.RegionID = 66

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectActiveOperatorAuth(store, user.ID, operator)
	expectOperatorManagesRegion(store, operator, operator.RegionID, true)
	store.EXPECT().
		ListFoodSafetyCasesByRegion(gomock.Any(), db.ListFoodSafetyCasesByRegionParams{
			RegionID: operator.RegionID,
			Limit:    20,
			Offset:   0,
		}).
		Return([]db.FoodSafetyCase{{
			ID:                  501,
			MerchantID:          9001,
			RegionID:            operator.RegionID,
			PrimaryProductKey:   "dish:301",
			PrimaryProductLabel: "酸辣粉",
			Status:              "merchant-suspended",
			TriggerReason:       "同商户同产品食安举报触发熔断",
			CreatedAt:           time.Now(),
			UpdatedAt:           time.Now(),
		}}, nil)
	store.EXPECT().CountFoodSafetyCasesByRegion(gomock.Any(), operator.RegionID).Return(int64(1), nil)

	server := newTestServer(t, store)
	request, err := http.NewRequest(http.MethodGet, "/v1/operator/food-safety/cases", nil)
	require.NoError(t, err)
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp foodSafetyCaseListResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Len(t, resp.Items, 1)
	require.EqualValues(t, 1, resp.Total)
	require.False(t, resp.HasMore)
	require.EqualValues(t, 501, resp.Items[0].ID)
	require.Equal(t, "dish:301", resp.Items[0].PrimaryProductKey)
}

func TestResolveOperatorFoodSafetyCase_UsesResolutionTx(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomOperator(user.ID)
	operator.RegionID = 77

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectActiveOperatorAuth(store, user.ID, operator)
	expectOperatorManagesRegion(store, operator, operator.RegionID, true)
	store.EXPECT().GetFoodSafetyCase(gomock.Any(), int64(81)).Return(db.FoodSafetyCase{
		ID:                  81,
		MerchantID:          2001,
		RegionID:            operator.RegionID,
		PrimaryProductKey:   "dish:901",
		PrimaryProductLabel: "黄焖鸡",
		Status:              "investigating",
		TriggerReason:       "同商户同产品食安举报触发熔断",
		InvestigationReport: pgtype.Text{String: "已现场核查", Valid: true},
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}, nil)
	store.EXPECT().ResolveFoodSafetyCaseTx(gomock.Any(), db.ResolveFoodSafetyCaseTxParams{
		CaseID:                      81,
		RegionID:                    operator.RegionID,
		InvestigationReport:         "现场核查确认同批次餐品存在风险",
		MerchantRectificationReport: "商户已完成后厨消杀并更换涉事原料",
		Resolution:                  "监管上报完成，同意恢复营业",
	}).Return(db.ResolveFoodSafetyCaseTxResult{Case: db.FoodSafetyCase{
		ID:                          81,
		MerchantID:                  2001,
		RegionID:                    operator.RegionID,
		PrimaryProductKey:           "dish:901",
		PrimaryProductLabel:         "黄焖鸡",
		Status:                      "resolved",
		TriggerReason:               "同商户同产品食安举报触发熔断",
		InvestigationReport:         pgtype.Text{String: "现场核查确认同批次餐品存在风险", Valid: true},
		MerchantRectificationReport: pgtype.Text{String: "商户已完成后厨消杀并更换涉事原料", Valid: true},
		Resolution:                  pgtype.Text{String: "监管上报完成，同意恢复营业", Valid: true},
		ResolvedAt:                  pgtype.Timestamptz{Time: time.Now(), Valid: true},
		CreatedAt:                   time.Now(),
		UpdatedAt:                   time.Now(),
	}}, nil)

	server := newTestServer(t, store)
	body, err := json.Marshal(map[string]any{
		"investigation_report":          "现场核查确认同批次餐品存在风险",
		"merchant_rectification_report": "商户已完成后厨消杀并更换涉事原料",
		"resolution":                    "监管上报完成，同意恢复营业",
	})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/operator/food-safety/cases/81/resolve", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resolved db.FoodSafetyCase
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resolved)
	require.Equal(t, int64(81), resolved.ID)
	require.Equal(t, "resolved", resolved.Status)
	require.Equal(t, "黄焖鸡", resolved.PrimaryProductLabel)
}

func TestResolveOperatorFoodSafetyCase_RejectsMissingInvestigationReport(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomOperator(user.ID)
	operator.RegionID = 78

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectActiveOperatorAuth(store, user.ID, operator)
	expectOperatorManagesRegion(store, operator, operator.RegionID, true)
	store.EXPECT().GetFoodSafetyCase(gomock.Any(), int64(82)).Return(db.FoodSafetyCase{
		ID:                  82,
		MerchantID:          2002,
		RegionID:            operator.RegionID,
		PrimaryProductKey:   "dish:902",
		PrimaryProductLabel: "宫保鸡丁",
		Status:              "merchant-suspended",
		TriggerReason:       "同商户同产品食安举报触发熔断",
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}, nil)

	server := newTestServer(t, store)
	body, err := json.Marshal(map[string]any{
		"merchant_rectification_report": "商户已停用涉事原料并完成后厨清洁消杀。",
		"resolution":                    "整改材料不完整前不得恢复营业。",
	})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/operator/food-safety/cases/82/resolve", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "investigation report")
}

func TestOperatorFoodSafetyCaseDetailAndActions_ForbidCrossRegion(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomOperator(user.ID)
	operator.RegionID = 79

	caseRecord := db.FoodSafetyCase{
		ID:                  83,
		MerchantID:          2003,
		RegionID:            999,
		PrimaryProductKey:   "dish:903",
		PrimaryProductLabel: "水煮鱼",
		Status:              "merchant-suspended",
		TriggerReason:       "同商户同产品食安举报触发熔断",
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	testCases := []struct {
		name   string
		method string
		url    string
		body   map[string]any
	}{
		{name: "Detail", method: http.MethodGet, url: "/v1/operator/food-safety/cases/83"},
		{name: "Investigate", method: http.MethodPost, url: "/v1/operator/food-safety/cases/83/investigate", body: map[string]any{
			"investigation_report": "运营商现场核查发现该案件不属于本区域。",
		}},
		{name: "Resolve", method: http.MethodPost, url: "/v1/operator/food-safety/cases/83/resolve", body: map[string]any{
			"investigation_report":          "运营商现场核查发现该案件不属于本区域。",
			"merchant_rectification_report": "商户提交整改材料但当前运营商无权处理。",
			"resolution":                    "跨区域案件不得由当前运营商结案。",
		}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			expectActiveOperatorAuth(store, user.ID, operator)
			expectOperatorManagesRegion(store, operator, operator.RegionID, true)
			store.EXPECT().GetFoodSafetyCase(gomock.Any(), int64(83)).Return(caseRecord, nil)

			server := newTestServer(t, store)
			request := newOperatorFoodSafetyCaseTestRequest(t, tc.method, tc.url, tc.body)
			addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

			recorder := httptest.NewRecorder()
			server.router.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusForbidden, recorder.Code)
		})
	}
}

func TestInvestigateOperatorFoodSafetyCase_RejectsResolvedUpdateRace(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomOperator(user.ID)
	operator.RegionID = 80

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectActiveOperatorAuth(store, user.ID, operator)
	expectOperatorManagesRegion(store, operator, operator.RegionID, true)
	store.EXPECT().GetFoodSafetyCase(gomock.Any(), int64(84)).Return(db.FoodSafetyCase{
		ID:                  84,
		MerchantID:          2004,
		RegionID:            operator.RegionID,
		PrimaryProductKey:   "dish:904",
		PrimaryProductLabel: "麻辣烫",
		Status:              "merchant-suspended",
		TriggerReason:       "同商户同产品食安举报触发熔断",
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}, nil)
	store.EXPECT().UpdateFoodSafetyCaseInvestigation(gomock.Any(), gomock.Any()).Return(db.FoodSafetyCase{}, db.ErrRecordNotFound)

	server := newTestServer(t, store)
	request := newOperatorFoodSafetyCaseTestRequest(t, http.MethodPost, "/v1/operator/food-safety/cases/84/investigate", map[string]any{
		"investigation_report": "运营商提交调查时案件已被其他流程结案。",
	})
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "resolved case cannot be investigated again")
}

func TestResolveOperatorFoodSafetyCase_RejectsAlreadyResolved(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomOperator(user.ID)
	operator.RegionID = 81

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectActiveOperatorAuth(store, user.ID, operator)
	expectOperatorManagesRegion(store, operator, operator.RegionID, true)
	store.EXPECT().GetFoodSafetyCase(gomock.Any(), int64(85)).Return(db.FoodSafetyCase{
		ID:                  85,
		MerchantID:          2005,
		RegionID:            operator.RegionID,
		PrimaryProductKey:   "dish:905",
		PrimaryProductLabel: "冒菜",
		Status:              "resolved",
		TriggerReason:       "同商户同产品食安举报触发熔断",
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}, nil)

	server := newTestServer(t, store)
	request := newOperatorFoodSafetyCaseTestRequest(t, http.MethodPost, "/v1/operator/food-safety/cases/85/resolve", map[string]any{
		"investigation_report":          "案件此前已经完成调查并结案。",
		"merchant_rectification_report": "商户此前已经完成整改并提交材料。",
		"resolution":                    "重复结案请求应被稳定拒绝。",
	})
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "already resolved")
}

func newOperatorFoodSafetyCaseTestRequest(t *testing.T, method, url string, body map[string]any) *http.Request {
	t.Helper()

	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		data, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(data)
	}

	request, err := http.NewRequest(method, url, reader)
	require.NoError(t, err)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	return request
}
