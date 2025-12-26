package db

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestRegionQueries_CreateAndGet(t *testing.T) {
	r := createRandomRegion(t)

	got, err := testStore.GetRegion(context.Background(), r.ID)
	require.NoError(t, err)
	require.NotEmpty(t, got)
	require.Equal(t, r.ID, got.ID)
	require.Equal(t, r.Code, got.Code)
	require.Equal(t, r.Name, got.Name)
}

func TestRegionQueries_GetRegion_NotFound(t *testing.T) {
	got, err := testStore.GetRegion(context.Background(), -1)
	require.Error(t, err)
	require.ErrorIs(t, err, pgx.ErrNoRows)
	require.Empty(t, got)
}

func TestRegionQueries_GetRegionByCode(t *testing.T) {
	code := fmt.Sprintf("R%s", util.RandomString(10))
	arg := CreateRegionParams{
		Code:      code,
		Name:      util.RandomString(10),
		Level:     1,
		ParentID:  pgtype.Int8{Valid: false},
		Longitude: pgtype.Numeric{},
		Latitude:  pgtype.Numeric{},
	}

	r, err := testStore.CreateRegion(context.Background(), arg)
	require.NoError(t, err)

	got, err := testStore.GetRegionByCode(context.Background(), code)
	require.NoError(t, err)
	require.Equal(t, r.ID, got.ID)
	require.Equal(t, code, got.Code)
}

func TestRegionQueries_ListRegions_Filters(t *testing.T) {
	ctx := context.Background()

	province := createRandomRegion(t)

	city1, err := testStore.CreateRegion(ctx, CreateRegionParams{
		Code:      util.RandomString(6),
		Name:      "city_a_" + util.RandomString(6),
		Level:     2,
		ParentID:  pgtype.Int8{Int64: province.ID, Valid: true},
		Longitude: pgtype.Numeric{},
		Latitude:  pgtype.Numeric{},
	})
	require.NoError(t, err)

	city2, err := testStore.CreateRegion(ctx, CreateRegionParams{
		Code:      util.RandomString(6),
		Name:      "city_b_" + util.RandomString(6),
		Level:     2,
		ParentID:  pgtype.Int8{Int64: province.ID, Valid: true},
		Longitude: pgtype.Numeric{},
		Latitude:  pgtype.Numeric{},
	})
	require.NoError(t, err)

	// 另一个省的城市（不应被 parent_id=province.ID 命中）
	otherProvince := createRandomRegion(t)
	_, err = testStore.CreateRegion(ctx, CreateRegionParams{
		Code:      util.RandomString(6),
		Name:      "city_other_" + util.RandomString(6),
		Level:     2,
		ParentID:  pgtype.Int8{Int64: otherProvince.ID, Valid: true},
		Longitude: pgtype.Numeric{},
		Latitude:  pgtype.Numeric{},
	})
	require.NoError(t, err)

	// parent_id + level 同时过滤
	regions, err := testStore.ListRegions(ctx, ListRegionsParams{
		Limit:    50,
		Offset:   0,
		ParentID: pgtype.Int8{Int64: province.ID, Valid: true},
		Level:    pgtype.Int2{Int16: 2, Valid: true},
	})
	require.NoError(t, err)
	require.Len(t, regions, 2)
	require.Equal(t, city1.ID, regions[0].ID)
	require.Equal(t, city2.ID, regions[1].ID)

	// parent_id 未指定时（NULL），SQL 约定返回 parent_id IS NULL 的根区域
	rootRegions, err := testStore.ListRegions(ctx, ListRegionsParams{Limit: 200, Offset: 0})
	require.NoError(t, err)
	require.NotEmpty(t, rootRegions)
	for _, r := range rootRegions {
		require.False(t, r.ParentID.Valid)
	}
}

func TestRegionQueries_ListRegionChildren(t *testing.T) {
	ctx := context.Background()

	parent := createRandomRegion(t)
	child1, err := testStore.CreateRegion(ctx, CreateRegionParams{
		Code:      util.RandomString(6),
		Name:      "child_1_" + util.RandomString(6),
		Level:     2,
		ParentID:  pgtype.Int8{Int64: parent.ID, Valid: true},
		Longitude: pgtype.Numeric{},
		Latitude:  pgtype.Numeric{},
	})
	require.NoError(t, err)
	child2, err := testStore.CreateRegion(ctx, CreateRegionParams{
		Code:      util.RandomString(6),
		Name:      "child_2_" + util.RandomString(6),
		Level:     2,
		ParentID:  pgtype.Int8{Int64: parent.ID, Valid: true},
		Longitude: pgtype.Numeric{},
		Latitude:  pgtype.Numeric{},
	})
	require.NoError(t, err)

	children, err := testStore.ListRegionChildren(ctx, pgtype.Int8{Int64: parent.ID, Valid: true})
	require.NoError(t, err)
	require.Len(t, children, 2)
	require.Equal(t, child1.ID, children[0].ID)
	require.Equal(t, child2.ID, children[1].ID)
}

