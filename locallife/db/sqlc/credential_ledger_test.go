package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

const credentialReminderWindowDays = 7

func credentialLedgerStartOfDay(value time.Time) time.Time {
	utc := value.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}

func TestCreateMerchantCredentialLedger_RejectsDuplicateActiveDocument(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)
	application := createRandomMerchantApplicationWithUser(t, owner.ID)
	assetOne := createRandomMediaAsset(t, owner.ID)
	assetTwo := createRandomMediaAsset(t, owner.ID)

	firstLedger, err := testStore.CreateMerchantCredentialLedger(context.Background(), CreateMerchantCredentialLedgerParams{
		MerchantID:            pgtype.Int8{Int64: merchant.ID, Valid: true},
		DocumentType:          CredentialDocumentTypeBusinessLicense,
		MerchantApplicationID: pgtype.Int8{Int64: application.ID, Valid: true},
		MediaAssetID:          assetOne.ID,
		NormalizedPayload:     []byte(`{"credit_code":"first"}`),
	})
	require.NoError(t, err)
	require.True(t, firstLedger.Active)

	_, err = testStore.CreateMerchantCredentialLedger(context.Background(), CreateMerchantCredentialLedgerParams{
		MerchantID:            pgtype.Int8{Int64: merchant.ID, Valid: true},
		DocumentType:          CredentialDocumentTypeBusinessLicense,
		MerchantApplicationID: pgtype.Int8{Int64: application.ID, Valid: true},
		MediaAssetID:          assetTwo.ID,
		NormalizedPayload:     []byte(`{"credit_code":"duplicate"}`),
	})
	require.Error(t, err)
	require.Equal(t, UniqueViolation, ErrorCode(err))

	activeLedgers, err := testStore.GetActiveMerchantCredentialLedgers(context.Background(), pgtype.Int8{Int64: merchant.ID, Valid: true})
	require.NoError(t, err)
	require.Len(t, activeLedgers, 1)
	require.Equal(t, firstLedger.ID, activeLedgers[0].ID)
}

