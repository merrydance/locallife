package logic

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
)

type credentialGovernanceStoreStub struct {
	activateMerchantFn  func(context.Context, db.ActivateMerchantCredentialLedgersTxParams) ([]db.CredentialLedger, error)
	activateRiderFn     func(context.Context, db.ActivateRiderCredentialLedgersTxParams) ([]db.CredentialLedger, error)
	getMerchantActiveFn func(context.Context, pgtype.Int8) ([]db.CredentialLedger, error)
	getRiderActiveFn    func(context.Context, pgtype.Int8) ([]db.CredentialLedger, error)
	restoreMerchantFn   func(context.Context, db.RestoreMerchantCredentialGovernanceTxParams) (int64, error)
	restoreRiderFn      func(context.Context, db.RestoreRiderCredentialGovernanceTxParams) (int64, error)
}

func (stub credentialGovernanceStoreStub) ActivateMerchantCredentialLedgersTx(ctx context.Context, arg db.ActivateMerchantCredentialLedgersTxParams) ([]db.CredentialLedger, error) {
	return stub.activateMerchantFn(ctx, arg)
}

func (stub credentialGovernanceStoreStub) ActivateRiderCredentialLedgersTx(ctx context.Context, arg db.ActivateRiderCredentialLedgersTxParams) ([]db.CredentialLedger, error) {
	return stub.activateRiderFn(ctx, arg)
}

func (stub credentialGovernanceStoreStub) GetActiveMerchantCredentialLedgers(ctx context.Context, merchantID pgtype.Int8) ([]db.CredentialLedger, error) {
	return stub.getMerchantActiveFn(ctx, merchantID)
}

func (stub credentialGovernanceStoreStub) GetActiveRiderCredentialLedgers(ctx context.Context, riderID pgtype.Int8) ([]db.CredentialLedger, error) {
	return stub.getRiderActiveFn(ctx, riderID)
}

func (stub credentialGovernanceStoreStub) RestoreMerchantCredentialGovernanceTx(ctx context.Context, arg db.RestoreMerchantCredentialGovernanceTxParams) (int64, error) {
	return stub.restoreMerchantFn(ctx, arg)
}

func (stub credentialGovernanceStoreStub) RestoreRiderCredentialGovernanceTx(ctx context.Context, arg db.RestoreRiderCredentialGovernanceTxParams) (int64, error) {
	return stub.restoreRiderFn(ctx, arg)
}

