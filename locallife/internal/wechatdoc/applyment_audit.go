package wechatdoc

import (
	"reflect"
	"sort"
	"strings"

	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	wechterrorcodes "github.com/merrydance/locallife/wechat/errorcodes"
)

type AlignmentAudit struct {
	Scope             string                       `json:"scope"`
	Summary           AlignmentAuditSummary        `json:"summary"`
	Endpoints         []EndpointAlignmentAudit     `json:"endpoints"`
	SuppressedGaps    []EndpointAlignmentAudit     `json:"suppressed_gaps,omitempty"`
	CompatibilityGaps []EndpointCompatibilityAudit `json:"compatibility_gaps,omitempty"`
}

type AlignmentAuditSummary struct {
	DocumentedEndpointCount        int `json:"documented_endpoint_count"`
	AuditedEndpointCount           int `json:"audited_endpoint_count"`
	MissingEndpointCount           int `json:"missing_endpoint_count"`
	MissingRequestFieldCount       int `json:"missing_request_field_count"`
	MissingResponseFieldCount      int `json:"missing_response_field_count"`
	MissingRequestConstraintCount  int `json:"missing_request_constraint_count"`
	MissingResponseConstraintCount int `json:"missing_response_constraint_count"`
	MissingRequestEnumCount        int `json:"missing_request_enum_count"`
	MissingResponseEnumCount       int `json:"missing_response_enum_count"`
	MissingErrorCodeCount          int `json:"missing_error_code_count"`
	SuppressedRequestFieldCount    int `json:"suppressed_request_field_count"`
	SuppressedResponseFieldCount   int `json:"suppressed_response_field_count"`
	SuppressedRequestEnumCount     int `json:"suppressed_request_enum_count"`
	SuppressedResponseEnumCount    int `json:"suppressed_response_enum_count"`
	SuppressedErrorCodeCount       int `json:"suppressed_error_code_count"`
	CompatibilityEndpointCount     int `json:"compatibility_endpoint_count"`
	CompatibilityErrorCodeCount    int `json:"compatibility_error_code_count"`
}

type EndpointAlignmentAudit struct {
	Method                     string                 `json:"method"`
	Path                       string                 `json:"path"`
	MissingEndpoint            bool                   `json:"missing_endpoint,omitempty"`
	MissingRequestFields       []string               `json:"missing_request_fields,omitempty"`
	MissingResponseFields      []string               `json:"missing_response_fields,omitempty"`
	MissingRequestConstraints  []FieldConstraintAudit `json:"missing_request_constraints,omitempty"`
	MissingResponseConstraints []FieldConstraintAudit `json:"missing_response_constraints,omitempty"`
	MissingRequestEnums        []FieldEnumAudit       `json:"missing_request_enums,omitempty"`
	MissingResponseEnums       []FieldEnumAudit       `json:"missing_response_enums,omitempty"`
	MissingErrorCodes          []string               `json:"missing_error_codes,omitempty"`
}

type EndpointCompatibilityAudit struct {
	Method                  string   `json:"method"`
	Path                    string   `json:"path"`
	CompatibilityErrorCodes []string `json:"compatibility_error_codes,omitempty"`
}

type FieldEnumAudit struct {
	Field         string   `json:"field"`
	MissingValues []string `json:"missing_values"`
}

type FieldConstraintAudit struct {
	Field              string   `json:"field"`
	MissingConstraints []string `json:"missing_constraints"`
}

type fieldEnumDiff struct {
	Missing    []FieldEnumAudit
	Suppressed []FieldEnumAudit
}

type applymentEndpointDoc struct {
	Method         string
	Path           string
	RequestFields  map[string]Field
	ResponseFields map[string]Field
	ErrorCodes     map[string]struct{}
}

type applymentEndpointContract struct {
	Method              string
	Path                string
	RequestFields       map[string]struct{}
	ResponseFields      map[string]struct{}
	RequestConstraints  map[string]map[string]struct{}
	ResponseConstraints map[string]map[string]struct{}
	RequestEnums        map[string]map[string]struct{}
	IgnoredRequestEnums map[string]map[string]struct{}
	ResponseEnums       map[string]map[string]struct{}
	ErrorCodes          map[string]struct{}
}

