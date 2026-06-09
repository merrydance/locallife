package db

const (
	UserRoleAdmin         = "admin"
	UserRoleOperator      = "operator"
	UserRoleMerchantOwner = "merchant_owner"
	UserRoleMerchantStaff = "merchant_staff"
	UserRoleRider         = "rider"
	UserRoleCustomer      = "customer"

	MerchantStaffRoleOwner      = "owner"
	MerchantStaffRoleManager    = "manager"
	MerchantStaffRoleChef       = "chef"
	MerchantStaffRoleCashier    = "cashier"
	MerchantStaffRolePending    = "pending"
	MerchantStaffStatusActive   = "active"
	MerchantStaffStatusDisabled = "disabled"

	UserRoleStatusActive   = "active"
	UserRoleStatusDisabled = "disabled"

	MerchantStatusApproved          = "approved"
	MerchantStatusActive            = "active"
	MerchantStatusBindbankSubmitted = "bindbank_submitted"
	MerchantStatusSuspended         = "suspended"

	WantedMerchantSourceManual = "manual"
	WantedMerchantSourceMap    = "map"

	WantedMerchantStatusActive  = "active"
	WantedMerchantStatusMatched = "matched"
	WantedMerchantStatusRemoved = "removed"

	WantedMerchantVoteResultCreated      = "created"
	WantedMerchantVoteResultVoted        = "voted"
	WantedMerchantVoteResultAlreadyVoted = "already_voted"

	MerchantApplicationStatusDraft     = "draft"
	MerchantApplicationStatusSubmitted = "submitted"
	MerchantApplicationStatusApproved  = "approved"
	MerchantApplicationStatusRejected  = "rejected"

	MerchantPaymentConfigStatusActive               = "active"
	MerchantPaymentConfigStatusPendingAuthorization = "pending_authorization"

	AccountAuthorizeStateUnauthorized = "AUTHORIZE_STATE_UNAUTHORIZED"
	AccountAuthorizeStateAuthorized   = "AUTHORIZE_STATE_AUTHORIZED"

	MerchantAppDevicePlatformAndroid = "android"

	MerchantAppDeviceProviderHuawei  = "huawei"
	MerchantAppDeviceProviderHonor   = "honor"
	MerchantAppDeviceProviderXiaomi  = "xiaomi"
	MerchantAppDeviceProviderOppo    = "oppo"
	MerchantAppDeviceProviderVivo    = "vivo"
	MerchantAppDeviceProviderUnknown = "unknown"

	MerchantAppDeviceStatusActive   = "active"
	MerchantAppDeviceStatusInactive = "inactive"

	TableTypeTable = "table"
	TableTypeRoom  = "room"

	TableStatusAvailable = "available"
	TableStatusOccupied  = "occupied"
	TableStatusDisabled  = "disabled"
	TableStatusReserved  = "reserved"

	ReservationStatusPending   = "pending"
	ReservationStatusPaid      = "paid"
	ReservationStatusConfirmed = "confirmed"
	ReservationStatusCheckedIn = "checked_in"
	ReservationStatusCompleted = "completed"
	ReservationStatusCancelled = "cancelled"
	ReservationStatusExpired   = "expired"
	ReservationStatusNoShow    = "no_show"

	AppVersionPlatformAndroid = "android"
	AppVersionChannelMerchant = "merchant_app"

	AppVersionStatusDraft    = "draft"
	AppVersionStatusActive   = "active"
	AppVersionStatusDisabled = "disabled"

	// Order statuses (SSOT — 所有层引用此处)
	OrderStatusPending                  = "pending"
	OrderStatusPaid                     = "paid"
	OrderStatusPreparing                = "preparing"
	OrderStatusReady                    = "ready"
	OrderStatusCourierAccepted          = "courier_accepted"
	OrderStatusPicked                   = "picked"
	OrderStatusDelivering               = "delivering"
	OrderStatusRiderDelivered           = "rider_delivered"
	OrderStatusUserDelivered            = "user_delivered"
	OrderStatusCompleted                = "completed"
	OrderStatusCancelled                = "cancelled"
	OrderExceptionStateFoodSafetyPaused = "food_safety_paused"
	ClaimChannelFoodSafety              = "food_safety"

	DeliveryStatusPending    = "pending"
	DeliveryStatusAssigned   = "assigned"
	DeliveryStatusPicking    = "picking"
	DeliveryStatusPicked     = "picked"
	DeliveryStatusDelivering = "delivering"
	DeliveryStatusDelivered  = "delivered"
	DeliveryStatusCompleted  = "completed"
	DeliveryStatusCancelled  = "cancelled"

	ClaimStatusPending                     = "pending"
	ClaimStatusAutoApproved                = "auto-approved"
	ClaimStatusWaitingCustomerConfirmation = "waiting_customer_confirmation"
	ClaimStatusApproved                    = "approved"
	ClaimStatusRejected                    = "rejected"
	ClaimStatusWithdrawn                   = "withdrawn"

	OrderTypeTakeout     = "takeout"
	OrderTypeDineIn      = "dine_in"
	OrderTypeTakeaway    = "takeaway"
	OrderTypeReservation = "reservation"

	BehaviorDecisionModeMerchantRecovery = "merchant_recovery"
	BehaviorDecisionModeRiderRecovery    = "rider_recovery"
	BehaviorDecisionModeUserRestricted   = "user_restricted"

	BehaviorResponsibilityDomainMerchant = "merchant_domain"
	BehaviorResponsibilityDomainRider    = "rider_domain"
	BehaviorResponsibilityDomainUser     = "user_domain"
	BehaviorResponsibilityDomainUnknown  = "unknown"

	BehaviorPayoutModeInstantPaid = "instant_paid"
	BehaviorPayoutModeLimitedPaid = "limited_paid"
	BehaviorPayoutModeRejected    = "rejected"

	BehaviorEffectiveStatusEffective = "effective"

	BehaviorDecisionEffectStatusApplied  = "applied"
	BehaviorDecisionEffectStatusReverted = "reverted"

	ClaimRecoveryEventTypeCreated        = "created"
	ClaimRecoveryEventTypePayable        = "payable"
	ClaimRecoveryEventTypePaymentStarted = "payment_started"
	ClaimRecoveryEventTypePaid           = "paid"
	ClaimRecoveryEventTypeOverdue        = "overdue"
	ClaimRecoveryEventTypeDisputed       = "disputed"
	ClaimRecoveryEventTypeWaived         = "waived"
	ClaimRecoveryEventTypeResumed        = "resumed"
	ClaimRecoveryEventTypeClosed         = "closed"
	ClaimRecoveryEventTypeOverturned     = "overturned"

	ClaimRecoveryBasisMerchantRecovery = "merchant_recovery"
	ClaimRecoveryBasisRiderRecovery    = "rider_recovery"

	BehaviorSnapshotWindowKey7d  = "7d"
	BehaviorSnapshotWindowKey30d = "30d"
	BehaviorSnapshotScopeRaw     = "raw"
	BehaviorSnapshotScopeNet     = "net_effective"
	BehaviorSnapshotVersionV2    = "v2"

	RiderStatusApproved  = "approved"
	RiderStatusActive    = "active"
	RiderStatusSuspended = "suspended"

	RiderApplicationStatusDraft     = "draft"
	RiderApplicationStatusSubmitted = "submitted"
	RiderApplicationStatusApproved  = "approved"

	PlatformConfigScopeGlobal        = "global"
	PlatformConfigKeyRiderDepositFen = "platform_rule.rider_deposit_fen"
	DefaultRiderDepositThresholdFen  = 20000

	FulfillmentStatusScheduled      = "scheduled"
	FulfillmentStatusPendingKitchen = "pending_kitchen"
	FulfillmentStatusPreparing      = "preparing"
	FulfillmentStatusReady          = "ready"
	FulfillmentStatusCompleted      = "completed"
	FulfillmentStatusCancelled      = "cancelled"

	CloudPrinterReconciliationActionRegister = "register"
	CloudPrinterReconciliationActionRemove   = "remove"

	CloudPrinterReconciliationSourceCreate = "create"
	CloudPrinterReconciliationSourceDelete = "delete"

	CloudPrinterReconciliationStatusPending  = "pending"
	CloudPrinterReconciliationStatusResolved = "resolved"

	CloudPrinterProviderYilianyun = "yilianyun"

	CloudPrinterAuthorizationStatusActive        = "active"
	CloudPrinterAuthorizationStatusRefreshFailed = "refresh_failed"
	CloudPrinterAuthorizationStatusRevoked       = "revoked"

	PrintLogStatusPending   = "pending"
	PrintLogStatusSuccess   = "success"
	PrintLogStatusFailed    = "failed"
	PrintLogStatusCancelled = "cancelled"

	MerchantCapabilityStatusUnknown = "unknown"
	MerchantCapabilityStatusYes     = "yes"
	MerchantCapabilityStatusNo      = "no"

	PaymentChannelDirect         = "direct"
	PaymentChannelBaofuAggregate = "baofu_aggregate"

	ReservationAdjustmentDirectionPositive = "positive"
	ReservationAdjustmentDirectionNegative = "negative"
	ReservationAdjustmentDirectionZero     = "zero"

	ReservationAdjustmentStatusCreatingPayment = "creating_payment"
	ReservationAdjustmentStatusPendingPayment  = "pending_payment"
	ReservationAdjustmentStatusApplying        = "applying"
	ReservationAdjustmentStatusApplied         = "applied"
	ReservationAdjustmentStatusClosed          = "closed"
	ReservationAdjustmentStatusFailed          = "failed"
	ReservationAdjustmentStatusExpired         = "expired"

	ReservationAdjustmentHoldStatusHeld      = "held"
	ReservationAdjustmentHoldStatusConverted = "converted"
	ReservationAdjustmentHoldStatusReleased  = "released"

	ExternalPaymentProviderWechat = "wechat"
	ExternalPaymentProviderBaofu  = "baofu"

	PlatformAlertTypeSystemError = "SYSTEM_ERROR"
	PlatformAlertLevelCritical   = "critical"

	BaofuAccountOwnerTypeMerchant = "merchant"
	BaofuAccountOwnerTypeRider    = "rider"
	BaofuAccountOwnerTypeOperator = "operator"
	BaofuAccountOwnerTypePlatform = "platform"

	BaofuAccountTypePersonal = "personal"
	BaofuAccountTypeBusiness = "business"

	BaofuAccountOpeningModePersonal                  = "personal"
	BaofuAccountOpeningModeMerchantPersonalMicro     = "merchant_personal_micro"
	BaofuAccountOpeningModeBusinessPublic            = "business_public"
	BaofuAccountOpeningModeIndividualBusinessPrivate = "individual_business_private"

	BaofuAccountOpenStateProcessing = "processing"
	BaofuAccountOpenStateActive     = "active"
	BaofuAccountOpenStateFailed     = "failed"
	BaofuAccountOpenStateAbnormal   = "abnormal"

	BaofuFeeTypePaymentFee           = "payment_fee"
	BaofuFeeTypeAccountOpenVerifyFee = "account_open_verify_fee"
	BaofuFeePayerTypeMerchant        = "merchant"
	BaofuFeePayerTypePlatform        = "platform"

	OrderPaymentFeeTypeProviderPaymentFee        = "provider_payment_fee"
	OrderPaymentFeeTypeMerchantPaymentServiceFee = "merchant_payment_service_fee"
	OrderPaymentFeeTypeRiderPaymentServiceFee    = "rider_payment_service_fee"
	OrderPaymentFeePayerTypePlatform             = "platform"
	OrderPaymentFeePayerTypeMerchant             = "merchant"
	OrderPaymentFeePayerTypeRider                = "rider"
	OrderPaymentFeePayeeTypeBaofu                = "baofu"
	OrderPaymentFeePayeeTypePlatform             = "platform"
	OrderPaymentFeeAmountSourceCalculated        = "calculated"
	OrderPaymentFeeAmountSourceActualCallback    = "actual_callback"
	OrderPaymentFeeAmountSourceActualQuery       = "actual_query"
	OrderPaymentFeeStatusRecorded                = "recorded"
	OrderPaymentFeeStatusReconciled              = "reconciled"
	OrderPaymentFeeStatusAdjusted                = "adjusted"

	ProfitSharingSettlementModeCommissionShare = "commission_share"
	ProfitSharingSettlementModeFeeOnlyShare    = "fee_only_share"
	ProfitSharingSettlementModeDirectNoShare   = "direct_no_share"

	BaofuWithdrawalStatusProcessing = "processing"
	BaofuWithdrawalStatusSucceeded  = "succeeded"
	BaofuWithdrawalStatusFailed     = "failed"
	BaofuWithdrawalStatusReturned   = "returned"

	BaofuMerchantReportTypeWechat = "WECHAT"
	BaofuMerchantReportTypeAlipay = "ALIPAY"

	BaofuMerchantReportStateProcessing = "processing"
	BaofuMerchantReportStateSucceeded  = "succeeded"
	BaofuMerchantReportStateFailed     = "failed"
	BaofuMerchantReportStateUnknown    = "unknown"

	BaofuMerchantReportAppletAuthStatePending     = "pending"
	BaofuMerchantReportAppletAuthStateSucceeded   = "succeeded"
	BaofuMerchantReportAppletAuthStateFailed      = "failed"
	BaofuMerchantReportAppletAuthStateNotRequired = "not_required"

	BaofuAccountOpeningProfileStatusIncomplete = "incomplete"
	BaofuAccountOpeningProfileStatusComplete   = "complete"

	BaofuAccountOpeningStateProfilePending           = "profile_pending"
	BaofuAccountOpeningStateVerifyFeePending         = "verify_fee_pending"
	BaofuAccountOpeningStateVerifyFeeProcessing      = "verify_fee_processing"
	BaofuAccountOpeningStateOpeningProcessing        = "opening_processing"
	BaofuAccountOpeningStateMerchantReportProcessing = "merchant_report_processing"
	BaofuAccountOpeningStateAppletAuthPending        = "applet_auth_pending"
	BaofuAccountOpeningStateReady                    = "ready"
	BaofuAccountOpeningStateFailed                   = "failed"
	BaofuAccountOpeningStateVoided                   = "voided"

	ProfitSharingReceiverOwnerTypeRider    = "rider"
	ProfitSharingReceiverOwnerTypeOperator = "operator"
	ProfitSharingReceiverOwnerTypeManual   = "manual"

	ProfitSharingReceiverTypePersonalOpenID = "PERSONAL_OPENID"
	ProfitSharingReceiverTypeMerchantID     = "MERCHANT_ID"

	ProfitSharingReceiverDesiredStatePresent = "present"
	ProfitSharingReceiverDesiredStateAbsent  = "absent"

	ProfitSharingReceiverSyncStatusPending    = "pending"
	ProfitSharingReceiverSyncStatusProcessing = "processing"
	ProfitSharingReceiverSyncStatusSynced     = "synced"
	ProfitSharingReceiverSyncStatusFailed     = "failed"
	ProfitSharingReceiverSyncStatusSkipped    = "skipped"

	ProfitSharingReceiverAttemptActionEnsure = "ensure"
	ProfitSharingReceiverAttemptActionDelete = "delete"

	ProfitSharingReceiverAttemptStatusProcessing = "processing"
	ProfitSharingReceiverAttemptStatusSucceeded  = "succeeded"
	ProfitSharingReceiverAttemptStatusFailed     = "failed"
	ProfitSharingReceiverAttemptStatusSkipped    = "skipped"

	ExternalPaymentCapabilityDirectJSAPIPayment  = "direct_jsapi_payment"
	ExternalPaymentCapabilityDirectRefund        = "direct_refund"
	ExternalPaymentCapabilityMerchantTransfer    = "merchant_transfer"
	ExternalPaymentCapabilityBaofuAccount        = "baofu_account"
	ExternalPaymentCapabilityBaofuMerchantReport = "baofu_merchant_report"
	ExternalPaymentCapabilityBaofuPayment        = "baofu_payment"
	ExternalPaymentCapabilityBaofuRefund         = "baofu_refund"
	ExternalPaymentCapabilityBaofuProfitSharing  = "baofu_profit_sharing"
	ExternalPaymentCapabilityBaofuWithdraw       = "baofu_withdraw"

	ExternalPaymentCommandTypeCreatePayment             = "create_payment"
	ExternalPaymentCommandTypeClosePayment              = "close_payment"
	ExternalPaymentCommandTypeCreateRefund              = "create_refund"
	ExternalPaymentCommandTypeCreateProfitSharing       = "create_profit_sharing"
	ExternalPaymentCommandTypeCreateProfitSharingReturn = "create_profit_sharing_return"
	ExternalPaymentCommandTypeFinishProfitSharing       = "finish_profit_sharing"
	ExternalPaymentCommandTypeCreateTransfer            = "create_transfer"
	ExternalPaymentCommandTypeOpenBaofuAccount          = "open_baofu_account"
	ExternalPaymentCommandTypeQueryBaofuAccount         = "query_baofu_account"
	ExternalPaymentCommandTypeQueryBaofuBalance         = "query_baofu_balance"
	ExternalPaymentCommandTypeCreateBaofuWithdraw       = "create_baofu_withdraw"
	ExternalPaymentCommandTypeQueryBaofuWithdraw        = "query_baofu_withdraw"
	ExternalPaymentCommandTypeBaofuMerchantReport       = "baofu_merchant_report"
	ExternalPaymentCommandTypeBaofuMerchantReportQuery  = "baofu_merchant_report_query"
	ExternalPaymentCommandTypeBaofuBindSubConfig        = "baofu_bind_sub_config"

	ExternalPaymentBusinessOwnerRiderDeposit   = "rider_deposit"
	ExternalPaymentBusinessOwnerBaofuVerifyFee = "baofu_account_verify_fee"
	ExternalPaymentBusinessOwnerOrder          = "order"
	ExternalPaymentBusinessOwnerReservation    = "reservation"
	ExternalPaymentBusinessOwnerClaimRecovery  = "claim_recovery"
	ExternalPaymentBusinessOwnerProfitSharing  = "profit_sharing"
	ExternalPaymentBusinessOwnerApplyment      = "applyment"
	ExternalPaymentBusinessOwnerMerchantFunds  = "merchant_finance"
	ExternalPaymentBusinessOwnerRiderIncome    = "rider_income"
	ExternalPaymentBusinessOwnerOperatorFunds  = "operator_finance"
	ExternalPaymentBusinessOwnerPlatformFunds  = "platform_finance"

	PaymentBusinessTypeBaofuAccountVerifyFee = "baofu_account_verify_fee"

	ExternalPaymentObjectPayment             = "payment"
	ExternalPaymentObjectCombinedPayment     = "combined_payment"
	ExternalPaymentObjectRefund              = "refund"
	ExternalPaymentObjectProfitSharing       = "profit_sharing"
	ExternalPaymentObjectProfitSharingReturn = "profit_sharing_return"
	ExternalPaymentObjectApplyment           = "applyment"
	ExternalPaymentObjectWithdraw            = "withdraw"
	ExternalPaymentObjectMerchantTransfer    = "merchant_transfer"
	ExternalPaymentObjectBaofuPaymentOrder   = "baofu_payment_order"

	ExternalPaymentCommandStatusSubmitted = "submitted"
	ExternalPaymentCommandStatusAccepted  = "accepted"
	ExternalPaymentCommandStatusRejected  = "rejected"
	ExternalPaymentCommandStatusUnknown   = "unknown"

	ExternalPaymentFactSourceCallback             = "callback"
	ExternalPaymentFactSourceCommandResponse      = "command_response"
	ExternalPaymentFactSourceQuery                = "query"
	ExternalPaymentFactSourceManualReconciliation = "manual_reconciliation"

	ExternalPaymentTerminalStatusSuccess    = "success"
	ExternalPaymentTerminalStatusFailed     = "failed"
	ExternalPaymentTerminalStatusClosed     = "closed"
	ExternalPaymentTerminalStatusExpired    = "expired"
	ExternalPaymentTerminalStatusProcessing = "processing"
	ExternalPaymentTerminalStatusUnknown    = "unknown"

	ExternalPaymentFactProcessingStatusReceived     = "received"
	ExternalPaymentFactProcessingStatusTerminalized = "terminalized"
	ExternalPaymentFactProcessingStatusIgnored      = "ignored"
	ExternalPaymentFactProcessingStatusFailed       = "failed"

	ExternalPaymentFactApplicationStatusPending    = "pending"
	ExternalPaymentFactApplicationStatusProcessing = "processing"
	ExternalPaymentFactApplicationStatusApplied    = "applied"
	ExternalPaymentFactApplicationStatusSkipped    = "skipped"
	ExternalPaymentFactApplicationStatusFailed     = "failed"

	PaymentDomainOutboxStatusPending    = "pending"
	PaymentDomainOutboxStatusProcessing = "processing"
	PaymentDomainOutboxStatusPublished  = "published"
	PaymentDomainOutboxStatusFailed     = "failed"

	PaymentDomainOutboxEventOrderPaymentSucceeded       = "order_payment_succeeded"
	PaymentDomainOutboxEventReservationPaymentSucceeded = "reservation_payment_succeeded"
	PaymentDomainOutboxEventProfitSharingResultReady    = "profit_sharing_result_ready"
	PaymentDomainOutboxEventApplymentActivated          = "applyment_activated"
	PaymentDomainOutboxEventApplymentPendingStateReady  = "applyment_pending_state_ready"
	PaymentDomainOutboxEventApplymentTerminalStateReady = "applyment_terminal_state_ready"
	PaymentDomainOutboxEventOrderRefundSucceeded        = "order_refund_succeeded"
	PaymentDomainOutboxEventOrderRefundAbnormal         = "order_refund_abnormal"
	PaymentDomainOutboxEventRiderDepositRefundAbnormal  = "rider_deposit_refund_abnormal"
	PaymentDomainOutboxEventReservationRefundAbnormal   = "reservation_refund_abnormal"
	PaymentDomainOutboxAggregatePaymentOrder            = "payment_order"
	PaymentDomainOutboxAggregateProfitSharingOrder      = "profit_sharing_order"
	PaymentDomainOutboxAggregateRefundOrder             = "refund_order"

	ProfitSharingOrderStatusPending    = "pending"
	ProfitSharingOrderStatusProcessing = "processing"
	ProfitSharingOrderStatusFinished   = "finished"
	ProfitSharingOrderStatusFailed     = "failed"

	MerchantCapabilitySourceSystemDefault   = "system_default"
	MerchantCapabilitySourceManualReview    = "manual_review"
	MerchantCapabilitySourceMerchantClaim   = "merchant_claim"
	MerchantCapabilitySourceMigration       = "migration_backfill"
	MerchantSystemLabelSourceReconciler     = "capability_reconciler"
	MerchantSystemLabelSourceManualOverride = "manual_override"
	MerchantSystemLabelSourceMigration      = "migration_backfill"

	CredentialDocumentTypeBusinessLicense = "business_license"
	CredentialDocumentTypeFoodPermit      = "food_permit"
	CredentialDocumentTypeHealthCert      = "health_cert"

	CredentialSuspensionReasonDocumentExpired = "document_expired"

	SystemTagHasOpenKitchen = "有明厨亮灶"
	SystemTagNoOpenKitchen  = "无明厨亮灶"
	SystemTagNoDineIn       = "无堂食"
)
