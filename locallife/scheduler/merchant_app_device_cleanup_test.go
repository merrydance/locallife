package scheduler

import (
	"context"
	"testing"
	"time"

	mockdb "github.com/merrydance/locallife/db/mock"
	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestDataCleanupScheduler_CleanupStaleMerchantAppDevices(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	s := NewDataCleanupScheduler(store, nil, nil)
	before := time.Now()

	store.EXPECT().
		DeactivateStaleMerchantAppDevices(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, cutoff time.Time) (int64, error) {
			require.True(t, cutoff.Before(before.Add(-89*24*time.Hour)))
			require.True(t, cutoff.After(before.Add(-91*24*time.Hour)))
			return int64(3), nil
		})

	s.cleanupStaleMerchantAppDevices()
}

func TestDataCleanupScheduler_StaleMerchantAppDeviceCleanupSchedule(t *testing.T) {
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(staleMerchantAppDeviceCleanupCron)
	require.NoError(t, err)

	base := time.Date(2026, time.June, 11, 0, 0, 0, 0, time.Local)
	expectedNext := time.Date(2026, time.June, 11, 4, 30, 0, 0, time.Local)
	require.Equal(t, expectedNext, schedule.Next(base))
}