func TestRegionQueries_SearchRegionsByName(t *testing.T) {
	ctx := context.Background()
	keyword := "kw_" + util.RandomString(6)

	_, err := testStore.CreateRegion(ctx, CreateRegionParams{
		Code:      util.RandomString(6),
		Name:      "prefix_" + keyword + "_suffix",
		Level:     1,
		ParentID:  pgtype.Int8{Valid: false},
		Longitude: pgtype.Numeric{},
		Latitude:  pgtype.Numeric{},
	})
	require.NoError(t, err)

	results, err := testStore.SearchRegionsByName(ctx, SearchRegionsByNameParams{
		Column1: pgtype.Text{String: keyword, Valid: true},
		Limit:   100,
	})
	require.NoError(t, err)
	require.NotEmpty(t, results)

	found := false
	for _, r := range results {
		if r.Name == "prefix_"+keyword+"_suffix" {
			found = true
			break
		}
	}
	require.True(t, found)
}

func TestRegionQueries_ListAvailableRegions_DefaultLevel3_ExcludesBoundOperators(t *testing.T) {
	ctx := context.Background()

	city := createRandomRegion(t)
	// city 默认 helper 是 level=1，这里改造一个 level=2 的 city
	city, err := testStore.CreateRegion(ctx, CreateRegionParams{
		Code:      util.RandomString(6),
		Name:      "city_" + util.RandomString(6),
		Level:     2,
		ParentID:  pgtype.Int8{Valid: false},
		Longitude: pgtype.Numeric{},
		Latitude:  pgtype.Numeric{},
	})
	require.NoError(t, err)

	district1, err := testStore.CreateRegion(ctx, CreateRegionParams{
		Code:      util.RandomString(6),
		Name:      "district_1_" + util.RandomString(6),
		Level:     3,
		ParentID:  pgtype.Int8{Int64: city.ID, Valid: true},
		Longitude: pgtype.Numeric{},
		Latitude:  pgtype.Numeric{},
	})
	require.NoError(t, err)
	district2, err := testStore.CreateRegion(ctx, CreateRegionParams{
		Code:      util.RandomString(6),
		Name:      "district_2_" + util.RandomString(6),
		Level:     3,
		ParentID:  pgtype.Int8{Int64: city.ID, Valid: true},
		Longitude: pgtype.Numeric{},
		Latitude:  pgtype.Numeric{},
	})
	require.NoError(t, err)

	// 绑定 district2 给一个运营商，应被 ListAvailableRegions 排除
	user := createRandomUser(t)
	commissionRate := pgtype.Numeric{}
	_ = commissionRate.Scan("0.03")
	_, err = testStore.CreateOperator(ctx, CreateOperatorParams{
		UserID:            user.ID,
		RegionID:          district2.ID,
		Name:              "op_" + util.RandomString(6),
		ContactName:       "contact_" + util.RandomString(6),
		ContactPhone:      "138" + util.RandomString(8),
		WechatMchID:       pgtype.Text{Valid: false},
		CommissionRate:    commissionRate,
		Status:            "active",
		ContractStartDate: pgtype.Date{Valid: false},
		ContractEndDate:   pgtype.Date{Valid: false},
		ContractYears:     1,
	})
	require.NoError(t, err)

	rows, err := testStore.ListAvailableRegions(ctx, ListAvailableRegionsParams{
		Limit:    50,
		Offset:   0,
		ParentID: pgtype.Int8{Int64: city.ID, Valid: true},
		Level:    pgtype.Int2{Valid: false}, // nil -> 默认 level=3
	})
	require.NoError(t, err)

	// 只应返回 district1
	require.Len(t, rows, 1)
	require.Equal(t, district1.ID, rows[0].ID)
	require.True(t, rows[0].ParentName.Valid)
	require.Equal(t, city.Name, rows[0].ParentName.String)
}