func AuditApplymentAlignment(extraction *Extraction) *AlignmentAudit {
	docEndpoints := collectDocEndpoints(extraction, wechterrorcodes.CanonicalApplymentCode)
	contractInventory := applymentContractInventory()
	keys := sortedEndpointKeys(docEndpoints)
	report := &AlignmentAudit{Scope: "applyment"}

	for _, key := range keys {
		doc := docEndpoints[key]
		if doc == nil {
			continue
		}
		report.Summary.DocumentedEndpointCount++
		endpointAudit := EndpointAlignmentAudit{Method: doc.Method, Path: doc.Path}
		contract, ok := contractInventory[key]
		if !ok {
			endpointAudit.MissingEndpoint = true
			endpointAudit.MissingRequestFields = sortedFieldNames(doc.RequestFields)
			endpointAudit.MissingResponseFields = sortedFieldNames(doc.ResponseFields)
			endpointAudit.MissingErrorCodes = sortedSetKeys(doc.ErrorCodes)
			report.Endpoints = append(report.Endpoints, endpointAudit)
			accumulateAlignmentSummary(&report.Summary, endpointAudit)
			continue
		}
		report.Summary.AuditedEndpointCount++
		endpointAudit.MissingRequestFields = diffFieldNames(doc.RequestFields, contract.RequestFields)
		endpointAudit.MissingResponseFields = diffFieldNames(doc.ResponseFields, contract.ResponseFields)
		endpointAudit.MissingRequestConstraints = diffFieldConstraints(doc.RequestFields, contract.RequestConstraints)
		endpointAudit.MissingResponseConstraints = diffFieldConstraints(doc.ResponseFields, contract.ResponseConstraints)
		requestEnumDiff := diffFieldEnums(doc.RequestFields, contract.RequestEnums, contract.IgnoredRequestEnums)
		responseEnumDiff := diffFieldEnums(doc.ResponseFields, contract.ResponseEnums, nil)
		endpointAudit.MissingRequestEnums = requestEnumDiff.Missing
		endpointAudit.MissingResponseEnums = responseEnumDiff.Missing
		endpointAudit.MissingErrorCodes = diffSet(doc.ErrorCodes, contract.ErrorCodes)

		suppressedAudit := EndpointAlignmentAudit{Method: doc.Method, Path: doc.Path}
		suppressedAudit.MissingRequestEnums = requestEnumDiff.Suppressed
		if endpointHasMissingCoverage(endpointAudit) {
			report.Endpoints = append(report.Endpoints, endpointAudit)
			accumulateAlignmentSummary(&report.Summary, endpointAudit)
		}
		if endpointHasMissingCoverage(suppressedAudit) {
			report.SuppressedGaps = append(report.SuppressedGaps, suppressedAudit)
			accumulateSuppressedAlignmentSummary(&report.Summary, suppressedAudit)
		}
	}

	return report
}

func endpointHasMissingCoverage(audit EndpointAlignmentAudit) bool {
	return audit.MissingEndpoint ||
		len(audit.MissingRequestFields) > 0 ||
		len(audit.MissingResponseFields) > 0 ||
		len(audit.MissingRequestConstraints) > 0 ||
		len(audit.MissingResponseConstraints) > 0 ||
		len(audit.MissingRequestEnums) > 0 ||
		len(audit.MissingResponseEnums) > 0 ||
		len(audit.MissingErrorCodes) > 0
}

func accumulateAlignmentSummary(summary *AlignmentAuditSummary, audit EndpointAlignmentAudit) {
	if audit.MissingEndpoint {
		summary.MissingEndpointCount++
	}
	summary.MissingRequestFieldCount += len(audit.MissingRequestFields)
	summary.MissingResponseFieldCount += len(audit.MissingResponseFields)
	for _, constraintAudit := range audit.MissingRequestConstraints {
		summary.MissingRequestConstraintCount += len(constraintAudit.MissingConstraints)
	}
	for _, constraintAudit := range audit.MissingResponseConstraints {
		summary.MissingResponseConstraintCount += len(constraintAudit.MissingConstraints)
	}
	summary.MissingErrorCodeCount += len(audit.MissingErrorCodes)
	for _, enumAudit := range audit.MissingRequestEnums {
		summary.MissingRequestEnumCount += len(enumAudit.MissingValues)
	}
	for _, enumAudit := range audit.MissingResponseEnums {
		summary.MissingResponseEnumCount += len(enumAudit.MissingValues)
	}
}

