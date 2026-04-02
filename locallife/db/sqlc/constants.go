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
	OrderStatusPending         = "pending"
	OrderStatusPaid            = "paid"
	OrderStatusPreparing       = "preparing"
	OrderStatusReady           = "ready"
	OrderStatusCourierAccepted = "courier_accepted"
	OrderStatusPicked          = "picked"
	OrderStatusDelivering      = "delivering"
	OrderStatusRiderDelivered  = "rider_delivered"
	OrderStatusUserDelivered   = "user_delivered"
	OrderStatusCompleted       = "completed"
	OrderStatusCancelled       = "cancelled"

	OrderTypeTakeout     = "takeout"
	OrderTypeReservation = "reservation"

	BehaviorDecisionModeMerchantRecovery = "merchant_recovery"
	BehaviorDecisionModeRiderRecovery    = "rider_recovery"
	BehaviorDecisionModePlatformFallback = "platform_fallback"
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

	ClaimRecoveryEventTypeCreated    = "created"
	ClaimRecoveryEventTypePaid       = "paid"
	ClaimRecoveryEventTypeWaived     = "waived"
	ClaimRecoveryEventTypeResumed    = "resumed"
	ClaimRecoveryEventTypeOverturned = "overturned"

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
)
