package wechatdoc

import (
	"reflect"

	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	wechaterrorcodes "github.com/merrydance/locallife/wechat/errorcodes"
)

type cancelWithdrawEndpointContract struct {
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

func AuditCancelWithdrawAlignment(extraction *Extraction) *AlignmentAudit {
	docEndpoints := collectDocEndpoints(extraction, wechaterrorcodes.CanonicalCancelWithdrawCode)
	contractInventory := cancelWithdrawContractInventory()
	keys := sortedEndpointKeys(docEndpoints)
	report := &AlignmentAudit{Scope: "cancel_withdraw"}

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

func cancelWithdrawContractInventory() map[string]*cancelWithdrawEndpointContract {
	validateResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.CancelWithdrawEligibilityResponse{}))
	createRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.CancelWithdrawRequest{}))
	createResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.CancelWithdrawCreateResponse{}))
	queryResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.CancelWithdrawQueryResponse{}))

	fenConstraint := setOf("unit_fen")
	rfc3339Constraint := setOf("format_rfc3339")

	validateResponseEnums := map[string]map[string]struct{}{
		"merchant_state":       setOf(wechatcontracts.CancelWithdrawMerchantStateNormal, wechatcontracts.CancelWithdrawMerchantStateCancelled),
		"validate_result":      setOf(wechatcontracts.CancelWithdrawValidateResultAllow, wechatcontracts.CancelWithdrawValidateResultNotAllow),
		"account_info.out_account_type": setOf(
			wechatcontracts.CancelWithdrawOutAccountTypeBasic,
			wechatcontracts.CancelWithdrawOutAccountTypeOperate,
			wechatcontracts.CancelWithdrawOutAccountTypeMargin,
			wechatcontracts.CancelWithdrawOutAccountTypeTradeFee,
		),
		"block_reasons.type": setOf(
			wechatcontracts.CancelWithdrawBlockReasonTypeConsumerComplaint,
			wechatcontracts.CancelWithdrawBlockReasonTypeBlockingControl,
			wechatcontracts.CancelWithdrawBlockReasonTypeFundsPending,
			wechatcontracts.CancelWithdrawBlockReasonTypeOtherReason,
		),
	}

	queryResponseEnums := map[string]map[string]struct{}{
		"cancel_state": setOf(
			wechatcontracts.CancelWithdrawCancelStateAccepted,
			wechatcontracts.CancelWithdrawCancelStateReviewing,
			wechatcontracts.CancelWithdrawCancelStateRejected,
			wechatcontracts.CancelWithdrawCancelStateWaitingMerchantConfirm,
			wechatcontracts.CancelWithdrawCancelStateRevoked,
			wechatcontracts.CancelWithdrawCancelStateSystemProcessing,
			wechatcontracts.CancelWithdrawCancelStateCanceled,
			wechatcontracts.CancelWithdrawCancelStateFundProcessing,
			wechatcontracts.CancelWithdrawCancelStateFinish,
		),
		"withdraw": setOf(
			wechatcontracts.CancelWithdrawModeNotApply,
			wechatcontracts.CancelWithdrawModeApply,
		),
		"withdraw_state": setOf(
			wechatcontracts.CancelWithdrawWithdrawStateProcessing,
			wechatcontracts.CancelWithdrawWithdrawStateException,
			wechatcontracts.CancelWithdrawWithdrawStateSucceed,
		),
		"account_withdraw_result.out_account_type": setOf(
			wechatcontracts.CancelWithdrawOutAccountTypeBasic,
			wechatcontracts.CancelWithdrawOutAccountTypeOperate,
			wechatcontracts.CancelWithdrawOutAccountTypeMargin,
			wechatcontracts.CancelWithdrawOutAccountTypeTradeFee,
		),
		"account_withdraw_result.pay_state": setOf(
			wechatcontracts.CancelWithdrawPayStateProcessing,
			wechatcontracts.CancelWithdrawPayStateSucceed,
			wechatcontracts.CancelWithdrawPayStateFail,
			wechatcontracts.CancelWithdrawPayStateBankRefunded,
		),
		"account_info.out_account_type": setOf(
			wechatcontracts.CancelWithdrawOutAccountTypeBasic,
			wechatcontracts.CancelWithdrawOutAccountTypeOperate,
			wechatcontracts.CancelWithdrawOutAccountTypeMargin,
			wechatcontracts.CancelWithdrawOutAccountTypeTradeFee,
		),
	}

	return map[string]*cancelWithdrawEndpointContract{
		endpointAuditKey("GET", "/v3/ecommerce/account/apply-cancel-withdraw/validate-cancel/{sub_mchid}"): {
			Method:         "GET",
			Path:           "/v3/ecommerce/account/apply-cancel-withdraw/validate-cancel/{sub_mchid}",
			RequestFields:  setOf("sub_mchid"),
			ResponseFields: validateResponseFields,
			ResponseConstraints: map[string]map[string]struct{}{
				"account_info.amount": fenConstraint,
			},
			ResponseEnums: validateResponseEnums,
			ErrorCodes:    cancelWithdrawCodeSetToMap(wechaterrorcodes.EcommerceCancelWithdrawValidateDocumentedCodes),
		},
		endpointAuditKey("POST", "/v3/ecommerce/account/apply-cancel-withdraw"): {
			Method:         "POST",
			Path:           "/v3/ecommerce/account/apply-cancel-withdraw",
			RequestFields:  createRequestFields,
			ResponseFields: createResponseFields,
			RequestEnums: map[string]map[string]struct{}{
				"withdraw": setOf(
					wechatcontracts.CancelWithdrawModeNotApply,
					wechatcontracts.CancelWithdrawModeApply,
				),
				"payee_info.account_type": setOf(
					wechatcontracts.CancelWithdrawAccountTypeCorporate,
					wechatcontracts.CancelWithdrawAccountTypePersonal,
				),
				"payee_info.identity_info.id_doc_type": setOf(
					wechatcontracts.CancelWithdrawIDDocTypeIDCard,
					wechatcontracts.CancelWithdrawIDDocTypeOverseaPassport,
					wechatcontracts.CancelWithdrawIDDocTypeHongkongPassport,
					wechatcontracts.CancelWithdrawIDDocTypeMacaoPassport,
					wechatcontracts.CancelWithdrawIDDocTypeTaiwanPassport,
					wechatcontracts.CancelWithdrawIDDocTypeForeignResident,
					wechatcontracts.CancelWithdrawIDDocTypeHongkongMacaoResident,
					wechatcontracts.CancelWithdrawIDDocTypeTaiwanResident,
				),
				"proof_medias.proof_media_type": setOf("WITHDRAWAL_APPLICATION"),
			},
			ErrorCodes: cancelWithdrawCodeSetToMap(wechaterrorcodes.EcommerceCancelWithdrawCreateDocumentedCodes),
		},
		endpointAuditKey("GET", "/v3/ecommerce/account/apply-cancel-withdraw/out-request-no/{out_request_no}"): {
			Method:         "GET",
			Path:           "/v3/ecommerce/account/apply-cancel-withdraw/out-request-no/{out_request_no}",
			RequestFields:  setOf("out_request_no"),
			ResponseFields: queryResponseFields,
			ResponseConstraints: map[string]map[string]struct{}{
				"modify_time":       rfc3339Constraint,
				"account_info.amount": fenConstraint,
			},
			ResponseEnums: queryResponseEnums,
			ErrorCodes:    cancelWithdrawCodeSetToMap(wechaterrorcodes.EcommerceCancelWithdrawQueryDocumentedCodes),
		},
		endpointAuditKey("GET", "/v3/ecommerce/account/apply-cancel-withdraw/applyment-id/{applyment_id}"): {
			Method:         "GET",
			Path:           "/v3/ecommerce/account/apply-cancel-withdraw/applyment-id/{applyment_id}",
			RequestFields:  setOf("applyment_id"),
			ResponseFields: queryResponseFields,
			ResponseConstraints: map[string]map[string]struct{}{
				"modify_time":       rfc3339Constraint,
				"account_info.amount": fenConstraint,
			},
			ResponseEnums: queryResponseEnums,
			ErrorCodes:    cancelWithdrawCodeSetToMap(wechaterrorcodes.EcommerceCancelWithdrawQueryDocumentedCodes),
		},
	}
}

func cancelWithdrawCodeSetToMap(set wechaterrorcodes.CancelWithdrawCodeSet) map[string]struct{} {
	result := make(map[string]struct{}, len(set))
	for code := range set {
		result[wechaterrorcodes.CanonicalCancelWithdrawCode(code)] = struct{}{}
	}
	return result
}