package scheduler

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	mockwk "github.com/merrydance/locallife/worker/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestDataCleanupScheduler_RemindExpiringCredentials_DistributesNotificationTask(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	allowPlatformAlertEventPersistence(store)
	s := NewDataCleanupScheduler(store, distributor, nil)

	now := time.Now()
	expiresAt := startOfDay(now).AddDate(0, 0, credentialReminderDays).Add(3 * time.Hour)
	ledger := db.CredentialLedger{
		ID:           11,
		SubjectType:  "merchant",
		MerchantID:   pgtype.Int8{Int64: 99, Valid: true},
		DocumentType: db.CredentialDocumentTypeBusinessLicense,
		ExpiresAt:    pgtype.Timestamptz{Time: expiresAt, Valid: true},
		Active:       true,
	}
	merchant := db.Merchant{ID: 99, OwnerUserID: 66}

	store.EXPECT().ListCredentialsForReminderWindow(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.ListCredentialsForReminderWindowParams) ([]db.CredentialLedger, error) {
			require.Equal(t, startOfDay(now).AddDate(0, 0, credentialReminderDays), arg.WindowStart.Time)
			return []db.CredentialLedger{ledger}, nil
		},
	).Times(1)
	store.EXPECT().GetMerchant(gomock.Any(), merchant.ID).Return(merchant, nil)
	distributor.EXPECT().DistributeTaskSendNotification(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, payload *worker.SendNotificationPayload, _ ...asynq.Option) error {
			require.Equal(t, merchant.OwnerUserID, payload.UserID)
			require.Equal(t, "system", payload.Type)
			require.Equal(t, "merchant", payload.RelatedType)
			require.Equal(t, merchant.ID, payload.RelatedID)
			require.Contains(t, payload.Title, "营业执照")
			require.Contains(t, payload.Title, "7 天")
			require.EqualValues(t, ledger.ID, payload.ExtraData["credential_ledger_id"])
			require.Equal(t, "merchant", payload.ExtraData["subject_type"])
			require.Equal(t, db.CredentialDocumentTypeBusinessLicense, payload.ExtraData["document_type"])
			require.EqualValues(t, credentialReminderDays, payload.ExtraData["days_remaining"])
			require.Equal(t, credentialNotificationSource("reminder"), payload.ExtraData["notification_source"])
			return nil
		},
	)
	store.EXPECT().MarkCredentialLedgerReminderSent(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.MarkCredentialLedgerReminderSentParams) (db.CredentialLedger, error) {
			require.Equal(t, ledger.ID, arg.ID)
			require.True(t, arg.LastRemindedAt.Valid)
			return ledger, nil
		},
	)
	s.remindExpiringCredentials()
}

func TestDataCleanupScheduler_RemindExpiringCredentials_SkipsAlreadyRemindedWindow(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	s := NewDataCleanupScheduler(store, nil, nil)

	now := time.Now()
	expiresAt := startOfDay(now).AddDate(0, 0, credentialReminderDays).Add(5 * time.Hour)
	ledger := db.CredentialLedger{
		ID:             12,
		SubjectType:    "rider",
		RiderID:        pgtype.Int8{Int64: 77, Valid: true},
		DocumentType:   db.CredentialDocumentTypeHealthCert,
		ExpiresAt:      pgtype.Timestamptz{Time: expiresAt, Valid: true},
		LastRemindedAt: pgtype.Timestamptz{Time: startOfDay(now).Add(2 * time.Hour), Valid: true},
		Active:         true,
	}

	store.EXPECT().ListCredentialsForReminderWindow(gomock.Any(), gomock.Any()).Return([]db.CredentialLedger{ledger}, nil)
	s.remindExpiringCredentials()
}

