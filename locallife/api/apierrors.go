package api

import "errors"

// APIError 是带数字错误码的 API 业务错误，实现 error 接口。
//
// 错误码规则（5 位数）：
//   - 前 3 位：对应 HTTP 状态码语义（400/403/404/409…）
//   - 后 2 位：同一 HTTP 类下的错误序号（01、02…）
//
// 例：40401 = 404 类，序号 01。
//
// 前端应以 code 字段为准做程序分支判断，error 字段仅用于降级展示和日志。
// 所有 API 错误常量均在此文件中集中定义，禁止在 handler 中散布 errors.New() 内联字符串。
type APIError struct {
	Code    int
	Message string
}

func (e *APIError) Error() string { return e.Message }

// apierr 构造一个 APIError，仅供本包内使用（包级变量初始化）。
func apierr(code int, message string) *APIError {
	return &APIError{Code: code, Message: message}
}

// AsAPIError 从任意 error 中提取 *APIError；若不是 *APIError 则返回 nil。
func AsAPIError(err error) *APIError {
	var e *APIError
	if errors.As(err, &e) {
		return e
	}
	return nil
}

// ==================== 通用权限 / 身份 (403xx) ====================

var (
	ErrNotMerchant      = apierr(40301, "not a merchant")
	ErrNotOperator      = apierr(40302, "not an operator")
	ErrNotRider         = apierr(40303, "not a rider")
	ErrPermissionDenied = apierr(40304, "permission denied")
)

// ==================== 通用资源未找到 (404xx) ====================

var (
	ErrUserNotFound     = apierr(40401, "user not found")
	ErrMerchantNotFound = apierr(40402, "merchant not found")
	ErrDishNotFound     = apierr(40403, "dish not found")
	ErrOrderNotFound    = apierr(40404, "order not found")
	ErrRiderNotFound    = apierr(40405, "rider not found")
)

// ==================== 购物车 (400xx) ====================

var (
	ErrCartEmpty      = apierr(40001, "cart is empty")
	ErrMerchantClosed = apierr(40002, "merchant is closed")
)

// ==================== 协议确认 (Agreement Consent) ====================

var (
	ErrUserAgreementVersionRequired = apierr(40101, "请先阅读并同意用户协议后再提交申请")
	ErrPrivacyPolicyVersionRequired = apierr(40102, "请先阅读并同意隐私政策后再提交申请")
	ErrAgreementConsentedAtRequired = apierr(40103, "协议确认状态缺失，请重新勾选协议后再提交")
	ErrAgreementConsentedAtInvalid  = apierr(40104, "协议确认时间格式无效，请重新勾选协议后再提交")
)

// ==================== 运费配置 (DeliveryFee) ====================

var (
	// 404 类
	ErrDeliveryFeeConfigNotFound    = apierr(40451, "delivery fee config not found for this region")
	ErrPeakHourConfigNotFound       = apierr(40452, "peak hour config not found")
	ErrDeliveryPromotionNotFound    = apierr(40453, "delivery promotion not found")
	ErrDeliveryMerchantInfoNotFound = apierr(40454, "merchant information not found")

	// 403 类
	ErrDeliveryServiceDisabled     = apierr(40351, "delivery service is disabled for this region")
	ErrPromotionNotOwnedByMerchant = apierr(40352, "promotion does not belong to this merchant")
	ErrPromotionManageMerchantOnly = apierr(40353, "you can only manage your own merchant's promotions")
	ErrPromotionViewMerchantOnly   = apierr(40354, "you can only view your own merchant's promotions")
	ErrPromotionDeleteMerchantOnly = apierr(40355, "you can only delete your own merchant's promotions")
	ErrPromotionUpdateMerchantOnly = apierr(40356, "you can only update your own merchant's promotions")

	// 409 类
	ErrDeliveryFeeConfigExists = apierr(40951, "delivery fee config already exists for this region")

	// 400 参数格式类
	ErrInvalidStartTimeFormat    = apierr(40051, "invalid start_time format, expected HH:MM")
	ErrInvalidEndTimeFormat      = apierr(40052, "invalid end_time format, expected HH:MM")
	ErrInvalidValidFromFormat    = apierr(40053, "invalid valid_from format, expected RFC3339")
	ErrInvalidValidUntilFormat   = apierr(40054, "invalid valid_until format, expected RFC3339")
	ErrValidUntilBeforeValidFrom = apierr(40055, "valid_until must be after valid_from")
	ErrDiscountExceedsMinOrder   = apierr(40056, "discount_amount cannot exceed min_order_amount")
)

