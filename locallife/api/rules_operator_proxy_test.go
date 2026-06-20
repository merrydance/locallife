package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestListOperatorRulesProxyAPI(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomOperator(user.ID)
	now := time.Now()

	rule1 := db.Rule{
		ID:        1,
		Name:      "rule_a",
		Category:  "order",
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
	rule2 := db.Rule{
		ID:        2,
		Name:      "rule_b",
		Category:  "order",
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}

	rule1Versions := []db.RuleVersion{{
		ID:        10,
		RuleID:    rule1.ID,
		Version:   1,
		Status:    "published",
		Scope:     []byte(`{"region_id":[` + itoa(operator.RegionID) + `]}`),
		CreatedAt: now,
	}}
	rule2Versions := []db.RuleVersion{{
		ID:        20,
		RuleID:    rule2.ID,
		Version:   1,
		Status:    "published",
		Scope:     []byte(`{"region_id":[999999]}`),
		CreatedAt: now,
	}}

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListUserRoles(gomock.Any(), user.ID).
					Return([]db.UserRole{{UserID: user.ID, Role: "operator", Status: "active"}}, nil)

				store.EXPECT().
					GetOperatorByUser(gomock.Any(), user.ID).
					Return(operator, nil)

				store.EXPECT().
					ListRules(gomock.Any(), gomock.Any()).
					Return([]db.Rule{rule1, rule2}, nil)

				gomock.InOrder(
					store.EXPECT().
						ListRuleVersionsByRule(gomock.Any(), rule1.ID).
						Return(rule1Versions, nil),
					store.EXPECT().
						ListRuleVersionsByRule(gomock.Any(), rule2.ID).
						Return(rule2Versions, nil),
				)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp struct {
					Rules []db.Rule `json:"rules"`
					Count int       `json:"count"`
				}
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, 1, resp.Count)
				require.Len(t, resp.Rules, 1)
				require.Equal(t, rule1.ID, resp.Rules[0].ID)
			},
		},
		{
			name:       "Unauthorized",
			setupAuth:  func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			if tc.buildStubs != nil {
				tc.buildStubs(store)
			}

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			request, err := http.NewRequest(http.MethodGet, "/v1/operators/me/rules", nil)
			require.NoError(t, err)

			if tc.setupAuth != nil {
				tc.setupAuth(t, request, server.tokenMaker)
			}
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestOperatorRulesProxyWriteAPIsAreDisabled(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomOperator(user.ID)

	createVersionBody := map[string]interface{}{
		"version":     1,
		"status":      "published",
		"priority":    10,
		"scope":       map[string]interface{}{},
		"condition":   map[string]interface{}{"behavior_blocklist": true},
		"action":      map[string]interface{}{"type": "deny"},
		"gray_config": map[string]interface{}{},
	}

	testCases := []struct {
		name string
		path string
		body map[string]interface{}
	}{
		{
			name: "CreateRule",
			path: "/v1/operators/me/rules",
			body: map[string]interface{}{"name": "operator unsafe rule", "category": "order"},
		},
		{
			name: "CreateRuleVersion",
			path: "/v1/operators/me/rules/101/versions",
			body: createVersionBody,
		},
		{
			name: "PublishRule",
			path: "/v1/operators/me/rules/101/publish",
			body: map[string]interface{}{"version_id": 999},
		},
		{
			name: "RollbackRule",
			path: "/v1/operators/me/rules/101/rollback",
			body: map[string]interface{}{"version_id": 998},
		},
		{
			name: "DisableRule",
			path: "/v1/operators/me/rules/101/disable",
			body: map[string]interface{}{"reason": "operator write proxy is not a product surface"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			store.EXPECT().
				ListUserRoles(gomock.Any(), user.ID).
				Return([]db.UserRole{{UserID: user.ID, Role: "operator", Status: "active"}}, nil)
			store.EXPECT().
				GetOperatorByUser(gomock.Any(), user.ID).
				Return(operator, nil)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			request, err := http.NewRequest(http.MethodPost, tc.path, bytes.NewReader(data))
			require.NoError(t, err)

			addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			server.router.ServeHTTP(recorder, request)

			require.Equal(t, http.StatusForbidden, recorder.Code)
		})
	}
}

func TestListOperatorRuleHitsProxyAPI(t *testing.T) {
	user, _ := randomUser(t)
	operator := randomOperator(user.ID)
	ruleID := int64(200)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListUserRoles(gomock.Any(), user.ID).
		Return([]db.UserRole{{UserID: user.ID, Role: "operator", Status: "active"}}, nil)

	store.EXPECT().
		GetOperatorByUser(gomock.Any(), user.ID).
		Return(operator, nil)

	hit := db.RuleHit{
		ID:        1,
		RuleID:    ruleID,
		Domain:    "order",
		Decision:  "deny",
		RegionID:  pgtype.Int8{Int64: operator.RegionID, Valid: true},
		CreatedAt: time.Now(),
	}

	store.EXPECT().
		ListRuleHitsByRuleAndRegion(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.ListRuleHitsByRuleAndRegionParams) ([]db.RuleHit, error) {
			require.Equal(t, ruleID, arg.RuleID)
			require.True(t, arg.RegionID.Valid)
			require.Equal(t, operator.RegionID, arg.RegionID.Int64)
			return []db.RuleHit{hit}, nil
		})

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	request, err := http.NewRequest(http.MethodGet, "/v1/operators/me/rules/hits?rule_id=200", nil)
	require.NoError(t, err)

	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp struct {
		Hits  []db.RuleHit `json:"hits"`
		Count int          `json:"count"`
	}
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, 1, resp.Count)
	require.Len(t, resp.Hits, 1)
}

func itoa(value int64) string {
	return strconv.FormatInt(value, 10)
}
