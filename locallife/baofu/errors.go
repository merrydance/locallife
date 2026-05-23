package baofu

import "strings"

type BaofuErrorCategory string

const (
	BaofuErrorCategoryUserActionRequired    BaofuErrorCategory = "user_action_required"
	BaofuErrorCategoryPlatformConfiguration BaofuErrorCategory = "platform_configuration"
	BaofuErrorCategoryRetryable             BaofuErrorCategory = "retryable"
	BaofuErrorCategoryManualReview          BaofuErrorCategory = "manual_review"
)

type ClassifiedError struct {
	Code          string
	Category      BaofuErrorCategory
	PublicMessage string
	PublicAction  string
	Retryable     bool
}

type baofuErrorRule struct {
	category BaofuErrorCategory
	message  string
	action   string
	retry    bool
}

const (
	baofuMessageCheckAndResubmit       = "资料信息不完整，请核对后重新提交"
	baofuMessageIdentityCheckFailed    = "身份或银行卡信息核验未通过，请核对后重新提交"
	baofuMessagePaymentChannelPending  = "商户微信支付通道待开通，请联系平台处理"
	baofuMessagePaymentConfigPending   = "支付通道配置待开通，请联系平台处理"
	baofuMessagePaymentProcessing      = "支付通道处理中，请稍后重试"
	baofuMessageTradeResultProcessing  = "交易结果处理中，请稍后查询"
	baofuMessageOrderCreated           = "支付订单已创建，请返回订单页查看支付状态"
	baofuMessagePaymentChannelAbnormal = "支付通道异常，请联系平台处理"

	baofuActionCheckAndResubmit = "check_and_resubmit"
	baofuActionContactPlatform  = "contact_platform"
	baofuActionRetryLater       = "retry_later"
	baofuActionQueryLater       = "query_later"
	baofuActionQueryOrder       = "query_order"
)