func accumulateSuppressedAlignmentSummary(summary *AlignmentAuditSummary, audit EndpointAlignmentAudit) {
	summary.SuppressedRequestFieldCount += len(audit.MissingRequestFields)
	summary.SuppressedResponseFieldCount += len(audit.MissingResponseFields)
	summary.SuppressedErrorCodeCount += len(audit.MissingErrorCodes)
	for _, enumAudit := range audit.MissingRequestEnums {
		summary.SuppressedRequestEnumCount += len(enumAudit.MissingValues)
	}
	for _, enumAudit := range audit.MissingResponseEnums {
		summary.SuppressedResponseEnumCount += len(enumAudit.MissingValues)
	}
}

func accumulateCompatibilitySummary(summary *AlignmentAuditSummary, audit EndpointCompatibilityAudit) {
	if len(audit.CompatibilityErrorCodes) == 0 {
		return
	}
	summary.CompatibilityEndpointCount++
	summary.CompatibilityErrorCodeCount += len(audit.CompatibilityErrorCodes)
}

func collectDocEndpoints(extraction *Extraction, canonicalizeCode func(string) string) map[string]*applymentEndpointDoc {
	result := make(map[string]*applymentEndpointDoc)
	if extraction == nil {
		return result
	}
	if canonicalizeCode == nil {
		canonicalizeCode = strings.TrimSpace
	}

	endpointSections := make(map[string][]Endpoint)
	for _, section := range extraction.Sections {
		if len(section.Endpoints) == 0 || len(section.Path) == 0 {
			continue
		}
		parentKey := sectionKey(section.Path[:len(section.Path)-1])
		endpointSections[parentKey] = append(endpointSections[parentKey], section.Endpoints...)
	}

	for _, section := range extraction.Sections {
		if len(section.Path) == 0 {
			continue
		}
		parentKey := sectionKey(section.Path[:len(section.Path)-1])
		endpoints := endpointSections[parentKey]
		if len(endpoints) == 0 {
			continue
		}
		for _, endpoint := range endpoints {
			key := endpointAuditKey(endpoint.Method, endpoint.Path)
			doc := ensureApplymentDocEndpoint(result, endpoint)
			switch classifySectionKind(section.Heading) {
			case "request":
				for _, field := range section.Fields {
					doc.RequestFields[field.Name] = field
				}
			case "response":
				for _, field := range section.Fields {
					doc.ResponseFields[field.Name] = field
				}
			case "error":
				for _, errorCode := range section.ErrorCodes {
					doc.ErrorCodes[canonicalizeCode(errorCode.Code)] = struct{}{}
				}
			}
			result[key] = doc
		}
	}

	return result
}

func ensureApplymentDocEndpoint(index map[string]*applymentEndpointDoc, endpoint Endpoint) *applymentEndpointDoc {
	key := endpointAuditKey(endpoint.Method, endpoint.Path)
	if existing := index[key]; existing != nil {
		return existing
	}
	doc := &applymentEndpointDoc{
		Method:         strings.ToUpper(strings.TrimSpace(endpoint.Method)),
		Path:           normalizePath(endpoint.Path),
		RequestFields:  make(map[string]Field),
		ResponseFields: make(map[string]Field),
		ErrorCodes:     make(map[string]struct{}),
	}
	index[key] = doc
	return doc
}

func classifySectionKind(heading string) string {
	normalized := strings.ToLower(strings.TrimSpace(heading))
	if normalized == "" {
		return ""
	}
	if strings.Contains(normalized, "错误码") {
		return "error"
	}
	if strings.Contains(normalized, "应答") || strings.Contains(normalized, "响应") {
		return "response"
	}
	if strings.Contains(normalized, "header") {
		return "ignore"
	}
	if strings.Contains(normalized, "path") || strings.Contains(normalized, "query") || strings.Contains(normalized, "body") || strings.Contains(normalized, "请求") || strings.Contains(normalized, "参数") {
		return "request"
	}
	return ""
}

