package logic

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

func merchantSubjectProfileFromDB(row db.MerchantSubjectProfile) (MerchantSubjectProfile, error) {
	var license MerchantReviewBusinessLicenseOCRData
	if len(row.BusinessLicensePayload) > 0 {
		if err := json.Unmarshal(row.BusinessLicensePayload, &license); err != nil {
			return MerchantSubjectProfile{}, err
		}
	}
	var foodPermit MerchantReviewFoodPermitOCRData
	if len(row.FoodPermitPayload) > 0 {
		if err := json.Unmarshal(row.FoodPermitPayload, &foodPermit); err != nil {
			return MerchantSubjectProfile{}, err
		}
	}
	var legalPayload struct {
		IDCardAddress       string `json:"id_card_address"`
		IDCardValidDate     string `json:"id_card_valid_date"`
		IDCardFrontOCRJobID *int64 `json:"id_card_front_ocr_job_id"`
		IDCardBackOCRJobID  *int64 `json:"id_card_back_ocr_job_id"`
	}
	if len(row.LegalPersonPayload) > 0 {
		if err := json.Unmarshal(row.LegalPersonPayload, &legalPayload); err != nil {
			return MerchantSubjectProfile{}, err
		}
	}
	return MerchantSubjectProfile{
		ID:            row.ID,
		ApplicationID: row.MerchantApplicationID,
		MerchantID:    int64FromPgInt8(row.MerchantID),
		UserID:        row.UserID,
		BusinessLicense: MerchantSubjectBusinessLicense{
			Number:              strings.TrimSpace(row.BusinessLicenseNumber),
			Name:                strings.TrimSpace(row.BusinessLicenseName),
			LegalRepresentative: strings.TrimSpace(row.LegalPersonName),
			Address:             strings.TrimSpace(row.BusinessLicenseAddress),
			BusinessScope:       strings.TrimSpace(license.BusinessScope),
			ValidPeriod:         strings.TrimSpace(license.ValidPeriod),
			TypeOfEnterprise:    strings.TrimSpace(license.TypeOfEnterprise),
			MediaAssetID:        int64FromPgInt8(row.BusinessLicenseMediaAssetID),
			OCRJobID:            license.OCRJobID,
		},
		FoodPermit: MerchantSubjectFoodPermit{
			PermitNo:     strings.TrimSpace(row.FoodPermitNumber),
			CompanyName:  strings.TrimSpace(row.FoodPermitCompanyName),
			OperatorName: strings.TrimSpace(foodPermit.OperatorName),
			ValidFrom:    strings.TrimSpace(foodPermit.ValidFrom),
			ValidTo:      strings.TrimSpace(foodPermit.ValidTo),
			MediaAssetID: int64FromPgInt8(row.FoodPermitMediaAssetID),
			OCRJobID:     foodPermit.OCRJobID,
		},
		LegalPerson: MerchantSubjectLegalPerson{
			Name:                strings.TrimSpace(row.LegalPersonName),
			IDNumber:            strings.TrimSpace(row.LegalPersonIDNumber),
			IDCardAddress:       strings.TrimSpace(legalPayload.IDCardAddress),
			IDCardValidDate:     strings.TrimSpace(legalPayload.IDCardValidDate),
			IDCardFrontMediaID:  int64FromPgInt8(row.IDCardFrontMediaAssetID),
			IDCardBackMediaID:   int64FromPgInt8(row.IDCardBackMediaAssetID),
			IDCardFrontOCRJobID: legalPayload.IDCardFrontOCRJobID,
			IDCardBackOCRJobID:  legalPayload.IDCardBackOCRJobID,
		},
		BusinessLicensePayload: row.BusinessLicensePayload,
		FoodPermitPayload:      row.FoodPermitPayload,
		LegalPersonPayload:     row.LegalPersonPayload,
		SourceSnapshot:         row.SourceSnapshot,
	}, nil
}

func merchantSubjectProfileVersionSnapshot(profile MerchantSubjectProfile) []byte {
	snapshot, err := json.Marshal(map[string]any{
		"business_license": profile.BusinessLicense,
		"food_permit":      profile.FoodPermit,
		"legal_person":     profile.LegalPerson,
	})
	if err != nil {
		return []byte(`{}`)
	}
	return snapshot
}

func optionalInt8Value(value int64) pgtype.Int8 {
	return pgtype.Int8{Int64: value, Valid: value > 0}
}

func int64FromPgInt8(value pgtype.Int8) int64 {
	if !value.Valid {
		return 0
	}
	return value.Int64
}

func isMerchantSubjectProfileNotFound(err error) bool {
	return errors.Is(err, db.ErrRecordNotFound) || errors.Is(err, pgx.ErrNoRows)
}
