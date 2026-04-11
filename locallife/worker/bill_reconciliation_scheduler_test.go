package worker

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/merrydance/locallife/wechat"
	"github.com/stretchr/testify/require"
)

func TestFetchBillRecords_RetriesStatementCreating(t *testing.T) {
	scheduler := &BillReconciliationScheduler{
		retryWait: func(ctx context.Context, delay time.Duration) error {
			return nil
		},
	}
	billDate := time.Date(2026, 4, 2, 0, 0, 0, 0, time.Local)
	attempts := 0

	records, err := scheduler.fetchBillRecords(context.Background(), "trade", billDate,
		func(ctx context.Context, date time.Time) (map[string]wechat.BillRecord, error) {
			attempts++
			if attempts < 3 {
				return nil, wechat.ErrBillNotReady
			}
			return map[string]wechat.BillRecord{
				"OT123": {OutTradeNo: "OT123", Amount: 100},
			}, nil
		})

	require.NoError(t, err)
	require.Equal(t, 3, attempts)
	require.Len(t, records, 1)
	require.EqualValues(t, 100, records["OT123"].Amount)
}

func TestFetchBillRecords_Treats404AsEmpty(t *testing.T) {
	scheduler := &BillReconciliationScheduler{}
	billDate := time.Date(2026, 4, 2, 0, 0, 0, 0, time.Local)
	attempts := 0

	records, err := scheduler.fetchBillRecords(context.Background(), "refund", billDate,
		func(ctx context.Context, date time.Time) (map[string]wechat.BillRecord, error) {
			attempts++
			return nil, wechat.ErrBillNotFound
		})

	require.NoError(t, err)
	require.Equal(t, 1, attempts)
	require.Empty(t, records)
}

func TestNextBillRetryDelay(t *testing.T) {
	require.Equal(t, 15*time.Second, nextBillRetryDelay(1))
	require.Equal(t, 30*time.Second, nextBillRetryDelay(2))
	require.Equal(t, 60*time.Second, nextBillRetryDelay(3))
	require.Equal(t, 2*time.Minute, nextBillRetryDelay(10))
}

func TestSafeReconciliationCount(t *testing.T) {
	t.Run("accepts int32 range", func(t *testing.T) {
		value, err := safeReconciliationCount(123)
		require.NoError(t, err)
		require.Equal(t, int32(123), value)
	})

	t.Run("rejects overflow", func(t *testing.T) {
		_, err := safeReconciliationCount(math.MaxInt32 + 1)
		require.Error(t, err)
		require.Contains(t, err.Error(), "exceeds int32 max")
	})
}
