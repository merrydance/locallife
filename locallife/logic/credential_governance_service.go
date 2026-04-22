package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

type credentialGovernanceStore interface {
	ActivateMerchantCredentialLedgersTx(ctx context.Context, arg db.ActivateMerchantCredentialLedgersTxParams) ([]db.CredentialLedger, error)
	ActivateRiderCredentialLedgersTx(ctx context.Context, arg db.ActivateRiderCredentialLedgersTxParams) ([]db.CredentialLedger, error)
	GetActiveMerchantCredentialLedgers(ctx context.Context, merchantID pgtype.Int8) ([]db.CredentialLedger, error)
	GetActiveRiderCredentialLedgers(ctx context.Context, riderID pgtype.Int8) ([]db.CredentialLedger, error)
	RestoreMerchantCredentialGovernanceTx(ctx context.Context, arg db.RestoreMerchantCredentialGovernanceTxParams) (int64, error)
	RestoreRiderCredentialGovernanceTx(ctx context.Context, arg db.RestoreRiderCredentialGovernanceTxParams) (int64, error)
}

type CredentialGovernanceService struct {
	store credentialGovernanceStore
	now   func() time.Time
}

type CredentialActivationInput struct {
	DocumentType      string
	MediaAssetID      int64
	ExpiresAt         *time.Time
	NormalizedPayload map[string]any
}

type ActivateMerchantCredentialsInput struct {
	MerchantID            int64
	MerchantApplicationID int64
	ReviewRunID           *int64
	Entries               []CredentialActivationInput
}

type ActivateRiderCredentialsInput struct {
	RiderID            int64
	RiderApplicationID int64
	ReviewRunID        *int64
	Entries            []CredentialActivationInput
}

type CredentialRestoreResult struct {
	MatrixSatisfied      bool
	Released             bool
	MissingDocumentTypes []string
	ExpiredDocumentTypes []string
}

func NewCredentialGovernanceService(store any) *CredentialGovernanceService {
	typedStore, ok := store.(credentialGovernanceStore)
	if !ok || typedStore == nil {
		return nil
	}
	return &CredentialGovernanceService{store: typedStore, now: time.Now}
}

func (service *CredentialGovernanceService) ActivateMerchantCredentials(ctx context.Context, input ActivateMerchantCredentialsInput) ([]db.CredentialLedger, error) {
	if service == nil || service.store == nil {
		return nil, nil
	}
	activatedAt := service.now().UTC()
	entries, err := buildMerchantCredentialActivationEntries(input)
	if err != nil {
		return nil, err
	}
	return service.store.ActivateMerchantCredentialLedgersTx(ctx, db.ActivateMerchantCredentialLedgersTxParams{
		MerchantID:  input.MerchantID,
		ActivatedAt: pgtype.Timestamptz{Time: activatedAt, Valid: true},
		Entries:     entries,
	})
}

func (service *CredentialGovernanceService) ActivateRiderCredentials(ctx context.Context, input ActivateRiderCredentialsInput) ([]db.CredentialLedger, error) {
	if service == nil || service.store == nil {
		return nil, nil
	}
	activatedAt := service.now().UTC()
	entries, err := buildRiderCredentialActivationEntries(input)
	if err != nil {
		return nil, err
	}
	return service.store.ActivateRiderCredentialLedgersTx(ctx, db.ActivateRiderCredentialLedgersTxParams{
		RiderID:     input.RiderID,
		ActivatedAt: pgtype.Timestamptz{Time: activatedAt, Valid: true},
		Entries:     entries,
	})
}

func (service *CredentialGovernanceService) RestoreMerchantIfEligible(ctx context.Context, merchantID int64) (CredentialRestoreResult, error) {
	if service == nil || service.store == nil {
		return CredentialRestoreResult{}, nil
	}
	ledgers, err := service.store.GetActiveMerchantCredentialLedgers(ctx, pgtype.Int8{Int64: merchantID, Valid: true})
	if err != nil {
		return CredentialRestoreResult{}, fmt.Errorf("get active merchant credential ledgers: %w", err)
	}
	result := evaluateCredentialRestoreEligibility(ledgers, service.now().UTC(), []string{
		db.CredentialDocumentTypeBusinessLicense,
		db.CredentialDocumentTypeFoodPermit,
	})
	if !result.MatrixSatisfied {
		return result, nil
	}
	releasedRows, err := service.store.RestoreMerchantCredentialGovernanceTx(ctx, db.RestoreMerchantCredentialGovernanceTxParams{
		MerchantID:           merchantID,
		CredentialLedgerIDs:  activeCredentialLedgerIDs(ledgers),
		ResumedAt:            pgtype.Timestamptz{Time: service.now().UTC(), Valid: true},
		TakeoutSuspendReason: pgtype.Text{String: db.CredentialSuspensionReasonDocumentExpired, Valid: true},
	})
	if err != nil {
		return CredentialRestoreResult{}, fmt.Errorf("restore merchant credential governance: %w", err)
	}
	result.Released = releasedRows > 0
	return result, nil
}

func (service *CredentialGovernanceService) RestoreRiderIfEligible(ctx context.Context, riderID int64) (CredentialRestoreResult, error) {
	if service == nil || service.store == nil {
		return CredentialRestoreResult{}, nil
	}
	ledgers, err := service.store.GetActiveRiderCredentialLedgers(ctx, pgtype.Int8{Int64: riderID, Valid: true})
	if err != nil {
		return CredentialRestoreResult{}, fmt.Errorf("get active rider credential ledgers: %w", err)
	}
	result := evaluateCredentialRestoreEligibility(ledgers, service.now().UTC(), []string{
		db.CredentialDocumentTypeHealthCert,
	})
	if !result.MatrixSatisfied {
		return result, nil
	}
	releasedRows, err := service.store.RestoreRiderCredentialGovernanceTx(ctx, db.RestoreRiderCredentialGovernanceTxParams{
		RiderID:             riderID,
		CredentialLedgerIDs: activeCredentialLedgerIDs(ledgers),
		ResumedAt:           pgtype.Timestamptz{Time: service.now().UTC(), Valid: true},
		SuspendReason:       pgtype.Text{String: db.CredentialSuspensionReasonDocumentExpired, Valid: true},
	})
	if err != nil {
		return CredentialRestoreResult{}, fmt.Errorf("restore rider credential governance: %w", err)
	}
	result.Released = releasedRows > 0
	return result, nil
}

