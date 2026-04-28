package db

import (
	"context"
	"testing"
	"time"

	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func TestListSearchHistoryUsesIDTieBreaker(t *testing.T) {
	user := createRandomUser(t)
	tiedCreatedAt := time.Now().UTC().Truncate(time.Microsecond)

	createHistory := func(keyword string) SearchHistory {
		history, err := testStore.CreateSearchHistory(context.Background(), CreateSearchHistoryParams{
			UserID:  user.ID,
			Keyword: keyword,
			Type:    "merchant",
		})
		require.NoError(t, err)
		return history
	}

	firstHistory := createHistory("kw_" + util.RandomString(8))
	secondHistory := createHistory("kw_" + util.RandomString(8))

	_, err := testStore.(*SQLStore).connPool.Exec(context.Background(),
		`UPDATE search_histories SET created_at = $1 WHERE id = ANY($2)`,
		tiedCreatedAt,
		[]int64{firstHistory.ID, secondHistory.ID},
	)
	require.NoError(t, err)

	rows, err := testStore.ListSearchHistory(context.Background(), ListSearchHistoryParams{
		UserID: user.ID,
		Limit:  2,
	})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Equal(t, secondHistory.ID, rows[0].ID)
	require.Equal(t, firstHistory.ID, rows[1].ID)
}
