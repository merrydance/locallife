package logic

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

func buildMerchantApprovalTxParams(application db.MerchantApplication) (db.ApproveMerchantApplicationTxParams, error) {
	profile, err := BuildMerchantSubjectProfileFromApplication(application)
	if err != nil {
		return db.ApproveMerchantApplicationTxParams{}, err
	}
	return buildMerchantApprovalTxParamsFromSubjectProfile(application, profile)
}

func buildMerchantApprovalTxParamsFromSubjectProfile(application db.MerchantApplication, profile MerchantSubjectProfile) (db.ApproveMerchantApplicationTxParams, error) {
	if !application.RegionID.Valid {
		return db.ApproveMerchantApplicationTxParams{}, fmt.Errorf("merchant application %d missing region_id", application.ID)
	}
	approvalProfile := profile
	approvalProfile.SourceSnapshot = merchantSubjectProfileApprovalSourceSnapshot(application)

	storefrontImages, merchantStorefrontImages := merchantApprovalImagePayload(application.StorefrontImages, 3)
	environmentImages, merchantEnvironmentImages := merchantApprovalImagePayload(application.EnvironmentImages, 5)

	appData, err := json.Marshal(map[string]any{
		"business_license_number":         strings.TrimSpace(profile.BusinessLicense.Number),
		"business_license_name":           strings.TrimSpace(profile.BusinessLicense.Name),
		"business_license_address":        strings.TrimSpace(profile.BusinessLicense.Address),
		"legal_person_name":               strings.TrimSpace(profile.LegalPerson.Name),
		"legal_person_id_number":          strings.TrimSpace(profile.LegalPerson.IDNumber),
		"food_permit_number":              strings.TrimSpace(profile.FoodPermit.PermitNo),
		"food_permit_company_name":        strings.TrimSpace(profile.FoodPermit.CompanyName),
		"business_license_media_asset_id": subjectProfileMediaID(profile.BusinessLicense.MediaAssetID, application.BusinessLicenseMediaAssetID),
		"id_card_front_media_asset_id":    subjectProfileMediaID(profile.LegalPerson.IDCardFrontMediaID, application.IDCardFrontMediaAssetID),
		"id_card_back_media_asset_id":     subjectProfileMediaID(profile.LegalPerson.IDCardBackMediaID, application.IDCardBackMediaAssetID),
		"food_permit_media_asset_id":      subjectProfileMediaID(profile.FoodPermit.MediaAssetID, application.FoodPermitMediaAssetID),
		"business_license_ocr":            json.RawMessage(profile.BusinessLicensePayload),
		"food_permit_ocr":                 json.RawMessage(profile.FoodPermitPayload),
		"legal_person_payload":            json.RawMessage(profile.LegalPersonPayload),
		"storefront_images":               storefrontImages,
		"environment_images":              environmentImages,
	})
	if err != nil {
		return db.ApproveMerchantApplicationTxParams{}, fmt.Errorf("marshal merchant application data: %w", err)
	}

	return db.ApproveMerchantApplicationTxParams{
		ApplicationID:                 application.ID,
		UserID:                        application.UserID,
		MerchantName:                  application.MerchantName,
		Phone:                         application.ContactPhone,
		Address:                       application.BusinessAddress,
		Latitude:                      application.Latitude,
		Longitude:                     application.Longitude,
		RegionID:                      application.RegionID.Int64,
		AppData:                       appData,
		StorefrontImages:              merchantStorefrontImages,
		EnvironmentImages:             merchantEnvironmentImages,
		SubjectProfile:                merchantSubjectProfileApprovalTxParams(approvalProfile),
		SubjectProfileVersionSnapshot: merchantSubjectProfileVersionSnapshot(approvalProfile),
	}, nil
}

func merchantSubjectProfileApprovalSourceSnapshot(application db.MerchantApplication) []byte {
	sourceSnapshot, err := json.Marshal(map[string]any{
		"source":         "merchant_application",
		"application_id": application.ID,
		"status":         db.MerchantApplicationStatusApproved,
	})
	if err != nil {
		return []byte(`{}`)
	}
	return sourceSnapshot
}

func merchantSubjectProfileApprovalTxParams(profile MerchantSubjectProfile) *db.UpsertMerchantSubjectProfileParams {
	if profile.ApplicationID <= 0 || profile.UserID <= 0 {
		return nil
	}
	return &db.UpsertMerchantSubjectProfileParams{
		MerchantApplicationID:       profile.ApplicationID,
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
	}
}

