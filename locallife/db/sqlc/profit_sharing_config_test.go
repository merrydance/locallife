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

func TestListProfitSharingConfigsForRegionsAggregatesManagedRegionsWithoutMerchantLeakage(t *testing.T) {
	ctx := context.Background()
	merchantInFirstRegion := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	merchantInSecondRegion := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	merchantOutsideRegions := createRandomMerchantWithOwner(t, createRandomUser(t).ID)
	require.NotEqual(t, merchantInFirstRegion.RegionID, merchantInSecondRegion.RegionID)
	require.NotEqual(t, merchantInFirstRegion.RegionID, merchantOutsideRegions.RegionID)
	require.NotEqual(t, merchantInSecondRegion.RegionID, merchantOutsideRegions.RegionID)

	createConfig := func(regionID pgtype.Int8, merchantID pgtype.Int8, priority int32) ProfitSharingConfig {
		t.Helper()

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

	platformGlobalDefault := createConfig(pgtype.Int8{Valid: false}, pgtype.Int8{Valid: false}, 10)
	firstRegionDefault := createConfig(pgtype.Int8{Int64: merchantInFirstRegion.RegionID, Valid: true}, pgtype.Int8{Valid: false}, 20)
	secondRegionDefault := createConfig(pgtype.Int8{Int64: merchantInSecondRegion.RegionID, Valid: true}, pgtype.Int8{Valid: false}, 30)
	outsideRegionDefault := createConfig(pgtype.Int8{Int64: merchantOutsideRegions.RegionID, Valid: true}, pgtype.Int8{Valid: false}, 40)
	firstMerchantOverride := createConfig(pgtype.Int8{Valid: false}, pgtype.Int8{Int64: merchantInFirstRegion.ID, Valid: true}, 50)
	secondMerchantOverride := createConfig(pgtype.Int8{Valid: false}, pgtype.Int8{Int64: merchantInSecondRegion.ID, Valid: true}, 60)
	outsideMerchantOverride := createConfig(pgtype.Int8{Valid: false}, pgtype.Int8{Int64: merchantOutsideRegions.ID, Valid: true}, 70)

	configs, err := testStore.ListProfitSharingConfigsForRegions(ctx, ListProfitSharingConfigsForRegionsParams{
		Status:      "active",
		OrderSource: "takeout",
		RegionIds:   []int64{merchantInFirstRegion.RegionID, merchantInSecondRegion.RegionID},
		MerchantID:  0,
		Limit:       50,
		Offset:      0,
	})
	require.NoError(t, err)

	ids := make(map[int64]bool, len(configs))
	for _, config := range configs {
		ids[config.ID] = true
	}

	require.True(t, ids[platformGlobalDefault.ID], "platform global defaults remain visible to every authorized operator region")
	require.True(t, ids[firstRegionDefault.ID], "first managed region default must be visible")
	require.True(t, ids[secondRegionDefault.ID], "second managed region default must be visible")
	require.True(t, ids[firstMerchantOverride.ID], "merchant override is visible when merchant belongs to the first managed region")
	require.True(t, ids[secondMerchantOverride.ID], "merchant override is visible when merchant belongs to the second managed region")
	require.False(t, ids[outsideRegionDefault.ID], "region default outside managed regions must not leak")
	require.False(t, ids[outsideMerchantOverride.ID], "merchant override outside managed regions must not leak")
}
