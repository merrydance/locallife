package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestListWithdrawalRecordsUsesIDTieBreaker(t *testing.T) {
	user := createRandomUser(t)
	tiedCreatedAt := time.Now().UTC().Truncate(time.Microsecond)

	createWithdrawal := func() WithdrawalRecord {
		record, err := testStore.CreateWithdrawalRecord(context.Background(), CreateWithdrawalRecordParams{
			UserID:      user.ID,
			Amount:      util.RandomMoney(),
			Status:      "completed",
			Channel:     "wechat",
			AccountInfo: []byte(`{"bank_account":"6222020202020202"}`),
			OutRequestNo: pgtype.Text{
				String: "withdraw_" + util.RandomString(12),
				Valid:  true,
			},
		})
		require.NoError(t, err)
		return record
	}

	firstRecord := createWithdrawal()
	secondRecord := createWithdrawal()

	_, err := testStore.(*SQLStore).connPool.Exec(context.Background(),
		`UPDATE withdrawal_records SET created_at = $1 WHERE id = ANY($2)`,
		tiedCreatedAt,
		[]int64{firstRecord.ID, secondRecord.ID},
	)
	require.NoError(t, err)

	rows, err := testStore.ListWithdrawalRecords(context.Background(), ListWithdrawalRecordsParams{
		UserID:  user.ID,
		Channel: "wechat",
		Limit:   2,
		Offset:  0,
	})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, secondRecord.ID, rows[0].ID)
	require.Equal(t, firstRecord.ID, rows[1].ID)
}