func TestDataCleanupScheduler_RemindExpiringCredentials_FallbackDirectNotificationAndPublishAlert(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	allowPlatformAlertEventPersistence(store)
	publisher := &recordingPublisher{}
	s := NewDataCleanupScheduler(store, worker.NewNoopTaskDistributor(), publisher)

	now := time.Now()
	expiresAt := startOfDay(now).AddDate(0, 0, credentialReminderDays).Add(4 * time.Hour)
	ledger := db.CredentialLedger{
		ID:           21,
		SubjectType:  "rider",
		RiderID:      pgtype.Int8{Int64: 88, Valid: true},
		DocumentType: db.CredentialDocumentTypeHealthCert,
		ExpiresAt:    pgtype.Timestamptz{Time: expiresAt, Valid: true},
		Active:       true,
	}
	rider := db.Rider{ID: 88, UserID: 55}

	store.EXPECT().ListCredentialsForReminderWindow(gomock.Any(), gomock.Any()).Return([]db.CredentialLedger{ledger}, nil)
	store.EXPECT().GetRider(gomock.Any(), rider.ID).Return(rider, nil)
	store.EXPECT().CreateNotification(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.CreateNotificationParams) (db.Notification, error) {
			require.Equal(t, rider.UserID, arg.UserID)
			require.Equal(t, "system", arg.Type)
			require.Contains(t, arg.Title, "健康证")
			require.True(t, arg.RelatedType.Valid)
			require.Equal(t, "rider", arg.RelatedType.String)
			require.True(t, arg.RelatedID.Valid)
			require.Equal(t, rider.ID, arg.RelatedID.Int64)

			var extra map[string]any
			require.NoError(t, json.Unmarshal(arg.ExtraData, &extra))
			require.EqualValues(t, ledger.ID, extra["credential_ledger_id"])
			require.Equal(t, credentialNotificationSource("reminder"), extra["notification_source"])
			return db.Notification{ID: 1, UserID: rider.UserID}, nil
		},
	)
	store.EXPECT().MarkCredentialLedgerReminderSent(gomock.Any(), gomock.Any()).Return(ledger, nil)
	s.remindExpiringCredentials()

	published := publisher.snapshot()
	require.Len(t, published, 1)
	require.Equal(t, worker.AlertChannel, published[0].channel)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(published[0].payload, &payload))
	alertData := payload["data"].(map[string]any)
	require.Equal(t, string(worker.AlertTypeCredentialExpiry), alertData["alert_type"])
	require.Equal(t, "入驻资质到期提醒已发送", alertData["title"])
}

func TestDataCleanupScheduler_EnforceExpiredCredentials_ClaimsSuspensionsAndPublishesAlert(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	allowPlatformAlertEventPersistence(store)
	publisher := &recordingPublisher{}
	s := NewDataCleanupScheduler(store, worker.NewNoopTaskDistributor(), publisher)

	now := time.Now()
	merchantLedger := db.CredentialLedger{
		ID:           31,
		SubjectType:  "merchant",
		MerchantID:   pgtype.Int8{Int64: 101, Valid: true},
		DocumentType: db.CredentialDocumentTypeFoodPermit,
		ExpiresAt:    pgtype.Timestamptz{Time: now.Add(-2 * time.Hour), Valid: true},
		Active:       true,
	}
	riderLedger := db.CredentialLedger{
		ID:           32,
		SubjectType:  "rider",
		RiderID:      pgtype.Int8{Int64: 202, Valid: true},
		DocumentType: db.CredentialDocumentTypeHealthCert,
		ExpiresAt:    pgtype.Timestamptz{Time: now.Add(-4 * time.Hour), Valid: true},
		Active:       true,
	}
	alreadySuspended := db.CredentialLedger{
		ID:                   33,
		SubjectType:          "merchant",
		MerchantID:           pgtype.Int8{Int64: 303, Valid: true},
		DocumentType:         db.CredentialDocumentTypeBusinessLicense,
		ExpiresAt:            pgtype.Timestamptz{Time: now.Add(-24 * time.Hour), Valid: true},
		SuspendedAt:          pgtype.Timestamptz{Time: now.Add(-2 * time.Hour), Valid: true},
		SuspensionReasonCode: pgtype.Text{String: db.CredentialSuspensionReasonDocumentExpired, Valid: true},
		Active:               true,
	}
	merchant := db.Merchant{ID: 101, OwnerUserID: 501}
	rider := db.Rider{ID: 202, UserID: 601}

	store.EXPECT().ListExpiredActiveCredentialLedgers(gomock.Any(), gomock.Any()).Return([]db.CredentialLedger{merchantLedger, riderLedger, alreadySuspended}, nil)
	store.EXPECT().ClaimMerchantTakeoutSuspensionIfAvailable(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.ClaimMerchantTakeoutSuspensionIfAvailableParams) (int64, error) {
			require.Equal(t, merchant.ID, arg.MerchantID)
			require.True(t, arg.TakeoutSuspendReason.Valid)
			require.Equal(t, db.CredentialSuspensionReasonDocumentExpired, arg.TakeoutSuspendReason.String)
			return 1, nil
		},
	)
	store.EXPECT().MarkCredentialLedgerSuspended(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.MarkCredentialLedgerSuspendedParams) (db.CredentialLedger, error) {
			require.Equal(t, merchantLedger.ID, arg.ID)
			require.True(t, arg.SuspendedAt.Valid)
			require.True(t, arg.SuspensionReasonCode.Valid)
			return merchantLedger, nil
		},
	)
	store.EXPECT().GetMerchant(gomock.Any(), merchant.ID).Return(merchant, nil)
	store.EXPECT().CreateNotification(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.CreateNotificationParams) (db.Notification, error) {
			require.Equal(t, merchant.OwnerUserID, arg.UserID)
			require.Contains(t, arg.Content, "自动暂停")
			return db.Notification{ID: 10, UserID: merchant.OwnerUserID}, nil
		},
	)
	store.EXPECT().ClaimRiderSuspensionIfAvailable(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.ClaimRiderSuspensionIfAvailableParams) (int64, error) {
			require.Equal(t, rider.ID, arg.RiderID)
			require.True(t, arg.SuspendReason.Valid)
			require.Equal(t, db.CredentialSuspensionReasonDocumentExpired, arg.SuspendReason.String)
			return 1, nil
		},
	)
	store.EXPECT().MarkCredentialLedgerSuspended(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.MarkCredentialLedgerSuspendedParams) (db.CredentialLedger, error) {
			require.Equal(t, riderLedger.ID, arg.ID)
			return riderLedger, nil
		},
	)
	store.EXPECT().GetRider(gomock.Any(), rider.ID).Return(rider, nil)
	store.EXPECT().CreateNotification(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, arg db.CreateNotificationParams) (db.Notification, error) {
			require.Equal(t, rider.UserID, arg.UserID)
			require.Contains(t, arg.Content, "接单资格")
			return db.Notification{ID: 11, UserID: rider.UserID}, nil
		},
	)
	s.enforceExpiredCredentials()

	published := publisher.snapshot()
	require.Len(t, published, 1)
	require.Equal(t, worker.AlertChannel, published[0].channel)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(published[0].payload, &payload))
	alertData := payload["data"].(map[string]any)
	require.Equal(t, string(worker.AlertTypeCredentialExpiry), alertData["alert_type"])
	require.Equal(t, "入驻资质过期已触发自动暂停", alertData["title"])
}

