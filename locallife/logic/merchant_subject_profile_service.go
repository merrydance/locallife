package logic

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

type merchantSubjectProfileStore interface {
	UpsertMerchantSubjectProfile(ctx context.Context, arg db.UpsertMerchantSubjectProfileParams) (db.MerchantSubjectProfile, error)
	DetachMerchantSubjectProfileMerchantFromOtherApplications(ctx context.Context, arg db.DetachMerchantSubjectProfileMerchantFromOtherApplicationsParams) (int64, error)
	CreateMerchantSubjectProfileVersion(ctx context.Context, arg db.CreateMerchantSubjectProfileVersionParams) (db.MerchantSubjectProfileVersion, error)
	GetMerchantSubjectProfileByApplication(ctx context.Context, merchantApplicationID int64) (db.MerchantSubjectProfile, error)
	GetMerchantSubjectProfileByMerchant(ctx context.Context, merchantID pgtype.Int8) (db.MerchantSubjectProfile, error)
}

type MerchantSubjectProfileService struct {
	store merchantSubjectProfileStore
}

func NewMerchantSubjectProfileService(store merchantSubjectProfileStore) *MerchantSubjectProfileService {
	if store == nil {
		return nil
	}
	return &MerchantSubjectProfileService{store: store}
}

type MerchantSubjectProfile struct {
	ID                     int64
	ApplicationID          int64
	MerchantID             int64
	UserID                 int64
	BusinessLicense        MerchantSubjectBusinessLicense
	FoodPermit             MerchantSubjectFoodPermit
	LegalPerson            MerchantSubjectLegalPerson
	BusinessLicensePayload []byte
	FoodPermitPayload      []byte
	LegalPersonPayload     []byte
	SourceSnapshot         []byte
}

type MerchantSubjectBusinessLicense struct {
	Number              string `json:"number"`
	Name                string `json:"name"`
	LegalRepresentative string `json:"legal_representative"`
	Address             string `json:"address"`
	BusinessScope       string `json:"business_scope"`
	ValidPeriod         string `json:"valid_period"`
	TypeOfEnterprise    string `json:"type_of_enterprise,omitempty"`
	MediaAssetID        int64  `json:"media_asset_id,omitempty"`
	OCRJobID            *int64 `json:"ocr_job_id,omitempty"`
}

type MerchantSubjectFoodPermit struct {
	PermitNo     string `json:"permit_no"`
	CompanyName  string `json:"company_name"`
	OperatorName string `json:"operator_name"`
	ValidFrom    string `json:"valid_from,omitempty"`
	ValidTo      string `json:"valid_to"`
	MediaAssetID int64  `json:"media_asset_id,omitempty"`
	OCRJobID     *int64 `json:"ocr_job_id,omitempty"`
}

type MerchantSubjectLegalPerson struct {
	Name                string `json:"name"`
	IDNumber            string `json:"id_number"`
	IDCardAddress       string `json:"id_card_address,omitempty"`
	IDCardValidDate     string `json:"id_card_valid_date"`
	IDCardFrontMediaID  int64  `json:"id_card_front_media_id,omitempty"`
	IDCardBackMediaID   int64  `json:"id_card_back_media_id,omitempty"`
	IDCardFrontOCRJobID *int64 `json:"id_card_front_ocr_job_id,omitempty"`
	IDCardBackOCRJobID  *int64 `json:"id_card_back_ocr_job_id,omitempty"`
}