func applymentContractInventory() map[string]*applymentEndpointContract {
	createRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.EcommerceApplymentRequest{}))
	createResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.EcommerceApplymentResponse{}))
	queryResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.EcommerceApplymentQueryResponse{}))
	settlementRequestFields := map[string]struct{}{
		"sub_mchid":           {},
		"account_number_rule": {},
	}
	settlementResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.SubMerchantSettlementResponse{}))
	modifySettlementRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.ModifySubMerchantSettlementRequest{}))
	modifySettlementRequestFields["sub_mchid"] = struct{}{}
	modifySettlementResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.ModifySubMerchantSettlementResponse{}))
	settlementApplicationRequestFields := map[string]struct{}{
		"sub_mchid":           {},
		"application_no":      {},
		"account_number_rule": {},
	}
	settlementApplicationResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.QuerySubMerchantSettlementApplicationResponse{}))
	uploadRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.ImageUploadRequest{}))
	uploadResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.ImageUploadResponse{}))

	return map[string]*applymentEndpointContract{
		endpointAuditKey("POST", "/v3/ecommerce/applyments/"): {
			Method:         "POST",
			Path:           "/v3/ecommerce/applyments/",
			RequestFields:  createRequestFields,
			ResponseFields: createResponseFields,
			RequestEnums: map[string]map[string]struct{}{
				"organization_type": setOf(
					wechatcontracts.ApplymentOrganizationTypeEnterprise,
					wechatcontracts.ApplymentOrganizationTypeIndividualBusiness,
				),
				"business_license_info.cert_type": setOf(
					wechatcontracts.ApplymentCertificateType2388,
					wechatcontracts.ApplymentCertificateType2389,
					wechatcontracts.ApplymentCertificateType2394,
					wechatcontracts.ApplymentCertificateType2395,
					wechatcontracts.ApplymentCertificateType2396,
					wechatcontracts.ApplymentCertificateType2399,
					wechatcontracts.ApplymentCertificateType2400,
					wechatcontracts.ApplymentCertificateType2520,
					wechatcontracts.ApplymentCertificateType2521,
					wechatcontracts.ApplymentCertificateType2522,
				),
				"id_holder_type": setOf(
					wechatcontracts.ApplymentIDHolderTypeLegal,
					wechatcontracts.ApplymentIDHolderTypeSuper,
				),
				"id_doc_type": setOf(
					wechatcontracts.ApplymentIdentificationTypeMainlandIDCard,
					wechatcontracts.ApplymentIdentificationTypeOverseaPassport,
					wechatcontracts.ApplymentIdentificationTypeHongKong,
					wechatcontracts.ApplymentIdentificationTypeMacao,
					wechatcontracts.ApplymentIdentificationTypeTaiwan,
					wechatcontracts.ApplymentIdentificationTypeForeignResident,
					wechatcontracts.ApplymentIdentificationTypeHongKongMacaoResident,
					wechatcontracts.ApplymentIdentificationTypeTaiwanResident,
				),
				"account_info.bank_account_type": setOf(
					wechatcontracts.ApplymentBankAccountTypeBusiness,
					wechatcontracts.ApplymentBankAccountTypePrivate,
				),
				"contact_info.contact_type": setOf(
					wechatcontracts.ApplymentContactTypeLegal,
					wechatcontracts.ApplymentContactTypeSuper,
				),
			},
			IgnoredRequestEnums: map[string]map[string]struct{}{
				"organization_type": setOf("1708", "2401", "2500", "2502", "3"),
			},
			ErrorCodes: codeSetToMap(wechterrorcodes.EcommerceApplymentCreateDocumentedCodes),
		},
		endpointAuditKey("GET", "/v3/ecommerce/applyments/out-request-no/{out_request_no}"): {
			Method:         "GET",
			Path:           "/v3/ecommerce/applyments/out-request-no/{out_request_no}",
			RequestFields:  map[string]struct{}{"out_request_no": {}},
			ResponseFields: queryResponseFields,
			ResponseEnums: map[string]map[string]struct{}{
				"applyment_state": setOf(
					wechatcontracts.ApplymentStateChecking,
					wechatcontracts.ApplymentStateAccountNeedVerify,
					wechatcontracts.ApplymentStateAuditing,
					wechatcontracts.ApplymentStateRejected,
					wechatcontracts.ApplymentStateNeedSign,
					wechatcontracts.ApplymentStateFinish,
					wechatcontracts.ApplymentStateFrozen,
					wechatcontracts.ApplymentStateCanceled,
				),
				"sign_state": setOf(
					wechatcontracts.ApplymentSignStateUnsigned,
					wechatcontracts.ApplymentSignStateSigned,
					wechatcontracts.ApplymentSignStateNotSignable,
				),
			},
			ErrorCodes: codeSetToMap(wechterrorcodes.EcommerceApplymentQueryDocumentedCodes),
		},
		endpointAuditKey("GET", "/v3/ecommerce/applyments/{applyment_id}"): {
			Method:         "GET",
			Path:           "/v3/ecommerce/applyments/{applyment_id}",
			RequestFields:  map[string]struct{}{"applyment_id": {}},
			ResponseFields: queryResponseFields,
			ResponseEnums: map[string]map[string]struct{}{
				"applyment_state": setOf(
					wechatcontracts.ApplymentStateChecking,
					wechatcontracts.ApplymentStateAccountNeedVerify,
					wechatcontracts.ApplymentStateAuditing,
					wechatcontracts.ApplymentStateRejected,
					wechatcontracts.ApplymentStateNeedSign,
					wechatcontracts.ApplymentStateFinish,
					wechatcontracts.ApplymentStateFrozen,
					wechatcontracts.ApplymentStateCanceled,
				),
				"sign_state": setOf(
					wechatcontracts.ApplymentSignStateUnsigned,
					wechatcontracts.ApplymentSignStateSigned,
					wechatcontracts.ApplymentSignStateNotSignable,
				),
			},
			ErrorCodes: codeSetToMap(wechterrorcodes.EcommerceApplymentQueryDocumentedCodes),
		},
		endpointAuditKey("POST", "/v3/apply4sub/sub_merchants/{sub_mchid}/modify-settlement"): {
			Method:         "POST",
			Path:           "/v3/apply4sub/sub_merchants/{sub_mchid}/modify-settlement",
			RequestFields:  modifySettlementRequestFields,
			ResponseFields: modifySettlementResponseFields,
			RequestEnums: map[string]map[string]struct{}{
				"account_type": setOf(
					wechatcontracts.SubMerchantSettlementAccountTypeBusiness,
					wechatcontracts.SubMerchantSettlementAccountTypePrivate,
				),
			},
			ErrorCodes: codeSetToMap(wechterrorcodes.SubMerchantSettlementModifyDocumentedCodes),
		},
		endpointAuditKey("GET", "/v3/apply4sub/sub_merchants/{sub_mchid}/settlement"): {
			Method:         "GET",
			Path:           "/v3/apply4sub/sub_merchants/{sub_mchid}/settlement",
			RequestFields:  settlementRequestFields,
			ResponseFields: settlementResponseFields,
			RequestEnums: map[string]map[string]struct{}{
				"account_number_rule": setOf(
					wechatcontracts.SubMerchantSettlementAccountNumberRuleMaskV1,
					wechatcontracts.SubMerchantSettlementAccountNumberRuleMaskV2,
				),
			},
			ResponseEnums: map[string]map[string]struct{}{
				"account_type": setOf(
					wechatcontracts.SubMerchantSettlementAccountTypeBusiness,
					wechatcontracts.SubMerchantSettlementAccountTypePrivate,
				),
				"verify_result": setOf(
					wechatcontracts.SubMerchantSettlementVerifyResultSuccess,
					wechatcontracts.SubMerchantSettlementVerifyResultFail,
					wechatcontracts.SubMerchantSettlementVerifyResultVerifying,
				),
			},
			ErrorCodes: codeSetToMap(wechterrorcodes.SubMerchantSettlementQueryDocumentedCodes),
		},
		endpointAuditKey("GET", "/v3/apply4sub/sub_merchants/{sub_mchid}/application/{application_no}"): {
			Method:         "GET",
			Path:           "/v3/apply4sub/sub_merchants/{sub_mchid}/application/{application_no}",
			RequestFields:  settlementApplicationRequestFields,
			ResponseFields: settlementApplicationResponseFields,
			RequestEnums: map[string]map[string]struct{}{
				"account_number_rule": setOf(
					wechatcontracts.SubMerchantSettlementAccountNumberRuleMaskV1,
					wechatcontracts.SubMerchantSettlementAccountNumberRuleMaskV2,
				),
			},
			ResponseEnums: map[string]map[string]struct{}{
				"account_type": setOf(
					wechatcontracts.SubMerchantSettlementAccountTypeBusiness,
					wechatcontracts.SubMerchantSettlementAccountTypePrivate,
				),
				"verify_result": setOf(
					wechatcontracts.SubMerchantSettlementApplicationAuditSuccess,
					wechatcontracts.SubMerchantSettlementApplicationAuditing,
					wechatcontracts.SubMerchantSettlementApplicationAuditFail,
				),
			},
			ErrorCodes: codeSetToMap(wechterrorcodes.SubMerchantSettlementApplicationQueryDocumentedCodes),
		},
		endpointAuditKey("POST", "/v3/merchant/media/upload"): {
			Method:         "POST",
			Path:           "/v3/merchant/media/upload",
			RequestFields:  uploadRequestFields,
			ResponseFields: uploadResponseFields,
			ErrorCodes:     codeSetToMap(wechterrorcodes.MerchantMediaUploadDocumentedCodes),
		},
	}
}

