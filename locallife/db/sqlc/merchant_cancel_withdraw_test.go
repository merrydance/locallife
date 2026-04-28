package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestListMerchantCancelWithdrawApplicationsByMerchantUsesIDTieBreaker(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	tiedCreatedAt := time.Now().UTC().Truncate(time.Microsecond)

	createApplication := func() MerchantCancelWithdrawApplication {
		application, err := testStore.CreateMerchantCancelWithdrawApplication(context.Background(), CreateMerchantCancelWithdrawApplicationParams{
			MerchantID:                       merchant.ID,
			CreatedByUserID:                  owner.ID,
			SubMchID:                         "1900" + util.RandomString(6),
			OutRequestNo:                     "cw" + util.RandomString(18),
			Withdraw:                         "APPLY_WITHDRAW",
			BusinessLicenseStatusDeclaration: pgtype.Text{Valid: false},
			ProofMediaAssetIds:               []byte(`[]`),
			AdditionalMaterialAssetIds:       []byte(`[]`),
			Remark:                           pgtype.Text{String: "test", Valid: true},
			LocalSyncState:                   "created",
		})
		require.NoError(t, err)
		return application
	}

	firstApplication := createApplication()
	secondApplication := createApplication()

	_, err := testStore.(*SQLStore).connPool.Exec(context.Background(),
		`UPDATE merchant_cancel_withdraw_applications SET created_at = $1 WHERE id = ANY($2)`,
		tiedCreatedAt,
		[]int64{firstApplication.ID, secondApplication.ID},
	)
	require.NoError(t, err)

	rows, err := testStore.ListMerchantCancelWithdrawApplicationsByMerchant(context.Background(), ListMerchantCancelWithdrawApplicationsByMerchantParams{
		MerchantID: merchant.ID,
		Limit:      2,
		Offset:     0,
	})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, secondApplication.ID, rows[0].ID)
	require.Equal(t, firstApplication.ID, rows[1].ID)
}