func merchantApprovalImagePayload(raw []byte, maxCount int) ([]string, []byte) {
	if len(raw) == 0 {
		return []string{}, nil
	}
	var rawImages []json.RawMessage
	if err := json.Unmarshal(raw, &rawImages); err != nil {
		return []string{}, nil
	}
	if len(rawImages) > maxCount {
		return []string{}, nil
	}
	images := make([]string, 0, len(rawImages))
	for _, rawImage := range rawImages {
		var image any
		if err := json.Unmarshal(rawImage, &image); err != nil {
			return []string{}, nil
		}
		imageString, ok := image.(string)
		if !ok {
			return []string{}, nil
		}
		images = append(images, imageString)
	}
	return images, raw
}

func buildMerchantCredentialActivationInputs(application db.MerchantApplication) ([]CredentialActivationInput, error) {
	profile, err := BuildMerchantSubjectProfileFromApplication(application)
	if err != nil {
		return nil, err
	}
	return buildMerchantCredentialActivationInputsFromSubjectProfile(application, profile)
}

func buildMerchantCredentialActivationInputsFromSubjectProfile(application db.MerchantApplication, profile MerchantSubjectProfile) ([]CredentialActivationInput, error) {
	reviewInput := profile.ReviewInput()
	businessLicenseMediaID := subjectProfileMediaID(profile.BusinessLicense.MediaAssetID, application.BusinessLicenseMediaAssetID)
	if businessLicenseMediaID <= 0 {
		return nil, fmt.Errorf("business_license media missing")
	}
	businessLicenseExpiry, err := parseMerchantCredentialExpiry(reviewInput.BusinessLicense.ValidPeriod)
	if err != nil {
		return nil, fmt.Errorf("parse business license expiry: %w", err)
	}

	foodPermitMediaID := subjectProfileMediaID(profile.FoodPermit.MediaAssetID, application.FoodPermitMediaAssetID)
	if foodPermitMediaID <= 0 {
		return nil, fmt.Errorf("food_permit media missing")
	}
	foodPermitExpiry, err := parseMerchantCredentialExpiry(reviewInput.FoodPermit.ValidTo)
	if err != nil {
		return nil, fmt.Errorf("parse food permit expiry: %w", err)
	}
	businessLicenseNumber := firstTrimmed(reviewInput.BusinessLicense.CreditCode, reviewInput.BusinessLicense.RegNum, profile.BusinessLicense.Number)

	return []CredentialActivationInput{
		{
			DocumentType: db.CredentialDocumentTypeBusinessLicense,
			MediaAssetID: businessLicenseMediaID,
			ExpiresAt:    businessLicenseExpiry,
			NormalizedPayload: map[string]any{
				"license_number":       businessLicenseNumber,
				"credit_code":          strings.TrimSpace(reviewInput.BusinessLicense.CreditCode),
				"enterprise_name":      strings.TrimSpace(reviewInput.BusinessLicense.EnterpriseName),
				"legal_representative": strings.TrimSpace(reviewInput.BusinessLicense.LegalRepresentative),
				"address":              strings.TrimSpace(reviewInput.BusinessLicense.Address),
				"business_scope":       strings.TrimSpace(reviewInput.BusinessLicense.BusinessScope),
				"valid_period":         strings.TrimSpace(reviewInput.BusinessLicense.ValidPeriod),
			},
		},
		{
			DocumentType: db.CredentialDocumentTypeFoodPermit,
			MediaAssetID: foodPermitMediaID,
			ExpiresAt:    foodPermitExpiry,
			NormalizedPayload: map[string]any{
				"permit_number": strings.TrimSpace(reviewInput.FoodPermit.PermitNo),
				"company_name":  strings.TrimSpace(reviewInput.FoodPermit.CompanyName),
				"operator_name": strings.TrimSpace(reviewInput.FoodPermit.OperatorName),
				"valid_from":    strings.TrimSpace(reviewInput.FoodPermit.ValidFrom),
				"valid_to":      strings.TrimSpace(reviewInput.FoodPermit.ValidTo),
			},
		},
	}, nil
}

func subjectProfileMediaID(profileValue int64, fallback pgtype.Int8) int64 {
	if profileValue > 0 {
		return profileValue
	}
	if fallback.Valid {
		return fallback.Int64
	}
	return 0
}
