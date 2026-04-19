package wechatdoc

import (
	"reflect"

	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	wechaterrorcodes "github.com/merrydance/locallife/wechat/errorcodes"
)

type subsidyEndpointContract struct {
	Method              string
	Path                string
	RequestFields       map[string]struct{}
	ResponseFields      map[string]struct{}
	RequestConstraints  map[string]map[string]struct{}
	ResponseConstraints map[string]map[string]struct{}
	RequestEnums        map[string]map[string]struct{}
	ResponseEnums       map[string]map[string]struct{}
	ErrorCodes          map[string]struct{}
}

func AuditSubsidyAlignment(extraction *Extraction) *AlignmentAudit {
	docEndpoints := collectDocEndpoints(extraction, wechaterrorcodes.CanonicalSubsidyCode)
	contractInventory := subsidyContractInventory()
	keys := sortedEndpointKeys(docEndpoints)
	report := &AlignmentAudit{Scope: "subsidy"}

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
		endpointAudit.MissingRequestEnums = diffFieldEnums(doc.RequestFields, contract.RequestEnums, nil).Missing
		endpointAudit.MissingResponseEnums = diffFieldEnums(doc.ResponseFields, contract.ResponseEnums, nil).Missing
		endpointAudit.MissingErrorCodes = diffSet(doc.ErrorCodes, contract.ErrorCodes)
		if endpointHasMissingCoverage(endpointAudit) {
			report.Endpoints = append(report.Endpoints, endpointAudit)
			accumulateAlignmentSummary(&report.Summary, endpointAudit)
		}
	}

	return report
}

func subsidyContractInventory() map[string]*subsidyEndpointContract {
	createResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.SubsidyResponse{}))
	returnResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.SubsidyReturnResponse{}))
	cancelResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.SubsidyCancelResponse{}))

	fenConstraint := setOf("unit_fen")
	rfc3339Constraint := setOf("format_rfc3339")
	createResultValues := setOf(
		wechatcontracts.SubsidyResultSuccess,
		wechatcontracts.SubsidyResultFail,
		wechatcontracts.SubsidyResultRefund,
	)
	returnResultValues := setOf(
		wechatcontracts.SubsidyResultSuccess,
		wechatcontracts.SubsidyResultFail,
	)
	returnAccountValues := setOf(
		wechatcontracts.SubsidyReturnAccountAvailable,
		wechatcontracts.SubsidyReturnAccountUnavailable,
	)

	return map[string]*subsidyEndpointContract{
		endpointAuditKey("POST", "/v3/ecommerce/subsidies/create"): {
			Method: "POST",
			Path:   "/v3/ecommerce/subsidies/create",
			RequestFields: setOf(
				"sub_mchid",
				"transaction_id",
				"amount",
				"description",
				"out_subsidy_no",
			),
			ResponseFields: createResponseFields,
			RequestConstraints: map[string]map[string]struct{}{
				"amount": fenConstraint,
			},
			ResponseConstraints: map[string]map[string]struct{}{
				"amount":        fenConstraint,
				"success_time":  rfc3339Constraint,
			},
			ResponseEnums: map[string]map[string]struct{}{
				"result": createResultValues,
			},
			ErrorCodes: subsidyCodeSetToMap(wechaterrorcodes.SubsidyCreateDocumentedCodes),
		},
		endpointAuditKey("POST", "/v3/ecommerce/subsidies/return"): {
			Method: "POST",
			Path:   "/v3/ecommerce/subsidies/return",
			RequestFields: setOf(
				"sub_mchid",
				"out_order_no",
				"transaction_id",
				"refund_id",
				"amount",
				"description",
				"subsidy_id",
				"from",
				"from.account",
				"from.amount",
			),
			ResponseFields: returnResponseFields,
			RequestConstraints: map[string]map[string]struct{}{
				"amount":      fenConstraint,
				"from.amount": fenConstraint,
			},
			ResponseConstraints: map[string]map[string]struct{}{
				"amount":        fenConstraint,
				"from.amount":   fenConstraint,
				"success_time":  rfc3339Constraint,
			},
			RequestEnums: map[string]map[string]struct{}{
				"from.account": returnAccountValues,
			},
			ResponseEnums: map[string]map[string]struct{}{
				"result":       returnResultValues,
				"from.account": returnAccountValues,
			},
			ErrorCodes: subsidyCodeSetToMap(wechaterrorcodes.SubsidyReturnDocumentedCodes),
		},
		endpointAuditKey("POST", "/v3/ecommerce/subsidies/cancel"): {
			Method: "POST",
			Path:   "/v3/ecommerce/subsidies/cancel",
			RequestFields: setOf(
				"sub_mchid",
				"transaction_id",
				"description",
			),
			ResponseFields: cancelResponseFields,
			ResponseEnums: map[string]map[string]struct{}{
				"result": returnResultValues,
			},
			ErrorCodes: subsidyCodeSetToMap(wechaterrorcodes.SubsidyCancelDocumentedCodes),
		},
	}
}

func subsidyCodeSetToMap(set wechaterrorcodes.SubsidyCodeSet) map[string]struct{} {
	result := make(map[string]struct{}, len(set))
	for code := range set {
		result[wechaterrorcodes.CanonicalSubsidyCode(code)] = struct{}{}
	}
	return result
}