func TestCredentialLedgerScans_FilterByWindowAndActiveStatus(t *testing.T) {
	now := time.Now().UTC()
	windowStart := credentialLedgerStartOfDay(now).AddDate(0, 0, credentialReminderWindowDays)
	windowEnd := windowStart.Add(24 * time.Hour)

	ownerOne := createRandomUser(t)
	merchantOne := createRandomMerchantWithOwner(t, ownerOne.ID)
	applicationOne := createRandomMerchantApplicationWithUser(t, ownerOne.ID)
	expiringAsset := createRandomMediaAsset(t, ownerOne.ID)
	expiringLedger, err := testStore.CreateMerchantCredentialLedger(context.Background(), CreateMerchantCredentialLedgerParams{
		MerchantID:            pgtype.Int8{Int64: merchantOne.ID, Valid: true},
		DocumentType:          CredentialDocumentTypeBusinessLicense,
		MerchantApplicationID: pgtype.Int8{Int64: applicationOne.ID, Valid: true},
		MediaAssetID:          expiringAsset.ID,
		NormalizedPayload:     []byte(`{"state":"expiring"}`),
		ExpiresAt:             pgtype.Timestamptz{Time: windowStart.Add(2 * time.Hour), Valid: true},
	})
	require.NoError(t, err)

	ownerTwo := createRandomUser(t)
	merchantTwo := createRandomMerchantWithOwner(t, ownerTwo.ID)
	applicationTwo := createRandomMerchantApplicationWithUser(t, ownerTwo.ID)
	remindedAsset := createRandomMediaAsset(t, ownerTwo.ID)
	remindedLedger, err := testStore.CreateMerchantCredentialLedger(context.Background(), CreateMerchantCredentialLedgerParams{
		MerchantID:            pgtype.Int8{Int64: merchantTwo.ID, Valid: true},
		DocumentType:          CredentialDocumentTypeBusinessLicense,
		MerchantApplicationID: pgtype.Int8{Int64: applicationTwo.ID, Valid: true},
		MediaAssetID:          remindedAsset.ID,
		NormalizedPayload:     []byte(`{"state":"reminded"}`),
		ExpiresAt:             pgtype.Timestamptz{Time: windowStart.Add(4 * time.Hour), Valid: true},
	})
	require.NoError(t, err)
	_, err = testStore.MarkCredentialLedgerReminderSent(context.Background(), MarkCredentialLedgerReminderSentParams{
		ID:             remindedLedger.ID,
		LastRemindedAt: pgtype.Timestamptz{Time: windowStart.Add(time.Minute), Valid: true},
	})
	require.NoError(t, err)

	ownerThree := createRandomUser(t)
	merchantThree := createRandomMerchantWithOwner(t, ownerThree.ID)
	applicationThree := createRandomMerchantApplicationWithUser(t, ownerThree.ID)
	expiredAsset := createRandomMediaAsset(t, ownerThree.ID)
	expiredLedger, err := testStore.CreateMerchantCredentialLedger(context.Background(), CreateMerchantCredentialLedgerParams{
		MerchantID:            pgtype.Int8{Int64: merchantThree.ID, Valid: true},
		DocumentType:          CredentialDocumentTypeFoodPermit,
		MerchantApplicationID: pgtype.Int8{Int64: applicationThree.ID, Valid: true},
		MediaAssetID:          expiredAsset.ID,
		NormalizedPayload:     []byte(`{"state":"expired"}`),
		ExpiresAt:             pgtype.Timestamptz{Time: now.Add(-2 * time.Hour), Valid: true},
	})
	require.NoError(t, err)

	ownerFour := createRandomUser(t)
	merchantFour := createRandomMerchantWithOwner(t, ownerFour.ID)
	applicationFour := createRandomMerchantApplicationWithUser(t, ownerFour.ID)
	inactiveAsset := createRandomMediaAsset(t, ownerFour.ID)
	inactiveLedger, err := testStore.CreateMerchantCredentialLedger(context.Background(), CreateMerchantCredentialLedgerParams{
		MerchantID:            pgtype.Int8{Int64: merchantFour.ID, Valid: true},
		DocumentType:          CredentialDocumentTypeFoodPermit,
		MerchantApplicationID: pgtype.Int8{Int64: applicationFour.ID, Valid: true},
		MediaAssetID:          inactiveAsset.ID,
		NormalizedPayload:     []byte(`{"state":"inactive"}`),
		ExpiresAt:             pgtype.Timestamptz{Time: now.Add(-4 * time.Hour), Valid: true},
	})
	require.NoError(t, err)
	deactivatedAt := now.Add(-time.Hour)
	rows, err := testStore.DeactivateMerchantActiveCredentialLedger(context.Background(), DeactivateMerchantActiveCredentialLedgerParams{
		MerchantID:    pgtype.Int8{Int64: merchantFour.ID, Valid: true},
		DocumentType:  CredentialDocumentTypeFoodPermit,
		DeactivatedAt: pgtype.Timestamptz{Time: deactivatedAt, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), rows)

	reminderWindowRows, err := testStore.ListCredentialsForReminderWindow(context.Background(), ListCredentialsForReminderWindowParams{
		WindowStart:   pgtype.Timestamptz{Time: windowStart, Valid: true},
		WindowEnd:     pgtype.Timestamptz{Time: windowEnd, Valid: true},
		LastExpiresAt: pgtype.Timestamptz{},
		LastID:        0,
		PageLimit:     20,
	})
	require.NoError(t, err)
	require.Len(t, reminderWindowRows, 1)
	require.Equal(t, expiringLedger.ID, reminderWindowRows[0].ID)

	expiredRows, err := testStore.ListExpiredActiveCredentialLedgers(context.Background(), ListExpiredActiveCredentialLedgersParams{
		ExpiredBefore: pgtype.Timestamptz{Time: now, Valid: true},
		LastExpiresAt: pgtype.Timestamptz{},
		LastID:        0,
		PageLimit:     20,
	})
	require.NoError(t, err)
	require.Len(t, expiredRows, 1)
	require.Equal(t, expiredLedger.ID, expiredRows[0].ID)
	require.NotEqual(t, inactiveLedger.ID, expiredRows[0].ID)
}

func TestReleaseMerchantTakeoutSuspensionIfOwned(t *testing.T) {
	owner := createRandomUser(t)
	merchant := createRandomMerchantWithOwner(t, owner.ID)

	_, err := testStore.CreateMerchantProfile(context.Background(), merchant.ID)
	require.NoError(t, err)

	err = testStore.SuspendMerchantTakeout(context.Background(), SuspendMerchantTakeoutParams{
		MerchantID:           merchant.ID,
		TakeoutSuspendReason: pgtype.Text{String: CredentialSuspensionReasonDocumentExpired, Valid: true},
		TakeoutSuspendUntil:  pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
	})
	require.NoError(t, err)

	releasedRows, err := testStore.ReleaseMerchantTakeoutSuspensionIfOwned(context.Background(), ReleaseMerchantTakeoutSuspensionIfOwnedParams{
		MerchantID:           merchant.ID,
		TakeoutSuspendReason: pgtype.Text{String: CredentialSuspensionReasonDocumentExpired, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), releasedRows)

	profile, err := testStore.GetMerchantProfile(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.False(t, profile.IsTakeoutSuspended)
	require.False(t, profile.TakeoutSuspendReason.Valid)

	err = testStore.SuspendMerchantTakeout(context.Background(), SuspendMerchantTakeoutParams{
		MerchantID:           merchant.ID,
		TakeoutSuspendReason: pgtype.Text{String: "claim recovery overdue", Valid: true},
		TakeoutSuspendUntil:  pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
	})
	require.NoError(t, err)

	releasedRows, err = testStore.ReleaseMerchantTakeoutSuspensionIfOwned(context.Background(), ReleaseMerchantTakeoutSuspensionIfOwnedParams{
		MerchantID:           merchant.ID,
		TakeoutSuspendReason: pgtype.Text{String: CredentialSuspensionReasonDocumentExpired, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, int64(0), releasedRows)

	profile, err = testStore.GetMerchantProfile(context.Background(), merchant.ID)
	require.NoError(t, err)
	require.True(t, profile.IsTakeoutSuspended)
	require.True(t, profile.TakeoutSuspendReason.Valid)
	require.Equal(t, "claim recovery overdue", profile.TakeoutSuspendReason.String)
}

func TestReleaseRiderSuspensionIfOwned(t *testing.T) {
	rider := createRandomRider(t)

	_, err := testStore.CreateRiderProfile(context.Background(), rider.ID)
	require.NoError(t, err)

	err = testStore.SuspendRider(context.Background(), SuspendRiderParams{
		RiderID:       rider.ID,
		SuspendReason: pgtype.Text{String: CredentialSuspensionReasonDocumentExpired, Valid: true},
		SuspendUntil:  pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
	})
	require.NoError(t, err)

	releasedRows, err := testStore.ReleaseRiderSuspensionIfOwned(context.Background(), ReleaseRiderSuspensionIfOwnedParams{
		RiderID:       rider.ID,
		SuspendReason: pgtype.Text{String: CredentialSuspensionReasonDocumentExpired, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), releasedRows)

	profile, err := testStore.GetRiderProfile(context.Background(), rider.ID)
	require.NoError(t, err)
	require.False(t, profile.IsSuspended)
	require.False(t, profile.SuspendReason.Valid)

	err = testStore.SuspendRider(context.Background(), SuspendRiderParams{
		RiderID:       rider.ID,
		SuspendReason: pgtype.Text{String: "manual_compliance_hold", Valid: true},
		SuspendUntil:  pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
	})
	require.NoError(t, err)

	releasedRows, err = testStore.ReleaseRiderSuspensionIfOwned(context.Background(), ReleaseRiderSuspensionIfOwnedParams{
		RiderID:       rider.ID,
		SuspendReason: pgtype.Text{String: CredentialSuspensionReasonDocumentExpired, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, int64(0), releasedRows)

	profile, err = testStore.GetRiderProfile(context.Background(), rider.ID)
	require.NoError(t, err)
	require.True(t, profile.IsSuspended)
	require.True(t, profile.SuspendReason.Valid)
	require.Equal(t, "manual_compliance_hold", profile.SuspendReason.String)
}