func BuildMerchantSubjectProfileFromApplication(application db.MerchantApplication) (MerchantSubjectProfile, error) {
	var businessLicense MerchantReviewBusinessLicenseOCRData
	if len(application.BusinessLicenseOcr) > 0 {
		if err := json.Unmarshal(application.BusinessLicenseOcr, &businessLicense); err != nil {
			return MerchantSubjectProfile{}, err
		}
	}
	var foodPermit MerchantReviewFoodPermitOCRData
	if len(application.FoodPermitOcr) > 0 {
		if err := json.Unmarshal(application.FoodPermitOcr, &foodPermit); err != nil {
			return MerchantSubjectProfile{}, err
		}
	}
	var idCardFront struct {
		Readiness *MerchantReviewOCRReadiness `json:"readiness,omitempty"`
		OCRJobID  *int64                      `json:"ocr_job_id,omitempty"`
		Name      string                      `json:"name,omitempty"`
		IDNumber  string                      `json:"id_number,omitempty"`
		Address   string                      `json:"address,omitempty"`
	}
	if len(application.IDCardFrontOcr) > 0 {
		if err := json.Unmarshal(application.IDCardFrontOcr, &idCardFront); err != nil {
			return MerchantSubjectProfile{}, err
		}
	}
	var idCardBack struct {
		Readiness *MerchantReviewOCRReadiness `json:"readiness,omitempty"`
		OCRJobID  *int64                      `json:"ocr_job_id,omitempty"`
		ValidDate string                      `json:"valid_date,omitempty"`
	}
	if len(application.IDCardBackOcr) > 0 {
		if err := json.Unmarshal(application.IDCardBackOcr, &idCardBack); err != nil {
			return MerchantSubjectProfile{}, err
		}
	}

	licenseNumber := merchantSubjectAuthorityValue(
		application.BusinessLicenseNumber,
		firstTrimmed(businessLicense.CreditCode, businessLicense.RegNum),
		businessLicense.Correction,
		"credit_code",
		"reg_num",
	)
	licenseName := merchantSubjectAuthorityValue(
		application.MerchantName,
		businessLicense.EnterpriseName,
		businessLicense.Correction,
		"enterprise_name",
	)
	legalName := merchantSubjectAuthorityValue(
		application.LegalPersonName,
		firstTrimmed(businessLicense.LegalRepresentative, idCardFront.Name),
		businessLicense.Correction,
		"legal_representative",
	)
	legalID := firstTrimmed(application.LegalPersonIDNumber, idCardFront.IDNumber)
	businessScope := merchantSubjectAuthorityValue(
		pgTextValue(application.BusinessScope),
		businessLicense.BusinessScope,
		businessLicense.Correction,
		"business_scope",
	)

	businessLicense.CreditCode = licenseNumber
	businessLicense.RegNum = firstTrimmed(licenseNumber, businessLicense.RegNum)
	businessLicense.EnterpriseName = licenseName
	businessLicense.LegalRepresentative = legalName
	businessLicense.BusinessScope = businessScope
	licensePayload, err := json.Marshal(businessLicense)
	if err != nil {
		return MerchantSubjectProfile{}, err
	}

	foodPermitCompanyName := merchantSubjectAuthorityValue(
		"",
		foodPermit.CompanyName,
		foodPermit.Correction,
		"company_name",
	)
	foodPermitOperatorName := merchantSubjectAuthorityValue(
		"",
		foodPermit.OperatorName,
		foodPermit.Correction,
		"operator_name",
	)
	foodPermit.CompanyName = foodPermitCompanyName
	foodPermit.OperatorName = foodPermitOperatorName
	foodPermitPayload, err := json.Marshal(foodPermit)
	if err != nil {
		return MerchantSubjectProfile{}, err
	}

	legalPayload, err := json.Marshal(map[string]any{
		"name":                     legalName,
		"id_number":                legalID,
		"id_card_address":          strings.TrimSpace(idCardFront.Address),
		"id_card_valid_date":       strings.TrimSpace(idCardBack.ValidDate),
		"id_card_front_ocr_job_id": idCardFront.OCRJobID,
		"id_card_back_ocr_job_id":  idCardBack.OCRJobID,
		"id_card_front_media_id":   int64FromPgInt8(application.IDCardFrontMediaAssetID),
		"id_card_back_media_id":    int64FromPgInt8(application.IDCardBackMediaAssetID),
		"id_card_front_readiness":  idCardFront.Readiness,
		"id_card_back_readiness":   idCardBack.Readiness,
	})
	if err != nil {
		return MerchantSubjectProfile{}, err
	}

	sourceSnapshot, err := json.Marshal(map[string]any{
		"source":         "merchant_application",
		"application_id": application.ID,
		"status":         application.Status,
	})
	if err != nil {
		return MerchantSubjectProfile{}, err
	}

	return MerchantSubjectProfile{
		ApplicationID: application.ID,
		UserID:        application.UserID,
		BusinessLicense: MerchantSubjectBusinessLicense{
			Number:              licenseNumber,
			Name:                licenseName,
			LegalRepresentative: legalName,
			Address:             strings.TrimSpace(businessLicense.Address),
			BusinessScope:       businessScope,
			ValidPeriod:         strings.TrimSpace(businessLicense.ValidPeriod),
			TypeOfEnterprise:    strings.TrimSpace(businessLicense.TypeOfEnterprise),
			MediaAssetID:        int64FromPgInt8(application.BusinessLicenseMediaAssetID),
			OCRJobID:            businessLicense.OCRJobID,
		},
		FoodPermit: MerchantSubjectFoodPermit{
			PermitNo:     strings.TrimSpace(foodPermit.PermitNo),
			CompanyName:  strings.TrimSpace(foodPermit.CompanyName),
			OperatorName: strings.TrimSpace(foodPermit.OperatorName),
			ValidFrom:    strings.TrimSpace(foodPermit.ValidFrom),
			ValidTo:      strings.TrimSpace(foodPermit.ValidTo),
			MediaAssetID: int64FromPgInt8(application.FoodPermitMediaAssetID),
			OCRJobID:     foodPermit.OCRJobID,
		},
		LegalPerson: MerchantSubjectLegalPerson{
			Name:                legalName,
			IDNumber:            legalID,
			IDCardAddress:       strings.TrimSpace(idCardFront.Address),
			IDCardValidDate:     strings.TrimSpace(idCardBack.ValidDate),
			IDCardFrontMediaID:  int64FromPgInt8(application.IDCardFrontMediaAssetID),
			IDCardBackMediaID:   int64FromPgInt8(application.IDCardBackMediaAssetID),
			IDCardFrontOCRJobID: idCardFront.OCRJobID,
			IDCardBackOCRJobID:  idCardBack.OCRJobID,
		},
		BusinessLicensePayload: licensePayload,
		FoodPermitPayload:      foodPermitPayload,
		LegalPersonPayload:     legalPayload,
		SourceSnapshot:         sourceSnapshot,
	}, nil
}