func structJSONFields(structType reflect.Type) map[string]struct{} {
	result := make(map[string]struct{})
	collectJSONFields(structType, "", result)
	return result
}

func collectJSONFields(fieldType reflect.Type, prefix string, dest map[string]struct{}) {
	if fieldType == nil {
		return
	}
	for fieldType.Kind() == reflect.Pointer || fieldType.Kind() == reflect.Slice || fieldType.Kind() == reflect.Array {
		fieldType = fieldType.Elem()
		if fieldType == nil {
			return
		}
	}
	if fieldType.Kind() != reflect.Struct {
		return
	}

	for index := 0; index < fieldType.NumField(); index++ {
		field := fieldType.Field(index)
		if field.PkgPath != "" && !field.Anonymous {
			continue
		}
		jsonName := jsonFieldName(field)
		if jsonName == "" || jsonName == "-" {
			continue
		}
		name := jsonName
		if prefix != "" {
			name = prefix + "." + jsonName
		}
		dest[name] = struct{}{}

		nestedType := field.Type
		for nestedType.Kind() == reflect.Pointer || nestedType.Kind() == reflect.Slice || nestedType.Kind() == reflect.Array {
			nestedType = nestedType.Elem()
		}
		if nestedType.Kind() == reflect.Struct && nestedType.PkgPath() != "time" {
			collectJSONFields(nestedType, name, dest)
		}
	}
}

