package wechat

import (
	"context"
	"mime/multipart"

	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
)

const (
	SecCheckMediaTypeVoice = 1
	SecCheckMediaTypeImage = 2
)

type MediaCheckAsyncRequest struct {
	MediaURL  string
	MediaType int
	Version   int
	OpenID    string
	Scene     int
}

type MediaCheckAsyncResponse struct {
	TraceID string `json:"trace_id"`
}

// WechatClient 微信客户端接口，便于测试mock
type WechatClient interface {
	// Code2Session 使用code换取openid和session_key
	Code2Session(ctx context.Context, code string) (*Code2SessionResponse, error)

	// ImgSecCheck 图片内容安全识别（用于头像/证照/证据等上传前审核）
	ImgSecCheck(ctx context.Context, imgFile multipart.File) error

	// MsgSecCheck 文本内容安全检测（msg_sec_check v2）
	// 返回 nil 表示通过；返回 ErrRiskyTextContent 表示不通过；其他 error 表示调用失败。
	MsgSecCheck(ctx context.Context, openid string, scene int, content string) error

	// MediaCheckAsync 异步检测图片/音频内容安全。
	MediaCheckAsync(ctx context.Context, req MediaCheckAsyncRequest) (*MediaCheckAsyncResponse, error)

	// OCRBusinessLicense 识别营业执照
	OCRBusinessLicense(ctx context.Context, imgFile multipart.File) (*BusinessLicenseOCRResponse, error)

	// OCRIDCard 识别身份证
	OCRIDCard(ctx context.Context, imgFile multipart.File, cardSide string) (*IDCardOCRResponse, error)

	// OCRPrintedText 通用印刷体识别（用于食品经营许可证等）
	OCRPrintedText(ctx context.Context, imgFile multipart.File) (*PrintedTextOCRResponse, error)

	// GetWXACodeUnlimited 获取小程序码（不限量版本）
	// scene: 场景参数，page: 跳转页面路径
	// 返回PNG图片数据
	GetWXACodeUnlimited(ctx context.Context, req *WXACodeRequest) ([]byte, error)

	// UploadShippingInfo 上传单商户支付发货信息（同城配送）
	// 在骑手取货后调用，logistics_type=2（同城配送）
	UploadShippingInfo(ctx context.Context, req *UploadShippingInfoRequest) error

	// UploadCombinedShippingInfo 上传合单支付发货信息（同城配送）
	UploadCombinedShippingInfo(ctx context.Context, req *UploadCombinedShippingInfoRequest) error
}

// DirectPaymentClientInterface 直连支付客户端接口
// 用于骑手押金、追偿付款及其配套退款/通知场景
type DirectPaymentClientInterface interface {
	// GetMchID 获取直连支付商户号
	GetMchID() string

	// GetAppID 获取直连支付 AppID
	GetAppID() string

	// CreateJSAPIOrder 创建 JSAPI 订单（小程序支付）
	CreateJSAPIOrder(ctx context.Context, req *wechatcontracts.DirectJSAPIOrderRequest) (*wechatcontracts.DirectJSAPIOrderResponse, *JSAPIPayParams, error)

	// QueryOrderByOutTradeNo 根据商户订单号查询订单
	QueryOrderByOutTradeNo(ctx context.Context, outTradeNo string) (*wechatcontracts.DirectOrderQueryResponse, error)

	// CloseOrder 关闭订单
	CloseOrder(ctx context.Context, outTradeNo string) error

	// CreateRefund 申请退款
	CreateRefund(ctx context.Context, req *RefundRequest) (*RefundResponse, error)

	// QueryRefund 查询退款
	QueryRefund(ctx context.Context, outRefundNo string) (*RefundResponse, error)

	// DecryptPaymentNotification 解密支付通知
	DecryptPaymentNotification(notification *PaymentNotification) (*wechatcontracts.DirectPaymentNotificationResource, error)

	// DecryptRefundNotification 解密退款通知
	DecryptRefundNotification(notification *PaymentNotification) (*wechatcontracts.DirectRefundNotificationResource, error)

	// DecryptNotificationRaw 解密通知原始数据（返回 JSON 字节）
	DecryptNotificationRaw(notification *PaymentNotification) ([]byte, error)

	// VerifyNotificationSignature 验证微信支付回调签名
	VerifyNotificationSignature(signature, timestamp, nonce, serial, body string) error

	// GenerateJSAPIPayParams 根据 prepay_id 重新生成小程序调起支付所需参数（用于幂等返回旧 pending 记录时重新签名）
	GenerateJSAPIPayParams(prepayID string) (*JSAPIPayParams, error)
}

