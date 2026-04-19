package api

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

const deliveryFeeDefaultConfigKey = "platform_rule.delivery_fee_default"

type deliveryFeeConfigSource string

const (
	deliveryFeeConfigSourceRegion   deliveryFeeConfigSource = "region"
	deliveryFeeConfigSourcePlatform deliveryFeeConfigSource = "platform"
	deliveryFeeConfigSourceDefault  deliveryFeeConfigSource = "default"
)

type deliveryFeeDefaultConfigValue struct {
	BaseFee       int64   `json:"base_fee"`
	BaseDistance  int32   `json:"base_distance"`
	ExtraFeePerKm int64   `json:"extra_fee_per_km"`
	ValueRatio    float64 `json:"value_ratio"`
	MaxFee        *int64  `json:"max_fee,omitempty"`
	MinFee        int64   `json:"min_fee"`
}

func defaultDeliveryFeeConfig() db.DeliveryFeeConfig {
	return db.DeliveryFeeConfig{
		BaseFee:       DefaultBaseFee,
		BaseDistance:  DefaultBaseDistance,
		ExtraFeePerKm: DefaultExtraFeePerKm,
		ValueRatio:    numericFromFloat(DefaultValueRatio),
		MinFee:        DefaultMinFee,
		IsActive:      true,
	}
}

func newDeliveryFeeDefaultConfigValue(config db.DeliveryFeeConfig) deliveryFeeDefaultConfigValue {
	valueRatio := DefaultValueRatio
	if config.ValueRatio.Valid {
		if value, err := config.ValueRatio.Float64Value(); err == nil {
			valueRatio = value.Float64
		}
	}

	var maxFee *int64
	if config.MaxFee.Valid {
		value := config.MaxFee.Int64
		maxFee = &value
	}

	return deliveryFeeDefaultConfigValue{
		BaseFee:       config.BaseFee,
		BaseDistance:  config.BaseDistance,
		ExtraFeePerKm: config.ExtraFeePerKm,
		ValueRatio:    valueRatio,
		MaxFee:        maxFee,
		MinFee:        config.MinFee,
	}
}

func deliveryFeeConfigFromDefaultValue(payload deliveryFeeDefaultConfigValue, regionID int64) db.DeliveryFeeConfig {
	config := db.DeliveryFeeConfig{
		RegionID:      regionID,
		BaseFee:       payload.BaseFee,
		BaseDistance:  payload.BaseDistance,
		ExtraFeePerKm: payload.ExtraFeePerKm,
		ValueRatio:    numericFromFloat(payload.ValueRatio),
		MinFee:        payload.MinFee,
		IsActive:      true,
	}
	if payload.MaxFee != nil {
		config.MaxFee = pgtype.Int8{Int64: *payload.MaxFee, Valid: true}
	}
	return config
}

func (server *Server) getGlobalDeliveryFeeDefaultConfig(ctx context.Context) (db.DeliveryFeeConfig, bool, error) {
	config, err := server.store.GetPlatformConfig(ctx, db.GetPlatformConfigParams{
		ConfigKey: deliveryFeeDefaultConfigKey,
		ScopeType: db.PlatformConfigScopeGlobal,
		ScopeID:   pgtype.Int8{Valid: false},
	})
	if err != nil {
		if isNotFoundError(err) {
			return db.DeliveryFeeConfig{}, false, nil
		}
		return db.DeliveryFeeConfig{}, false, err
	}

	if len(config.ConfigValue) == 0 {
		return db.DeliveryFeeConfig{}, false, nil
	}

	var payload deliveryFeeDefaultConfigValue
	if err := json.Unmarshal(config.ConfigValue, &payload); err != nil {
		return db.DeliveryFeeConfig{}, false, err
	}

	return deliveryFeeConfigFromDefaultValue(payload, 0), true, nil
}

func (server *Server) upsertGlobalDeliveryFeeDefaultConfig(ctx context.Context, config db.DeliveryFeeConfig) error {
	payload, err := json.Marshal(newDeliveryFeeDefaultConfigValue(config))
	if err != nil {
		return err
	}

	_, err = server.store.UpsertPlatformConfig(ctx, db.UpsertPlatformConfigParams{
		ConfigKey:   deliveryFeeDefaultConfigKey,
		ConfigValue: payload,
		ScopeType:   db.PlatformConfigScopeGlobal,
		ScopeID:     pgtype.Int8{Valid: false},
	})
	return err
}

func (server *Server) resolveEffectiveDeliveryFeeConfig(ctx context.Context, regionID int64) (db.DeliveryFeeConfig, deliveryFeeConfigSource, error) {
	config, err := server.store.GetDeliveryFeeConfigByRegion(ctx, regionID)
	if err == nil {
		return config, deliveryFeeConfigSourceRegion, nil
	}
	if !isNotFoundError(err) {
		return db.DeliveryFeeConfig{}, "", err
	}

	globalConfig, ok, err := server.getGlobalDeliveryFeeDefaultConfig(ctx)
	if err != nil {
		return db.DeliveryFeeConfig{}, "", err
	}
	if ok {
		globalConfig.RegionID = regionID
		return globalConfig, deliveryFeeConfigSourcePlatform, nil
	}

	defaultConfig := defaultDeliveryFeeConfig()
	defaultConfig.RegionID = regionID
	return defaultConfig, deliveryFeeConfigSourceDefault, nil
}

func (server *Server) resolveOperatorRuleDeliveryFeeConfig(ctx context.Context, regionID int64) (db.DeliveryFeeConfig, deliveryFeeConfigSource, error) {
	config, err := server.store.GetDeliveryFeeConfigByRegion(ctx, regionID)
	if err == nil {
		return config, deliveryFeeConfigSourceRegion, nil
	}
	if !isNotFoundError(err) {
		return db.DeliveryFeeConfig{}, "", err
	}

	globalConfig, ok, err := server.getGlobalDeliveryFeeDefaultConfig(ctx)
	if err != nil {
		return db.DeliveryFeeConfig{}, "", err
	}
	if ok {
		globalConfig.RegionID = regionID
		return globalConfig, deliveryFeeConfigSourcePlatform, nil
	}

	defaultConfig := defaultDeliveryFeeConfig()
	defaultConfig.RegionID = regionID
	return defaultConfig, deliveryFeeConfigSourceDefault, nil
}
