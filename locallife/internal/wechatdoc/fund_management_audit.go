package wechatdoc

import (
	"reflect"

	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	wechaterrorcodes "github.com/merrydance/locallife/wechat/errorcodes"
)

type fundManagementEndpointContract struct {
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

func AuditFundManagementAlignment(extraction *Extraction) *AlignmentAudit {
	docEndpoints := collectDocEndpoints(extraction, wechaterrorcodes.CanonicalFundManagementCode)
	contractInventory := fundManagementContractInventory()
	ignoredRequestEnums := fundManagementIgnoredRequestEnums()
	ignoredResponseEnums := fundManagementIgnoredResponseEnums()
	keys := sortedEndpointKeys(docEndpoints)
	report := &AlignmentAudit{Scope: "fund_management"}

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
		requestEnumDiff := diffFieldEnums(doc.RequestFields, contract.RequestEnums, ignoredRequestEnums)
		responseEnumDiff := diffFieldEnums(doc.ResponseFields, contract.ResponseEnums, ignoredResponseEnums)
		endpointAudit.MissingRequestEnums = requestEnumDiff.Missing
		endpointAudit.MissingResponseEnums = responseEnumDiff.Missing
		endpointAudit.MissingErrorCodes = diffSet(doc.ErrorCodes, contract.ErrorCodes)

		suppressedAudit := EndpointAlignmentAudit{Method: doc.Method, Path: doc.Path}
		suppressedAudit.MissingRequestEnums = requestEnumDiff.Suppressed
		suppressedAudit.MissingResponseEnums = responseEnumDiff.Suppressed
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

func fundManagementIgnoredRequestEnums() map[string]map[string]struct{} {
	return map[string]map[string]struct{}{
		"notify_url":  setOf("URL", "HTTPS", "https"),
		"create_time": setOf("RFC3339", "rfc3339", "yyyy-MM-DDTHH:mm:ss+TIMEZONE"),
		"update_time": setOf("RFC3339", "rfc3339", "yyyy-MM-DDTHH:mm:ss+TIMEZONE"),
	}
}

func fundManagementIgnoredResponseEnums() map[string]map[string]struct{} {
	return map[string]map[string]struct{}{
		"create_time": setOf("RFC3339", "rfc3339", "yyyy-MM-DDTHH:mm:ss+TIMEZONE"),
		"update_time": setOf("RFC3339", "rfc3339", "yyyy-MM-DDTHH:mm:ss+TIMEZONE"),
	}
}

func fundManagementContractInventory() map[string]*fundManagementEndpointContract {
	ecommerceBalanceRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.EcommerceFundBalanceQueryRequest{}))
	ecommerceBalanceDayEndRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.EcommerceFundDayEndBalanceQueryRequest{}))
	ecommerceBalanceResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.EcommerceFundBalanceResponse{}))
	platformBalanceRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.PlatformFundBalanceQueryRequest{}))
	platformBalanceDayEndRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.PlatformFundDayEndBalanceQueryRequest{}))
	platformBalanceResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.PlatformFundBalanceResponse{}))
	ecommerceWithdrawRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.EcommerceWithdrawRequest{}))
	ecommerceWithdrawCreateResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.EcommerceWithdrawCreateResponse{}))
	ecommerceWithdrawQueryByOutRequestNoFields := structJSONFields(reflect.TypeOf(wechatcontracts.EcommerceWithdrawQueryByOutRequestNoRequest{}))
	ecommerceWithdrawQueryByWithdrawIDFields := structJSONFields(reflect.TypeOf(wechatcontracts.EcommerceWithdrawQueryByWithdrawIDRequest{}))
	ecommerceWithdrawQueryResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.EcommerceWithdrawQueryResponse{}))
	platformWithdrawRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.PlatformWithdrawRequest{}))
	platformWithdrawCreateResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.PlatformWithdrawCreateResponse{}))
	platformWithdrawQueryByOutRequestNoFields := structJSONFields(reflect.TypeOf(wechatcontracts.PlatformWithdrawQueryByOutRequestNoRequest{}))
	platformWithdrawQueryByWithdrawIDFields := structJSONFields(reflect.TypeOf(wechatcontracts.PlatformWithdrawQueryByWithdrawIDRequest{}))
	platformWithdrawQueryResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.PlatformWithdrawQueryResponse{}))
	dayEndWithdrawRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.DayEndBalanceWithdrawRequest{}))
	dayEndWithdrawQueryRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.DayEndBalanceWithdrawQueryRequest{}))
	dayEndWithdrawResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.DayEndBalanceWithdrawResponse{}))
	withdrawBillRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.WithdrawBillRequest{}))
	withdrawBillResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.WithdrawBillResponse{}))
	withdrawNotifyRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.WithdrawNotificationResource{}))

	fenConstraint := setOf("unit_fen")
	rfc3339Constraint := setOf("format_rfc3339")
	dateConstraint := setOf("format_date_yyyy_mm_dd")
	balanceAccountValues := setOf(
		wechatcontracts.FundManagementAccountTypeBasic,
		wechatcontracts.FundManagementAccountTypeFees,
		wechatcontracts.FundManagementAccountTypeOperation,
		wechatcontracts.FundManagementAccountTypeDeposit,
	)
	dayEndBalanceAccountValues := setOf(
		wechatcontracts.FundManagementAccountTypeBasic,
		wechatcontracts.FundManagementAccountTypeDeposit,
	)
	platformBalanceAccountValues := setOf(
		wechatcontracts.FundManagementAccountTypeBasic,
		wechatcontracts.FundManagementAccountTypeFees,
		wechatcontracts.FundManagementAccountTypeOperation,
	)
	withdrawStatusValues := setOf(
		wechatcontracts.FundManagementWithdrawStatusCreateSuccess,
		wechatcontracts.FundManagementWithdrawStatusSuccess,
		wechatcontracts.FundManagementWithdrawStatusFail,
		wechatcontracts.FundManagementWithdrawStatusRefund,
		wechatcontracts.FundManagementWithdrawStatusClose,
		wechatcontracts.FundManagementWithdrawStatusInit,
	)
	dayEndWithdrawStatusValues := setOf(
		wechatcontracts.FundManagementDayEndWithdrawStatusCreated,
		wechatcontracts.FundManagementDayEndWithdrawStatusProcessing,
		wechatcontracts.FundManagementDayEndWithdrawStatusFinished,
		wechatcontracts.FundManagementDayEndWithdrawStatusAbnormal,
	)
	withdrawAccountValues := setOf(
		wechatcontracts.FundManagementAccountTypeBasic,
		wechatcontracts.FundManagementAccountTypeFees,
		wechatcontracts.FundManagementAccountTypeOperation,
	)
	calculateAmountTypeValues := setOf(
		wechatcontracts.FundManagementCalculateAmountTypeOnlyDayEndBalance,
		wechatcontracts.FundManagementCalculateAmountTypeAllowCurrentBalance,
	)
	billTypeValues := setOf(wechatcontracts.FundManagementBillTypeNoSucc)
	tarTypeValues := setOf(wechatcontracts.FundManagementTarTypeGzip)
	hashTypeValues := setOf(wechatcontracts.FundManagementHashTypeSHA1)
	notifyStatusValues := setOf(
		wechatcontracts.FundManagementWithdrawStatusCreateSuccess,
		wechatcontracts.FundManagementWithdrawStatusSuccess,
		wechatcontracts.FundManagementWithdrawStatusFail,
		wechatcontracts.FundManagementWithdrawStatusRefund,
		wechatcontracts.FundManagementWithdrawStatusClose,
		wechatcontracts.FundManagementWithdrawStatusInit,
		wechatcontracts.FundManagementDayEndWithdrawStatusCreated,
		wechatcontracts.FundManagementDayEndWithdrawStatusProcessing,
		wechatcontracts.FundManagementDayEndWithdrawStatusFinished,
		wechatcontracts.FundManagementDayEndWithdrawStatusAbnormal,
	)

	balanceResponseConstraints := map[string]map[string]struct{}{
		"available_amount": fenConstraint,
		"pending_amount":   fenConstraint,
	}
	ecommerceDayEndRequestConstraints := map[string]map[string]struct{}{
		"date": dateConstraint,
	}
	platformDayEndRequestConstraints := map[string]map[string]struct{}{
		"date": dateConstraint,
	}
	ecommerceWithdrawRequestConstraints := map[string]map[string]struct{}{
		"amount": fenConstraint,
	}
	queryWithdrawResponseConstraints := map[string]map[string]struct{}{
		"amount":      fenConstraint,
		"create_time": rfc3339Constraint,
		"update_time": rfc3339Constraint,
	}
	dayEndWithdrawRequestConstraints := map[string]map[string]struct{}{
		"reserve_amount": fenConstraint,
	}
	dayEndWithdrawResponseConstraints := map[string]map[string]struct{}{
		"total_amount":   fenConstraint,
		"success_amount": fenConstraint,
		"fail_amount":    fenConstraint,
		"refund_amount":  fenConstraint,
		"create_time":    rfc3339Constraint,
		"update_time":    rfc3339Constraint,
	}
	withdrawBillRequestConstraints := map[string]map[string]struct{}{
		"bill_date": dateConstraint,
	}
	withdrawNotifyRequestConstraints := map[string]map[string]struct{}{
		"amount":         fenConstraint,
		"total_amount":   fenConstraint,
		"success_amount": fenConstraint,
		"fail_amount":    fenConstraint,
		"refund_amount":  fenConstraint,
		"create_time":    rfc3339Constraint,
		"update_time":    rfc3339Constraint,
	}

	return map[string]*fundManagementEndpointContract{
		endpointAuditKey("GET", "/v3/ecommerce/fund/balance/{sub_mchid}"): {
			Method:              "GET",
			Path:                "/v3/ecommerce/fund/balance/{sub_mchid}",
			RequestFields:       ecommerceBalanceRequestFields,
			ResponseFields:      ecommerceBalanceResponseFields,
			ResponseConstraints: balanceResponseConstraints,
			RequestEnums: map[string]map[string]struct{}{
				"account_type": balanceAccountValues,
			},
			ResponseEnums: map[string]map[string]struct{}{
				"account_type": balanceAccountValues,
			},
			ErrorCodes: fundManagementCodeSetToMap(wechaterrorcodes.FundManagementBalanceDocumentedCodes),
		},
		endpointAuditKey("GET", "/v3/ecommerce/fund/enddaybalance/{sub_mchid}"): {
			Method:              "GET",
			Path:                "/v3/ecommerce/fund/enddaybalance/{sub_mchid}",
			RequestFields:       ecommerceBalanceDayEndRequestFields,
			ResponseFields:      ecommerceBalanceResponseFields,
			RequestConstraints:  ecommerceDayEndRequestConstraints,
			ResponseConstraints: balanceResponseConstraints,
			RequestEnums: map[string]map[string]struct{}{
				"account_type": dayEndBalanceAccountValues,
			},
			ResponseEnums: map[string]map[string]struct{}{
				"account_type": dayEndBalanceAccountValues,
			},
			ErrorCodes: fundManagementCodeSetToMap(wechaterrorcodes.FundManagementBalanceDocumentedCodes),
		},
		endpointAuditKey("GET", "/v3/merchant/fund/balance/{account_type}"): {
			Method:              "GET",
			Path:                "/v3/merchant/fund/balance/{account_type}",
			RequestFields:       platformBalanceRequestFields,
			ResponseFields:      platformBalanceResponseFields,
			ResponseConstraints: balanceResponseConstraints,
			RequestEnums: map[string]map[string]struct{}{
				"account_type": platformBalanceAccountValues,
			},
			ErrorCodes: fundManagementCodeSetToMap(wechaterrorcodes.FundManagementBalanceDocumentedCodes),
		},
		endpointAuditKey("GET", "/v3/merchant/fund/dayendbalance/{account_type}"): {
			Method:              "GET",
			Path:                "/v3/merchant/fund/dayendbalance/{account_type}",
			RequestFields:       platformBalanceDayEndRequestFields,
			ResponseFields:      platformBalanceResponseFields,
			RequestConstraints:  platformDayEndRequestConstraints,
			ResponseConstraints: balanceResponseConstraints,
			RequestEnums: map[string]map[string]struct{}{
				"account_type": platformBalanceAccountValues,
			},
			ErrorCodes: fundManagementCodeSetToMap(wechaterrorcodes.FundManagementBalanceDocumentedCodes),
		},
		endpointAuditKey("POST", "/v3/ecommerce/fund/withdraw"): {
			Method:             "POST",
			Path:               "/v3/ecommerce/fund/withdraw",
			RequestFields:      ecommerceWithdrawRequestFields,
			ResponseFields:     ecommerceWithdrawCreateResponseFields,
			RequestConstraints: ecommerceWithdrawRequestConstraints,
			RequestEnums: map[string]map[string]struct{}{
				"account_type": withdrawAccountValues,
			},
			ErrorCodes: fundManagementCodeSetToMap(wechaterrorcodes.FundManagementWithdrawDocumentedCodes),
		},
		endpointAuditKey("GET", "/v3/ecommerce/fund/withdraw/out-request-no/{out_request_no}"): {
			Method:              "GET",
			Path:                "/v3/ecommerce/fund/withdraw/out-request-no/{out_request_no}",
			RequestFields:       ecommerceWithdrawQueryByOutRequestNoFields,
			ResponseFields:      ecommerceWithdrawQueryResponseFields,
			ResponseConstraints: queryWithdrawResponseConstraints,
			ResponseEnums: map[string]map[string]struct{}{
				"status":       withdrawStatusValues,
				"account_type": withdrawAccountValues,
			},
			ErrorCodes: fundManagementCodeSetToMap(wechaterrorcodes.FundManagementWithdrawDocumentedCodes),
		},
		endpointAuditKey("GET", "/v3/ecommerce/fund/withdraw/{withdraw_id}"): {
			Method:              "GET",
			Path:                "/v3/ecommerce/fund/withdraw/{withdraw_id}",
			RequestFields:       ecommerceWithdrawQueryByWithdrawIDFields,
			ResponseFields:      ecommerceWithdrawQueryResponseFields,
			ResponseConstraints: queryWithdrawResponseConstraints,
			ResponseEnums: map[string]map[string]struct{}{
				"status":       withdrawStatusValues,
				"account_type": withdrawAccountValues,
			},
			ErrorCodes: fundManagementCodeSetToMap(wechaterrorcodes.FundManagementWithdrawDocumentedCodes),
		},
		endpointAuditKey("POST", "/v3/merchant/fund/withdraw"): {
			Method:             "POST",
			Path:               "/v3/merchant/fund/withdraw",
			RequestFields:      platformWithdrawRequestFields,
			ResponseFields:     platformWithdrawCreateResponseFields,
			RequestConstraints: ecommerceWithdrawRequestConstraints,
			RequestEnums: map[string]map[string]struct{}{
				"account_type": withdrawAccountValues,
			},
			ErrorCodes: fundManagementCodeSetToMap(wechaterrorcodes.FundManagementWithdrawDocumentedCodes),
		},
		endpointAuditKey("GET", "/v3/merchant/fund/withdraw/out-request-no/{out_request_no}"): {
			Method:              "GET",
			Path:                "/v3/merchant/fund/withdraw/out-request-no/{out_request_no}",
			RequestFields:       platformWithdrawQueryByOutRequestNoFields,
			ResponseFields:      platformWithdrawQueryResponseFields,
			ResponseConstraints: queryWithdrawResponseConstraints,
			ResponseEnums: map[string]map[string]struct{}{
				"status":       withdrawStatusValues,
				"account_type": withdrawAccountValues,
			},
			ErrorCodes: fundManagementCodeSetToMap(wechaterrorcodes.FundManagementWithdrawDocumentedCodes),
		},
		endpointAuditKey("GET", "/v3/merchant/fund/withdraw/withdraw-id/{withdraw_id}"): {
			Method:              "GET",
			Path:                "/v3/merchant/fund/withdraw/withdraw-id/{withdraw_id}",
			RequestFields:       platformWithdrawQueryByWithdrawIDFields,
			ResponseFields:      platformWithdrawQueryResponseFields,
			ResponseConstraints: queryWithdrawResponseConstraints,
			ResponseEnums: map[string]map[string]struct{}{
				"status":       withdrawStatusValues,
				"account_type": withdrawAccountValues,
			},
			ErrorCodes: fundManagementCodeSetToMap(wechaterrorcodes.FundManagementWithdrawDocumentedCodes),
		},
		endpointAuditKey("POST", "/v3/platsolution/ecommerce/withdraw/day-end-balance-withdraw"): {
			Method:              "POST",
			Path:                "/v3/platsolution/ecommerce/withdraw/day-end-balance-withdraw",
			RequestFields:       dayEndWithdrawRequestFields,
			ResponseFields:      dayEndWithdrawResponseFields,
			RequestConstraints:  dayEndWithdrawRequestConstraints,
			ResponseConstraints: dayEndWithdrawResponseConstraints,
			RequestEnums: map[string]map[string]struct{}{
				"calculate_amount_type": calculateAmountTypeValues,
			},
			ResponseEnums: map[string]map[string]struct{}{
				"status":       dayEndWithdrawStatusValues,
				"account_type": withdrawAccountValues,
			},
			ErrorCodes: fundManagementCodeSetToMap(wechaterrorcodes.FundManagementWithdrawDocumentedCodes),
		},
		endpointAuditKey("GET", "/v3/platsolution/ecommerce/withdraw/day-end-balance-withdraw/out-request-no/{out_request_no}"): {
			Method:              "GET",
			Path:                "/v3/platsolution/ecommerce/withdraw/day-end-balance-withdraw/out-request-no/{out_request_no}",
			RequestFields:       dayEndWithdrawQueryRequestFields,
			ResponseFields:      dayEndWithdrawResponseFields,
			ResponseConstraints: dayEndWithdrawResponseConstraints,
			ResponseEnums: map[string]map[string]struct{}{
				"status":       dayEndWithdrawStatusValues,
				"account_type": withdrawAccountValues,
			},
			ErrorCodes: fundManagementCodeSetToMap(wechaterrorcodes.FundManagementWithdrawDocumentedCodes),
		},
		endpointAuditKey("GET", "/v3/merchant/fund/withdraw/bill-type/{bill_type}"): {
			Method:             "GET",
			Path:               "/v3/merchant/fund/withdraw/bill-type/{bill_type}",
			RequestFields:      withdrawBillRequestFields,
			ResponseFields:     withdrawBillResponseFields,
			RequestConstraints: withdrawBillRequestConstraints,
			RequestEnums: map[string]map[string]struct{}{
				"bill_type": billTypeValues,
				"tar_type":  tarTypeValues,
			},
			ResponseEnums: map[string]map[string]struct{}{
				"hash_type": hashTypeValues,
			},
			ErrorCodes: fundManagementCodeSetToMap(wechaterrorcodes.FundManagementWithdrawBillDocumentedCodes),
		},
		endpointAuditKey("POST", "/v1/webhooks/wechat-ecommerce/withdraw-notify"): {
			Method:             "POST",
			Path:               "/v1/webhooks/wechat-ecommerce/withdraw-notify",
			RequestFields:      withdrawNotifyRequestFields,
			ResponseFields:     map[string]struct{}{},
			RequestConstraints: withdrawNotifyRequestConstraints,
			RequestEnums: map[string]map[string]struct{}{
				"status":       notifyStatusValues,
				"account_type": withdrawAccountValues,
			},
		},
	}
}

func fundManagementCodeSetToMap(set wechaterrorcodes.FundManagementCodeSet) map[string]struct{} {
	result := make(map[string]struct{}, len(set))
	for code := range set {
		result[wechaterrorcodes.CanonicalFundManagementCode(code)] = struct{}{}
	}
	return result
}