func merchantSubjectAuthorityValue(structured string, ocrValue string, correction json.RawMessage, fields ...string) string {
	structured = strings.TrimSpace(structured)
	ocrValue = strings.TrimSpace(ocrValue)
	if ocrValue == "" {
		return structured
	}
	metadata := merchantSubjectCorrectionMetadata(correction)
	for _, field := range fields {
		if !metadata.correctedFields[field] {
			continue
		}
		previous := strings.TrimSpace(metadata.previous[field])
		if structured == "" || (previous != "" && structured == previous) {
			return ocrValue
		}
	}
	if structured != "" {
		return structured
	}
	return ocrValue
}

type merchantSubjectCorrectionMetadataResult struct {
	correctedFields map[string]bool
	previous        map[string]string
}

func merchantSubjectCorrectionMetadata(raw json.RawMessage) merchantSubjectCorrectionMetadataResult {
	result := merchantSubjectCorrectionMetadataResult{
		correctedFields: map[string]bool{},
		previous:        map[string]string{},
	}
	if len(raw) == 0 || string(raw) == "null" {
		return result
	}
	var correction struct {
		Fields   []string          `json:"fields,omitempty"`
		Previous map[string]string `json:"previous,omitempty"`
	}
	if err := json.Unmarshal(raw, &correction); err != nil {
		return result
	}
	for _, field := range correction.Fields {
		if trimmed := strings.TrimSpace(field); trimmed != "" {
			result.correctedFields[trimmed] = true
		}
	}
	for field, value := range correction.Previous {
		if trimmed := strings.TrimSpace(field); trimmed != "" {
			result.previous[trimmed] = strings.TrimSpace(value)
		}
	}
	return result
}