func TestCredentialGovernanceServiceActivateMerchantCredentials(t *testing.T) {
	now := time.Date(2026, 4, 22, 10, 30, 0, 0, time.UTC)
	expiresAt := now.AddDate(1, 0, 0)
	service := NewCredentialGovernanceService(credentialGovernanceStoreStub{
		activateMerchantFn: func(_ context.Context, arg db.ActivateMerchantCredentialLedgersTxParams) ([]db.CredentialLedger, error) {
			require.Equal(t, int64(2001), arg.MerchantID)
			require.Equal(t, now, arg.ActivatedAt.Time)
			require.Len(t, arg.Entries, 2)
			require.Equal(t, int64(301), arg.Entries[0].MerchantApplicationID)
			require.Equal(t, int64(888), arg.Entries[0].ReviewRunID.Int64)
			require.Equal(t, int64(9001), arg.Entries[0].MediaAssetID)
			require.Equal(t, expiresAt, arg.Entries[0].ExpiresAt.Time)
			var payload map[string]any
			require.NoError(t, json.Unmarshal(arg.Entries[0].NormalizedPayload, &payload))
			require.Equal(t, "9135X", payload["credit_code"])
			return []db.CredentialLedger{{ID: 1}}, nil
		},
		activateRiderFn: func(_ context.Context, _ db.ActivateRiderCredentialLedgersTxParams) ([]db.CredentialLedger, error) {
			return nil, nil
		},
		getMerchantActiveFn: func(_ context.Context, _ pgtype.Int8) ([]db.CredentialLedger, error) { return nil, nil },
		getRiderActiveFn:    func(_ context.Context, _ pgtype.Int8) ([]db.CredentialLedger, error) { return nil, nil },
		restoreMerchantFn: func(_ context.Context, _ db.RestoreMerchantCredentialGovernanceTxParams) (int64, error) {
			return 0, nil
		},
		restoreRiderFn: func(_ context.Context, _ db.RestoreRiderCredentialGovernanceTxParams) (int64, error) { return 0, nil },
	})
	require.NotNil(t, service)
	service.now = func() time.Time { return now }

	result, err := service.ActivateMerchantCredentials(context.Background(), ActivateMerchantCredentialsInput{
		MerchantID:            2001,
		MerchantApplicationID: 301,
		ReviewRunID:           int64Ptr(888),
		Entries: []CredentialActivationInput{
			{
				DocumentType: "business_license",
				MediaAssetID: 9001,
				ExpiresAt:    &expiresAt,
				NormalizedPayload: map[string]any{
					"credit_code": "9135X",
				},
			},
			{
				DocumentType: "food_permit",
				MediaAssetID: 9002,
				NormalizedPayload: map[string]any{
					"permit_number": "SP-1",
				},
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, result, 1)
}

func TestCredentialGovernanceServiceActivateRiderCredentials(t *testing.T) {
	now := time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC)
	service := NewCredentialGovernanceService(credentialGovernanceStoreStub{
		activateMerchantFn: func(_ context.Context, _ db.ActivateMerchantCredentialLedgersTxParams) ([]db.CredentialLedger, error) {
			return nil, nil
		},
		activateRiderFn: func(_ context.Context, arg db.ActivateRiderCredentialLedgersTxParams) ([]db.CredentialLedger, error) {
			require.Equal(t, int64(7001), arg.RiderID)
			require.Equal(t, now, arg.ActivatedAt.Time)
			require.Len(t, arg.Entries, 1)
			require.Equal(t, "health_cert", arg.Entries[0].DocumentType)
			require.True(t, arg.Entries[0].ReviewRunID.Valid)
			require.Equal(t, int64(990), arg.Entries[0].ReviewRunID.Int64)
			require.False(t, arg.Entries[0].ExpiresAt.Valid)
			return []db.CredentialLedger{{ID: 2, RiderID: pgtype.Int8{Int64: 7001, Valid: true}}}, nil
		},
		getMerchantActiveFn: func(_ context.Context, _ pgtype.Int8) ([]db.CredentialLedger, error) { return nil, nil },
		getRiderActiveFn:    func(_ context.Context, _ pgtype.Int8) ([]db.CredentialLedger, error) { return nil, nil },
		restoreMerchantFn: func(_ context.Context, _ db.RestoreMerchantCredentialGovernanceTxParams) (int64, error) {
			return 0, nil
		},
		restoreRiderFn: func(_ context.Context, _ db.RestoreRiderCredentialGovernanceTxParams) (int64, error) { return 0, nil },
	})
	require.NotNil(t, service)
	service.now = func() time.Time { return now }

	result, err := service.ActivateRiderCredentials(context.Background(), ActivateRiderCredentialsInput{
		RiderID:            7001,
		RiderApplicationID: 401,
		ReviewRunID:        int64Ptr(990),
		Entries: []CredentialActivationInput{{
			DocumentType: "health_cert",
			MediaAssetID: 12001,
			NormalizedPayload: map[string]any{
				"cert_number": "JK-2026",
			},
		}},
	})
	require.NoError(t, err)
	require.Len(t, result, 1)
}

func int64Ptr(value int64) *int64 {
	return &value
}

func TestCredentialGovernanceServiceRestoreMerchantIfEligible(t *testing.T) {
	now := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)
	service := NewCredentialGovernanceService(credentialGovernanceStoreStub{
		activateMerchantFn: func(_ context.Context, _ db.ActivateMerchantCredentialLedgersTxParams) ([]db.CredentialLedger, error) {
			return nil, nil
		},
		activateRiderFn: func(_ context.Context, _ db.ActivateRiderCredentialLedgersTxParams) ([]db.CredentialLedger, error) {
			return nil, nil
		},
		getMerchantActiveFn: func(_ context.Context, merchantID pgtype.Int8) ([]db.CredentialLedger, error) {
			require.Equal(t, int64(5001), merchantID.Int64)
			return []db.CredentialLedger{
				{ID: 11, DocumentType: db.CredentialDocumentTypeBusinessLicense, ExpiresAt: pgtype.Timestamptz{Time: now.AddDate(0, 1, 0), Valid: true}},
				{ID: 12, DocumentType: db.CredentialDocumentTypeFoodPermit, ExpiresAt: pgtype.Timestamptz{Time: now.AddDate(0, 2, 0), Valid: true}},
			}, nil
		},
		getRiderActiveFn: func(_ context.Context, _ pgtype.Int8) ([]db.CredentialLedger, error) { return nil, nil },
		restoreMerchantFn: func(_ context.Context, arg db.RestoreMerchantCredentialGovernanceTxParams) (int64, error) {
			require.Equal(t, int64(5001), arg.MerchantID)
			require.Equal(t, []int64{11, 12}, arg.CredentialLedgerIDs)
			require.Equal(t, db.CredentialSuspensionReasonDocumentExpired, arg.TakeoutSuspendReason.String)
			return 1, nil
		},
		restoreRiderFn: func(_ context.Context, _ db.RestoreRiderCredentialGovernanceTxParams) (int64, error) { return 0, nil },
	})
	service.now = func() time.Time { return now }

	result, err := service.RestoreMerchantIfEligible(context.Background(), 5001)
	require.NoError(t, err)
	require.True(t, result.MatrixSatisfied)
	require.True(t, result.Released)
	require.Empty(t, result.MissingDocumentTypes)
	require.Empty(t, result.ExpiredDocumentTypes)
}

func TestCredentialGovernanceServiceRestoreMerchantIfEligible_BlockedByMatrix(t *testing.T) {
	now := time.Date(2026, 4, 22, 12, 30, 0, 0, time.UTC)
	service := NewCredentialGovernanceService(credentialGovernanceStoreStub{
		activateMerchantFn: func(_ context.Context, _ db.ActivateMerchantCredentialLedgersTxParams) ([]db.CredentialLedger, error) {
			return nil, nil
		},
		activateRiderFn: func(_ context.Context, _ db.ActivateRiderCredentialLedgersTxParams) ([]db.CredentialLedger, error) {
			return nil, nil
		},
		getMerchantActiveFn: func(_ context.Context, _ pgtype.Int8) ([]db.CredentialLedger, error) {
			return []db.CredentialLedger{{ID: 21, DocumentType: db.CredentialDocumentTypeBusinessLicense}}, nil
		},
		getRiderActiveFn: func(_ context.Context, _ pgtype.Int8) ([]db.CredentialLedger, error) { return nil, nil },
		restoreMerchantFn: func(_ context.Context, _ db.RestoreMerchantCredentialGovernanceTxParams) (int64, error) {
			t.Fatalf("restore merchant tx should not be called when matrix is incomplete")
			return 0, nil
		},
		restoreRiderFn: func(_ context.Context, _ db.RestoreRiderCredentialGovernanceTxParams) (int64, error) { return 0, nil },
	})
	service.now = func() time.Time { return now }

	result, err := service.RestoreMerchantIfEligible(context.Background(), 5002)
	require.NoError(t, err)
	require.False(t, result.MatrixSatisfied)
	require.False(t, result.Released)
	require.Equal(t, []string{db.CredentialDocumentTypeFoodPermit}, result.MissingDocumentTypes)
}

func TestCredentialGovernanceServiceRestoreRiderIfEligible_BlockedByExpiredCredential(t *testing.T) {
	now := time.Date(2026, 4, 22, 13, 0, 0, 0, time.UTC)
	service := NewCredentialGovernanceService(credentialGovernanceStoreStub{
		activateMerchantFn: func(_ context.Context, _ db.ActivateMerchantCredentialLedgersTxParams) ([]db.CredentialLedger, error) {
			return nil, nil
		},
		activateRiderFn: func(_ context.Context, _ db.ActivateRiderCredentialLedgersTxParams) ([]db.CredentialLedger, error) {
			return nil, nil
		},
		getMerchantActiveFn: func(_ context.Context, _ pgtype.Int8) ([]db.CredentialLedger, error) { return nil, nil },
		getRiderActiveFn: func(_ context.Context, riderID pgtype.Int8) ([]db.CredentialLedger, error) {
			require.Equal(t, int64(7001), riderID.Int64)
			return []db.CredentialLedger{{
				ID:           31,
				DocumentType: db.CredentialDocumentTypeHealthCert,
				ExpiresAt:    pgtype.Timestamptz{Time: now.Add(-time.Minute), Valid: true},
			}}, nil
		},
		restoreMerchantFn: func(_ context.Context, _ db.RestoreMerchantCredentialGovernanceTxParams) (int64, error) {
			return 0, nil
		},
		restoreRiderFn: func(_ context.Context, _ db.RestoreRiderCredentialGovernanceTxParams) (int64, error) {
			t.Fatalf("restore rider tx should not be called when credential is expired")
			return 0, nil
		},
	})
	service.now = func() time.Time { return now }

	result, err := service.RestoreRiderIfEligible(context.Background(), 7001)
	require.NoError(t, err)
	require.False(t, result.MatrixSatisfied)
	require.False(t, result.Released)
	require.Equal(t, []string{db.CredentialDocumentTypeHealthCert}, result.ExpiredDocumentTypes)
}
