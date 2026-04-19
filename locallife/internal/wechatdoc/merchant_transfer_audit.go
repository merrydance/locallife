package wechatdoc

import (
	"reflect"

	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	wechaterrorcodes "github.com/merrydance/locallife/wechat/errorcodes"
)

type merchantTransferEndpointContract struct {
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

func AuditMerchantTransferAlignment(extraction *Extraction) *AlignmentAudit {
	docEndpoints := collectDocEndpoints(extraction, wechaterrorcodes.CanonicalMerchantTransferCode)
	contractInventory := merchantTransferContractInventory()
	keys := sortedEndpointKeys(docEndpoints)
	report := &AlignmentAudit{Scope: "merchant_transfer"}

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

func merchantTransferContractInventory() map[string]*merchantTransferEndpointContract {
	createRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.DirectMerchantTransferCreateRequestBody{}))
	createResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.DirectMerchantTransferCreateResponse{}))
	queryByOutBillNoRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.DirectMerchantTransferQueryByOutBillNoRequest{}))
	queryByTransferBillNoRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.DirectMerchantTransferQueryByTransferBillNoRequest{}))
	queryResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.DirectMerchantTransferQueryResponse{}))
	cancelRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.DirectMerchantTransferCancelRequest{}))
	cancelResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.DirectMerchantTransferCancelResponse{}))
	notifyRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.DirectMerchantTransferNotificationResource{}))

	fenConstraint := setOf("unit_fen")
	rfc3339Constraint := setOf("format_rfc3339")
	allStateValues := setOf(
		wechatcontracts.DirectMerchantTransferStateAccepted,
		wechatcontracts.DirectMerchantTransferStateProcessing,
		wechatcontracts.DirectMerchantTransferStateWaitUserConfirm,
		wechatcontracts.DirectMerchantTransferStateTransfering,
		wechatcontracts.DirectMerchantTransferStateSuccess,
		wechatcontracts.DirectMerchantTransferStateFail,
		wechatcontracts.DirectMerchantTransferStateCanceling,
		wechatcontracts.DirectMerchantTransferStateCancelled,
	)
	terminalStateValues := setOf(
		wechatcontracts.DirectMerchantTransferStateSuccess,
		wechatcontracts.DirectMerchantTransferStateFail,
		wechatcontracts.DirectMerchantTransferStateCancelled,
	)
	enterpriseCompensationValues := setOf(wechatcontracts.DirectMerchantTransferSceneEnterpriseCompensation)
	userRecvPerceptionValues := setOf(
		wechatcontracts.DirectMerchantTransferUserRecvPerceptionRefund,
		wechatcontracts.DirectMerchantTransferUserRecvPerceptionMerchantCompensation,
	)
	reportInfoTypeValues := setOf(wechatcontracts.DirectMerchantTransferReportInfoTypeCompensationReason)
	paymentMethodTypeValues := setOf(
		wechatcontracts.DirectMerchantTransferPaymentMethodTypeWallet,
		wechatcontracts.DirectMerchantTransferPaymentMethodTypeHKWallet,
	)
	failReasonValues := setOf(
		wechatcontracts.DirectMerchantTransferFailReasonAccountFrozen,
		wechatcontracts.DirectMerchantTransferFailReasonAccountNotExist,
		wechatcontracts.DirectMerchantTransferFailReasonPayeeAccountAbnormal,
		wechatcontracts.DirectMerchantTransferFailReasonPayerAccountAbnormal,
		wechatcontracts.DirectMerchantTransferFailReasonTransferSceneInvalid,
		wechatcontracts.DirectMerchantTransferFailReasonTransferSceneUnavailable,
		wechatcontracts.DirectMerchantTransferFailReasonTransferRisk,
		wechatcontracts.DirectMerchantTransferFailReasonOverdueClose,
	)

	return map[string]*merchantTransferEndpointContract{
		endpointAuditKey("POST", "/v3/fund-app/mch-transfer/transfer-bills"): {
			Method:         "POST",
			Path:           "/v3/fund-app/mch-transfer/transfer-bills",
			RequestFields:  createRequestFields,
			ResponseFields: createResponseFields,
			RequestConstraints: map[string]map[string]struct{}{
				"transfer_amount": fenConstraint,
			},
			ResponseConstraints: map[string]map[string]struct{}{
				"create_time": rfc3339Constraint,
			},
			RequestEnums: map[string]map[string]struct{}{
				"transfer_scene_id":                     enterpriseCompensationValues,
				"user_recv_perception":                  userRecvPerceptionValues,
				"transfer_scene_report_infos.info_type": reportInfoTypeValues,
			},
			ResponseEnums: map[string]map[string]struct{}{
				"state": allStateValues,
			},
			ErrorCodes: merchantTransferCodeSetToMap(wechaterrorcodes.MerchantTransferCreateDocumentedCodes),
		},
		endpointAuditKey("GET", "/v3/fund-app/mch-transfer/transfer-bills/out-bill-no/{out_bill_no}"): {
			Method:              "GET",
			Path:                "/v3/fund-app/mch-transfer/transfer-bills/out-bill-no/{out_bill_no}",
			RequestFields:       queryByOutBillNoRequestFields,
			ResponseFields:      queryResponseFields,
			ResponseConstraints: map[string]map[string]struct{}{"transfer_amount": fenConstraint, "create_time": rfc3339Constraint, "update_time": rfc3339Constraint},
			ResponseEnums:       map[string]map[string]struct{}{"state": allStateValues, "fail_reason": failReasonValues},
			ErrorCodes:          merchantTransferCodeSetToMap(wechaterrorcodes.MerchantTransferQueryDocumentedCodes),
		},
		endpointAuditKey("GET", "/v3/fund-app/mch-transfer/transfer-bills/transfer-bill-no/{transfer_bill_no}"): {
			Method:              "GET",
			Path:                "/v3/fund-app/mch-transfer/transfer-bills/transfer-bill-no/{transfer_bill_no}",
			RequestFields:       queryByTransferBillNoRequestFields,
			ResponseFields:      queryResponseFields,
			ResponseConstraints: map[string]map[string]struct{}{"transfer_amount": fenConstraint, "create_time": rfc3339Constraint, "update_time": rfc3339Constraint},
			ResponseEnums:       map[string]map[string]struct{}{"state": allStateValues, "fail_reason": failReasonValues},
			ErrorCodes:          merchantTransferCodeSetToMap(wechaterrorcodes.MerchantTransferQueryDocumentedCodes),
		},
		endpointAuditKey("POST", "/v1/webhooks/wechat-pay/merchant-transfer-notify"): {
			Method:             "POST",
			Path:               "/v1/webhooks/wechat-pay/merchant-transfer-notify",
			RequestFields:      notifyRequestFields,
			ResponseFields:     map[string]struct{}{},
			RequestConstraints: map[string]map[string]struct{}{"transfer_amount": fenConstraint, "create_time": rfc3339Constraint, "update_time": rfc3339Constraint},
			RequestEnums:       map[string]map[string]struct{}{"state": terminalStateValues, "payment_method_type": paymentMethodTypeValues, "fail_reason": failReasonValues},
		},
		endpointAuditKey("POST", "/v3/fund-app/mch-transfer/transfer-bills/out-bill-no/{out_bill_no}/cancel"): {
			Method:              "POST",
			Path:                "/v3/fund-app/mch-transfer/transfer-bills/out-bill-no/{out_bill_no}/cancel",
			RequestFields:       cancelRequestFields,
			ResponseFields:      cancelResponseFields,
			ResponseConstraints: map[string]map[string]struct{}{"update_time": rfc3339Constraint},
			ResponseEnums:       map[string]map[string]struct{}{"state": setOf(wechatcontracts.DirectMerchantTransferStateCanceling, wechatcontracts.DirectMerchantTransferStateCancelled)},
			ErrorCodes:          merchantTransferCodeSetToMap(wechaterrorcodes.MerchantTransferCancelDocumentedCodes),
		},
	}
}

func merchantTransferCodeSetToMap(set wechaterrorcodes.MerchantTransferCodeSet) map[string]struct{} {
	result := make(map[string]struct{}, len(set))
	for code := range set {
		result[wechaterrorcodes.CanonicalMerchantTransferCode(code)] = struct{}{}
	}
	return result
}