// ==================== 资源未找到补充 (Resource Not Found - Extended) ====================

var (
	ErrTableNotFound       = apierr(40406, "table not found")
	ErrApplicationNotFound = apierr(40407, "application not found")
	// ErrRiderNotRegistered: 骑手档案未创建（区别于 ErrNotRider 权限检查）
	ErrRiderNotRegistered        = apierr(40408, "rider profile not found: please complete registration")
	ErrRegionNotFound            = apierr(40409, "region not found")
	ErrSessionNotFound           = apierr(40410, "login session not found")
	ErrOperatorNotFound          = apierr(40411, "operator not found")
	ErrClaimNotFound             = apierr(40412, "claim not found")
	ErrNoLocationAvailable       = apierr(40413, "no location data available")
	ErrDeliveryRecordNotFound    = apierr(40414, "delivery record not found")
	ErrCoordinateNoDistrict      = apierr(40415, "the provided coordinates do not match any district")
	ErrOperatorProfileIncomplete = apierr(40416, "operator identity information is incomplete: please complete ID verification first")
)

// ==================== 申请流程 (Application Workflow) ====================

var (
	// 400 类：申请状态约束
	ErrApplicationNotDraft     = apierr(40003, "application can only be modified in draft state")
	ErrApplicationSubmitDraft  = apierr(40004, "application can only be submitted in draft state")
	ErrApplicationCannotReset  = apierr(40005, "application can only be reset while pending review")
	ErrApplicationInvalidState = apierr(40006, "application cannot be submitted in its current state")
	ErrApplicationNotPending   = apierr(40027, "application can only be reviewed when in pending state")
	// 409 类：申请冲突
	ErrAlreadyOperator             = apierr(40961, "already registered as an operator")
	ErrOperatorApplicationPending  = apierr(40962, "there is already a pending operator application")
	ErrRegionHasOperator           = apierr(40963, "the selected region already has an active operator")
	ErrRegionHasPendingApplication = apierr(40964, "the selected region already has a pending application")
	ErrImageModerationPending      = apierr(40965, "image moderation is pending, please retry later")
)

// ==================== 图片/文件上传 (Image/Upload) ====================

var (
	ErrImageContentSafetyFailed = apierr(40007, "image content safety check failed")
	ErrImageTooLarge            = apierr(40008, "image too large, please compress before uploading")
	ErrInvalidIDCardSide        = apierr(40009, "side must be Front or Back")
	// ErrInvalidDocumentImageURL: 营业执照/食品经营许可证/身份证图片 URL 需来自上传接口
	ErrInvalidDocumentImageURL = apierr(40010, "invalid document image URL: must use URL generated by the upload API")
	ErrTextContentSafetyFailed = apierr(40011, "text content safety check failed")
	ErrInvalidDishImageURL     = apierr(40020, "invalid dish image URL: must use URL from the dish image upload API")
	ErrInvalidClaimID          = apierr(40021, "invalid claim ID")
	ErrIDCardImageRequired     = apierr(40024, "ID card image is required")
	ErrTooManyStorefrontPhotos = apierr(40025, "too many storefront photos: maximum 3 allowed")
	ErrTooManyAmbientPhotos    = apierr(40026, "too many ambient photos: maximum 5 allowed")
)

// ==================== 骑手业务 (Rider Business) ====================

var (
	ErrRiderNotActivated        = apierr(40012, "rider account has not been activated")
	ErrRiderInsufficientBalance = apierr(40013, "insufficient available balance")
	ErrRiderDepositInsufficient = apierr(40014, "insufficient deposit, please top up first")
	ErrRiderHasActiveOrders     = apierr(40015, "cannot perform this action while you have active delivery orders")
	ErrRiderNoRegionAssigned    = apierr(40016, "rider has no region assigned")
	ErrRiderDepositFrozen       = apierr(40971, "cannot withdraw while deposit is frozen")
)

// ==================== 金融/金额 (Finance) ====================

var (
	ErrAmountNegative          = apierr(40017, "amount cannot be negative")
	ErrInvalidNumberFormat     = apierr(40018, "invalid number format")
	ErrProfitShareExceedsLimit = apierr(40019, "total share ratio (platform + operator) cannot exceed 100%")
	ErrRatioOutOfRange         = apierr(40023, "ratio must be between 0 and 100")
)

