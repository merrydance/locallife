package db

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestListProfitSharingConfigsForRegionExcludesMerchantOverridesOutsideRegion(t *testing.T) {
	ctx := context.Background()
	operatorMerchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	otherMerchant := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	require.NotEqual(t, operatorMerchant.RegionID, otherMerchant.RegionID)

	createConfig := func(regionID pgtype.Int8, merchantID pgtype.Int8, priority int32) ProfitSharingConfig {
		config, err := testStore.CreateProfitSharingConfig(ctx, CreateProfitSharingConfigParams{
			Status:       "active",
			OrderSource:  "takeout",
			RegionID:     regionID,
			MerchantID:   merchantID,
			PlatformRate: 2,
			OperatorRate: 3,
			RiderEnabled: true,
			Priority:     priority,
		})
		require.NoError(t, err)
		return config
	}

	globalDefault := createConfig(pgtype.Int8{Valid: false}, pgtype.Int8{Valid: false}, 10)
	regionDefault := createConfig(pgtype.Int8{Int64: operatorMerchant.RegionID, Valid: true}, pgtype.Int8{Valid: false}, 20)
	operatorMerchantOverride := createConfig(pgtype.Int8{Valid: false}, pgtype.Int8{Int64: operatorMerchant.ID, Valid: true}, 30)
	otherRegionMerchantOverride := createConfig(pgtype.Int8{Valid: false}, pgtype.Int8{Int64: otherMerchant.ID, Valid: true}, 40)

	configs, err := testStore.ListProfitSharingConfigsForRegion(ctx, ListProfitSharingConfigsForRegionParams{
		Column1:  "active",
		Column2:  "takeout",
		RegionID: pgtype.Int8{Int64: operatorMerchant.RegionID, Valid: true},
		Column4:  0,
		Limit:    20,
		Offset:   0,
	})
	require.NoError(t, err)

	ids := make(map[int64]bool, len(configs))
	for _, config := range configs {
		ids[config.ID] = true
	}

	require.True(t, ids[globalDefault.ID], "platform global defaults remain visible to every operator region")
	require.True(t, ids[regionDefault.ID], "region default remains visible to its operator")
	require.True(t, ids[operatorMerchantOverride.ID], "merchant override is visible when merchant belongs to operator region")
	require.False(t, ids[otherRegionMerchantOverride.ID], "merchant-scoped global override from another region must not leak")
}
