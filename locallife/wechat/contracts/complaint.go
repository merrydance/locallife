package contracts

// 官方文档：消费者投诉 2.0
// 查询投诉单列表：https://pay.weixin.qq.com/doc/v3/partner/4012691285.md
// 查询投诉单详情：https://pay.weixin.qq.com/doc/v3/partner/4012691648.md
// 查询投诉单协商历史：https://pay.weixin.qq.com/doc/v3/partner/4012691802.md
// 投诉通知回调：https://pay.weixin.qq.com/doc/v3/partner/4012076174.md
// 创建投诉通知回调地址：https://pay.weixin.qq.com/doc/v3/partner/4012458106.md
// 查询投诉通知回调地址：https://pay.weixin.qq.com/doc/v3/partner/4012459065.md
// 更新投诉通知回调地址：https://pay.weixin.qq.com/doc/v3/partner/4012459287.md
// 删除投诉通知回调地址：https://pay.weixin.qq.com/doc/v3/partner/4012460474.md
// 回复用户：https://pay.weixin.qq.com/doc/v3/partner/4012467213.md
// 反馈处理完成：https://pay.weixin.qq.com/doc/v3/partner/4012467217.md
// 更新退款审批结果：https://pay.weixin.qq.com/doc/v3/partner/4012467218.md
// 回复需要即时服务的投诉单：https://pay.weixin.qq.com/doc/v3/partner/4017151726.md
// 图片上传接口：https://pay.weixin.qq.com/doc/v3/partner/4012467222.md
// 图片请求接口：https://pay.weixin.qq.com/doc/v3/partner/4012467223.md

const (
	ComplaintStatePending    = "PENDING"
	ComplaintStateProcessing = "PROCESSING"
	ComplaintStateProcessed  = "PROCESSED"
)

const (
	ComplaintProblemTypeRefund         = "REFUND"
	ComplaintProblemTypeServiceNotWork = "SERVICE_NOT_WORK"
	ComplaintProblemTypeOthers         = "OTHERS"
)

const (
	ComplaintUserTagTrusted  = "TRUSTED"
	ComplaintUserTagHighRisk = "HIGH_RISK"
)

const (
	ComplaintServiceOrderStateDoing   = "DOING"
	ComplaintServiceOrderStateRevoked = "REVOKED"
	ComplaintServiceOrderStateWaitPay = "WAITPAY"
	ComplaintServiceOrderStateDone    = "DONE"
)

const (
	ComplaintAdditionalTypeSharePower = "SHARE_POWER_TYPE"
)

const (
	ComplaintMediaTypeUserComplaintImage = "USER_COMPLAINT_IMAGE"
	ComplaintMediaTypeOperationImage     = "OPERATION_IMAGE"
)

const (
	ComplaintNegotiationOperateTypeUserCreateComplaint                   = "USER_CREATE_COMPLAINT"
	ComplaintNegotiationOperateTypeUserContinueComplaint                 = "USER_CONTINUE_COMPLAINT"
	ComplaintNegotiationOperateTypeUserResponse                          = "USER_RESPONSE"
	ComplaintNegotiationOperateTypePlatformResponse                      = "PLATFORM_RESPONSE"
	ComplaintNegotiationOperateTypeMerchantResponse                      = "MERCHANT_RESPONSE"
	ComplaintNegotiationOperateTypeMerchantConfirmComplete               = "MERCHANT_CONFIRM_COMPLETE"
	ComplaintNegotiationOperateTypeUserCreateComplaintSystemMessage      = "USER_CREATE_COMPLAINT_SYSTEM_MESSAGE"
	ComplaintNegotiationOperateTypeComplaintFullRefundedSystemMessage    = "COMPLAINT_FULL_REFUNDED_SYSTEM_MESSAGE"
	ComplaintNegotiationOperateTypeUserContinueComplaintSystemMessage    = "USER_CONTINUE_COMPLAINT_SYSTEM_MESSAGE"
	ComplaintNegotiationOperateTypeUserRevokeComplaint                   = "USER_REVOKE_COMPLAINT"
	ComplaintNegotiationOperateTypeUserComfirmComplaint                  = "USER_COMFIRM_COMPLAINT"
	ComplaintNegotiationOperateTypePlatformHelpApplication               = "PLATFORM_HELP_APPLICATION"
	ComplaintNegotiationOperateTypeUserApplyPlatformHelp                 = "USER_APPLY_PLATFORM_HELP"
	ComplaintNegotiationOperateTypeMerchantApproveRefund                 = "MERCHANT_APPROVE_REFUND"
	ComplaintNegotiationOperateTypeMerchantRefuseRerund                  = "MERCHANT_REFUSE_RERUND"
	ComplaintNegotiationOperateTypeUserSubmitSatisfaction                = "USER_SUBMIT_SATISFACTION"
	ComplaintNegotiationOperateTypeServiceOrderCancel                    = "SERVICE_ORDER_CANCEL"
	ComplaintNegotiationOperateTypeServiceOrderComplete                  = "SERVICE_ORDER_COMPLETE"
	ComplaintNegotiationOperateTypeComplaintPartialRefundedSystemMessage = "COMPLAINT_PARTIAL_REFUNDED_SYSTEM_MESSAGE"
	ComplaintNegotiationOperateTypeComplaintRefundReceivedSystemMessage  = "COMPLAINT_REFUND_RECEIVED_SYSTEM_MESSAGE"
	ComplaintNegotiationOperateTypeComplaintEntrustedRefundSystemMessage = "COMPLAINT_ENTRUSTED_REFUND_SYSTEM_MESSAGE"
	ComplaintNegotiationOperateTypeUserApplyPlatformService              = "USER_APPLY_PLATFORM_SERVICE"
	ComplaintNegotiationOperateTypeUserCancelPlatformService             = "USER_CANCEL_PLATFORM_SERVICE"
	ComplaintNegotiationOperateTypePlatformServiceFinished               = "PLATFORM_SERVICE_FINISHED"
	ComplaintNegotiationOperateTypeUserClickResponse                     = "USER_CLICK_RESPONSE"
)