// TransferClientInterface 商家转账客户端接口。
// 用于索赔赔付等转账到微信零钱场景，不属于直连 JSAPI 支付主链路。
type TransferClientInterface interface {
	// GetMchID 获取商家转账商户号。
	GetMchID() string

	// GetAppID 获取商家转账 AppID。
	GetAppID() string

	// CreateTransfer 发起转账（商家转账到零钱）。
	CreateTransfer(ctx context.Context, req *wechatcontracts.DirectMerchantTransferCreateRequest) (*wechatcontracts.DirectMerchantTransferCreateResponse, error)

	// QueryTransferByOutBillNo 按商户单号查询转账单状态。
	QueryTransferByOutBillNo(ctx context.Context, outBillNo string) (*wechatcontracts.DirectMerchantTransferQueryResponse, error)

	// VerifyNotificationSignature 验证商家转账回调签名。
	VerifyNotificationSignature(signature, timestamp, nonce, serial, body string) error

	// DecryptMerchantTransferNotification 解密商家转账回调通知资源。
	DecryptMerchantTransferNotification(notification *PaymentNotification) (*wechatcontracts.DirectMerchantTransferNotificationResource, error)
}

// EcommerceClientInterface 平台收付通客户端接口
// 用于订单支付分账场景：外卖、堂食、预定等
type EcommerceClientInterface interface {
	// GetSpMchID 获取服务商商户号
	GetSpMchID() string

	// GetSpAppID 获取服务商AppID
	GetSpAppID() string

	// GetSpMchName 获取服务商名称
	GetSpMchName() string

	// GetPlatformPublicKeyID 获取请求头 Wechatpay-Serial 所需的平台公钥 ID。
	GetPlatformPublicKeyID() string

	// GenerateJSAPIPayParams 根据 prepay_id 重新生成小程序调起支付所需参数（用于幂等返回旧 pending 记录时重新签名）
	GenerateJSAPIPayParams(prepayID string) (*JSAPIPayParams, error)

	// EncryptSensitiveData 使用微信支付平台公钥加密敏感数据。
	EncryptSensitiveData(plaintext string) (string, error)

	// UploadImage 上传图片到微信支付获取 MediaID
	UploadImage(ctx context.Context, filename string, fileData []byte) (*ImageUploadResponse, error)

	// ==================== 二级商户进件 ====================
	// CreateEcommerceApplyment 提交二级商户进件申请
	CreateEcommerceApplyment(ctx context.Context, req *EcommerceApplymentRequest) (*wechatcontracts.EcommerceApplymentResponse, error)

	// QueryEcommerceApplymentByID 通过申请单号查询进件状态
	QueryEcommerceApplymentByID(ctx context.Context, applymentID int64) (*wechatcontracts.EcommerceApplymentQueryResponse, error)

	// QueryEcommerceApplymentByOutRequestNo 通过业务申请编号查询进件状态
	QueryEcommerceApplymentByOutRequestNo(ctx context.Context, outRequestNo string) (*wechatcontracts.EcommerceApplymentQueryResponse, error)

	// QuerySubMerchantSettlement 查询特约商户/二级商户结算账户信息（敏感信息掩码）
	// accountNumberRule 为空时使用微信默认规则（ACCOUNT_NUMBER_RULE_MASK_V1）
	QuerySubMerchantSettlement(ctx context.Context, subMchID string, accountNumberRule string) (*wechatcontracts.SubMerchantSettlementResponse, error)

	// ModifySubMerchantSettlement 修改特约商户/二级商户结算账户
	ModifySubMerchantSettlement(ctx context.Context, subMchID string, req *wechatcontracts.ModifySubMerchantSettlementRequest) (*wechatcontracts.ModifySubMerchantSettlementResponse, error)

	// QuerySubMerchantSettlementApplication 查询结算账户修改申请单状态
	// applicationNo: 修改申请单号；accountNumberRule: 账号展示规则（空字符串使用微信默认）
	QuerySubMerchantSettlementApplication(ctx context.Context, subMchID, applicationNo, accountNumberRule string) (*wechatcontracts.QuerySubMerchantSettlementApplicationResponse, error)

	// ListPersonalBankingBanks 查询支持个人业务的银行列表
	ListPersonalBankingBanks(ctx context.Context, offset, limit int) (*wechatcontracts.CapitalBankListResponse, error)

	// ListCorporateBankingBanks 查询支持对公业务的银行列表
	ListCorporateBankingBanks(ctx context.Context, offset, limit int) (*wechatcontracts.CapitalBankListResponse, error)

	// SearchBanksByBankAccount 根据个人银行卡号识别开户银行候选
	SearchBanksByBankAccount(ctx context.Context, accountNumber string) (*wechatcontracts.CapitalBankAccountSearchResponse, error)

	// ListProvinceAreas 查询省份列表
	ListProvinceAreas(ctx context.Context) (*wechatcontracts.CapitalProvinceListResponse, error)

	// ListCityAreas 查询省份下城市列表
	ListCityAreas(ctx context.Context, provinceCode int) (*wechatcontracts.CapitalCityListResponse, error)

	// ListBankBranches 查询支行列表
	ListBankBranches(ctx context.Context, bankAliasCode string, cityCode, offset, limit int) (*wechatcontracts.CapitalBranchListResponse, error)

	// ==================== 合单支付 ====================
	// CreatePartnerJSAPIOrder 创建服务商模式单笔 JSAPI 订单（平台收付通）
	CreatePartnerJSAPIOrder(ctx context.Context, req *wechatcontracts.PartnerJSAPIOrderRequest) (*wechatcontracts.PartnerJSAPIOrderResponse, *JSAPIPayParams, error)

	// QueryPartnerOrderByTransactionID 通过微信支付订单号查询服务商模式单笔订单
	QueryPartnerOrderByTransactionID(ctx context.Context, transactionID, subMchID string) (*wechatcontracts.PartnerOrderQueryResponse, error)

	// QueryPartnerOrderByOutTradeNo 通过商户订单号查询服务商模式单笔订单
	QueryPartnerOrderByOutTradeNo(ctx context.Context, outTradeNo, subMchID string) (*wechatcontracts.PartnerOrderQueryResponse, error)

	// ClosePartnerOrder 关闭服务商模式单笔订单
	ClosePartnerOrder(ctx context.Context, outTradeNo, subMchID string) error

	// CreateCombineOrder 创建合单订单（平台收付通）
	CreateCombineOrder(ctx context.Context, req *wechatcontracts.CombineOrderRequest) (*wechatcontracts.CombineOrderResponse, *JSAPIPayParams, error)

	// QueryCombineOrder 查询合单订单
	QueryCombineOrder(ctx context.Context, combineOutTradeNo string) (*wechatcontracts.CombineQueryResponse, error)

	// CloseCombineOrder 关闭合单订单
	CloseCombineOrder(ctx context.Context, combineOutTradeNo string, subOrders []wechatcontracts.SubOrderClose) error

	// ==================== 分账 ====================
	// CreateProfitSharing 请求分账
	CreateProfitSharing(ctx context.Context, req *wechatcontracts.ProfitSharingRequest) (*wechatcontracts.ProfitSharingResponse, error)

	// QueryProfitSharing 查询分账结果
	QueryProfitSharing(ctx context.Context, subMchID, transactionID, outOrderNo string) (*wechatcontracts.ProfitSharingQueryResponse, error)

	// QueryProfitSharingAmounts 查询订单剩余待分账金额
	QueryProfitSharingAmounts(ctx context.Context, transactionID string) (*wechatcontracts.ProfitSharingAmountsResponse, error)

	// FinishProfitSharing 完结分账
	FinishProfitSharing(ctx context.Context, subMchID, transactionID, outOrderNo, description string) (*wechatcontracts.ProfitSharingResponse, error)

	// ==================== 分账接收方管理 ====================
	// AddProfitSharingReceiver 添加分账接收方
	AddProfitSharingReceiver(ctx context.Context, req *wechatcontracts.AddReceiverRequest) (*wechatcontracts.AddReceiverResponse, error)

	// DeleteProfitSharingReceiver 删除分账接收方
	DeleteProfitSharingReceiver(ctx context.Context, req *wechatcontracts.DeleteReceiverRequest) (*wechatcontracts.DeleteReceiverResponse, error)

	// ==================== 分账回退 ====================
	// CreateProfitSharingReturn 请求分账回退
	CreateProfitSharingReturn(ctx context.Context, req *wechatcontracts.ProfitSharingReturnRequest) (*wechatcontracts.ProfitSharingReturnResponse, error)

	// QueryProfitSharingReturn 查询分账回退结果
	QueryProfitSharingReturn(ctx context.Context, subMchID, outReturnNo, outOrderNo string) (*wechatcontracts.ProfitSharingReturnResponse, error)

	// ==================== 退款 ====================
	// CreateEcommerceRefund 申请电商退款
	CreateEcommerceRefund(ctx context.Context, req *EcommerceRefundRequest) (*EcommerceRefundResponse, error)

	// ApplyEcommerceAbnormalRefund 发起电商异常退款处理
	ApplyEcommerceAbnormalRefund(ctx context.Context, req *EcommerceAbnormalRefundRequest) (*EcommerceRefundResponse, error)

	// QueryEcommerceRefund 查询电商退款
	QueryEcommerceRefund(ctx context.Context, subMchID, outRefundNo string) (*EcommerceRefundResponse, error)

	// ==================== 账户资金管理 ====================
	// ValidateEcommerceCancelWithdraw 校验二级商户是否满足注销提现条件
	ValidateEcommerceCancelWithdraw(ctx context.Context, subMchID string) (*wechatcontracts.CancelWithdrawEligibilityResponse, error)

	// CreateEcommerceCancelWithdraw 提交商户注销提现申请
	CreateEcommerceCancelWithdraw(ctx context.Context, req *wechatcontracts.CancelWithdrawRequest) (*wechatcontracts.CancelWithdrawCreateResponse, error)

	// QueryEcommerceCancelWithdrawByOutRequestNo 按平台申请单号查询注销提现申请状态
	QueryEcommerceCancelWithdrawByOutRequestNo(ctx context.Context, outRequestNo string) (*wechatcontracts.CancelWithdrawQueryResponse, error)

	// QueryEcommerceCancelWithdrawByApplymentID 按微信申请单号查询注销提现申请状态
	QueryEcommerceCancelWithdrawByApplymentID(ctx context.Context, applymentID string) (*wechatcontracts.CancelWithdrawQueryResponse, error)

	// QueryEcommerceFundBalance 查询二级商户可用余额
	QueryEcommerceFundBalance(ctx context.Context, subMchID string) (*wechatcontracts.EcommerceFundBalanceResponse, error)

	// QueryEcommerceFundBalanceByAccountType 按账户类型查询二级商户实时余额
	QueryEcommerceFundBalanceByAccountType(ctx context.Context, subMchID, accountType string) (*wechatcontracts.EcommerceFundBalanceResponse, error)

	// QueryEcommerceFundDayEndBalance 查询二级商户指定日期日终余额
	QueryEcommerceFundDayEndBalance(ctx context.Context, subMchID, date, accountType string) (*wechatcontracts.EcommerceFundBalanceResponse, error)

	// QueryPlatformFundBalance 查询平台商户实时余额
	QueryPlatformFundBalance(ctx context.Context, accountType string) (*wechatcontracts.PlatformFundBalanceResponse, error)

	// QueryPlatformFundDayEndBalance 查询平台商户指定日期日终余额
	QueryPlatformFundDayEndBalance(ctx context.Context, accountType, date string) (*wechatcontracts.PlatformFundBalanceResponse, error)

	// CreateEcommerceWithdraw 发起二级商户提现
	CreateEcommerceWithdraw(ctx context.Context, req *wechatcontracts.EcommerceWithdrawRequest) (*wechatcontracts.EcommerceWithdrawCreateResponse, error)

	// QueryEcommerceWithdrawByOutRequestNo 通过外部申请单号查询提现状态
	QueryEcommerceWithdrawByOutRequestNo(ctx context.Context, subMchID, outRequestNo string) (*wechatcontracts.EcommerceWithdrawQueryResponse, error)

	// ==================== 商户违规通知 ====================
	// QueryViolationNotification 查询商户违规通知回调地址
	QueryViolationNotification(ctx context.Context) (*ViolationNotificationConfigResponse, error)

	// CreateViolationNotification 创建商户违规通知回调地址
	CreateViolationNotification(ctx context.Context, req *ViolationNotificationConfigRequest) (*ViolationNotificationConfigResponse, error)

	// UpdateViolationNotification 修改商户违规通知回调地址
	UpdateViolationNotification(ctx context.Context, req *ViolationNotificationConfigRequest) (*ViolationNotificationConfigResponse, error)

	// DeleteViolationNotification 删除商户违规通知回调地址
	DeleteViolationNotification(ctx context.Context) error

	// ==================== 通知解密 ====================
	// DecryptPartnerPaymentNotification 解密服务商模式单笔支付通知
	DecryptPartnerPaymentNotification(notification *PaymentNotification) (*wechatcontracts.PartnerPaymentNotificationResource, error)

	// DecryptCombinePaymentNotification 解密合单支付通知
	DecryptCombinePaymentNotification(notification *PaymentNotification) (*wechatcontracts.CombinePaymentNotification, error)

	// DecryptProfitSharingNotification 解密分账通知
	DecryptProfitSharingNotification(notification *PaymentNotification) (*wechatcontracts.ProfitSharingNotification, error)

	// DecryptEcommerceRefundNotification 解密电商退款通知
	DecryptEcommerceRefundNotification(notification *PaymentNotification) (*EcommerceRefundNotification, error)

	// DecryptNotificationRaw 解密通知原始数据（返回 JSON 字节）
	DecryptNotificationRaw(notification *PaymentNotification) ([]byte, error)

	// DecryptSettlementNotification 解密结算事件通知（trade_manage_order_settlement）
	DecryptSettlementNotification(notification *PaymentNotification) (*SettlementNotificationResource, error)

	// DecryptComplaintNotification 解密用户投诉通知
	DecryptComplaintNotification(notification *PaymentNotification) (*ComplaintNotification, error)

	// DecryptViolationNotification 解密商户违规通知
	DecryptViolationNotification(notification *PaymentNotification) (*ViolationNotificationResource, error)

	// VerifyNotificationSignature 验证微信支付回调签名
	VerifyNotificationSignature(signature, timestamp, nonce, serial, body string) error

	// ==================== 用户投诉 ====================
	// ListComplaints 查询投诉单列表（分页）
	ListComplaints(ctx context.Context, req ListComplaintsRequest) (*ListComplaintsResponse, error)
	// GetComplaintDetail 查询投诉单详情
	GetComplaintDetail(ctx context.Context, complaintID string) (*ComplaintDetail, error)
	// RespondComplaint 回复投诉
	RespondComplaint(ctx context.Context, req ComplaintResponseRequest) error
	// CompleteComplaint 完结投诉
	CompleteComplaint(ctx context.Context, complaintID string) error

	// ==================== 补差 ====================
	// CreateSubsidy 向二级商户发起补差（平台出资营销）
	CreateSubsidy(ctx context.Context, req wechatcontracts.SubsidyRequest) (*wechatcontracts.SubsidyResponse, error)
	// ReturnSubsidy 退回补差（退款时回收平台补贴款）
	ReturnSubsidy(ctx context.Context, req wechatcontracts.SubsidyReturnRequest) (*wechatcontracts.SubsidyReturnResponse, error)
	// CancelSubsidy 取消补差（尚未分账前可取消）
	CancelSubsidy(ctx context.Context, req wechatcontracts.SubsidyCancelRequest) (*wechatcontracts.SubsidyCancelResponse, error)
}

// 确保 *Client 实现了 WechatClient 接口
var _ WechatClient = (*Client)(nil)

// 确保 *DirectPaymentClient 实现了 DirectPaymentClientInterface 接口
var _ DirectPaymentClientInterface = (*DirectPaymentClient)(nil)

// 确保 *EcommerceClient 实现了 EcommerceClientInterface 接口
var _ EcommerceClientInterface = (*EcommerceClient)(nil)