func (service *MerchantSubjectProfileService) SaveApplicationProfile(ctx context.Context, application db.MerchantApplication, merchantID ...int64) (MerchantSubjectProfile, error) {
	if service == nil || service.store == nil {
		return MerchantSubjectProfile{}, nil
	}
	profile, err := BuildMerchantSubjectProfileFromApplication(application)
	if err != nil {
		return MerchantSubjectProfile{}, err
	}
	if len(merchantID) > 0 && merchantID[0] > 0 {
		profile.MerchantID = merchantID[0]
	}
	if profile.MerchantID > 0 {
		if _, err := service.store.DetachMerchantSubjectProfileMerchantFromOtherApplications(ctx, db.DetachMerchantSubjectProfileMerchantFromOtherApplicationsParams{
			MerchantID:            pgtype.Int8{Int64: profile.MerchantID, Valid: true},
			MerchantApplicationID: profile.ApplicationID,
		}); err != nil {
			return MerchantSubjectProfile{}, err
		}
	}
	saved, err := service.store.UpsertMerchantSubjectProfile(ctx, db.UpsertMerchantSubjectProfileParams{
		MerchantApplicationID:       profile.ApplicationID,
		MerchantID:                  optionalInt8Value(profile.MerchantID),
		UserID:                      profile.UserID,
		BusinessLicenseNumber:       strings.TrimSpace(profile.BusinessLicense.Number),
		BusinessLicenseName:         strings.TrimSpace(profile.BusinessLicense.Name),
		BusinessLicenseAddress:      strings.TrimSpace(profile.BusinessLicense.Address),
		LegalPersonName:             strings.TrimSpace(profile.LegalPerson.Name),
		LegalPersonIDNumber:         strings.TrimSpace(profile.LegalPerson.IDNumber),
		FoodPermitNumber:            strings.TrimSpace(profile.FoodPermit.PermitNo),
		FoodPermitCompanyName:       strings.TrimSpace(profile.FoodPermit.CompanyName),
		BusinessLicenseMediaAssetID: optionalInt8Value(profile.BusinessLicense.MediaAssetID),
		FoodPermitMediaAssetID:      optionalInt8Value(profile.FoodPermit.MediaAssetID),
		IDCardFrontMediaAssetID:     optionalInt8Value(profile.LegalPerson.IDCardFrontMediaID),
		IDCardBackMediaAssetID:      optionalInt8Value(profile.LegalPerson.IDCardBackMediaID),
		BusinessLicensePayload:      profile.BusinessLicensePayload,
		FoodPermitPayload:           profile.FoodPermitPayload,
		LegalPersonPayload:          profile.LegalPersonPayload,
		SourceSnapshot:              profile.SourceSnapshot,
	})
	if err != nil {
		return MerchantSubjectProfile{}, err
	}
	result, err := merchantSubjectProfileFromDB(saved)
	if err != nil {
		return MerchantSubjectProfile{}, err
	}
	if _, err := service.store.CreateMerchantSubjectProfileVersion(ctx, db.CreateMerchantSubjectProfileVersionParams{
		ProfileID:             saved.ID,
		MerchantApplicationID: saved.MerchantApplicationID,
		MerchantID:            saved.MerchantID,
		UserID:                saved.UserID,
		Version:               saved.Version,
		Snapshot:              merchantSubjectProfileVersionSnapshot(result),
	}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return MerchantSubjectProfile{}, err
	}
	return result, nil
}

func (service *MerchantSubjectProfileService) GetByApplication(ctx context.Context, applicationID int64) (MerchantSubjectProfile, bool, error) {
	if service == nil || service.store == nil || applicationID <= 0 {
		return MerchantSubjectProfile{}, false, nil
	}
	row, err := service.store.GetMerchantSubjectProfileByApplication(ctx, applicationID)
	if err != nil {
		if isMerchantSubjectProfileNotFound(err) {
			return MerchantSubjectProfile{}, false, nil
		}
		return MerchantSubjectProfile{}, false, err
	}
	profile, err := merchantSubjectProfileFromDB(row)
	if err != nil {
		return MerchantSubjectProfile{}, false, err
	}
	return profile, true, nil
}

func (service *MerchantSubjectProfileService) GetByMerchant(ctx context.Context, merchantID int64) (MerchantSubjectProfile, bool, error) {
	if service == nil || service.store == nil || merchantID <= 0 {
		return MerchantSubjectProfile{}, false, nil
	}
	row, err := service.store.GetMerchantSubjectProfileByMerchant(ctx, pgtype.Int8{Int64: merchantID, Valid: true})
	if err != nil {
		if isMerchantSubjectProfileNotFound(err) {
			return MerchantSubjectProfile{}, false, nil
		}
		return MerchantSubjectProfile{}, false, err
	}
	profile, err := merchantSubjectProfileFromDB(row)
	if err != nil {
		return MerchantSubjectProfile{}, false, err
	}
	return profile, true, nil
}

