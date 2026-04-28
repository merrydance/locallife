package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestListMerchantGroupsUsesIDTieBreaker(t *testing.T) {
	owner := createRandomUser(t)
	region := createRandomRegion(t)
	tiedCreatedAt := time.Now().UTC().Truncate(time.Microsecond)
	keyword := "group_" + util.RandomString(8)

	createGroup := func(name string) MerchantGroup {
		group, err := testStore.CreateMerchantGroup(context.Background(), CreateMerchantGroupParams{
			Name:                name,
			OwnerUserID:         owner.ID,
			ContactPhone:        pgtype.Text{String: "13800000000", Valid: true},
			LicenseNumber:       pgtype.Text{String: "LIC-" + util.RandomString(8), Valid: true},
			LicenseMediaAssetID: pgtype.Int8{Valid: false},
			Address:             pgtype.Text{String: "测试地址", Valid: true},
			RegionID:            pgtype.Int8{Int64: region.ID, Valid: true},
			ApplicationData:     []byte(`{"source":"test"}`),
		})
		require.NoError(t, err)
		return group
	}

	firstGroup := createGroup(keyword + "_a")
	secondGroup := createGroup(keyword + "_b")

	_, err := testStore.(*SQLStore).connPool.Exec(context.Background(),
		`UPDATE merchant_groups SET created_at = $1 WHERE id = ANY($2)`,
		tiedCreatedAt,
		[]int64{firstGroup.ID, secondGroup.ID},
	)
	require.NoError(t, err)

	rows, err := testStore.ListMerchantGroups(context.Background(), ListMerchantGroupsParams{
		Column1: keyword,
		Limit:   2,
		Offset:  0,
	})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, secondGroup.ID, rows[0].ID)
	require.Equal(t, firstGroup.ID, rows[1].ID)
}
