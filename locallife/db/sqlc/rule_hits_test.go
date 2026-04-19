package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestRuleHitListQueriesUseIDTieBreaker(t *testing.T) {
	user := createRandomUser(t)
	region := createRandomRegion(t)
	tiedCreatedAt := time.Now().UTC().Truncate(time.Microsecond)

	rule, err := testStore.CreateRule(context.Background(), CreateRuleParams{
		Name:             "rule_" + util.RandomString(8),
		Category:         "audit",
		Status:           "active",
		CurrentVersionID: pgtype.Int8{Valid: false},
		CreatedBy:        pgtype.Int8{Int64: user.ID, Valid: true},
	})
	require.NoError(t, err)

	createRuleHit := func() RuleHit {
		hit, err := testStore.CreateRuleHit(context.Background(), CreateRuleHitParams{
			RuleID:        rule.ID,
			RuleVersionID: pgtype.Int8{Valid: false},
			Domain:        "merchant",
			Decision:      "allow",
			Reason:        pgtype.Text{String: "test hit", Valid: true},
			Inputs:        []byte(`{"score":95}`),
			Outputs:       []byte(`{"result":"pass"}`),
			ActorID:       pgtype.Int8{Int64: user.ID, Valid: true},
			ActorRole:     pgtype.Text{String: "platform_admin", Valid: true},
			RegionID:      pgtype.Int8{Int64: region.ID, Valid: true},
			MerchantID:    pgtype.Int8{Valid: false},
		})
		require.NoError(t, err)
		return hit
	}

	firstHit := createRuleHit()
	secondHit := createRuleHit()

	_, err = testStore.(*SQLStore).connPool.Exec(context.Background(),
		`UPDATE rule_hits SET created_at = $1 WHERE id = ANY($2)`,
		tiedCreatedAt,
		[]int64{firstHit.ID, secondHit.ID},
	)
	require.NoError(t, err)

	byRule, err := testStore.ListRuleHitsByRule(context.Background(), ListRuleHitsByRuleParams{
		RuleID: rule.ID,
		Limit:  2,
		Offset: 0,
	})
	require.NoError(t, err)
	require.Len(t, byRule, 2)
	require.Equal(t, secondHit.ID, byRule[0].ID)
	require.Equal(t, firstHit.ID, byRule[1].ID)

	byRuleAndRegion, err := testStore.ListRuleHitsByRuleAndRegion(context.Background(), ListRuleHitsByRuleAndRegionParams{
		RuleID:   rule.ID,
		RegionID: pgtype.Int8{Int64: region.ID, Valid: true},
		Limit:    2,
		Offset:   0,
	})
	require.NoError(t, err)
	require.Len(t, byRuleAndRegion, 2)
	require.Equal(t, secondHit.ID, byRuleAndRegion[0].ID)
	require.Equal(t, firstHit.ID, byRuleAndRegion[1].ID)
}
