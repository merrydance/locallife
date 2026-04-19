package db

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

type stubRiderDepositThresholdReader struct {
	activeOperator     Operator
	operatorErr        error
	platformConfig     PlatformConfig
	platformErr        error
	platformConfigs    map[string]PlatformConfig
	platformConfigErrs map[string]error
}

type stubRiderOperationalStatusReconciler struct {
	stubRiderDepositThresholdReader
	activeDeliveries []Delivery
	listErr          error
	updatedStatusArg *UpdateRiderStatusParams
	updatedOnlineArg *UpdateRiderOnlineStatusParams
	statusResult     Rider
	onlineResult     Rider
	statusErr        error
	onlineErr        error
}

func depositScopeKey(arg GetPlatformConfigParams) string {
	scopeID := int64(0)
	if arg.ScopeID.Valid {
		scopeID = arg.ScopeID.Int64
	}
	return fmt.Sprintf("%s|%s|%d", arg.ConfigKey, arg.ScopeType, scopeID)
}

func (s stubRiderDepositThresholdReader) GetPlatformConfig(_ context.Context, arg GetPlatformConfigParams) (PlatformConfig, error) {
	if s.platformConfigs != nil {
		key := depositScopeKey(arg)
		if err, ok := s.platformConfigErrs[key]; ok {
			return PlatformConfig{}, err
		}
		if config, ok := s.platformConfigs[key]; ok {
			return config, nil
		}
		return PlatformConfig{}, ErrRecordNotFound
	}
	return s.platformConfig, s.platformErr
}

func (s stubRiderDepositThresholdReader) GetActiveOperatorByRegion(context.Context, int64) (Operator, error) {
	return s.activeOperator, s.operatorErr
}

func (s *stubRiderOperationalStatusReconciler) UpdateRiderStatus(_ context.Context, arg UpdateRiderStatusParams) (Rider, error) {
	argCopy := arg
	s.updatedStatusArg = &argCopy
	if s.statusErr != nil {
		return Rider{}, s.statusErr
	}
	if s.statusResult.ID == 0 {
		s.statusResult = Rider{ID: arg.ID, Status: arg.Status}
	}
	return s.statusResult, nil
}

func (s *stubRiderOperationalStatusReconciler) UpdateRiderOnlineStatus(_ context.Context, arg UpdateRiderOnlineStatusParams) (Rider, error) {
	argCopy := arg
	s.updatedOnlineArg = &argCopy
	if s.onlineErr != nil {
		return Rider{}, s.onlineErr
	}
	if s.onlineResult.ID == 0 {
		s.onlineResult = Rider{ID: arg.ID, IsOnline: arg.IsOnline}
	}
	return s.onlineResult, nil
}

func (s *stubRiderOperationalStatusReconciler) ListRiderActiveDeliveries(context.Context, pgtype.Int8) ([]Delivery, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.activeDeliveries, nil
}

func TestGetEffectiveRiderDepositThreshold_FallsBackToDefaultWithoutOperatorOrPlatformConfig(t *testing.T) {
	threshold, err := GetEffectiveRiderDepositThreshold(context.Background(), stubRiderDepositThresholdReader{
		operatorErr: ErrRecordNotFound,
		platformErr: ErrRecordNotFound,
	}, pgtype.Int8{Int64: 11, Valid: true})
	require.NoError(t, err)
	require.Equal(t, int64(DefaultRiderDepositThresholdFen), threshold)
}

func TestGetEffectiveRiderDepositThreshold_UsesOperatorConfigForRegion(t *testing.T) {
	threshold, err := GetEffectiveRiderDepositThreshold(context.Background(), stubRiderDepositThresholdReader{
		activeOperator: Operator{ID: 9, RiderDeposit: 26000},
	}, pgtype.Int8{Int64: 11, Valid: true})
	require.NoError(t, err)
	require.Equal(t, int64(26000), threshold)
}

