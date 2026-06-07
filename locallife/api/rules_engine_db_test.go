package api

import (
	"context"
	"testing"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/rules"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestDBRulesEngineEvaluate_ReturnsErrorOnMalformedActiveRuleVersionJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		ListActiveRuleVersions(gomock.Any()).
		Times(1).
		Return([]db.RuleVersion{{
			ID:         11,
			RuleID:     21,
			Scope:      []byte(`{"domain":"claim"`),
			Condition:  []byte(`{}`),
			Action:     []byte(`{"type":"deny"}`),
			GrayConfig: []byte(`{}`),
		}}, nil)

	engine := NewDBRulesEngine(store)
	decision, err := engine.Evaluate(context.Background(), rules.Context{Domain: rules.DomainClaim})

	require.Equal(t, rules.Decision{}, decision)
	require.Error(t, err)
	require.ErrorContains(t, err, "decode active rule version 11 field scope")
}

func TestDBRulesEngineEvaluateBalanceSceneAllowedParity(t *testing.T) {
	const merchantID int64 = 88

	tests := []struct {
		name         string
		orderType    string
		settings     db.MerchantMembershipSetting
		settingsErr  error
		wantMatched  bool
		expectLookup bool
	}{
		{
			name:         "default settings allow dine in",
			orderType:    "dine_in",
			settingsErr:  db.ErrRecordNotFound,
			wantMatched:  true,
			expectLookup: true,
		},
		{
			name:         "default settings allow takeaway",
			orderType:    "takeaway",
			settingsErr:  db.ErrRecordNotFound,
			wantMatched:  true,
			expectLookup: true,
		},
		{
			name:      "explicit settings allow takeaway",
			orderType: "takeaway",
			settings: db.MerchantMembershipSetting{
				MerchantID:          merchantID,
				BalanceUsableScenes: []string{"takeaway"},
			},
			wantMatched:  true,
			expectLookup: true,
		},
		{
			name:      "explicit settings reject omitted dine in",
			orderType: "dine_in",
			settings: db.MerchantMembershipSetting{
				MerchantID:          merchantID,
				BalanceUsableScenes: []string{"takeaway"},
			},
			expectLookup: true,
		},
		{
			name:      "legacy takeout scene is ignored",
			orderType: "takeout",
			settings: db.MerchantMembershipSetting{
				MerchantID:          merchantID,
				BalanceUsableScenes: []string{"takeout", "reservation"},
			},
		},
		{
			name:      "reservation scene is unsupported",
			orderType: "reservation",
			settings: db.MerchantMembershipSetting{
				MerchantID:          merchantID,
				BalanceUsableScenes: []string{"reservation"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			store.EXPECT().
				ListActiveRuleVersions(gomock.Any()).
				Times(1).
				Return([]db.RuleVersion{{
					ID:         31,
					RuleID:     41,
					Scope:      []byte(`{"domain":"order"}`),
					Condition:  []byte(`{"balance_scene_allowed":true}`),
					Action:     []byte(`{"type":"alert","reason":"balance scene allowed"}`),
					GrayConfig: []byte(`{}`),
				}}, nil)

			if tt.expectLookup {
				store.EXPECT().
					GetMerchantMembershipSettings(gomock.Any(), merchantID).
					Times(1).
					Return(tt.settings, tt.settingsErr)
			}

			engine := NewDBRulesEngine(store)
			decision, err := engine.Evaluate(context.Background(), rules.Context{
				Domain:     rules.DomainOrder,
				MerchantID: merchantID,
				OrderType:  tt.orderType,
			})

			require.NoError(t, err)
			if tt.wantMatched {
				require.Equal(t, int64(41), decision.RuleID)
				require.Equal(t, int64(31), decision.RuleVersionID)
				require.Equal(t, "alert", decision.Action)
				return
			}

			require.True(t, decision.Allow)
			require.Equal(t, "allow", decision.Action)
			require.Zero(t, decision.RuleID)
			require.Zero(t, decision.RuleVersionID)
		})
	}
}