const (
	ComplaintNotificationEventTypeCreate      = "COMPLAINT.CREATE"
	ComplaintNotificationEventTypeStateChange = "COMPLAINT.STATE_CHANGE"
)

const (
	ComplaintNotificationResourceType = "encrypt-resource"
	ComplaintNotificationOriginalType = "complaint"
)

const (
	ComplaintNotificationActionTypeCreateComplaint           = "CREATE_COMPLAINT"
	ComplaintNotificationActionTypeContinueComplaint         = "CONTINUE_COMPLAINT"
	ComplaintNotificationActionTypeUserResponse              = "USER_RESPONSE"
	ComplaintNotificationActionTypeResponseByPlatform        = "RESPONSE_BY_PLATFORM"
	ComplaintNotificationActionTypeSellerRefund              = "SELLER_REFUND"
	ComplaintNotificationActionTypeMerchantResponse          = "MERCHANT_RESPONSE"
	ComplaintNotificationActionTypeMerchantConfirmComplete   = "MERCHANT_CONFIRM_COMPLETE"
	ComplaintNotificationActionTypeUserApplyPlatformService  = "USER_APPLY_PLATFORM_SERVICE"
	ComplaintNotificationActionTypeUserCancelPlatformService = "USER_CANCEL_PLATFORM_SERVICE"
	ComplaintNotificationActionTypePlatformServiceFinished   = "PLATFORM_SERVICE_FINISHED"
	ComplaintNotificationActionTypeMerchantApproveRefund     = "MERCHANT_APPROVE_REFUND"
	ComplaintNotificationActionTypeMerchantRejectRefund      = "MERCHANT_REJECT_REFUND"
	ComplaintNotificationActionTypeRefundSuccess             = "REFUND_SUCCESS"
)

const (
	ComplaintRefundProgressActionReject  = "REJECT"
	ComplaintRefundProgressActionApprove = "APPROVE"
)

type ComplaintOrderInfo struct {
	TransactionID string `json:"transaction_id"`
	OutTradeNo    string `json:"out_trade_no"`
	Amount        int64  `json:"amount"`
}

type ComplaintMedia struct {
	MediaType string   `json:"media_type"`
	MediaURL  []string `json:"media_url"`
}

type ComplaintServiceOrderInfo struct {
	OrderID    string `json:"order_id,omitempty"`
	OutOrderNo string `json:"out_order_no,omitempty"`
	State      string `json:"state,omitempty"`
}

type ComplaintReturnAddressInfo struct {
	ReturnAddress string `json:"return_address,omitempty"`
	Longitude     string `json:"longitude,omitempty"`
	Latitude      string `json:"latitude,omitempty"`
}