func TestDataCleanupScheduler_RemindExpiringCredentials_UsesCursorPagination(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	allowPlatformAlertEventPersistence(store)
	s := NewDataCleanupScheduler(store, nil, nil)

	now := time.Now()
	first := db.CredentialLedger{
		ID:           101,
		SubjectType:  "merchant",
		MerchantID:   pgtype.Int8{Int64: 1, Valid: true},
		DocumentType: db.CredentialDocumentTypeBusinessLicense,
		ExpiresAt:    pgtype.Timestamptz{Time: startOfDay(now).AddDate(0, 0, credentialReminderDays).Add(time.Hour), Valid: true},
		Active:       true,
	}
	firstBatch := make([]db.CredentialLedger, 0, credentialReminderBatchLimit)
	for index := int32(0); index < credentialReminderBatchLimit; index++ {
		ledger := first
		ledger.ID = int64(index) + 1
		ledger.ExpiresAt = pgtype.Timestamptz{Time: first.ExpiresAt.Time.Add(time.Duration(index) * time.Minute), Valid: true}
		firstBatch = append(firstBatch, ledger)
	}
	first = firstBatch[len(firstBatch)-1]
	second := db.CredentialLedger{
		ID:           202,
		SubjectType:  "merchant",
		MerchantID:   pgtype.Int8{Int64: 2, Valid: true},
		DocumentType: db.CredentialDocumentTypeFoodPermit,
		ExpiresAt:    pgtype.Timestamptz{Time: first.ExpiresAt.Time.Add(time.Hour), Valid: true},
		Active:       true,
	}

	store.EXPECT().GetMerchant(gomock.Any(), int64(1)).AnyTimes().Return(db.Merchant{ID: 1, OwnerUserID: 11}, nil)
	store.EXPECT().GetMerchant(gomock.Any(), int64(2)).AnyTimes().Return(db.Merchant{ID: 2, OwnerUserID: 22}, nil)
	store.EXPECT().CreateNotification(gomock.Any(), gomock.Any()).AnyTimes().Return(db.Notification{ID: 1}, nil)
	store.EXPECT().MarkCredentialLedgerReminderSent(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(
		func(ctx context.Context, arg db.MarkCredentialLedgerReminderSentParams) (db.CredentialLedger, error) {
			return db.CredentialLedger{ID: arg.ID}, nil
		},
	)
	callIndex := 0
	store.EXPECT().ListCredentialsForReminderWindow(gomock.Any(), gomock.Any()).Times(2).DoAndReturn(
		func(ctx context.Context, arg db.ListCredentialsForReminderWindowParams) ([]db.CredentialLedger, error) {
			callIndex++
			switch callIndex {
			case 1:
				require.False(t, arg.LastExpiresAt.Valid)
				require.Zero(t, arg.LastID)
				return firstBatch, nil
			case 2:
				require.True(t, arg.LastExpiresAt.Valid)
				require.Equal(t, first.ExpiresAt.Time, arg.LastExpiresAt.Time)
				require.Equal(t, first.ID, arg.LastID)
				return []db.CredentialLedger{second}, nil
			default:
				return nil, nil
			}
		},
	)

	s.remindExpiringCredentials()
}

func TestDataCleanupScheduler_EnforceExpiredCredentials_UsesCursorPagination(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	allowPlatformAlertEventPersistence(store)
	s := NewDataCleanupScheduler(store, nil, nil)

	now := time.Now()
	first := db.CredentialLedger{
		ID:           301,
		SubjectType:  "rider",
		RiderID:      pgtype.Int8{Int64: 9, Valid: true},
		DocumentType: db.CredentialDocumentTypeHealthCert,
		ExpiresAt:    pgtype.Timestamptz{Time: now.Add(-2 * time.Hour), Valid: true},
		Active:       true,
	}
	firstBatch := make([]db.CredentialLedger, 0, credentialExpireBatchLimit)
	for index := int32(0); index < credentialExpireBatchLimit; index++ {
		ledger := first
		ledger.ID = int64(index) + 1
		ledger.ExpiresAt = pgtype.Timestamptz{Time: first.ExpiresAt.Time.Add(time.Duration(index) * time.Minute), Valid: true}
		firstBatch = append(firstBatch, ledger)
	}
	first = firstBatch[len(firstBatch)-1]
	second := db.CredentialLedger{
		ID:           302,
		SubjectType:  "rider",
		RiderID:      pgtype.Int8{Int64: 10, Valid: true},
		DocumentType: db.CredentialDocumentTypeHealthCert,
		ExpiresAt:    pgtype.Timestamptz{Time: first.ExpiresAt.Time.Add(time.Minute), Valid: true},
		Active:       true,
	}

	store.EXPECT().ClaimRiderSuspensionIfAvailable(gomock.Any(), gomock.Any()).AnyTimes().Return(int64(1), nil)
	store.EXPECT().MarkCredentialLedgerSuspended(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(
		func(ctx context.Context, arg db.MarkCredentialLedgerSuspendedParams) (db.CredentialLedger, error) {
			return db.CredentialLedger{ID: arg.ID}, nil
		},
	)
	store.EXPECT().GetRider(gomock.Any(), int64(9)).AnyTimes().Return(db.Rider{ID: 9, UserID: 99}, nil)
	store.EXPECT().GetRider(gomock.Any(), int64(10)).AnyTimes().Return(db.Rider{ID: 10, UserID: 100}, nil)
	store.EXPECT().CreateNotification(gomock.Any(), gomock.Any()).AnyTimes().Return(db.Notification{ID: 1}, nil)
	callIndex := 0
	store.EXPECT().ListExpiredActiveCredentialLedgers(gomock.Any(), gomock.Any()).Times(2).DoAndReturn(
		func(ctx context.Context, arg db.ListExpiredActiveCredentialLedgersParams) ([]db.CredentialLedger, error) {
			callIndex++
			switch callIndex {
			case 1:
				require.False(t, arg.LastExpiresAt.Valid)
				require.Zero(t, arg.LastID)
				return firstBatch, nil
			case 2:
				require.True(t, arg.LastExpiresAt.Valid)
				require.Equal(t, first.ExpiresAt.Time, arg.LastExpiresAt.Time)
				require.Equal(t, first.ID, arg.LastID)
				return []db.CredentialLedger{second}, nil
			default:
				return nil, nil
			}
		},
	)

	s.enforceExpiredCredentials()
}

func TestDataCleanupScheduler_EnforceExpiredCredentials_DoesNotResuspendRestoredLedger(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	s := NewDataCleanupScheduler(store, nil, nil)

	now := time.Now()
	ledger := db.CredentialLedger{
		ID:                   401,
		SubjectType:          "merchant",
		MerchantID:           pgtype.Int8{Int64: 88, Valid: true},
		DocumentType:         db.CredentialDocumentTypeBusinessLicense,
		ExpiresAt:            pgtype.Timestamptz{Time: now.Add(-2 * time.Hour), Valid: true},
		SuspendedAt:          pgtype.Timestamptz{Time: now.Add(-time.Hour), Valid: true},
		ResumedAt:            pgtype.Timestamptz{Time: now.Add(-30 * time.Minute), Valid: true},
		SuspensionReasonCode: pgtype.Text{String: db.CredentialSuspensionReasonDocumentExpired, Valid: true},
		Active:               true,
	}

	store.EXPECT().ListExpiredActiveCredentialLedgers(gomock.Any(), gomock.Any()).Return([]db.CredentialLedger{ledger}, nil)

	s.enforceExpiredCredentials()
}