func jsonFieldName(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	if tag == "" {
		return ""
	}
	name := strings.Split(tag, ",")[0]
	return strings.TrimSpace(name)
}

func diffFieldNames(documented map[string]Field, known map[string]struct{}) []string {
	missing := make([]string, 0)
	for name := range documented {
		if _, ok := known[name]; ok {
			continue
		}
		missing = append(missing, name)
	}
	sort.Strings(missing)
	return missing
}

func diffFieldEnums(documented map[string]Field, known map[string]map[string]struct{}, ignored map[string]map[string]struct{}) fieldEnumDiff {
	if len(documented) == 0 {
		return fieldEnumDiff{}
	}
	missingDiffs := make([]FieldEnumAudit, 0)
	suppressedDiffs := make([]FieldEnumAudit, 0)
	for name, field := range documented {
		if len(field.EnumValues) == 0 {
			continue
		}
		knownValues := known[name]
		ignoredValues := ignored[name]
		missingValues := make([]string, 0)
		suppressedValues := make([]string, 0)
		for _, value := range field.EnumValues {
			normalized := strings.TrimSpace(value)
			if normalized == "" || !isComparableEnumValue(normalized) {
				continue
			}
			if _, ok := knownValues[normalized]; ok {
				continue
			}
			if _, ok := ignoredValues[normalized]; ok {
				suppressedValues = append(suppressedValues, normalized)
				continue
			}
			missingValues = append(missingValues, normalized)
		}
		if len(missingValues) > 0 {
			sort.Strings(missingValues)
			missingDiffs = append(missingDiffs, FieldEnumAudit{Field: name, MissingValues: missingValues})
		}
		if len(suppressedValues) > 0 {
			sort.Strings(suppressedValues)
			suppressedDiffs = append(suppressedDiffs, FieldEnumAudit{Field: name, MissingValues: suppressedValues})
		}
	}
	sort.Slice(missingDiffs, func(i, j int) bool {
		return missingDiffs[i].Field < missingDiffs[j].Field
	})
	sort.Slice(suppressedDiffs, func(i, j int) bool {
		return suppressedDiffs[i].Field < suppressedDiffs[j].Field
	})
	return fieldEnumDiff{Missing: missingDiffs, Suppressed: suppressedDiffs}
}