// ==================== 登录/会话 (Login Session) ====================

var (
	ErrSessionExpired        = apierr(40022, "login session has expired")
	ErrMissingLoginCode      = apierr(40028, "login code is required")
	ErrMissingPollingToken   = apierr(40029, "polling token is required")
	ErrMissingSignatureParam = apierr(40030, "signature parameters are required")
	ErrMissingSignatureKey   = apierr(40098, "signature key is missing")
	ErrInvalidSignature      = apierr(40031, "invalid or expired signature")
	ErrInvalidPollingToken   = apierr(40032, "invalid polling token")
	ErrSessionMissingUser    = apierr(40033, "login session is missing user information")
	ErrSessionNotConfirmed   = apierr(40034, "login session has not been confirmed yet")

	// 409 类
	ErrSessionAlreadyUsed     = apierr(40965, "login session has already been used")
	ErrSessionConflictAccount = apierr(40966, "login session has been confirmed by another account")
)

// ==================== 骑手操作/配送 (Rider Operations / Delivery) ====================

var (
	ErrRiderOffline                = apierr(40035, "rider is currently offline")
	ErrRiderActiveOrderOnly        = apierr(40036, "can only report the current active delivery order")
	ErrDeliveryAddressRequired     = apierr(40037, "a delivery address is required for merged takeout orders")
	ErrDeliveryDistanceUnavailable = apierr(40038, "delivery distance unavailable: please reselect the address")
	ErrDeliveryDistanceCalcFailed  = apierr(40039, "delivery distance calculation failed, please try again later")
	ErrLocationTimestampFuture     = apierr(40040, "location timestamp cannot be more than 5 minutes in the future")
	ErrLocationTimestampTooOld     = apierr(40041, "location timestamp cannot be earlier than 1 hour ago")
	ErrDistanceNegative            = apierr(40042, "distance cannot be negative")
	ErrRiderNotPending             = apierr(40043, "rider is not in pending review state")
	ErrRiderCannotApprove          = apierr(40044, "rider's current state does not allow approval")
	ErrRiderNotOnlineForOrders     = apierr(40045, "please go online first to receive real-time order notifications")
)

// ==================== 规则/系数 (Rules / Coefficients) ====================

var (
	ErrCoefficientTooLow          = apierr(40046, "coefficient cannot be less than 1.0")
	ErrWeatherCoefficientReadOnly = apierr(40047, "weather coefficient is auto-managed by the system and cannot be modified manually")
	ErrValueRateOutOfRange        = apierr(40048, "value rate must be between 0 and 100")
	ErrUnknownRuleKey             = apierr(40049, "unknown rule key")
	ErrRulePlatformOnly           = apierr(40050, "this rule can only be modified by the platform")
)

// ==================== 索赔/风控 (Claims / Risk Management) ====================

var (
	ErrOrderNotEligibleForClaim              = apierr(40057, "claims can only be filed for completed orders")
	ErrMerchantFoodPermitNameUnreadable      = apierr(40058, "未能从食品经营许可证中识别出主体名称，请上传完整清晰的食品经营许可证后重试")
	ErrMerchantBusinessLicenseNameUnreadable = apierr(40059, "未能从营业执照中识别出企业名称，请上传完整清晰的营业执照后重试")
	ErrMerchantFoodPermitNameMismatch        = apierr(40061, "食品经营许可证主体名称与营业执照企业名称不一致，请核对证照信息后重试")
	ErrClaimAmountExceedsOrder               = apierr(40060, "claim amount cannot exceed the order total")
	ErrFoodSafetyClaimUnsupported            = apierr(40063, "food safety claims are handled by the dedicated food safety workflow")

	// 403 类
	ErrClaimNotOwned             = apierr(40357, "this claim does not belong to the current user")
	ErrAccountBehaviorRestricted = apierr(40358, "account has behavioral restrictions that prevent this action")
	ErrOrderNotOwned             = apierr(40359, "this order does not belong to the current user")
	ErrTableNotOwned             = apierr(40360, "this table does not belong to your merchant")
	ErrIllegalPath               = apierr(40361, "illegal file path")
	ErrRiderNotDeliverer         = apierr(40362, "you are not the assigned rider for this order")

	// 409 类
	ErrOrderAlreadyHasClaim     = apierr(40967, "a claim record already exists for this order")
	ErrApplicationStateChanged  = apierr(40968, "application state has changed, action is no longer valid")
	ErrAccountApplymentPending  = apierr(40969, "there is already an account registration application in progress")
	ErrAccountAlreadyRegistered = apierr(40970, "account registration has already been completed")
)

