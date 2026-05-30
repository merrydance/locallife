package algorithm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFinalAdjudicator_DamageUsesRiderBaseline(t *testing.T) {
	adjudicator := NewClaimFinalAdjudicator(DefaultClaimFinalAdjudicatorConfig())

	result, err := adjudicator.Adjudicate(ClaimFinalAdjudicationInput{
		ClaimType: ClaimTypeDamage,
		User: PartyWindowStats{
			EntityType:        "user",
			TotalOrders30d:    20,
			AbnormalClaims30d: 0,
		},
		Rider: &PartyWindowStats{
			EntityType:        "rider",
			TotalOrders30d:    30,
			AbnormalClaims30d: 4,
		},
		Merchant: PartyWindowStats{
			EntityType:        "merchant",
			TotalOrders30d:    100,
			AbnormalClaims30d: 0,
		},
	})

	require.NoError(t, err)
	require.Equal(t, FinalDecisionRiderRecovery, result.DecisionMode)
	require.Equal(t, "rider", result.ResponsibleParty)
	require.Equal(t, CompensationSourceRider, result.CompensationSource)
	require.Equal(t, "rider", result.BaseResponsibleParty)
	require.Contains(t, result.ReasonCodes, "base_type_damage_rider")
	require.GreaterOrEqual(t, result.ScoreBreakdown.Scores.RiderLiability.Score, int32(70))
}

func TestFinalAdjudicator_ForeignObjectUsesMerchantBaseline(t *testing.T) {
	adjudicator := NewClaimFinalAdjudicator(DefaultClaimFinalAdjudicatorConfig())

	result, err := adjudicator.Adjudicate(ClaimFinalAdjudicationInput{
		ClaimType: ClaimTypeForeignObject,
		User: PartyWindowStats{
			EntityType:        "user",
			TotalOrders30d:    20,
			AbnormalClaims30d: 1,
		},
		Rider: &PartyWindowStats{
			EntityType:        "rider",
			TotalOrders30d:    80,
			AbnormalClaims30d: 10,
		},
		Merchant: PartyWindowStats{
			EntityType:        "merchant",
			TotalOrders30d:    20,
			AbnormalClaims30d: 3,
		},
	})

	require.NoError(t, err)
	require.Equal(t, FinalDecisionMerchantRecovery, result.DecisionMode)
	require.Equal(t, "merchant", result.ResponsibleParty)
	require.Equal(t, CompensationSourceMerchant, result.CompensationSource)
	require.Equal(t, "merchant", result.BaseResponsibleParty)
	require.Contains(t, result.ReasonCodes, "base_type_foreign_object_merchant")
	require.NotContains(t, result.ReasonCodes, "rider_abnormal_rate_30d_exceeded")
}

func TestFinalAdjudicator_UserSmallSampleDoesNotOverrideBaseline(t *testing.T) {
	adjudicator := NewClaimFinalAdjudicator(DefaultClaimFinalAdjudicatorConfig())

	result, err := adjudicator.Adjudicate(ClaimFinalAdjudicationInput{
		ClaimType: ClaimTypeTimeout,
		User: PartyWindowStats{
			EntityType:        "user",
			TotalOrders30d:    2,
			AbnormalClaims30d: 1,
		},
		Rider: &PartyWindowStats{
			EntityType:        "rider",
			TotalOrders30d:    3,
			AbnormalClaims30d: 0,
		},
		Merchant: PartyWindowStats{
			EntityType:        "merchant",
			TotalOrders30d:    10,
			AbnormalClaims30d: 0,
		},
	})

	require.NoError(t, err)
	require.Equal(t, FinalDecisionRiderRecovery, result.DecisionMode)
	require.Equal(t, "rider", result.ResponsibleParty)
	require.NotContains(t, result.ReasonCodes, "user_claim_rate_30d_exceeded")
	require.Less(t, result.ScoreBreakdown.Scores.UserRisk.Score, int32(70))
}

func TestFinalAdjudicator_UserStrongRiskOverridesBaseline(t *testing.T) {
	adjudicator := NewClaimFinalAdjudicator(DefaultClaimFinalAdjudicatorConfig())

	result, err := adjudicator.Adjudicate(ClaimFinalAdjudicationInput{
		ClaimType: ClaimTypeDamage,
		User: PartyWindowStats{
			EntityType:        "user",
			TotalOrders7d:     5,
			AbnormalClaims7d:  3,
			TotalOrders30d:    8,
			AbnormalClaims30d: 5,
		},
		Rider: &PartyWindowStats{
			EntityType:        "rider",
			TotalOrders30d:    40,
			AbnormalClaims30d: 0,
		},
		Merchant: PartyWindowStats{
			EntityType:        "merchant",
			TotalOrders30d:    50,
			AbnormalClaims30d: 0,
		},
	})

	require.NoError(t, err)
	require.Equal(t, FinalDecisionUserRestricted, result.DecisionMode)
	require.Equal(t, "user", result.ResponsibleParty)
	require.Equal(t, CompensationSourcePlatform, result.CompensationSource)
	require.Equal(t, ClaimBehaviorUserRestricted, result.BehaviorStatus)
	require.Contains(t, result.ReasonCodes, "user_claim_rate_30d_exceeded")
	require.GreaterOrEqual(t, result.ScoreBreakdown.Scores.UserRisk.Score, int32(70))
}

func TestFinalAdjudicator_MaliciousHistoryOverridesBaseline(t *testing.T) {
	adjudicator := NewClaimFinalAdjudicator(DefaultClaimFinalAdjudicatorConfig())

	result, err := adjudicator.Adjudicate(ClaimFinalAdjudicationInput{
		ClaimType: ClaimTypeForeignObject,
		User: PartyWindowStats{
			EntityType:               "user",
			TotalOrders30d:           30,
			AbnormalClaims30d:        1,
			MaliciousConfirmedClaims: 1,
			SharedDeviceOtherUsers:   0,
			SharedAddressOtherUsers:  0,
		},
		Merchant: PartyWindowStats{
			EntityType:        "merchant",
			TotalOrders30d:    100,
			AbnormalClaims30d: 0,
		},
	})

	require.NoError(t, err)
	require.Equal(t, FinalDecisionUserRestricted, result.DecisionMode)
	require.Equal(t, "user", result.ResponsibleParty)
	require.Contains(t, result.ReasonCodes, "historical_malicious_confirmed")
	require.Equal(t, int32(100), result.ScoreBreakdown.Scores.UserRisk.Score)
}
