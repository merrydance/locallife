package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newJSONTestContext(method, target, body string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx, recorder
}

func findRuleByKey(t *testing.T, rules []RuleItem, key string) RuleItem {
	t.Helper()
	for _, rule := range rules {
		if rule.Key == key {
			return rule
		}
	}
	t.Fatalf("rule %s not found", key)
	return RuleItem{}
}

func findPlatformRuleByKey(t *testing.T, rules []platformOperatorRuleItem, key string) platformOperatorRuleItem {
	t.Helper()
	for _, rule := range rules {
		if rule.Key == key {
			return rule
		}
	}
	t.Fatalf("rule %s not found", key)
	return platformOperatorRuleItem{}
}

func TestListOperatorRules_RiderDepositUsesOperatorConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	operator := db.Operator{
		ID:                8,
		UserID:            88,
		RegionID:          12,
		RiderDeposit:      26000,
		MerchantDeposit:   500000,
		WeatherCoeffLight: numericFromFloat(1.1),
	}

	store.EXPECT().
		GetRegionRuleConfigByRegion(gomock.Any(), int64(12)).
		Return(db.RegionRuleConfig{
			RegionID:             12,
			MerchantDeposit:      800000,
			RiderDeposit:         99000,
			WeatherCoeffExtreme:  numericFromFloat(2.0),
			WeatherCoeffHeavy:    numericFromFloat(1.8),
			WeatherCoeffModerate: numericFromFloat(1.5),
			WeatherCoeffLight:    numericFromFloat(1.2),
		}, nil)
	store.EXPECT().
		GetActiveProfitSharingConfig(gomock.Any(), gomock.Any()).
		Return(db.ProfitSharingConfig{}, db.ErrRecordNotFound)
	merchantConfig, err := json.Marshal(operatorDepositConfigValue{AmountFen: 880000})
	require.NoError(t, err)
	store.EXPECT().
		GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{
			ConfigKey: operatorMerchantDepositConfigKey,
			ScopeType: db.PlatformConfigScopeGlobal,
			ScopeID:   pgtype.Int8{Valid: false},
		}).
		Return(db.PlatformConfig{ConfigValue: merchantConfig}, nil)
	store.EXPECT().
		GetActiveDeliveryFeeConfigByRegion(gomock.Any(), int64(12)).
		Return(db.DeliveryFeeConfig{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetLatestWeatherCoefficient(gomock.Any(), int64(12)).
		Return(db.WeatherCoefficient{}, db.ErrRecordNotFound)

	ctx, recorder := newJSONTestContext(http.MethodGet, "/v1/operator/rules", "")
	ctx.Set(operatorKey, operator)

	server.listOperatorRules(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp ListRulesResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	rule := findRuleByKey(t, resp.Rules, "RIDER_DEPOSIT")
	require.Equal(t, "260.00", rule.Value)
	require.True(t, rule.Editable)
	require.Contains(t, rule.Desc, "当前运营商配置")
}

func TestUpdateOperatorRule_RiderDepositUpdatesOperatorOnly(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	operator := db.Operator{ID: 8, UserID: 88, RegionID: 12, RiderDeposit: 26000}

	store.EXPECT().
		UpdateOperatorRules(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.UpdateOperatorRulesParams) (db.Operator, error) {
			require.Equal(t, operator.ID, arg.ID)
			require.True(t, arg.RiderDeposit.Valid)
			require.Equal(t, int64(27000), arg.RiderDeposit.Int64)
			return operator, nil
		})
	store.EXPECT().
		ListRidersByRegion(gomock.Any(), db.ListRidersByRegionParams{
			RegionID: pgtype.Int8{Int64: 12, Valid: true},
			Limit:    riderOperationalStatusSyncPageSize,
			Offset:   0,
		}).
		Return([]db.Rider{}, nil)

	ctx, recorder := newJSONTestContext(http.MethodPatch, "/v1/operator/rules/RIDER_DEPOSIT", `{"value":"270"}`)
	ctx.Params = gin.Params{{Key: "key", Value: "RIDER_DEPOSIT"}}
	ctx.Set(operatorKey, operator)

	server.updateOperatorRule(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestListPlatformOperatorRules_RiderDepositUsesPlatformDefaultInsteadOfBaseline(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	store.EXPECT().
		GetActiveProfitSharingConfig(gomock.Any(), gomock.Any()).
		Return(db.ProfitSharingConfig{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetPlatformOperatorRuleBaselineFromRegion(gomock.Any()).
		Return(db.GetPlatformOperatorRuleBaselineFromRegionRow{MerchantDeposit: 520000, RiderDeposit: 99000}, nil)
	store.EXPECT().
		GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{
			ConfigKey: merchantDepositConfigKey,
			ScopeType: db.PlatformConfigScopeGlobal,
			ScopeID:   pgtype.Int8{Valid: false},
		}).
		Return(db.PlatformConfig{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{
			ConfigKey: riderDepositConfigKey,
			ScopeType: db.PlatformConfigScopeGlobal,
			ScopeID:   pgtype.Int8{Valid: false},
		}).
		Return(db.PlatformConfig{}, db.ErrRecordNotFound)

	ctx, recorder := newJSONTestContext(http.MethodGet, "/v1/platform/operator-rules", "")
	ctx.Set(authorizationPayloadKey, &token.Payload{UserID: 1})

	server.listPlatformOperatorRules(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp listPlatformOperatorRulesResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	rule := findPlatformRuleByKey(t, resp.Rules, "RIDER_DEPOSIT")
	require.Equal(t, "200.00", rule.Value)
	require.Contains(t, rule.Desc, "平台默认值")
}

func TestUpdatePlatformOperatorRule_RiderDepositOnlyWritesPlatformDefault(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	store.EXPECT().
		UpsertPlatformConfig(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.UpsertPlatformConfigParams) (db.PlatformConfig, error) {
			require.Equal(t, riderDepositConfigKey, arg.ConfigKey)
			require.Equal(t, db.PlatformConfigScopeGlobal, arg.ScopeType)
			var payload depositConfigValue
			require.NoError(t, json.Unmarshal(arg.ConfigValue, &payload))
			require.Equal(t, int64(31000), payload.AmountFen)
			return db.PlatformConfig{}, nil
		})
	store.EXPECT().
		ListRidersByStatus(gomock.Any(), db.ListRidersByStatusParams{
			Status: db.RiderStatusApproved,
			Limit:  riderOperationalStatusSyncPageSize,
			Offset: 0,
		}).
		Return([]db.Rider{}, nil)
	store.EXPECT().
		ListRidersByStatus(gomock.Any(), db.ListRidersByStatusParams{
			Status: db.RiderStatusActive,
			Limit:  riderOperationalStatusSyncPageSize,
			Offset: 0,
		}).
		Return([]db.Rider{}, nil)

	ctx, recorder := newJSONTestContext(http.MethodPatch, "/v1/platform/operator-rules/RIDER_DEPOSIT", `{"value":"310"}`)
	ctx.Params = gin.Params{{Key: "key", Value: "RIDER_DEPOSIT"}}
	ctx.Set(authorizationPayloadKey, &token.Payload{UserID: 1001})

	server.updatePlatformOperatorRule(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestUpdateOperatorRule_RiderDepositDemotesOnlineRiderWhenThresholdIncreases(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	operator := db.Operator{ID: 8, UserID: 88, RegionID: 12, RiderDeposit: 26000}
	rider := db.Rider{
		ID:            21,
		Status:        db.RiderStatusActive,
		IsOnline:      true,
		DepositAmount: 26000,
		RegionID:      pgtype.Int8{Int64: 12, Valid: true},
	}

	store.EXPECT().
		UpdateOperatorRules(gomock.Any(), gomock.Any()).
		Return(operator, nil)
	store.EXPECT().
		ListRidersByRegion(gomock.Any(), db.ListRidersByRegionParams{
			RegionID: pgtype.Int8{Int64: 12, Valid: true},
			Limit:    riderOperationalStatusSyncPageSize,
			Offset:   0,
		}).
		Return([]db.Rider{rider}, nil)
	store.EXPECT().
		GetActiveOperatorByRegion(gomock.Any(), int64(12)).
		Return(db.Operator{ID: 8, RegionID: 12, RiderDeposit: 27000}, nil)
	store.EXPECT().
		UpdateRiderStatus(gomock.Any(), db.UpdateRiderStatusParams{ID: rider.ID, Status: db.RiderStatusApproved}).
		Return(db.Rider{
			ID:            rider.ID,
			Status:        db.RiderStatusApproved,
			IsOnline:      true,
			DepositAmount: rider.DepositAmount,
			RegionID:      rider.RegionID,
		}, nil)
	store.EXPECT().
		ListRiderActiveDeliveries(gomock.Any(), pgtype.Int8{Int64: rider.ID, Valid: true}).
		Return([]db.Delivery{}, nil)
	store.EXPECT().
		UpdateRiderOnlineStatus(gomock.Any(), db.UpdateRiderOnlineStatusParams{ID: rider.ID, IsOnline: false}).
		Return(db.Rider{
			ID:            rider.ID,
			Status:        db.RiderStatusApproved,
			IsOnline:      false,
			DepositAmount: rider.DepositAmount,
			RegionID:      rider.RegionID,
		}, nil)

	ctx, recorder := newJSONTestContext(http.MethodPatch, "/v1/operator/rules/RIDER_DEPOSIT", `{"value":"270"}`)
	ctx.Params = gin.Params{{Key: "key", Value: "RIDER_DEPOSIT"}}
	ctx.Set(operatorKey, operator)

	server.updateOperatorRule(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestUpdatePlatformOperatorRule_RiderDepositPromotesApprovedRiderUsingPlatformDefault(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	rider := db.Rider{
		ID:            31,
		Status:        db.RiderStatusApproved,
		DepositAmount: 32000,
		RegionID:      pgtype.Int8{Int64: 18, Valid: true},
	}

	store.EXPECT().
		UpsertPlatformConfig(gomock.Any(), gomock.Any()).
		Return(db.PlatformConfig{}, nil)
	store.EXPECT().
		ListRidersByStatus(gomock.Any(), db.ListRidersByStatusParams{
			Status: db.RiderStatusApproved,
			Limit:  riderOperationalStatusSyncPageSize,
			Offset: 0,
		}).
		Return([]db.Rider{rider}, nil)
	store.EXPECT().
		ListRidersByStatus(gomock.Any(), db.ListRidersByStatusParams{
			Status: db.RiderStatusActive,
			Limit:  riderOperationalStatusSyncPageSize,
			Offset: 0,
		}).
		Return([]db.Rider{}, nil)
	store.EXPECT().
		GetActiveOperatorByRegion(gomock.Any(), int64(18)).
		Return(db.Operator{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{
			ConfigKey: riderDepositConfigKey,
			ScopeType: db.PlatformConfigScopeGlobal,
			ScopeID:   pgtype.Int8{Valid: false},
		}).
		Return(db.PlatformConfig{ConfigValue: []byte(`{"amount_fen":31000}`)}, nil)
	store.EXPECT().
		UpdateRiderStatus(gomock.Any(), db.UpdateRiderStatusParams{ID: rider.ID, Status: db.RiderStatusActive}).
		Return(db.Rider{
			ID:            rider.ID,
			Status:        db.RiderStatusActive,
			DepositAmount: rider.DepositAmount,
			RegionID:      rider.RegionID,
		}, nil)

	ctx, recorder := newJSONTestContext(http.MethodPatch, "/v1/platform/operator-rules/RIDER_DEPOSIT", `{"value":"310"}`)
	ctx.Params = gin.Params{{Key: "key", Value: "RIDER_DEPOSIT"}}
	ctx.Set(authorizationPayloadKey, &token.Payload{UserID: 1001})

	server.updatePlatformOperatorRule(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
}