// ==================== 区域/运营商扩张 (Region / Operator Expansion) ====================

var (
	ErrRegionAlreadyManaged    = apierr(40063, "you are already managing this region")
	ErrRegionExpansionPending  = apierr(40064, "you have already submitted an expansion application for this region")
	ErrApplicationNotSubmitted = apierr(40065, "application can only be reviewed when in submitted state")
)

// ==================== 文件/图片路径 (File / Image Path) ====================

var (
	ErrInvalidFilePath                         = apierr(40066, "path must be a relative path under the uploads directory")
	ErrInvalidTableImageURL                    = apierr(40067, "invalid table image URL: must use URL from the table image upload API")
	ErrInvalidReviewImageURL                   = apierr(40068, "invalid review image URL: must use URL from the review image upload API")
	ErrApplymentIDCardValidityInvalid          = apierr(40069, "ID card validity period is incomplete or invalid: please re-upload the ID card back image")
	ErrApplymentBusinessLicenseValidityInvalid = apierr(40071, "business license validity period is invalid: please re-upload the business license image")
	ErrInvalidTypeParam                        = apierr(40070, "type must be merchant or dish")
	ErrInvalidTableID                          = apierr(40072, "invalid table ID")
	ErrInvalidAddress                          = apierr(40073, "invalid address")
	ErrInvalidLatitudeFormat                   = apierr(40074, "invalid latitude format")
	ErrInvalidLongitudeFormat                  = apierr(40075, "invalid longitude format")
	ErrApplymentWebSceneDomainRequired         = apierr(40109, "web applyment domain is not configured")
)

// ==================== 必填字段/文件上传校验 (Required Fields / Document Upload) ====================

var (
	ErrMerchantNameRequired          = apierr(40076, "merchant name is required")
	ErrMerchantAddressRequired       = apierr(40077, "merchant address is required")
	ErrMerchantLocationRequired      = apierr(40078, "merchant geographic location must be selected")
	ErrMerchantRegionRequired        = apierr(40079, "merchant region must be selected")
	ErrRegionSelectionRequired       = apierr(40080, "please select a region to apply for")
	ErrBusinessLicenseRequired       = apierr(40081, "business license is required")
	ErrFoodLicenseRequired           = apierr(40082, "food service license is required")
	ErrIDCardFrontRequired           = apierr(40083, "front side of ID card is required")
	ErrIDCardBackRequired            = apierr(40084, "back side of ID card is required")
	ErrLegalRepIDCardFrontRequired   = apierr(40085, "front side of legal representative's ID card is required")
	ErrLegalRepIDCardBackRequired    = apierr(40086, "back side of legal representative's ID card is required")
	ErrHealthCertRequired            = apierr(40087, "health certificate image is required")
	ErrPhoneRequired                 = apierr(40088, "phone number is required")
	ErrContactNameRequired           = apierr(40089, "contact name is required")
	ErrOperatorNameRequired          = apierr(40090, "operator name is required")
	ErrIDNumberRequired              = apierr(40091, "ID number is required")
	ErrRejectionReasonRequired       = apierr(40092, "rejection reason is required")
	ErrTargetRegionRequired          = apierr(40093, "target region ID is required")
	ErrBusinessLicenseNotYetUploaded = apierr(40094, "please upload your business license first")
	ErrFoodLicenseNotYetUploaded     = apierr(40095, "please upload your food service license first")
	ErrIDCardNotYetUploaded          = apierr(40096, "please upload your ID card first")
	ErrTableDisabled                 = apierr(40097, "table is currently disabled")
	ErrOrderStateNotEligible         = apierr(40099, "current order state does not allow this action")
	ErrLocationAddressRequired       = apierr(40100, "location could not be determined: please select on map or provide a detailed address")
	ErrMerchantServiceUnavailable    = apierr(50301, "merchant service is temporarily unavailable")
	ErrClaimPayoutServiceUnavailable = apierr(50302, "claim payout service is temporarily unavailable")
	ErrAppealCompensationUnavailable = apierr(50303, "appeal compensation service is temporarily unavailable")
)