func TestGetEffectiveRiderDepositThreshold_UsesPlatformFallbackWhenOperatorStillOnLegacyDefault(t *testing.T) {
	threshold, err := GetEffectiveRiderDepositThreshold(context.Background(), stubRiderDepositThresholdReader{
		activeOperator: Operator{ID: 9, RiderDeposit: DefaultRiderDepositThresholdFen},
		platformConfigs: map[string]PlatformConfig{
			depositScopeKey(GetPlatformConfigParams{
				ConfigKey: PlatformConfigKeyRiderDepositFen,
				ScopeType: PlatformConfigScopeGlobal,
				ScopeID:   pgtype.Int8{Valid: false},
			}): {
				ConfigKey:   PlatformConfigKeyRiderDepositFen,
				ScopeType:   PlatformConfigScopeGlobal,
				ScopeID:     pgtype.Int8{Valid: false},
				ConfigValue: []byte(`{"amount_fen":32000}`),
			},
		},
	}, pgtype.Int8{Int64: 11, Valid: true})
	require.NoError(t, err)
	require.Equal(t, int64(32000), threshold)
}

func TestGetEffectiveRiderDepositThreshold_UsesOperatorScopedConfigAtLegacyDefaultValue(t *testing.T) {
	threshold, err := GetEffectiveRiderDepositThreshold(context.Background(), stubRiderDepositThresholdReader{
		activeOperator: Operator{ID: 9, RiderDeposit: DefaultRiderDepositThresholdFen},
		platformConfigs: map[string]PlatformConfig{
			depositScopeKey(GetPlatformConfigParams{
				ConfigKey: PlatformConfigKeyRiderDepositFen,
				ScopeType: PlatformConfigScopeOperator,
				ScopeID:   pgtype.Int8{Int64: 9, Valid: true},
			}): {
				ConfigKey:   PlatformConfigKeyRiderDepositFen,
				ScopeType:   PlatformConfigScopeOperator,
				ScopeID:     pgtype.Int8{Int64: 9, Valid: true},
				ConfigValue: []byte(`{"amount_fen":20000}`),
			},
			depositScopeKey(GetPlatformConfigParams{
				ConfigKey: PlatformConfigKeyRiderDepositFen,
				ScopeType: PlatformConfigScopeGlobal,
				ScopeID:   pgtype.Int8{Valid: false},
			}): {
				ConfigKey:   PlatformConfigKeyRiderDepositFen,
				ScopeType:   PlatformConfigScopeGlobal,
				ScopeID:     pgtype.Int8{Valid: false},
				ConfigValue: []byte(`{"amount_fen":32000}`),
			},
		},
	}, pgtype.Int8{Int64: 11, Valid: true})
	require.NoError(t, err)
	require.Equal(t, int64(DefaultRiderDepositThresholdFen), threshold)
}

func TestGetEffectiveRiderDepositThreshold_UsesPlatformFallbackWhenOperatorMissing(t *testing.T) {
	threshold, err := GetEffectiveRiderDepositThreshold(context.Background(), stubRiderDepositThresholdReader{
		operatorErr: ErrRecordNotFound,
		platformErr: nil,
		platformConfig: PlatformConfig{
			ConfigKey:   PlatformConfigKeyRiderDepositFen,
			ScopeType:   PlatformConfigScopeGlobal,
			ScopeID:     pgtype.Int8{Valid: false},
			ConfigValue: []byte(`{"amount_fen":28000}`),
		},
	}, pgtype.Int8{Int64: 11, Valid: true})
	require.NoError(t, err)
	require.Equal(t, int64(28000), threshold)
}

func TestGetEffectiveRiderDepositThreshold_IgnoresZeroOperatorConfigAndFallsBackToPlatform(t *testing.T) {
	threshold, err := GetEffectiveRiderDepositThreshold(context.Background(), stubRiderDepositThresholdReader{
		activeOperator: Operator{ID: 9, RiderDeposit: 0},
		platformConfig: PlatformConfig{
			ConfigKey:   PlatformConfigKeyRiderDepositFen,
			ScopeType:   PlatformConfigScopeGlobal,
			ScopeID:     pgtype.Int8{Valid: false},
			ConfigValue: []byte(`{"amount_fen":32000}`),
		},
	}, pgtype.Int8{Int64: 11, Valid: true})
	require.NoError(t, err)
	require.Equal(t, int64(32000), threshold)
}

func TestGetEffectiveRiderDepositThreshold_ReturnsOperatorLookupError(t *testing.T) {
	boom := errors.New("boom")
	_, err := GetEffectiveRiderDepositThreshold(context.Background(), stubRiderDepositThresholdReader{
		operatorErr: boom,
	}, pgtype.Int8{Int64: 11, Valid: true})
	require.ErrorIs(t, err, boom)
}