type ComplaintSharePowerInfo struct {
	ReturnTime              string                      `json:"return_time,omitempty"`
	ReturnAddressInfo       *ComplaintReturnAddressInfo `json:"return_address_info,omitempty"`
	IsReturnedToSameMachine bool                        `json:"is_returned_to_same_machine,omitempty"`
}

type ComplaintAdditionalInfo struct {
	Type           string                   `json:"type,omitempty"`
	SharePowerInfo *ComplaintSharePowerInfo `json:"share_power_info,omitempty"`
}

type EcommerceComplaintInfo struct {
	ComplaintID           string                      `json:"complaint_id"`
	ComplaintTime         string                      `json:"complaint_time"`
	ComplaintDetail       string                      `json:"complaint_detail"`
	ComplaintState        string                      `json:"complaint_state"`
	ComplaintedMchID      string                      `json:"complainted_mchid"`
	PayerPhone            string                      `json:"payer_phone,omitempty"`
	PayerOpenID           string                      `json:"payer_openid,omitempty"`
	ComplaintOrderInfo    []ComplaintOrderInfo        `json:"complaint_order_info,omitempty"`
	ComplaintFullRefunded bool                        `json:"complaint_full_refunded"`
	IncomingUserResponse  bool                        `json:"incoming_user_response"`
	UserComplaintTimes    int64                       `json:"user_complaint_times"`
	ComplaintMediaList    []ComplaintMedia            `json:"complaint_media_list,omitempty"`
	ProblemDescription    string                      `json:"problem_description"`
	ProblemType           string                      `json:"problem_type,omitempty"`
	ApplyRefundAmount     int64                       `json:"apply_refund_amount,omitempty"`
	UserTagList           []string                    `json:"user_tag_list,omitempty"`
	ServiceOrderInfo      []ComplaintServiceOrderInfo `json:"service_order_info,omitempty"`
	AdditionalInfo        *ComplaintAdditionalInfo    `json:"additional_info,omitempty"`
	InPlatformService     bool                        `json:"in_platform_service,omitempty"`
	NeedImmediateService  bool                        `json:"need_immediate_service,omitempty"`
	IsAgentMode           bool                        `json:"is_agent_mode,omitempty"`
}

// 官方文档：GET /v3/merchant-service/complaints-v2
type EcommerceComplaintListRequest struct {
	Limit            int64  `json:"limit,omitempty"`
	Offset           int64  `json:"offset,omitempty"`
	BeginDate        string `json:"begin_date"`
	EndDate          string `json:"end_date"`
	ComplaintedMchID string `json:"complainted_mchid,omitempty"`
}

// 官方文档：GET /v3/merchant-service/complaints-v2
type EcommerceComplaintListResponse struct {
	Data       []EcommerceComplaintInfo `json:"data,omitempty"`
	Limit      int64                    `json:"limit,omitempty"`
	Offset     int64                    `json:"offset,omitempty"`
	TotalCount int64                    `json:"total_count,omitempty"`
}

// 官方文档：GET /v3/merchant-service/complaints-v2/{complaint_id}
type EcommerceComplaintDetailRequest struct {
	ComplaintID string `json:"complaint_id"`
}

type ComplaintNegotiationHistory struct {
	LogID                                    string                  `json:"log_id"`
	Operator                                 string                  `json:"operator"`
	OperateTime                              string                  `json:"operate_time"`
	OperateType                              string                  `json:"operate_type"`
	OperateDetails                           string                  `json:"operate_details,omitempty"`
	ImageList                                []string                `json:"image_list,omitempty"`
	ComplaintMediaList                       *ComplaintMedia         `json:"complaint_media_list,omitempty"`
	UserAppyPlatformServiceReason            string                  `json:"user_appy_platform_service_reason,omitempty"`
	UserAppyPlatformServiceReasonDescription string                  `json:"user_appy_platform_service_reason_description,omitempty"`
	NormalMessage                            *ComplaintNormalMessage `json:"normal_message,omitempty"`
	ClickMessage                             *ComplaintClickMessage  `json:"click_message,omitempty"`
}

// 官方文档：GET /v3/merchant-service/complaints-v2/{complaint_id}/negotiation-historys
type ComplaintNegotiationHistoryRequest struct {
	ComplaintID string `json:"complaint_id"`
	Limit       int64  `json:"limit,omitempty"`
	Offset      int64  `json:"offset,omitempty"`
}