func buildMerchantCredentialActivationEntries(input ActivateMerchantCredentialsInput) ([]db.ActivateMerchantCredentialLedgerEntry, error) {
	if input.MerchantID <= 0 {
		return nil, fmt.Errorf("merchant id must be positive")
	}
	if input.MerchantApplicationID <= 0 {
		return nil, fmt.Errorf("merchant application id must be positive")
	}
	entries := make([]db.ActivateMerchantCredentialLedgerEntry, 0, len(input.Entries))
	seenDocumentTypes := make(map[string]struct{}, len(input.Entries))
	for _, entry := range input.Entries {
		if entry.DocumentType == "" {
			return nil, fmt.Errorf("merchant credential document type is required")
		}
		if entry.MediaAssetID <= 0 {
			return nil, fmt.Errorf("merchant credential media asset id must be positive")
		}
		if _, exists := seenDocumentTypes[entry.DocumentType]; exists {
			return nil, fmt.Errorf("duplicate merchant credential document type: %s", entry.DocumentType)
		}
		seenDocumentTypes[entry.DocumentType] = struct{}{}
		encodedPayload, err := marshalCredentialActivationPayload(entry.NormalizedPayload)
		if err != nil {
			return nil, err
		}
		entries = append(entries, db.ActivateMerchantCredentialLedgerEntry{
			DocumentType:          entry.DocumentType,
			MerchantApplicationID: input.MerchantApplicationID,
			ReviewRunID:           optionalInt8(input.ReviewRunID),
			MediaAssetID:          entry.MediaAssetID,
			NormalizedPayload:     encodedPayload,
			ExpiresAt:             optionalTimestamptz(entry.ExpiresAt),
		})
	}
	return entries, nil
}

func buildRiderCredentialActivationEntries(input ActivateRiderCredentialsInput) ([]db.ActivateRiderCredentialLedgerEntry, error) {
	if input.RiderID <= 0 {
		return nil, fmt.Errorf("rider id must be positive")
	}
	if input.RiderApplicationID <= 0 {
		return nil, fmt.Errorf("rider application id must be positive")
	}
	entries := make([]db.ActivateRiderCredentialLedgerEntry, 0, len(input.Entries))
	seenDocumentTypes := make(map[string]struct{}, len(input.Entries))
	for _, entry := range input.Entries {
		if entry.DocumentType == "" {
			return nil, fmt.Errorf("rider credential document type is required")
		}
		if entry.MediaAssetID <= 0 {
			return nil, fmt.Errorf("rider credential media asset id must be positive")
		}
		if _, exists := seenDocumentTypes[entry.DocumentType]; exists {
			return nil, fmt.Errorf("duplicate rider credential document type: %s", entry.DocumentType)
		}
		seenDocumentTypes[entry.DocumentType] = struct{}{}
		encodedPayload, err := marshalCredentialActivationPayload(entry.NormalizedPayload)
		if err != nil {
			return nil, err
		}
		entries = append(entries, db.ActivateRiderCredentialLedgerEntry{
			DocumentType:       entry.DocumentType,
			RiderApplicationID: input.RiderApplicationID,
			ReviewRunID:        optionalInt8(input.ReviewRunID),
			MediaAssetID:       entry.MediaAssetID,
			NormalizedPayload:  encodedPayload,
			ExpiresAt:          optionalTimestamptz(entry.ExpiresAt),
		})
	}
	return entries, nil
}

func marshalCredentialActivationPayload(payload map[string]any) ([]byte, error) {
	if len(payload) == 0 {
		return []byte("{}"), nil
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal credential activation payload: %w", err)
	}
	return encoded, nil
}

func optionalTimestamptz(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}

func evaluateCredentialRestoreEligibility(ledgers []db.CredentialLedger, now time.Time, requiredDocumentTypes []string) CredentialRestoreResult {
	ledgerByDocumentType := make(map[string]db.CredentialLedger, len(ledgers))
	for _, ledger := range ledgers {
		ledgerByDocumentType[ledger.DocumentType] = ledger
	}
	result := CredentialRestoreResult{MatrixSatisfied: true}
	for _, documentType := range requiredDocumentTypes {
		ledger, ok := ledgerByDocumentType[documentType]
		if !ok {
			result.MatrixSatisfied = false
			result.MissingDocumentTypes = append(result.MissingDocumentTypes, documentType)
			continue
		}
		if isCredentialLedgerExpired(ledger, now) {
			result.MatrixSatisfied = false
			result.ExpiredDocumentTypes = append(result.ExpiredDocumentTypes, documentType)
		}
	}
	return result
}

func isCredentialLedgerExpired(ledger db.CredentialLedger, now time.Time) bool {
	if !ledger.ExpiresAt.Valid {
		return false
	}
	return now.After(ledger.ExpiresAt.Time)
}

func activeCredentialLedgerIDs(ledgers []db.CredentialLedger) []int64 {
	ids := make([]int64, 0, len(ledgers))
	for _, ledger := range ledgers {
		ids = append(ids, ledger.ID)
	}
	return ids
}