var baofuOfficialErrorRules = map[string]baofuErrorRule{
	// Account retCode / parameter and validation codes.
	"0":                         {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageCheckAndResubmit, action: baofuActionCheckAndResubmit},
	"FAIL":                      {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageCheckAndResubmit, action: baofuActionCheckAndResubmit},
	"PARAM_ERROR":               {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageCheckAndResubmit, action: baofuActionCheckAndResubmit},
	"PARAMS_ERROR":              {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageCheckAndResubmit, action: baofuActionCheckAndResubmit},
	"INVALID_PARAM":             {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageCheckAndResubmit, action: baofuActionCheckAndResubmit},
	"INVALID_PARAMETER":         {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageCheckAndResubmit, action: baofuActionCheckAndResubmit},
	"ARGUMENT_INVALID":          {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageCheckAndResubmit, action: baofuActionCheckAndResubmit},
	"PARAMETER_VALID_NOT_PASS":  {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageCheckAndResubmit, action: baofuActionCheckAndResubmit},
	"PARAMETER_VALID":           {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageCheckAndResubmit, action: baofuActionCheckAndResubmit},
	"BF0001":                    {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageCheckAndResubmit, action: baofuActionCheckAndResubmit},
	"BF00062":                   {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageCheckAndResubmit, action: baofuActionCheckAndResubmit},
	"BF00064":                   {category: BaofuErrorCategoryManualReview, message: baofuMessagePaymentProcessing, action: baofuActionQueryLater},
	"BF00107":                   {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageCheckAndResubmit, action: baofuActionCheckAndResubmit},
	"BF00108":                   {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageCheckAndResubmit, action: baofuActionCheckAndResubmit},
	"BF00110":                   {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageCheckAndResubmit, action: baofuActionCheckAndResubmit},
	"ORDER_CREATED_FAIL":        {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageCheckAndResubmit, action: baofuActionCheckAndResubmit},
	"AGENT_RELATION_NOT_EXISTS": {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageCheckAndResubmit, action: baofuActionCheckAndResubmit},
	"DEPLOY_NOT_CORRECT":        {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageCheckAndResubmit, action: baofuActionCheckAndResubmit},
	"FEE_MER_ID_ERROR":          {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageCheckAndResubmit, action: baofuActionCheckAndResubmit},
	"PAY_CODE_ERROR":            {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageCheckAndResubmit, action: baofuActionCheckAndResubmit},
	"NOT_SUPPORT_PAY_CODE":      {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageCheckAndResubmit, action: baofuActionCheckAndResubmit},
	"SHARE_INFO_NOT_CORRECT":    {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageCheckAndResubmit, action: baofuActionCheckAndResubmit},
	"REFUND_AMT_EXCEEDS":        {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageCheckAndResubmit, action: baofuActionCheckAndResubmit},
	"TRADE_AMT_EXCEEDS_LIMIT":   {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageCheckAndResubmit, action: baofuActionCheckAndResubmit},
	"ORDER_NOT_SUPPORT_REFUNDS": {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageCheckAndResubmit, action: baofuActionCheckAndResubmit},
	"NOT_SUPPORT_CONCURRENT":    {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageCheckAndResubmit, action: baofuActionCheckAndResubmit},
	"ID_CARD_CHECK_FAILED":      {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageIdentityCheckFailed, action: baofuActionCheckAndResubmit},
	"BF00105":                   {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageIdentityCheckFailed, action: baofuActionCheckAndResubmit},
	"BF00063":                   {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageIdentityCheckFailed, action: baofuActionCheckAndResubmit},
	"BF00111":                   {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageIdentityCheckFailed, action: baofuActionCheckAndResubmit},
	"BF00106":                   {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageIdentityCheckFailed, action: baofuActionCheckAndResubmit},
	"BF00061":                   {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageIdentityCheckFailed, action: baofuActionCheckAndResubmit},
	"BF00217":                   {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageIdentityCheckFailed, action: baofuActionCheckAndResubmit},
	"BF00218":                   {category: BaofuErrorCategoryUserActionRequired, message: baofuMessageIdentityCheckFailed, action: baofuActionCheckAndResubmit},
	"EXISTED_LOGIN_NO":          {category: BaofuErrorCategoryUserActionRequired, message: "该主体已存在宝付开户记录，请联系平台核对账户状态", action: baofuActionContactPlatform},
	"BF00060":                   {category: BaofuErrorCategoryUserActionRequired, message: "该主体已存在宝付开户记录，请联系平台核对账户状态", action: baofuActionContactPlatform},
	"REPEATED_REQUEST":          {category: BaofuErrorCategoryRetryable, message: baofuMessageOrderCreated, action: baofuActionQueryOrder, retry: true},
	"BF0013":                    {category: BaofuErrorCategoryRetryable, message: baofuMessageOrderCreated, action: baofuActionQueryOrder, retry: true},

	// Product, merchant, terminal, signing, and report configuration codes.
	"MERCHANT_NOT_REPORTED":   {category: BaofuErrorCategoryPlatformConfiguration, message: baofuMessagePaymentChannelPending, action: baofuActionContactPlatform},
	"SUB_MCH_NOT_REPORTED":    {category: BaofuErrorCategoryPlatformConfiguration, message: baofuMessagePaymentChannelPending, action: baofuActionContactPlatform},
	"MERCHANT_NOT_REPORT":     {category: BaofuErrorCategoryPlatformConfiguration, message: baofuMessagePaymentChannelPending, action: baofuActionContactPlatform},
	"MERCHANT_REPORT_LIMIT":   {category: BaofuErrorCategoryPlatformConfiguration, message: "该主体已有微信渠道报备记录，请联系平台核对开通状态", action: baofuActionContactPlatform},
	"MERCHANT_NOT_AUTHORIZED": {category: BaofuErrorCategoryPlatformConfiguration, message: baofuMessagePaymentChannelPending, action: baofuActionContactPlatform},
	"NO_AUTH":                 {category: BaofuErrorCategoryPlatformConfiguration, message: baofuMessagePaymentChannelPending, action: baofuActionContactPlatform},
	"PAY_CHANNEL_NOT_SUPPORT": {category: BaofuErrorCategoryPlatformConfiguration, message: baofuMessagePaymentChannelPending, action: baofuActionContactPlatform},
	"MCH_NOT_EXISTS":          {category: BaofuErrorCategoryPlatformConfiguration, message: baofuMessagePaymentConfigPending, action: baofuActionContactPlatform},
	"MERCHANT_NOT_EXIST":      {category: BaofuErrorCategoryPlatformConfiguration, message: baofuMessagePaymentConfigPending, action: baofuActionContactPlatform},
	"TERMINAL_NOT_EXIST":      {category: BaofuErrorCategoryPlatformConfiguration, message: baofuMessagePaymentConfigPending, action: baofuActionContactPlatform},
	"MERID_TERID_NOT_MATCH":   {category: BaofuErrorCategoryPlatformConfiguration, message: baofuMessagePaymentConfigPending, action: baofuActionContactPlatform},
	"UNOPENED_PRODUCT":        {category: BaofuErrorCategoryPlatformConfiguration, message: baofuMessagePaymentConfigPending, action: baofuActionContactPlatform},
	"SHARE_DEPLOY_NOT_EXIST":  {category: BaofuErrorCategoryPlatformConfiguration, message: baofuMessagePaymentConfigPending, action: baofuActionContactPlatform},
	"BF00214":                 {category: BaofuErrorCategoryPlatformConfiguration, message: baofuMessagePaymentConfigPending, action: baofuActionContactPlatform},
	"BF00077":                 {category: BaofuErrorCategoryPlatformConfiguration, message: baofuMessagePaymentConfigPending, action: baofuActionContactPlatform},
	"BF00058":                 {category: BaofuErrorCategoryPlatformConfiguration, message: baofuMessagePaymentConfigPending, action: baofuActionContactPlatform},
	"BF00059":                 {category: BaofuErrorCategoryPlatformConfiguration, message: baofuMessagePaymentConfigPending, action: baofuActionContactPlatform},
	"VERIFY_ERROR":            {category: BaofuErrorCategoryPlatformConfiguration, message: baofuMessagePaymentConfigPending, action: baofuActionContactPlatform},
	"DGTL_DEC_ERROR":          {category: BaofuErrorCategoryPlatformConfiguration, message: baofuMessagePaymentConfigPending, action: baofuActionContactPlatform},

	// Retryable or queryable provider states.
	"2":                  {category: BaofuErrorCategoryRetryable, message: baofuMessagePaymentProcessing, action: baofuActionRetryLater, retry: true},
	"PROCESSING":         {category: BaofuErrorCategoryRetryable, message: baofuMessagePaymentProcessing, action: baofuActionRetryLater, retry: true},
	"ABNORMAL":           {category: BaofuErrorCategoryRetryable, message: baofuMessagePaymentProcessing, action: baofuActionRetryLater, retry: true},
	"SYSTEM_BUSY":        {category: BaofuErrorCategoryRetryable, message: baofuMessagePaymentProcessing, action: baofuActionRetryLater, retry: true},
	"SYSTEM_ERROR":       {category: BaofuErrorCategoryRetryable, message: baofuMessagePaymentProcessing, action: baofuActionRetryLater, retry: true},
	"SYSTEM_INNER_ERROR": {category: BaofuErrorCategoryRetryable, message: baofuMessagePaymentProcessing, action: baofuActionRetryLater, retry: true},
	"TIMEOUT":            {category: BaofuErrorCategoryRetryable, message: baofuMessagePaymentProcessing, action: baofuActionRetryLater, retry: true},
	"BF0005":             {category: BaofuErrorCategoryRetryable, message: baofuMessagePaymentProcessing, action: baofuActionRetryLater, retry: true},
	"BF0002":             {category: BaofuErrorCategoryRetryable, message: baofuMessagePaymentProcessing, action: baofuActionRetryLater, retry: true},
	"TRADE_UNCONFIRMED":  {category: BaofuErrorCategoryRetryable, message: baofuMessageTradeResultProcessing, action: baofuActionQueryLater, retry: true},
	"ORDER_EXIST":        {category: BaofuErrorCategoryRetryable, message: baofuMessageOrderCreated, action: baofuActionQueryOrder, retry: true},

	// Manual review or provider/channel decision codes.
	"RISK_REFUSED":       {category: BaofuErrorCategoryManualReview, message: baofuMessagePaymentChannelAbnormal, action: baofuActionContactPlatform},
	"CHANNEL_RETURN_ERR": {category: BaofuErrorCategoryManualReview, message: baofuMessagePaymentChannelAbnormal, action: baofuActionContactPlatform},
	"ORDER_NOT_EXIST":    {category: BaofuErrorCategoryManualReview, message: baofuMessagePaymentChannelAbnormal, action: baofuActionContactPlatform},
}

