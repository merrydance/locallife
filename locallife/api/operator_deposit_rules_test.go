package api

import (
	"encoding/json"
	"errors"
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

func mustMarshalDeliveryFeeDefaultConfig(t *testing.T, value deliveryFeeDefaultConfigValue) []byte {
	t.Helper()
	payload, err := json.Marshal(value)
	require.NoError(t, err)
	return payload
}

func weatherOnlyRegionRuleConfig(regionID int64) db.RegionRuleConfig {
	return db.RegionRuleConfig{
		RegionID:             regionID,
		WeatherCoeffExtreme:  numericFromFloat(2.0),
		WeatherCoeffHeavy:    numericFromFloat(1.8),
		WeatherCoeffModerate: numericFromFloat(1.5),
		WeatherCoeffLight:    numericFromFloat(1.2),
	}
}

func TestListOperatorRules_RiderDepositUsesRegionConfigBeforeOperatorConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	operator := db.Operator{
		ID:                8,
		UserID:            88,
		RegionID:          12,
		RiderDeposit:      26000,
		WeatherCoeffLight: numericFromFloat(1.1),
	}

	store.EXPECT().
		GetRegionRuleConfigByRegion(gomock.Any(), int64(12)).
		Return(db.RegionRuleConfig{
			RegionID:             12,
			RiderDeposit:         99000,
			WeatherCoeffExtreme:  numericFromFloat(2.0),
			WeatherCoeffHeavy:    numericFromFloat(1.8),
			WeatherCoeffModerate: numericFromFloat(1.5),
			WeatherCoeffLight:    numericFromFloat(1.2),
		}, nil)
	store.EXPECT().
		GetActiveProfitSharingConfig(gomock.Any(), gomock.Any()).
		Return(db.ProfitSharingConfig{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetDeliveryFeeConfigByRegion(gomock.Any(), int64(12)).
		Return(db.DeliveryFeeConfig{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{
			ConfigKey: deliveryFeeDefaultConfigKey,
			ScopeType: db.PlatformConfigScopeGlobal,
			ScopeID:   pgtype.Int8{Valid: false},
		}).
		Return(db.PlatformConfig{}, db.ErrRecordNotFound)
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
	require.Equal(t, "990.00", rule.Value)
	require.True(t, rule.Editable)
	require.Contains(t, rule.Desc, "当前区域配置")
}

func TestListOperatorRules_RiderDepositUsesPlatformDefaultWhenOperatorStillOnLegacyBaseline(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	operator := db.Operator{
		ID:                8,
		UserID:            88,
		RegionID:          12,
		RiderDeposit:      db.DefaultRiderDepositThresholdFen,
		WeatherCoeffLight: numericFromFloat(1.1),
	}

	store.EXPECT().
		GetRegionRuleConfigByRegion(gomock.Any(), int64(12)).
		Return(weatherOnlyRegionRuleConfig(12), nil)
	store.EXPECT().
		GetActiveProfitSharingConfig(gomock.Any(), gomock.Any()).
		Return(db.ProfitSharingConfig{}, db.ErrRecordNotFound)
	riderConfig, err := json.Marshal(operatorDepositConfigValue{AmountFen: 31000})
	require.NoError(t, err)
	store.EXPECT().
		GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{
			ConfigKey: operatorRiderDepositConfigKey,
			ScopeType: db.PlatformConfigScopeGlobal,
			ScopeID:   pgtype.Int8{Valid: false},
		}).
		Return(db.PlatformConfig{ConfigValue: riderConfig}, nil)
	store.EXPECT().
		GetDeliveryFeeConfigByRegion(gomock.Any(), int64(12)).
		Return(db.DeliveryFeeConfig{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{
			ConfigKey: deliveryFeeDefaultConfigKey,
			ScopeType: db.PlatformConfigScopeGlobal,
			ScopeID:   pgtype.Int8{Valid: false},
		}).
		Return(db.PlatformConfig{}, db.ErrRecordNotFound)
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
	require.Equal(t, "310.00", rule.Value)
	require.Contains(t, rule.Desc, "当前使用平台默认值")
}

func TestListOperatorRules_RiderDepositUsesSystemDefaultWhenNoOverridesExist(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	operator := db.Operator{
		ID:                8,
		UserID:            88,
		RegionID:          12,
		RiderDeposit:      db.DefaultRiderDepositThresholdFen,
		WeatherCoeffLight: numericFromFloat(1.1),
	}

	store.EXPECT().
		GetRegionRuleConfigByRegion(gomock.Any(), int64(12)).
		Return(weatherOnlyRegionRuleConfig(12), nil)
	store.EXPECT().
		GetActiveProfitSharingConfig(gomock.Any(), gomock.Any()).
		Return(db.ProfitSharingConfig{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{
			ConfigKey: operatorRiderDepositConfigKey,
			ScopeType: db.PlatformConfigScopeGlobal,
			ScopeID:   pgtype.Int8{Valid: false},
		}).
		Return(db.PlatformConfig{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetDeliveryFeeConfigByRegion(gomock.Any(), int64(12)).
		Return(db.DeliveryFeeConfig{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{
			ConfigKey: deliveryFeeDefaultConfigKey,
			ScopeType: db.PlatformConfigScopeGlobal,
			ScopeID:   pgtype.Int8{Valid: false},
		}).
		Return(db.PlatformConfig{}, db.ErrRecordNotFound)
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
	require.Equal(t, "200.00", rule.Value)
	require.Contains(t, rule.Desc, "当前使用系统默认值")
}

func TestUpdateOperatorRule_RiderDepositUpdatesRegionWithoutTouchingPlatformDefault(t *testing.T) {
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
		UpsertPlatformConfig(gomock.Any(), db.UpsertPlatformConfigParams{
			ConfigKey:   operatorRiderDepositConfigKey,
			ConfigValue: []byte(`{"amount_fen":27000}`),
			ScopeType:   db.PlatformConfigScopeOperator,
			ScopeID:     pgtype.Int8{Int64: operator.ID, Valid: true},
		}).
		Return(db.PlatformConfig{}, nil)
	store.EXPECT().
		UpsertRegionRuleConfig(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.UpsertRegionRuleConfigParams) (db.RegionRuleConfig, error) {
			require.Equal(t, int64(12), arg.RegionID)
			require.True(t, arg.RiderDeposit.Valid)
			require.Equal(t, int64(27000), arg.RiderDeposit.Int64)
			return db.RegionRuleConfig{RegionID: 12, RiderDeposit: 27000}, nil
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

func TestListOperatorRules_RiderDepositIgnoresOperatorScopedConfigAndUsesPlatformDefault(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	operator := db.Operator{
		ID:                8,
		UserID:            88,
		RegionID:          12,
		RiderDeposit:      db.DefaultRiderDepositThresholdFen,
		WeatherCoeffLight: numericFromFloat(1.1),
	}

	store.EXPECT().
		GetRegionRuleConfigByRegion(gomock.Any(), int64(12)).
		Return(weatherOnlyRegionRuleConfig(12), nil)
	store.EXPECT().
		GetActiveProfitSharingConfig(gomock.Any(), gomock.Any()).
		Return(db.ProfitSharingConfig{}, db.ErrRecordNotFound)
	riderConfig, err := json.Marshal(operatorDepositConfigValue{AmountFen: 31000})
	require.NoError(t, err)
	store.EXPECT().
		GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{
			ConfigKey: operatorRiderDepositConfigKey,
			ScopeType: db.PlatformConfigScopeGlobal,
			ScopeID:   pgtype.Int8{Valid: false},
		}).
		Return(db.PlatformConfig{ConfigValue: riderConfig}, nil)
	store.EXPECT().
		GetDeliveryFeeConfigByRegion(gomock.Any(), int64(12)).
		Return(db.DeliveryFeeConfig{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{
			ConfigKey: deliveryFeeDefaultConfigKey,
			ScopeType: db.PlatformConfigScopeGlobal,
			ScopeID:   pgtype.Int8{Valid: false},
		}).
		Return(db.PlatformConfig{}, db.ErrRecordNotFound)
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
	require.Equal(t, "310.00", rule.Value)
	require.Contains(t, rule.Desc, "当前使用平台默认值")
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
		GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{
			ConfigKey: riderDepositConfigKey,
			ScopeType: db.PlatformConfigScopeGlobal,
			ScopeID:   pgtype.Int8{Valid: false},
		}).
		Return(db.PlatformConfig{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{
			ConfigKey: deliveryFeeDefaultConfigKey,
			ScopeType: db.PlatformConfigScopeGlobal,
			ScopeID:   pgtype.Int8{Valid: false},
		}).
		Return(db.PlatformConfig{}, db.ErrRecordNotFound)

	ctx, recorder := newJSONTestContext(http.MethodGet, "/v1/platform/operator-rules", "")
	ctx.Set(authorizationPayloadKey, &token.Payload{UserID: 1})

	server.listPlatformOperatorRules(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "true", recorder.Header().Get("Deprecation"))
	require.Contains(t, recorder.Header().Get("Link"), "/v1/platform/operational-configs")
	var resp listPlatformOperatorRulesResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	rule := findPlatformRuleByKey(t, resp.Rules, "RIDER_DEPOSIT")
	require.Equal(t, "200.00", rule.Value)
	require.Contains(t, rule.Desc, "平台默认值")
}

func TestListPlatformOperationalConfigs_DoesNotSetDeprecationHeader(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	store.EXPECT().
		GetActiveProfitSharingConfig(gomock.Any(), gomock.Any()).
		Return(db.ProfitSharingConfig{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{
			ConfigKey: riderDepositConfigKey,
			ScopeType: db.PlatformConfigScopeGlobal,
			ScopeID:   pgtype.Int8{Valid: false},
		}).
		Return(db.PlatformConfig{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{
			ConfigKey: deliveryFeeDefaultConfigKey,
			ScopeType: db.PlatformConfigScopeGlobal,
			ScopeID:   pgtype.Int8{Valid: false},
		}).
		Return(db.PlatformConfig{}, db.ErrRecordNotFound)

	ctx, recorder := newJSONTestContext(http.MethodGet, "/v1/platform/operational-configs", "")
	ctx.Set(authorizationPayloadKey, &token.Payload{UserID: 1})

	server.listPlatformOperationalConfigs(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Empty(t, recorder.Header().Get("Deprecation"))
	require.Empty(t, recorder.Header().Get("X-Deprecated-Route"))
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
	require.Equal(t, "true", recorder.Header().Get("Deprecation"))
	require.Contains(t, recorder.Header().Get("Link"), "/v1/platform/operational-configs")
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
		UpsertPlatformConfig(gomock.Any(), db.UpsertPlatformConfigParams{
			ConfigKey:   operatorRiderDepositConfigKey,
			ConfigValue: []byte(`{"amount_fen":27000}`),
			ScopeType:   db.PlatformConfigScopeOperator,
			ScopeID:     pgtype.Int8{Int64: operator.ID, Valid: true},
		}).
		Return(db.PlatformConfig{}, nil)
	store.EXPECT().
		UpsertRegionRuleConfig(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.UpsertRegionRuleConfigParams) (db.RegionRuleConfig, error) {
			require.Equal(t, int64(12), arg.RegionID)
			require.True(t, arg.RiderDeposit.Valid)
			require.Equal(t, int64(27000), arg.RiderDeposit.Int64)
			return db.RegionRuleConfig{RegionID: 12, RiderDeposit: 27000}, nil
		})
	store.EXPECT().
		ListRidersByRegion(gomock.Any(), db.ListRidersByRegionParams{
			RegionID: pgtype.Int8{Int64: 12, Valid: true},
			Limit:    riderOperationalStatusSyncPageSize,
			Offset:   0,
		}).
		Return([]db.Rider{rider}, nil)
	store.EXPECT().
		GetRegionRuleConfigByRegion(gomock.Any(), int64(12)).
		Return(db.RegionRuleConfig{RegionID: 12, RiderDeposit: 27000}, nil)
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

func TestUpdatePlatformOperationalConfig_RiderDepositPromotesApprovedRiderUsingPlatformDefault(t *testing.T) {
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
		GetRegionRuleConfigByRegion(gomock.Any(), int64(18)).
		Return(db.RegionRuleConfig{}, db.ErrRecordNotFound)
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

	ctx, recorder := newJSONTestContext(http.MethodPatch, "/v1/platform/operational-configs/RIDER_DEPOSIT", `{"value":"310"}`)
	ctx.Params = gin.Params{{Key: "key", Value: "RIDER_DEPOSIT"}}
	ctx.Set(authorizationPayloadKey, &token.Payload{UserID: 1001})

	server.updatePlatformOperationalConfig(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Empty(t, recorder.Header().Get("Deprecation"))
	require.Empty(t, recorder.Header().Get("X-Deprecated-Route"))
}

func TestListOperatorRules_DeliveryFeeUsesPlatformDefaultWhenRegionConfigMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	operator := db.Operator{ID: 8, UserID: 88, RegionID: 12, RiderDeposit: 26000}

	store.EXPECT().
		GetRegionRuleConfigByRegion(gomock.Any(), int64(12)).
		Return(db.RegionRuleConfig{RegionID: 12}, nil)
	store.EXPECT().
		GetActiveProfitSharingConfig(gomock.Any(), gomock.Any()).
		Return(db.ProfitSharingConfig{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{
			ConfigKey: operatorRiderDepositConfigKey,
			ScopeType: db.PlatformConfigScopeGlobal,
			ScopeID:   pgtype.Int8{Valid: false},
		}).
		Return(db.PlatformConfig{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetDeliveryFeeConfigByRegion(gomock.Any(), int64(12)).
		Return(db.DeliveryFeeConfig{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{
			ConfigKey: deliveryFeeDefaultConfigKey,
			ScopeType: db.PlatformConfigScopeGlobal,
			ScopeID:   pgtype.Int8{Valid: false},
		}).
		Return(db.PlatformConfig{ConfigValue: mustMarshalDeliveryFeeDefaultConfig(t, deliveryFeeDefaultConfigValue{
			BaseFee:       680,
			BaseDistance:  3200,
			ExtraFeePerKm: 150,
			ValueRatio:    0.015,
			MinFee:        600,
		})}, nil)
	for _, key := range []string{"WEATHER_COEFF_EXTREME", "WEATHER_COEFF_HEAVY", "WEATHER_COEFF_MODERATE", "WEATHER_COEFF_LIGHT"} {
		store.EXPECT().
			GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{
				ConfigKey: key,
				ScopeType: "city",
				ScopeID:   pgtype.Int8{Int64: 12, Valid: true},
			}).
			Return(db.PlatformConfig{}, db.ErrRecordNotFound)
	}
	store.EXPECT().
		GetLatestWeatherCoefficient(gomock.Any(), int64(12)).
		Return(db.WeatherCoefficient{}, db.ErrRecordNotFound)

	ctx, recorder := newJSONTestContext(http.MethodGet, "/v1/operator/rules", "")
	ctx.Set(operatorKey, operator)

	server.listOperatorRules(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp ListRulesResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	baseRule := findRuleByKey(t, resp.Rules, "BASE_DELIVERY_FEE")
	require.Equal(t, "6.80", baseRule.Value)
	require.Contains(t, baseRule.Desc, "平台默认值")
	valueRatioRule := findRuleByKey(t, resp.Rules, "DELIVERY_VALUE_RATIO")
	require.Equal(t, "1.50", valueRatioRule.Value)
}

func TestListOperatorRules_WeatherUsesCityScopedPlatformDefaultWhenRegionRuleMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	operator := db.Operator{ID: 8, UserID: 88, RegionID: 12, RiderDeposit: 26000}

	store.EXPECT().
		GetRegionRuleConfigByRegion(gomock.Any(), int64(12)).
		Return(db.RegionRuleConfig{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetActiveProfitSharingConfig(gomock.Any(), gomock.Any()).
		Return(db.ProfitSharingConfig{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{
			ConfigKey: operatorRiderDepositConfigKey,
			ScopeType: db.PlatformConfigScopeGlobal,
			ScopeID:   pgtype.Int8{Valid: false},
		}).
		Return(db.PlatformConfig{}, db.ErrRecordNotFound)
	for _, tc := range []struct {
		key   string
		value float64
	}{
		{key: "WEATHER_COEFF_EXTREME", value: 2.4},
		{key: "WEATHER_COEFF_HEAVY", value: 1.9},
		{key: "WEATHER_COEFF_MODERATE", value: 1.4},
		{key: "WEATHER_COEFF_LIGHT", value: 1.2},
	} {
		payload, err := json.Marshal(tc.value)
		require.NoError(t, err)
		store.EXPECT().
			GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{
				ConfigKey: tc.key,
				ScopeType: "city",
				ScopeID:   pgtype.Int8{Int64: 12, Valid: true},
			}).
			Return(db.PlatformConfig{ConfigValue: payload}, nil)
	}
	store.EXPECT().
		GetDeliveryFeeConfigByRegion(gomock.Any(), int64(12)).
		Return(db.DeliveryFeeConfig{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{
			ConfigKey: deliveryFeeDefaultConfigKey,
			ScopeType: db.PlatformConfigScopeGlobal,
			ScopeID:   pgtype.Int8{Valid: false},
		}).
		Return(db.PlatformConfig{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetLatestWeatherCoefficient(gomock.Any(), int64(12)).
		Return(db.WeatherCoefficient{}, db.ErrRecordNotFound)

	ctx, recorder := newJSONTestContext(http.MethodGet, "/v1/operator/rules", "")
	ctx.Set(operatorKey, operator)

	server.listOperatorRules(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp ListRulesResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &resp))
	extremeRule := findRuleByKey(t, resp.Rules, "WEATHER_COEFF_EXTREME")
	require.Equal(t, "2.40", extremeRule.Value)
	require.Contains(t, extremeRule.Desc, "平台城市默认值")
	lightRule := findRuleByKey(t, resp.Rules, "WEATHER_COEFF_LIGHT")
	require.Equal(t, "1.20", lightRule.Value)
	require.Contains(t, lightRule.Desc, "平台城市默认值")
}

func TestUpdatePlatformOperationalConfig_BaseDeliveryFeeWritesGlobalDefaultConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)

	store.EXPECT().
		GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{
			ConfigKey: deliveryFeeDefaultConfigKey,
			ScopeType: db.PlatformConfigScopeGlobal,
			ScopeID:   pgtype.Int8{Valid: false},
		}).
		Return(db.PlatformConfig{ConfigValue: mustMarshalDeliveryFeeDefaultConfig(t, deliveryFeeDefaultConfigValue{
			BaseFee:       500,
			BaseDistance:  3000,
			ExtraFeePerKm: 100,
			ValueRatio:    0.01,
			MinFee:        500,
		})}, nil)
	store.EXPECT().
		UpsertPlatformConfig(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.UpsertPlatformConfigParams) (db.PlatformConfig, error) {
			require.Equal(t, deliveryFeeDefaultConfigKey, arg.ConfigKey)
			require.Equal(t, db.PlatformConfigScopeGlobal, arg.ScopeType)
			var payload deliveryFeeDefaultConfigValue
			require.NoError(t, json.Unmarshal(arg.ConfigValue, &payload))
			require.Equal(t, int64(680), payload.BaseFee)
			require.Equal(t, int32(3000), payload.BaseDistance)
			require.Equal(t, int64(100), payload.ExtraFeePerKm)
			require.Equal(t, int64(500), payload.MinFee)
			require.InDelta(t, 0.01, payload.ValueRatio, 0.00001)
			return db.PlatformConfig{}, nil
		})

	ctx, recorder := newJSONTestContext(http.MethodPatch, "/v1/platform/operational-configs/BASE_DELIVERY_FEE", `{"value":"6.8"}`)
	ctx.Params = gin.Params{{Key: "key", Value: "BASE_DELIVERY_FEE"}}
	ctx.Set(authorizationPayloadKey, &token.Payload{UserID: 1001})

	server.updatePlatformOperationalConfig(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestUpdatePlatformOperationalConfig_MaxDeliveryFeeAllowsUnlimited(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	currentMaxFee := int64(1800)

	store.EXPECT().
		GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{
			ConfigKey: deliveryFeeDefaultConfigKey,
			ScopeType: db.PlatformConfigScopeGlobal,
			ScopeID:   pgtype.Int8{Valid: false},
		}).
		Return(db.PlatformConfig{ConfigValue: mustMarshalDeliveryFeeDefaultConfig(t, deliveryFeeDefaultConfigValue{
			BaseFee:       500,
			BaseDistance:  3000,
			ExtraFeePerKm: 100,
			ValueRatio:    0.01,
			MaxFee:        &currentMaxFee,
			MinFee:        500,
		})}, nil)
	store.EXPECT().
		UpsertPlatformConfig(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.UpsertPlatformConfigParams) (db.PlatformConfig, error) {
			var payload deliveryFeeDefaultConfigValue
			require.NoError(t, json.Unmarshal(arg.ConfigValue, &payload))
			require.Nil(t, payload.MaxFee)
			return db.PlatformConfig{}, nil
		})

	ctx, recorder := newJSONTestContext(http.MethodPatch, "/v1/platform/operational-configs/MAX_DELIVERY_FEE", `{"value":"不限"}`)
	ctx.Params = gin.Params{{Key: "key", Value: "MAX_DELIVERY_FEE"}}
	ctx.Set(authorizationPayloadKey, &token.Payload{UserID: 1001})

	server.updatePlatformOperationalConfig(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestUpdateOperatorRule_WeatherDoesNotOverwritePlatformCityDefault(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	operator := db.Operator{ID: 8, UserID: 88, RegionID: 12, RiderDeposit: 26000}

	store.EXPECT().
		UpsertRegionRuleConfig(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.UpsertRegionRuleConfigParams) (db.RegionRuleConfig, error) {
			require.Equal(t, int64(12), arg.RegionID)
			value, err := arg.WeatherCoeffHeavy.Float64Value()
			require.NoError(t, err)
			require.InDelta(t, 1.9, value.Float64, 0.00001)
			return db.RegionRuleConfig{RegionID: 12}, nil
		})

	ctx, recorder := newJSONTestContext(http.MethodPatch, "/v1/operator/rules/WEATHER_COEFF_HEAVY", `{"value":"1.9"}`)
	ctx.Params = gin.Params{{Key: "key", Value: "WEATHER_COEFF_HEAVY"}}
	ctx.Set(operatorKey, operator)

	server.updateOperatorRule(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestUpdateOperatorRule_WeatherCoefficientInvalidatesWeatherCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	cache := &testWeatherCache{}
	server.weatherCache = cache
	operator := db.Operator{ID: 8, UserID: 88, RegionID: 12}

	store.EXPECT().
		UpsertRegionRuleConfig(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.UpsertRegionRuleConfigParams) (db.RegionRuleConfig, error) {
			require.Equal(t, int64(12), arg.RegionID)
			value, err := arg.WeatherCoeffHeavy.Float64Value()
			require.NoError(t, err)
			require.InDelta(t, 1.9, value.Float64, 0.0001)
			return db.RegionRuleConfig{RegionID: arg.RegionID}, nil
		})

	ctx, recorder := newJSONTestContext(http.MethodPatch, "/v1/operator/rules/WEATHER_COEFF_HEAVY", `{"value":"1.9"}`)
	ctx.Params = gin.Params{{Key: "key", Value: "WEATHER_COEFF_HEAVY"}}
	ctx.Set(operatorKey, operator)

	server.updateOperatorRule(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, []int64{12}, cache.deletedRegions)
}

func TestUpdateOperatorRule_WeatherCoefficientCacheDeleteFailureIsNonFatal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	server.weatherCache = &testWeatherCache{deleteErr: errors.New("redis unavailable")}
	operator := db.Operator{ID: 8, UserID: 88, RegionID: 12}

	store.EXPECT().
		UpsertRegionRuleConfig(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.UpsertRegionRuleConfigParams) (db.RegionRuleConfig, error) {
			require.Equal(t, int64(12), arg.RegionID)
			return db.RegionRuleConfig{RegionID: arg.RegionID}, nil
		})

	ctx, recorder := newJSONTestContext(http.MethodPatch, "/v1/operator/rules/WEATHER_COEFF_HEAVY", `{"value":"1.9"}`)
	ctx.Params = gin.Params{{Key: "key", Value: "WEATHER_COEFF_HEAVY"}}
	ctx.Set(operatorKey, operator)

	server.updateOperatorRule(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestUpdateOperatorRule_BaseDeliveryFeeSeedsRegionConfigWithPlatformMaxFee(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	operator := db.Operator{ID: 8, UserID: 88, RegionID: 12, RiderDeposit: 26000}
	seedMaxFee := int64(1880)
	seedConfig := db.DeliveryFeeConfig{
		RegionID:      12,
		BaseFee:       680,
		BaseDistance:  3200,
		ExtraFeePerKm: 150,
		ValueRatio:    numericFromFloat(0.015),
		MaxFee:        pgtype.Int8{Int64: seedMaxFee, Valid: true},
		MinFee:        600,
		IsActive:      true,
	}
	createdConfig := seedConfig
	createdConfig.ID = 99

	store.EXPECT().
		GetDeliveryFeeConfigByRegion(gomock.Any(), int64(12)).
		Return(db.DeliveryFeeConfig{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetDeliveryFeeConfigByRegion(gomock.Any(), int64(12)).
		Return(db.DeliveryFeeConfig{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetPlatformConfig(gomock.Any(), db.GetPlatformConfigParams{
			ConfigKey: deliveryFeeDefaultConfigKey,
			ScopeType: db.PlatformConfigScopeGlobal,
			ScopeID:   pgtype.Int8{Valid: false},
		}).
		Return(db.PlatformConfig{ConfigValue: mustMarshalDeliveryFeeDefaultConfig(t, deliveryFeeDefaultConfigValue{
			BaseFee:       seedConfig.BaseFee,
			BaseDistance:  seedConfig.BaseDistance,
			ExtraFeePerKm: seedConfig.ExtraFeePerKm,
			ValueRatio:    0.015,
			MaxFee:        &seedMaxFee,
			MinFee:        seedConfig.MinFee,
		})}, nil)
	store.EXPECT().
		CreateDeliveryFeeConfig(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.CreateDeliveryFeeConfigParams) (db.DeliveryFeeConfig, error) {
			require.Equal(t, seedConfig.BaseFee, arg.BaseFee)
			require.Equal(t, seedConfig.BaseDistance, arg.BaseDistance)
			require.Equal(t, seedConfig.ExtraFeePerKm, arg.ExtraFeePerKm)
			require.Equal(t, seedConfig.MinFee, arg.MinFee)
			require.True(t, arg.MaxFee.Valid)
			require.Equal(t, seedMaxFee, arg.MaxFee.Int64)
			return createdConfig, nil
		})
	store.EXPECT().
		UpdateDeliveryFeeConfig(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.UpdateDeliveryFeeConfigParams) (db.DeliveryFeeConfig, error) {
			require.Equal(t, createdConfig.ID, arg.ID)
			require.True(t, arg.BaseFee.Valid)
			require.Equal(t, int64(720), arg.BaseFee.Int64)
			return createdConfig, nil
		})

	ctx, recorder := newJSONTestContext(http.MethodPatch, "/v1/operator/rules/BASE_DELIVERY_FEE", `{"value":"7.2"}`)
	ctx.Params = gin.Params{{Key: "key", Value: "BASE_DELIVERY_FEE"}}
	ctx.Set(operatorKey, operator)

	server.updateOperatorRule(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestUpdateOperatorRule_BaseDeliveryFeeReusesInactiveRegionConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	server := newTestServer(t, store)
	operator := db.Operator{ID: 8, UserID: 88, RegionID: 12, RiderDeposit: 26000}
	existingConfig := db.DeliveryFeeConfig{
		ID:            77,
		RegionID:      12,
		BaseFee:       600,
		BaseDistance:  3000,
		ExtraFeePerKm: 100,
		ValueRatio:    numericFromFloat(0.01),
		MinFee:        500,
		IsActive:      false,
	}

	store.EXPECT().
		GetDeliveryFeeConfigByRegion(gomock.Any(), int64(12)).
		Return(existingConfig, nil)
	store.EXPECT().
		UpdateDeliveryFeeConfig(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, arg db.UpdateDeliveryFeeConfigParams) (db.DeliveryFeeConfig, error) {
			require.Equal(t, existingConfig.ID, arg.ID)
			require.True(t, arg.IsActive.Valid)
			require.True(t, arg.IsActive.Bool)
			require.True(t, arg.BaseFee.Valid)
			require.Equal(t, int64(720), arg.BaseFee.Int64)
			return existingConfig, nil
		})

	ctx, recorder := newJSONTestContext(http.MethodPatch, "/v1/operator/rules/BASE_DELIVERY_FEE", `{"value":"7.2"}`)
	ctx.Params = gin.Params{{Key: "key", Value: "BASE_DELIVERY_FEE"}}
	ctx.Set(operatorKey, operator)

	server.updateOperatorRule(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
}