// 官方文档：GET /v3/merchant-service/complaints-v2/{complaint_id}/negotiation-historys
type ComplaintNegotiationHistoryResponse struct {
	Data       []ComplaintNegotiationHistory `json:"data,omitempty"`
	Limit      int64                         `json:"limit,omitempty"`
	Offset     int64                         `json:"offset,omitempty"`
	TotalCount int64                         `json:"total_count,omitempty"`
}

type ComplaintNotificationEnvelopeResource struct {
	Algorithm      string `json:"algorithm"`
	Ciphertext     string `json:"ciphertext"`
	OriginalType   string `json:"original_type"`
	AssociatedData string `json:"associated_data,omitempty"`
	Nonce          string `json:"nonce"`
}

// 官方文档：投诉通知回调外层 envelope
type ComplaintNotificationEnvelope struct {
	ID           string                                `json:"id"`
	CreateTime   string                                `json:"create_time"`
	EventType    string                                `json:"event_type"`
	ResourceType string                                `json:"resource_type"`
	Summary      string                                `json:"summary"`
	Resource     ComplaintNotificationEnvelopeResource `json:"resource"`
}

// 官方文档：投诉通知回调 resource 解密后字段
type ComplaintNotificationResource struct {
	ComplaintID string `json:"complaint_id"`
	ActionType  string `json:"action_type"`
}

// 官方文档：POST/PUT /v3/merchant-service/complaint-notifications
type ComplaintNotificationConfigRequest struct {
	URL string `json:"url"`
}

// 官方文档：POST/GET/PUT /v3/merchant-service/complaint-notifications
type ComplaintNotificationConfig struct {
	MchID string `json:"mchid"`
	URL   string `json:"url"`
}

type ComplaintResponseMiniProgramJumpInfo struct {
	AppID string `json:"appid"`
	Path  string `json:"path"`
	Text  string `json:"text"`
}

// 官方文档：POST /v3/merchant-service/complaints-v2/{complaint_id}/response
type ComplaintResponseRequest struct {
	ComplaintID         string                                `json:"complaint_id"`
	ComplaintedMchID    string                                `json:"complainted_mchid"`
	ResponseContent     string                                `json:"response_content"`
	ResponseImages      []string                              `json:"response_images,omitempty"`
	JumpURL             string                                `json:"jump_url,omitempty"`
	JumpURLText         string                                `json:"jump_url_text,omitempty"`
	MiniProgramJumpInfo *ComplaintResponseMiniProgramJumpInfo `json:"mini_program_jump_info,omitempty"`
}

// 官方文档：POST /v3/merchant-service/complaints-v2/{complaint_id}/complete
type ComplaintCompleteRequest struct {
	ComplaintID      string `json:"complaint_id"`
	ComplaintedMchID string `json:"complainted_mchid"`
}

// 官方文档：POST /v3/merchant-service/complaints-v2/{complaint_id}/update-refund-progress
type ComplaintRefundProgressUpdateRequest struct {
	ComplaintID     string   `json:"complaint_id"`
	Action          string   `json:"action"`
	LaunchRefundDay int64    `json:"launch_refund_day,omitempty"`
	RejectReason    string   `json:"reject_reason,omitempty"`
	RejectMediaList []string `json:"reject_media_list,omitempty"`
	Remark          string   `json:"remark,omitempty"`
}

// 官方文档：POST /v3/merchant-service/complaints-v2/{complaint_id}/response-immediate-service
type ComplaintImmediateServiceReplyRequest struct {
	ComplaintID      string                  `json:"complaint_id"`
	ComplaintedMchID string                  `json:"complainted_mchid"`
	Message          *ComplaintNormalMessage `json:"message"`
	IdempotentID     string                  `json:"idempotent_id"`
}

// 官方文档：POST /v3/merchant-service/complaints-v2/{complaint_id}/response-immediate-service
type ComplaintImmediateServiceReplyResponse struct {
	LogID string `json:"log_id"`
}

type ComplaintImageMeta struct {
	Filename string `json:"filename"`
	SHA256   string `json:"sha256"`
}

// 官方文档：POST /v3/merchant-service/images/upload
type ComplaintImageUploadRequest struct {
	File []byte              `json:"file"`
	Meta *ComplaintImageMeta `json:"meta"`
}

// 官方文档：POST /v3/merchant-service/images/upload
type ComplaintImageUploadResponse struct {
	MediaID string `json:"media_id"`
}

// 官方文档：GET /v3/merchant-service/images/{media_id}
type ComplaintImageQueryRequest struct {
	MediaID string `json:"media_id"`
}