func TestReconcileRiderOperationalStatus_DemotesActiveRiderAndForcesOffline(t *testing.T) {
	reconciler := &stubRiderOperationalStatusReconciler{
		stubRiderDepositThresholdReader: stubRiderDepositThresholdReader{
			activeOperator: Operator{ID: 9, RiderDeposit: 26000},
		},
		statusResult: Rider{
			ID:            33,
			Status:        RiderStatusApproved,
			IsOnline:      true,
			DepositAmount: 25000,
		},
		onlineResult: Rider{
			ID:            33,
			Status:        RiderStatusApproved,
			IsOnline:      false,
			DepositAmount: 25000,
		},
	}

	updated, err := ReconcileRiderOperationalStatus(context.Background(), reconciler, Rider{
		ID:            33,
		Status:        RiderStatusActive,
		IsOnline:      true,
		DepositAmount: 25000,
		RegionID:      pgtype.Int8{Int64: 11, Valid: true},
	})
	require.NoError(t, err)
	require.NotNil(t, reconciler.updatedStatusArg)
	require.Equal(t, RiderStatusApproved, reconciler.updatedStatusArg.Status)
	require.NotNil(t, reconciler.updatedOnlineArg)
	require.False(t, reconciler.updatedOnlineArg.IsOnline)
	require.Equal(t, RiderStatusApproved, updated.Status)
	require.False(t, updated.IsOnline)
}

func TestReconcileRiderOperationalStatus_PromotesApprovedRiderWhenPlatformThresholdSatisfied(t *testing.T) {
	reconciler := &stubRiderOperationalStatusReconciler{
		stubRiderDepositThresholdReader: stubRiderDepositThresholdReader{
			operatorErr: ErrRecordNotFound,
			platformConfig: PlatformConfig{
				ConfigKey:   PlatformConfigKeyRiderDepositFen,
				ScopeType:   PlatformConfigScopeGlobal,
				ScopeID:     pgtype.Int8{Valid: false},
				ConfigValue: []byte(`{"amount_fen":28000}`),
			},
		},
		statusResult: Rider{
			ID:            44,
			Status:        RiderStatusActive,
			IsOnline:      false,
			DepositAmount: 30000,
		},
	}

	updated, err := ReconcileRiderOperationalStatus(context.Background(), reconciler, Rider{
		ID:            44,
		Status:        RiderStatusApproved,
		IsOnline:      false,
		DepositAmount: 30000,
		RegionID:      pgtype.Int8{Int64: 21, Valid: true},
	})
	require.NoError(t, err)
	require.NotNil(t, reconciler.updatedStatusArg)
	require.Equal(t, RiderStatusActive, reconciler.updatedStatusArg.Status)
	require.Nil(t, reconciler.updatedOnlineArg)
	require.Equal(t, RiderStatusActive, updated.Status)
}

func TestReconcileRiderOperationalStatus_DemotesActiveRiderWithActiveDeliveriesKeepsOnline(t *testing.T) {
	reconciler := &stubRiderOperationalStatusReconciler{
		stubRiderDepositThresholdReader: stubRiderDepositThresholdReader{
			activeOperator: Operator{ID: 9, RiderDeposit: 26000},
		},
		activeDeliveries: []Delivery{{ID: 91}},
		statusResult: Rider{
			ID:            55,
			Status:        RiderStatusApproved,
			IsOnline:      true,
			DepositAmount: 25000,
		},
	}

	updated, err := ReconcileRiderOperationalStatus(context.Background(), reconciler, Rider{
		ID:            55,
		Status:        RiderStatusActive,
		IsOnline:      true,
		DepositAmount: 25000,
		RegionID:      pgtype.Int8{Int64: 11, Valid: true},
	})
	require.NoError(t, err)
	require.NotNil(t, reconciler.updatedStatusArg)
	require.Equal(t, RiderStatusApproved, reconciler.updatedStatusArg.Status)
	require.Nil(t, reconciler.updatedOnlineArg)
	require.Equal(t, RiderStatusApproved, updated.Status)
	require.True(t, updated.IsOnline)
}