func diffFieldConstraints(documented map[string]Field, known map[string]map[string]struct{}) []FieldConstraintAudit {
	if len(documented) == 0 || len(known) == 0 {
		return nil
	}
	missingDiffs := make([]FieldConstraintAudit, 0)
	for name, requiredConstraints := range known {
		field, ok := documented[name]
		if !ok {
			continue
		}
		missingConstraints := make([]string, 0)
		for constraint := range requiredConstraints {
			if fieldSatisfiesConstraint(field, constraint) {
				continue
			}
			missingConstraints = append(missingConstraints, constraint)
		}
		if len(missingConstraints) == 0 {
			continue
		}
		sort.Strings(missingConstraints)
		missingDiffs = append(missingDiffs, FieldConstraintAudit{Field: name, MissingConstraints: missingConstraints})
	}
	sort.Slice(missingDiffs, func(i, j int) bool {
		return missingDiffs[i].Field < missingDiffs[j].Field
	})
	return missingDiffs
}

func fieldSatisfiesConstraint(field Field, constraint string) bool {
	description := strings.ToLower(strings.TrimSpace(field.Description))
	switch strings.TrimSpace(constraint) {
	case "unit_fen":
		return strings.Contains(description, "单位为分") ||
			strings.Contains(description, "单位为：分") ||
			strings.Contains(description, "单位（分）") ||
			strings.Contains(description, "单位(分)") ||
			strings.Contains(description, "单位:分") ||
			strings.Contains(description, "单位为人民币分") ||
			strings.Contains(description, "单位：分") ||
			strings.Contains(description, "人民币分")
	case "format_rfc3339":
		return strings.Contains(description, "rfc3339") ||
			strings.Contains(description, "yyyy-mm-ddthh:mm:ss") ||
			strings.Contains(description, "timezone")
	case "format_date_yyyy_mm_dd":
		return strings.Contains(description, "yyyy-mm-dd") && !strings.Contains(description, "hh:mm:ss")
	default:
		return false
	}
}

func isComparableEnumValue(value string) bool {
	for _, r := range value {
		if isCJKRune(r) || r == ' ' || r == '\t' {
			return false
		}
	}
	return true
}

func isCJKRune(r rune) bool {
	return r >= 0x4E00 && r <= 0x9FFF
}

func diffSet(documented map[string]struct{}, known map[string]struct{}) []string {
	missing := make([]string, 0)
	for value := range documented {
		if _, ok := known[value]; ok {
			continue
		}
		missing = append(missing, value)
	}
	sort.Strings(missing)
	return missing
}

func sortedFieldNames(fields map[string]Field) []string {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedSetKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for value := range values {
		keys = append(keys, value)
	}
	sort.Strings(keys)
	return keys
}

func codeSetToMap(set wechterrorcodes.ApplymentCodeSet) map[string]struct{} {
	result := make(map[string]struct{}, len(set))
	for code := range set {
		result[wechterrorcodes.CanonicalApplymentCode(code)] = struct{}{}
	}
	return result
}

func setOf(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		result[trimmed] = struct{}{}
	}
	return result
}

func sortedEndpointKeys(endpoints map[string]*applymentEndpointDoc) []string {
	keys := make([]string, 0, len(endpoints))
	for key := range endpoints {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func endpointAuditKey(method, path string) string {
	return strings.ToUpper(strings.TrimSpace(method)) + " " + normalizePath(path)
}
