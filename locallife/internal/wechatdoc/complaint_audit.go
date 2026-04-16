package wechatdoc

import (
	"reflect"

	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	wechaterrorcodes "github.com/merrydance/locallife/wechat/errorcodes"
)

type complaintEndpointContract struct {
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

func AuditComplaintAlignment(extraction *Extraction) *AlignmentAudit {
	docEndpoints := collectDocEndpoints(extraction, wechaterrorcodes.CanonicalComplaintCode)
	contractInventory := complaintContractInventory()
	keys := sortedEndpointKeys(docEndpoints)
	report := &AlignmentAudit{Scope: "complaint"}

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

func complaintContractInventory() map[string]*complaintEndpointContract {
	listRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.EcommerceComplaintListRequest{}))
	listResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.EcommerceComplaintListResponse{}))
	detailRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.EcommerceComplaintDetailRequest{}))
	detailResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.EcommerceComplaintInfo{}))
	historyRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.ComplaintNegotiationHistoryRequest{}))
	historyResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.ComplaintNegotiationHistoryResponse{}))
	webhookRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.ComplaintNotificationResource{}))
	configRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.ComplaintNotificationConfigRequest{}))
	configResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.ComplaintNotificationConfig{}))
	responseRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.ComplaintResponseRequest{}))
	completeRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.ComplaintCompleteRequest{}))
	refundProgressRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.ComplaintRefundProgressUpdateRequest{}))
	immediateReplyRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.ComplaintImmediateServiceReplyRequest{}))
	immediateReplyResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.ComplaintImmediateServiceReplyResponse{}))
	imageUploadRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.ComplaintImageUploadRequest{}))
	imageUploadResponseFields := structJSONFields(reflect.TypeOf(wechatcontracts.ComplaintImageUploadResponse{}))
	imageQueryRequestFields := structJSONFields(reflect.TypeOf(wechatcontracts.ComplaintImageQueryRequest{}))

	noFields := map[string]struct{}{}
	fenConstraint := setOf("unit_fen")
	rfc3339Constraint := setOf("format_rfc3339")
	dateConstraint := setOf("format_date_yyyy_mm_dd")

	complaintStateValues := setOf(
		wechatcontracts.ComplaintStatePending,
		wechatcontracts.ComplaintStateProcessing,
		wechatcontracts.ComplaintStateProcessed,
	)
	problemTypeValues := setOf(
		wechatcontracts.ComplaintProblemTypeRefund,
		wechatcontracts.ComplaintProblemTypeServiceNotWork,
		wechatcontracts.ComplaintProblemTypeOthers,
	)
	userTagValues := setOf(
		wechatcontracts.ComplaintUserTagTrusted,
		wechatcontracts.ComplaintUserTagHighRisk,
	)
	serviceOrderStateValues := setOf(
		wechatcontracts.ComplaintServiceOrderStateDoing,
		wechatcontracts.ComplaintServiceOrderStateRevoked,
		wechatcontracts.ComplaintServiceOrderStateWaitPay,
		wechatcontracts.ComplaintServiceOrderStateDone,
	)
	additionalTypeValues := setOf(wechatcontracts.ComplaintAdditionalTypeSharePower)
	mediaTypeValues := setOf(
		wechatcontracts.ComplaintMediaTypeUserComplaintImage,
		wechatcontracts.ComplaintMediaTypeOperationImage,
	)
	negotiationOperateTypeValues := setOf(
		wechatcontracts.ComplaintNegotiationOperateTypeUserCreateComplaint,
		wechatcontracts.ComplaintNegotiationOperateTypeUserContinueComplaint,
		wechatcontracts.ComplaintNegotiationOperateTypeUserResponse,
		wechatcontracts.ComplaintNegotiationOperateTypePlatformResponse,
		wechatcontracts.ComplaintNegotiationOperateTypeMerchantResponse,
		wechatcontracts.ComplaintNegotiationOperateTypeMerchantConfirmComplete,
		wechatcontracts.ComplaintNegotiationOperateTypeUserCreateComplaintSystemMessage,
		wechatcontracts.ComplaintNegotiationOperateTypeComplaintFullRefundedSystemMessage,
		wechatcontracts.ComplaintNegotiationOperateTypeUserContinueComplaintSystemMessage,
		wechatcontracts.ComplaintNegotiationOperateTypeUserRevokeComplaint,
		wechatcontracts.ComplaintNegotiationOperateTypeUserComfirmComplaint,
		wechatcontracts.ComplaintNegotiationOperateTypePlatformHelpApplication,
		wechatcontracts.ComplaintNegotiationOperateTypeUserApplyPlatformHelp,
		wechatcontracts.ComplaintNegotiationOperateTypeMerchantApproveRefund,
		wechatcontracts.ComplaintNegotiationOperateTypeMerchantRefuseRerund,
		wechatcontracts.ComplaintNegotiationOperateTypeUserSubmitSatisfaction,
		wechatcontracts.ComplaintNegotiationOperateTypeServiceOrderCancel,
		wechatcontracts.ComplaintNegotiationOperateTypeServiceOrderComplete,
		wechatcontracts.ComplaintNegotiationOperateTypeComplaintPartialRefundedSystemMessage,
		wechatcontracts.ComplaintNegotiationOperateTypeComplaintRefundReceivedSystemMessage,
		wechatcontracts.ComplaintNegotiationOperateTypeComplaintEntrustedRefundSystemMessage,
		wechatcontracts.ComplaintNegotiationOperateTypeUserApplyPlatformService,
		wechatcontracts.ComplaintNegotiationOperateTypeUserCancelPlatformService,
		wechatcontracts.ComplaintNegotiationOperateTypePlatformServiceFinished,
		wechatcontracts.ComplaintNegotiationOperateTypeUserClickResponse,
	)
	notificationActionValues := setOf(
		wechatcontracts.ComplaintNotificationActionTypeCreateComplaint,
		wechatcontracts.ComplaintNotificationActionTypeContinueComplaint,
		wechatcontracts.ComplaintNotificationActionTypeUserResponse,
		wechatcontracts.ComplaintNotificationActionTypeResponseByPlatform,
		wechatcontracts.ComplaintNotificationActionTypeSellerRefund,
		wechatcontracts.ComplaintNotificationActionTypeMerchantResponse,
		wechatcontracts.ComplaintNotificationActionTypeMerchantConfirmComplete,
		wechatcontracts.ComplaintNotificationActionTypeUserApplyPlatformService,
		wechatcontracts.ComplaintNotificationActionTypeUserCancelPlatformService,
		wechatcontracts.ComplaintNotificationActionTypePlatformServiceFinished,
		wechatcontracts.ComplaintNotificationActionTypeMerchantApproveRefund,
		wechatcontracts.ComplaintNotificationActionTypeMerchantRejectRefund,
		wechatcontracts.ComplaintNotificationActionTypeRefundSuccess,
	)
	refundProgressActionValues := setOf(
		wechatcontracts.ComplaintRefundProgressActionReject,
		wechatcontracts.ComplaintRefundProgressActionApprove,
	)
	messageBlockTypeValues := setOf(
		wechatcontracts.ComplaintMessageBlockTypeText,
		wechatcontracts.ComplaintMessageBlockTypeImage,
		wechatcontracts.ComplaintMessageBlockTypeLink,
		wechatcontracts.ComplaintMessageBlockTypeFAQList,
		wechatcontracts.ComplaintMessageBlockTypeButton,
		wechatcontracts.ComplaintMessageBlockTypeButtonGroup,
	)
	messageTextColorValues := setOf(
		wechatcontracts.ComplaintMessageTextColorDefault,
		wechatcontracts.ComplaintMessageTextColorSecondary,
	)
	messageImageStyleValues := setOf(
		wechatcontracts.ComplaintMessageImageStyleTypeNarrow,
		wechatcontracts.ComplaintMessageImageStyleTypeWide,
	)
	messageActionValues := setOf(
		wechatcontracts.ComplaintMessageActionTypeSendMessage,
		wechatcontracts.ComplaintMessageActionTypeJumpURL,
		wechatcontracts.ComplaintMessageActionTypeJumpMiniApp,
	)
	messageButtonLayoutValues := setOf(
		wechatcontracts.ComplaintMessageButtonLayoutUnknown,
		wechatcontracts.ComplaintMessageButtonLayoutHorizontal,
		wechatcontracts.ComplaintMessageButtonLayoutVertical,
	)
	messageSenderIdentityValues := setOf(
		wechatcontracts.ComplaintMessageSenderIdentityUnknown,
		wechatcontracts.ComplaintMessageSenderIdentityManual,
		wechatcontracts.ComplaintMessageSenderIdentityMachine,
	)

	listRequestConstraints := map[string]map[string]struct{}{
		"begin_date": dateConstraint,
		"end_date":   dateConstraint,
	}
	listResponseConstraints := map[string]map[string]struct{}{
		"data.complaint_time":                               rfc3339Constraint,
		"data.complaint_order_info.amount":                  fenConstraint,
		"data.apply_refund_amount":                          fenConstraint,
		"data.additional_info.share_power_info.return_time": rfc3339Constraint,
	}
	detailResponseConstraints := map[string]map[string]struct{}{
		"complaint_time":                               rfc3339Constraint,
		"complaint_order_info.amount":                  fenConstraint,
		"apply_refund_amount":                          fenConstraint,
		"additional_info.share_power_info.return_time": rfc3339Constraint,
	}
	historyResponseConstraints := mergeConstraintAudits(
		map[string]map[string]struct{}{
			"data.operate_time": rfc3339Constraint,
		},
		complaintMessageConstraintInventory("data.normal_message", rfc3339Constraint),
	)
	immediateReplyRequestConstraints := complaintMessageConstraintInventory("message", rfc3339Constraint)

	listResponseEnums := map[string]map[string]struct{}{
		"data.complaint_state":                 complaintStateValues,
		"data.complaint_media_list.media_type": mediaTypeValues,
		"data.problem_type":                    problemTypeValues,
		"data.user_tag_list":                   userTagValues,
		"data.service_order_info.state":        serviceOrderStateValues,
		"data.additional_info.type":            additionalTypeValues,
	}
	detailResponseEnums := map[string]map[string]struct{}{
		"complaint_state":                 complaintStateValues,
		"complaint_media_list.media_type": mediaTypeValues,
		"problem_type":                    problemTypeValues,
		"user_tag_list":                   userTagValues,
		"service_order_info.state":        serviceOrderStateValues,
		"additional_info.type":            additionalTypeValues,
	}
	historyResponseEnums := mergeEnumAudits(
		map[string]map[string]struct{}{
			"data.operate_type":                    negotiationOperateTypeValues,
			"data.complaint_media_list.media_type": mediaTypeValues,
		},
		complaintMessageEnumInventory("data.normal_message", messageBlockTypeValues, messageTextColorValues, messageImageStyleValues, messageActionValues, messageButtonLayoutValues, messageSenderIdentityValues),
	)
	webhookRequestEnums := map[string]map[string]struct{}{
		"action_type": notificationActionValues,
	}
	refundProgressRequestEnums := map[string]map[string]struct{}{
		"action": refundProgressActionValues,
	}
	immediateReplyRequestEnums := complaintMessageEnumInventory("message", messageBlockTypeValues, messageTextColorValues, messageImageStyleValues, messageActionValues, messageButtonLayoutValues, messageSenderIdentityValues)

	return map[string]*complaintEndpointContract{
		endpointAuditKey("GET", "/v3/merchant-service/complaints-v2"): {
			Method:              "GET",
			Path:                "/v3/merchant-service/complaints-v2",
			RequestFields:       listRequestFields,
			ResponseFields:      listResponseFields,
			RequestConstraints:  listRequestConstraints,
			ResponseConstraints: listResponseConstraints,
			ResponseEnums:       listResponseEnums,
			ErrorCodes:          complaintCodeSetToMap(wechaterrorcodes.ComplaintQueryDocumentedCodes),
		},
		endpointAuditKey("GET", "/v3/merchant-service/complaints-v2/{complaint_id}"): {
			Method:              "GET",
			Path:                "/v3/merchant-service/complaints-v2/{complaint_id}",
			RequestFields:       detailRequestFields,
			ResponseFields:      detailResponseFields,
			ResponseConstraints: detailResponseConstraints,
			ResponseEnums:       detailResponseEnums,
			ErrorCodes:          complaintCodeSetToMap(wechaterrorcodes.ComplaintQueryDocumentedCodes),
		},
		endpointAuditKey("GET", "/v3/merchant-service/complaints-v2/{complaint_id}/negotiation-historys"): {
			Method:              "GET",
			Path:                "/v3/merchant-service/complaints-v2/{complaint_id}/negotiation-historys",
			RequestFields:       historyRequestFields,
			ResponseFields:      historyResponseFields,
			ResponseConstraints: historyResponseConstraints,
			ResponseEnums:       historyResponseEnums,
			ErrorCodes:          complaintCodeSetToMap(wechaterrorcodes.ComplaintQueryDocumentedCodes),
		},
		endpointAuditKey("POST", "/v1/webhooks/wechat-ecommerce/complaint-notify"): {
			Method:         "POST",
			Path:           "/v1/webhooks/wechat-ecommerce/complaint-notify",
			RequestFields:  webhookRequestFields,
			ResponseFields: noFields,
			RequestEnums:   webhookRequestEnums,
		},
		endpointAuditKey("POST", "/v3/merchant-service/complaint-notifications"): {
			Method:         "POST",
			Path:           "/v3/merchant-service/complaint-notifications",
			RequestFields:  configRequestFields,
			ResponseFields: configResponseFields,
			ErrorCodes:     complaintCodeSetToMap(wechaterrorcodes.ComplaintNotificationConfigDocumentedCodes),
		},
		endpointAuditKey("GET", "/v3/merchant-service/complaint-notifications"): {
			Method:         "GET",
			Path:           "/v3/merchant-service/complaint-notifications",
			RequestFields:  noFields,
			ResponseFields: configResponseFields,
			ErrorCodes:     complaintCodeSetToMap(wechaterrorcodes.ComplaintNotificationConfigDocumentedCodes),
		},
		endpointAuditKey("PUT", "/v3/merchant-service/complaint-notifications"): {
			Method:         "PUT",
			Path:           "/v3/merchant-service/complaint-notifications",
			RequestFields:  configRequestFields,
			ResponseFields: configResponseFields,
			ErrorCodes:     complaintCodeSetToMap(wechaterrorcodes.ComplaintNotificationConfigDocumentedCodes),
		},
		endpointAuditKey("DELETE", "/v3/merchant-service/complaint-notifications"): {
			Method:         "DELETE",
			Path:           "/v3/merchant-service/complaint-notifications",
			RequestFields:  noFields,
			ResponseFields: noFields,
			ErrorCodes:     complaintCodeSetToMap(wechaterrorcodes.ComplaintNotificationConfigDocumentedCodes),
		},
		endpointAuditKey("POST", "/v3/merchant-service/complaints-v2/{complaint_id}/response"): {
			Method:         "POST",
			Path:           "/v3/merchant-service/complaints-v2/{complaint_id}/response",
			RequestFields:  responseRequestFields,
			ResponseFields: noFields,
			ErrorCodes:     complaintCodeSetToMap(wechaterrorcodes.ComplaintHandlingDocumentedCodes),
		},
		endpointAuditKey("POST", "/v3/merchant-service/complaints-v2/{complaint_id}/complete"): {
			Method:         "POST",
			Path:           "/v3/merchant-service/complaints-v2/{complaint_id}/complete",
			RequestFields:  completeRequestFields,
			ResponseFields: noFields,
			ErrorCodes:     complaintCodeSetToMap(wechaterrorcodes.ComplaintHandlingDocumentedCodes),
		},
		endpointAuditKey("POST", "/v3/merchant-service/complaints-v2/{complaint_id}/update-refund-progress"): {
			Method:         "POST",
			Path:           "/v3/merchant-service/complaints-v2/{complaint_id}/update-refund-progress",
			RequestFields:  refundProgressRequestFields,
			ResponseFields: noFields,
			RequestEnums:   refundProgressRequestEnums,
			ErrorCodes:     complaintCodeSetToMap(wechaterrorcodes.ComplaintHandlingDocumentedCodes),
		},
		endpointAuditKey("POST", "/v3/merchant-service/complaints-v2/{complaint_id}/response-immediate-service"): {
			Method:             "POST",
			Path:               "/v3/merchant-service/complaints-v2/{complaint_id}/response-immediate-service",
			RequestFields:      immediateReplyRequestFields,
			ResponseFields:     immediateReplyResponseFields,
			RequestConstraints: immediateReplyRequestConstraints,
			RequestEnums:       immediateReplyRequestEnums,
			ErrorCodes:         complaintCodeSetToMap(wechaterrorcodes.ComplaintHandlingDocumentedCodes),
		},
		endpointAuditKey("POST", "/v3/merchant-service/images/upload"): {
			Method:         "POST",
			Path:           "/v3/merchant-service/images/upload",
			RequestFields:  imageUploadRequestFields,
			ResponseFields: imageUploadResponseFields,
			ErrorCodes:     complaintCodeSetToMap(wechaterrorcodes.ComplaintImageDocumentedCodes),
		},
		endpointAuditKey("GET", "/v3/merchant-service/images/{media_id}"): {
			Method:         "GET",
			Path:           "/v3/merchant-service/images/{media_id}",
			RequestFields:  imageQueryRequestFields,
			ResponseFields: noFields,
			ErrorCodes:     complaintCodeSetToMap(wechaterrorcodes.ComplaintImageDocumentedCodes),
		},
	}
}