func ClassifyBaofuError(code string, upstreamMessage string) ClassifiedError {
	_ = upstreamMessage
	canonical := strings.ToUpper(strings.TrimSpace(code))
	classified := ClassifiedError{Code: canonical}
	if rule, ok := baofuOfficialErrorRules[canonical]; ok {
		classified.Category = rule.category
		classified.PublicMessage = rule.message
		classified.PublicAction = rule.action
		classified.Retryable = rule.retry
		return classified
	}
	classified.Category = BaofuErrorCategoryManualReview
	classified.PublicMessage = baofuMessagePaymentChannelAbnormal
	classified.PublicAction = baofuActionContactPlatform
	return classified
}

func BaofuCommandMessage(code string, upstreamMessage string) string {
	classified := ClassifyBaofuError(code, upstreamMessage)
	message := strings.TrimSpace(classified.PublicMessage)
	if action := strings.TrimSpace(classified.PublicAction); action != "" {
		message = strings.TrimSpace(message + "，" + action)
	}
	return message
}

func (c ClassifiedError) FrontendGuidance() FrontendGuidance {
	return FrontendGuidance{
		Code:      "BAOFU_" + strings.ToUpper(string(c.Category)),
		Message:   c.PublicMessage,
		Action:    c.PublicAction,
		Retryable: c.Retryable,
	}
}