func (profile MerchantSubjectProfile) ReviewInput() MerchantDocumentReviewInput {
	input := MerchantDocumentReviewInput{}
	_ = json.Unmarshal(profile.BusinessLicensePayload, &input.BusinessLicense)
	_ = json.Unmarshal(profile.FoodPermitPayload, &input.FoodPermit)
	var legalPayload struct {
		Name                 string                      `json:"name,omitempty"`
		IDNumber             string                      `json:"id_number,omitempty"`
		IDCardValidDate      string                      `json:"id_card_valid_date,omitempty"`
		IDCardFrontOCRJobID  *int64                      `json:"id_card_front_ocr_job_id,omitempty"`
		IDCardBackOCRJobID   *int64                      `json:"id_card_back_ocr_job_id,omitempty"`
		IDCardFrontReadiness *MerchantReviewOCRReadiness `json:"id_card_front_readiness,omitempty"`
		IDCardBackReadiness  *MerchantReviewOCRReadiness `json:"id_card_back_readiness,omitempty"`
	}
	_ = json.Unmarshal(profile.LegalPersonPayload, &legalPayload)

	input.BusinessLicense.OCRJobID = firstInt64Ptr(input.BusinessLicense.OCRJobID, profile.BusinessLicense.OCRJobID)
	input.BusinessLicense.CreditCode = strings.TrimSpace(profile.BusinessLicense.Number)
	input.BusinessLicense.RegNum = strings.TrimSpace(profile.BusinessLicense.Number)
	input.BusinessLicense.EnterpriseName = strings.TrimSpace(profile.BusinessLicense.Name)
	input.BusinessLicense.LegalRepresentative = strings.TrimSpace(profile.BusinessLicense.LegalRepresentative)
	input.BusinessLicense.Address = strings.TrimSpace(profile.BusinessLicense.Address)
	input.BusinessLicense.BusinessScope = strings.TrimSpace(profile.BusinessLicense.BusinessScope)
	input.BusinessLicense.ValidPeriod = strings.TrimSpace(profile.BusinessLicense.ValidPeriod)
	input.BusinessLicense.TypeOfEnterprise = strings.TrimSpace(profile.BusinessLicense.TypeOfEnterprise)

	input.FoodPermit.OCRJobID = firstInt64Ptr(input.FoodPermit.OCRJobID, profile.FoodPermit.OCRJobID)
	input.FoodPermit.PermitNo = strings.TrimSpace(profile.FoodPermit.PermitNo)
	input.FoodPermit.CompanyName = strings.TrimSpace(profile.FoodPermit.CompanyName)
	input.FoodPermit.OperatorName = strings.TrimSpace(profile.FoodPermit.OperatorName)
	input.FoodPermit.ValidFrom = strings.TrimSpace(profile.FoodPermit.ValidFrom)
	input.FoodPermit.ValidTo = strings.TrimSpace(profile.FoodPermit.ValidTo)

	input.IDCardFront.OCRJobID = firstInt64Ptr(input.IDCardFront.OCRJobID, profile.LegalPerson.IDCardFrontOCRJobID, legalPayload.IDCardFrontOCRJobID)
	input.IDCardFront.Readiness = legalPayload.IDCardFrontReadiness
	input.IDCardFront.Name = strings.TrimSpace(profile.LegalPerson.Name)
	input.IDCardFront.IDNumber = strings.TrimSpace(profile.LegalPerson.IDNumber)
	input.IDCardBack.OCRJobID = firstInt64Ptr(input.IDCardBack.OCRJobID, profile.LegalPerson.IDCardBackOCRJobID, legalPayload.IDCardBackOCRJobID)
	input.IDCardBack.Readiness = legalPayload.IDCardBackReadiness
	input.IDCardBack.ValidDate = strings.TrimSpace(profile.LegalPerson.IDCardValidDate)

	return input
}

func (profile MerchantSubjectProfile) BaofuOpeningProfileDefaults() BaofuAccountOpeningProfileInput {
	return BaofuAccountOpeningProfileInput{
		LegalName:           strings.TrimSpace(profile.BusinessLicense.Name),
		BusinessLicenseNo:   strings.TrimSpace(profile.BusinessLicense.Number),
		LegalPersonName:     strings.TrimSpace(profile.LegalPerson.Name),
		LegalPersonIDNumber: strings.TrimSpace(profile.LegalPerson.IDNumber),
	}
}

func pgTextValue(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func firstInt64Ptr(values ...*int64) *int64 {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}
