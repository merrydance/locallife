package db

const (
	UserRoleAdmin         = "admin"
	UserRoleOperator      = "operator"
	UserRoleMerchantOwner = "merchant_owner"
	UserRoleMerchantStaff = "merchant_staff"
	UserRoleRider         = "rider"
	UserRoleCustomer      = "customer"

	MerchantStaffRoleOwner    = "owner"
	MerchantStaffStatusActive = "active"

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

	ClaimStatusPending                     = "pending"
	ClaimStatusAutoApproved                = "auto-approved"
	ClaimStatusWaitingCustomerConfirmation = "waiting_customer_confirmation"
	ClaimStatusApproved                    = "approved"
	ClaimStatusRejected                    = "rejected"
	ClaimStatusWithdrawn                   = "withdrawn"

	OrderTypeTakeout     = "takeout"
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
	PlatformConfigScopeOperator      = "operator"
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

	MerchantCapabilityStatusUnknown = "unknown"
	MerchantCapabilityStatusYes     = "yes"
	MerchantCapabilityStatusNo      = "no"

	MerchantCancelWithdrawLocalSyncStateCreated         = "created"
	MerchantCancelWithdrawLocalSyncStateSubmitSucceeded = "submit_succeeded"
	MerchantCancelWithdrawLocalSyncStateSubmitUnknown   = "submit_unknown"
	MerchantCancelWithdrawLocalSyncStateSyncFailed      = "sync_failed"

	MerchantCancelWithdrawModeNoWithdraw = "NOT_APPLY_WITHDRAW"
	MerchantCancelWithdrawModeWithdraw   = "APPLY_WITHDRAW"

	MerchantCancelWithdrawBusinessLicenseStatusActive   = "ACTIVE"
	MerchantCancelWithdrawBusinessLicenseStatusCanceled = "CANCELED"
	MerchantCancelWithdrawBusinessLicenseStatusRevoked  = "REVOKED"

	MerchantCancelStateAccepted               = "ACCEPTED"
	MerchantCancelStateReviewing              = "REVIEWING"
	MerchantCancelStateRejected               = "REJECTED"
	MerchantCancelStateWaitingMerchantConfirm = "WAITING_MERCHANT_CONFIRM"
	MerchantCancelStateRevoked                = "REVOKED"
	MerchantCancelStateSystemProcessing       = "SYSTEM_PROCESSING"
	MerchantCancelStateCanceled               = "CANCELED"
	MerchantCancelStateFundProcessing         = "FUND_PROCESSING"
	MerchantCancelStateFinish                 = "FINISH"

	PaymentChannelDirect    = "direct"
	PaymentChannelEcommerce = "ecommerce"

	MerchantCancelWithdrawStateProcessing = "WITHDRAW_PROCESSING"
	MerchantCancelWithdrawStateException  = "WITHDRAW_EXCEPTION"
	MerchantCancelWithdrawStateSucceed    = "WITHDRAW_SUCCEED"

	MerchantCapabilitySourceSystemDefault   = "system_default"
	MerchantCapabilitySourceManualReview    = "manual_review"
	MerchantCapabilitySourceMerchantClaim   = "merchant_claim"
	MerchantCapabilitySourceMigration       = "migration_backfill"
	MerchantSystemLabelSourceReconciler     = "capability_reconciler"
	MerchantSystemLabelSourceManualOverride = "manual_override"
	MerchantSystemLabelSourceMigration      = "migration_backfill"

	SystemTagHasOpenKitchen = "有明厨亮灶"
	SystemTagNoOpenKitchen  = "无明厨亮灶"
	SystemTagNoDineIn       = "无堂食"
)