func complaintMessageEnumInventory(prefix string, blockTypes map[string]struct{}, textColors map[string]struct{}, imageStyles map[string]struct{}, actionTypes map[string]struct{}, buttonLayouts map[string]struct{}, senderIdentities map[string]struct{}) map[string]map[string]struct{} {
	return map[string]map[string]struct{}{
		prefix + ".blocks.type":                                    blockTypes,
		prefix + ".blocks.text.color":                              textColors,
		prefix + ".blocks.image.image_style_type":                  imageStyles,
		prefix + ".blocks.link.action.action_type":                 actionTypes,
		prefix + ".blocks.faq_list.faqs.action.action_type":        actionTypes,
		prefix + ".blocks.button.action.action_type":               actionTypes,
		prefix + ".blocks.button_group.buttons.action.action_type": actionTypes,
		prefix + ".blocks.button_group.button_layout":              buttonLayouts,
		prefix + ".sender_identity":                                senderIdentities,
	}
}

func complaintMessageConstraintInventory(prefix string, rfc3339Constraint map[string]struct{}) map[string]map[string]struct{} {
	return map[string]map[string]struct{}{
		prefix + ".blocks.link.invalid_info.expired_time":         rfc3339Constraint,
		prefix + ".blocks.button.invalid_info.expired_time":       rfc3339Constraint,
		prefix + ".blocks.button_group.invalid_info.expired_time": rfc3339Constraint,
	}
}

func mergeConstraintAudits(parts ...map[string]map[string]struct{}) map[string]map[string]struct{} {
	merged := make(map[string]map[string]struct{})
	for _, part := range parts {
		for field, constraints := range part {
			merged[field] = constraints
		}
	}
	return merged
}

func mergeEnumAudits(parts ...map[string]map[string]struct{}) map[string]map[string]struct{} {
	merged := make(map[string]map[string]struct{})
	for _, part := range parts {
		for field, values := range part {
			merged[field] = values
		}
	}
	return merged
}

func complaintCodeSetToMap(set wechaterrorcodes.ComplaintCodeSet) map[string]struct{} {
	result := make(map[string]struct{}, len(set))
	for code := range set {
		result[wechaterrorcodes.CanonicalComplaintCode(code)] = struct{}{}
	}
	return result
}
