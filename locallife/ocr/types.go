package ocr

import (
	"encoding/json"
	"fmt"
	"time"
)

// DocumentType identifies the semantic document class for OCR routing.
type DocumentType string

const (
	DocumentTypeBusinessLicense DocumentType = "business_license"
	DocumentTypeIDCard          DocumentType = "id_card"
	DocumentTypeFoodPermit      DocumentType = "food_permit"
	DocumentTypeHealthCert      DocumentType = "health_cert"
)

// OwnerType identifies which business aggregate owns the OCR job.
type OwnerType string

const (
	OwnerTypeMerchantApplication OwnerType = "merchant_application"
	OwnerTypeOperatorApplication OwnerType = "operator_application"
	OwnerTypeRiderApplication    OwnerType = "rider_application"
	OwnerTypeGroupApplication    OwnerType = "group_application"
)

// JobStatus represents the OCR job lifecycle state.
type JobStatus string

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusProcessing JobStatus = "processing"
	JobStatusSucceeded  JobStatus = "succeeded"
	JobStatusFailed     JobStatus = "failed"
	JobStatusCancelled  JobStatus = "cancelled"
)

// ProviderName identifies the OCR provider implementation.
type ProviderName string

const (
	ProviderNameAliyun ProviderName = "aliyun"
	ProviderNameWechat ProviderName = "wechat"
)

// Capability identifies the provider capability selected for a document type.
type Capability string

const (
	CapabilityAliyunBusinessLicense Capability = "aliyun.business_license"
	CapabilityAliyunIDCard          Capability = "aliyun.id_card"
	CapabilityAliyunFoodPermit      Capability = "aliyun.food_permit"
	CapabilityAliyunHealthCert      Capability = "aliyun.health_cert"
	CapabilityWechatBusinessLicense Capability = "wechat.business_license"
	CapabilityWechatIDCard          Capability = "wechat.id_card"
	CapabilityWechatPrintedText     Capability = "wechat.printed_text"
)

// DocumentSide identifies the side for two-sided documents.
type DocumentSide string

const (
	DocumentSideUnknown DocumentSide = ""
	DocumentSideFront   DocumentSide = "front"
	DocumentSideBack    DocumentSide = "back"
)

// BusinessLicenseResult stores normalized OCR fields for business licenses.
type BusinessLicenseResult struct {
	CreditCode          string `json:"credit_code,omitempty"`
	RegistrationNumber  string `json:"registration_number,omitempty"`
	EnterpriseName      string `json:"enterprise_name,omitempty"`
	LegalRepresentative string `json:"legal_representative,omitempty"`
	Address             string `json:"address,omitempty"`
	BusinessScope       string `json:"business_scope,omitempty"`
	ValidPeriod         string `json:"valid_period,omitempty"`
}

// IDCardResult stores normalized OCR fields for ID cards.
type IDCardResult struct {
	Name        string `json:"name,omitempty"`
	IDNumber    string `json:"id_number,omitempty"`
	Gender      string `json:"gender,omitempty"`
	Ethnicity   string `json:"ethnicity,omitempty"`
	Address     string `json:"address,omitempty"`
	BirthDate   string `json:"birth_date,omitempty"`
	Authority   string `json:"authority,omitempty"`
	ValidPeriod string `json:"valid_period,omitempty"`
}

// FoodPermitResult stores normalized OCR fields for food permits.
type FoodPermitResult struct {
	LicenseNumber string `json:"license_number,omitempty"`
	OperatorName  string `json:"operator_name,omitempty"`
	BusinessName  string `json:"business_name,omitempty"`
	Address       string `json:"address,omitempty"`
	ValidPeriod   string `json:"valid_period,omitempty"`
	RawText       string `json:"raw_text,omitempty"`
}

// HealthCertResult stores normalized OCR fields for health certificates.
type HealthCertResult struct {
	Name        string `json:"name,omitempty"`
	Certificate string `json:"certificate,omitempty"`
	ValidPeriod string `json:"valid_period,omitempty"`
	RawText     string `json:"raw_text,omitempty"`
}

// NormalizedResult is the provider-agnostic OCR output payload.
type NormalizedResult struct {
	DocumentType    DocumentType           `json:"document_type"`
	Side            DocumentSide           `json:"side,omitempty"`
	BusinessLicense *BusinessLicenseResult `json:"business_license,omitempty"`
	IDCard          *IDCardResult          `json:"id_card,omitempty"`
	FoodPermit      *FoodPermitResult      `json:"food_permit,omitempty"`
	HealthCert      *HealthCertResult      `json:"health_cert,omitempty"`
	RecognizedAt    time.Time              `json:"recognized_at"`
}

// Validate checks whether the document type is supported.
func (t DocumentType) Validate() error {
	switch t {
	case DocumentTypeBusinessLicense, DocumentTypeIDCard, DocumentTypeFoodPermit, DocumentTypeHealthCert:
		return nil
	default:
		return fmt.Errorf("unsupported document type: %s", t)
	}
}

// Validate checks whether the owner type is supported.
func (t OwnerType) Validate() error {
	switch t {
	case OwnerTypeMerchantApplication, OwnerTypeOperatorApplication, OwnerTypeRiderApplication, OwnerTypeGroupApplication:
		return nil
	default:
		return fmt.Errorf("unsupported owner type: %s", t)
	}
}

// Validate checks whether the side value is supported.
func (s DocumentSide) Validate() error {
	switch s {
	case DocumentSideUnknown, DocumentSideFront, DocumentSideBack:
		return nil
	default:
		return fmt.Errorf("unsupported document side: %s", s)
	}
}

// MarshalNormalizedResult converts the normalized result to JSON for persistence.
func MarshalNormalizedResult(result NormalizedResult) (json.RawMessage, error) {
	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

// UnmarshalNormalizedResult decodes persisted normalized OCR output.
func UnmarshalNormalizedResult(data []byte) (NormalizedResult, error) {
	if len(data) == 0 {
		return NormalizedResult{}, nil
	}
	var result NormalizedResult
	err := json.Unmarshal(data, &result)
	return result, err
}
