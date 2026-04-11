package wechat

import (
	"context"
	"mime/multipart"
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

// PaymentClientInterface 微信支付客户端接口（小程序直连支付）
// 用于押金、充值等平台直接收款场景
type PaymentClientInterface interface {
	// GetMchID 获取直连支付商户号
	GetMchID() string

	// GetAppID 获取直连支付 AppID
	GetAppID() string

	// CreateJSAPIOrder 创建 JSAPI 订单（小程序支付）
	CreateJSAPIOrder(ctx context.Context, req *JSAPIOrderRequest) (*JSAPIOrderResponse, *JSAPIPayParams, error)

	// QueryOrderByOutTradeNo 根据商户订单号查询订单
	QueryOrderByOutTradeNo(ctx context.Context, outTradeNo string) (*OrderQueryResponse, error)

	// CloseOrder 关闭订单
	CloseOrder(ctx context.Context, outTradeNo string) error

	// CreateRefund 申请退款
	CreateRefund(ctx context.Context, req *RefundRequest) (*RefundResponse, error)

	// QueryRefund 查询退款
	QueryRefund(ctx context.Context, outRefundNo string) (*RefundResponse, error)

	// CreateTransfer 发起转账（商家转账到零钱）
	CreateTransfer(ctx context.Context, req *TransferRequest) (*TransferResponse, error)

	// QueryTransfer 按商户批次单号查询转账批次状态
	QueryTransfer(ctx context.Context, outBatchNo string) (*TransferQueryResponse, error)

	// DecryptPaymentNotification 解密支付通知
	DecryptPaymentNotification(notification *PaymentNotification) (*PaymentNotificationResource, error)

	// DecryptRefundNotification 解密退款通知
	DecryptRefundNotification(notification *PaymentNotification) (*RefundNotificationResource, error)

	// DecryptNotificationRaw 解密通知原始数据（返回 JSON 字节）
	DecryptNotificationRaw(notification *PaymentNotification) ([]byte, error)

	// VerifyNotificationSignature 验证微信支付回调签名
	VerifyNotificationSignature(signature, timestamp, nonce, body string) error

	// GenerateJSAPIPayParams 根据 prepay_id 重新生成小程序调起支付所需参数（用于幂等返回旧 pending 记录时重新签名）
	GenerateJSAPIPayParams(prepayID string) (*JSAPIPayParams, error)
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

	// GetPlatformCertificateSerial 获取微信支付平台证书序列号
	GetPlatformCertificateSerial() string

	// GenerateJSAPIPayParams 根据 prepay_id 重新生成小程序调起支付所需参数（用于幂等返回旧 pending 记录时重新签名）
	GenerateJSAPIPayParams(prepayID string) (*JSAPIPayParams, error)

	// EncryptSensitiveData 使用微信支付平台证书公钥加密敏感数据
	EncryptSensitiveData(plaintext string) (string, error)

	// UploadImage 上传图片到微信支付获取 MediaID
	UploadImage(ctx context.Context, filename string, fileData []byte) (*ImageUploadResponse, error)

	// ==================== 二级商户进件 ====================
	// CreateEcommerceApplyment 提交二级商户进件申请
	CreateEcommerceApplyment(ctx context.Context, req *EcommerceApplymentRequest) (*EcommerceApplymentResponse, error)

	// QueryEcommerceApplymentByID 通过申请单号查询进件状态
	QueryEcommerceApplymentByID(ctx context.Context, applymentID int64) (*EcommerceApplymentQueryResponse, error)

	// QueryEcommerceApplymentByOutRequestNo 通过业务申请编号查询进件状态
	QueryEcommerceApplymentByOutRequestNo(ctx context.Context, outRequestNo string) (*EcommerceApplymentQueryResponse, error)

	// QuerySubMerchantSettlement 查询特约商户/二级商户结算账户信息（敏感信息掩码）
	// accountNumberRule 为空时使用微信默认规则（ACCOUNT_NUMBER_RULE_MASK_V1）
	QuerySubMerchantSettlement(ctx context.Context, subMchID string, accountNumberRule string) (*SubMerchantSettlementResponse, error)

	// ModifySubMerchantSettlement 修改特约商户/二级商户结算账户
	ModifySubMerchantSettlement(ctx context.Context, subMchID string, req *ModifySubMerchantSettlementRequest) (*ModifySubMerchantSettlementResponse, error)

	// QuerySubMerchantSettlementApplication 查询结算账户修改申请单状态
	// applicationNo: 修改申请单号；accountNumberRule: 账号展示规则（空字符串使用微信默认）
	QuerySubMerchantSettlementApplication(ctx context.Context, subMchID, applicationNo, accountNumberRule string) (*QuerySubMerchantSettlementApplicationResponse, error)

	// ListPersonalBankingBanks 查询支持个人业务的银行列表
	ListPersonalBankingBanks(ctx context.Context, offset, limit int) (*CapitalBankListResponse, error)

	// ListCorporateBankingBanks 查询支持对公业务的银行列表
	ListCorporateBankingBanks(ctx context.Context, offset, limit int) (*CapitalBankListResponse, error)

	// SearchBanksByBankAccount 根据个人银行卡号识别开户银行候选
	SearchBanksByBankAccount(ctx context.Context, accountNumber string) (*CapitalBankAccountSearchResponse, error)

	// ListProvinceAreas 查询省份列表
	ListProvinceAreas(ctx context.Context) (*CapitalProvinceListResponse, error)

	// ListCityAreas 查询省份下城市列表
	ListCityAreas(ctx context.Context, provinceCode int) (*CapitalCityListResponse, error)

	// ListBankBranches 查询支行列表
	ListBankBranches(ctx context.Context, bankAliasCode string, cityCode, offset, limit int) (*CapitalBranchListResponse, error)

	// ==================== 合单支付 ====================
	// CreatePartnerJSAPIOrder 创建服务商模式单笔 JSAPI 订单（平台收付通）
	CreatePartnerJSAPIOrder(ctx context.Context, req *PartnerJSAPIOrderRequest) (*PartnerJSAPIOrderResponse, *JSAPIPayParams, error)

	// QueryPartnerOrderByTransactionID 通过微信支付订单号查询服务商模式单笔订单
	QueryPartnerOrderByTransactionID(ctx context.Context, transactionID, subMchID string) (*PartnerOrderQueryResponse, error)

	// QueryPartnerOrderByOutTradeNo 通过商户订单号查询服务商模式单笔订单
	QueryPartnerOrderByOutTradeNo(ctx context.Context, outTradeNo, subMchID string) (*PartnerOrderQueryResponse, error)

	// ClosePartnerOrder 关闭服务商模式单笔订单
	ClosePartnerOrder(ctx context.Context, outTradeNo, subMchID string) error

	// CreateCombineOrder 创建合单订单（平台收付通）
	CreateCombineOrder(ctx context.Context, req *CombineOrderRequest) (*CombineOrderResponse, *JSAPIPayParams, error)

	// QueryCombineOrder 查询合单订单
	QueryCombineOrder(ctx context.Context, combineOutTradeNo string) (*CombineQueryResponse, error)

	// CloseCombineOrder 关闭合单订单
	CloseCombineOrder(ctx context.Context, combineOutTradeNo string, subOrders []SubOrderClose) error

	// ==================== 分账 ====================
	// CreateProfitSharing 请求分账
	CreateProfitSharing(ctx context.Context, req *ProfitSharingRequest) (*ProfitSharingResponse, error)

	// QueryProfitSharing 查询分账结果
	QueryProfitSharing(ctx context.Context, subMchID, transactionID, outOrderNo string) (*ProfitSharingQueryResponse, error)

	// QueryProfitSharingAmounts 查询订单剩余待分账金额
	QueryProfitSharingAmounts(ctx context.Context, transactionID string) (*ProfitSharingAmountsResponse, error)

	// FinishProfitSharing 完结分账
	FinishProfitSharing(ctx context.Context, subMchID, transactionID, outOrderNo, description string) (*ProfitSharingResponse, error)

	// ==================== 分账接收方管理 ====================
	// AddProfitSharingReceiver 添加分账接收方
	AddProfitSharingReceiver(ctx context.Context, req *AddReceiverRequest) (*AddReceiverResponse, error)

	// DeleteProfitSharingReceiver 删除分账接收方
	DeleteProfitSharingReceiver(ctx context.Context, req *DeleteReceiverRequest) (*DeleteReceiverResponse, error)

	// ==================== 分账回退 ====================
	// CreateProfitSharingReturn 请求分账回退
	CreateProfitSharingReturn(ctx context.Context, req *ProfitSharingReturnRequest) (*ProfitSharingReturnResponse, error)

	// QueryProfitSharingReturn 查询分账回退结果
	QueryProfitSharingReturn(ctx context.Context, subMchID, outReturnNo, outOrderNo string) (*ProfitSharingReturnResponse, error)

	// ==================== 退款 ====================
	// CreateRefund 申请直连退款（仅用于骑手押金等平台直收业务场景的退款）
	CreateRefund(ctx context.Context, req *RefundRequest) (*RefundResponse, error)

	// CreateEcommerceRefund 申请电商退款
	CreateEcommerceRefund(ctx context.Context, req *EcommerceRefundRequest) (*EcommerceRefundResponse, error)

	// ApplyEcommerceAbnormalRefund 发起电商异常退款处理
	ApplyEcommerceAbnormalRefund(ctx context.Context, req *EcommerceAbnormalRefundRequest) (*EcommerceRefundResponse, error)

	// QueryEcommerceRefund 查询电商退款
	QueryEcommerceRefund(ctx context.Context, subMchID, outRefundNo string) (*EcommerceRefundResponse, error)

	// ==================== 账户资金管理 ====================
	// QueryEcommerceFundBalance 查询二级商户可用余额
	QueryEcommerceFundBalance(ctx context.Context, subMchID string) (*EcommerceFundBalanceResponse, error)

	// QueryEcommerceFundBalanceByAccountType 按账户类型查询二级商户实时余额
	QueryEcommerceFundBalanceByAccountType(ctx context.Context, subMchID, accountType string) (*EcommerceFundBalanceResponse, error)

	// QueryEcommerceFundDayEndBalance 查询二级商户指定日期日终余额
	QueryEcommerceFundDayEndBalance(ctx context.Context, subMchID, date, accountType string) (*EcommerceFundBalanceResponse, error)

	// QueryPlatformFundBalance 查询平台商户实时余额
	QueryPlatformFundBalance(ctx context.Context, accountType string) (*PlatformFundBalanceResponse, error)

	// QueryPlatformFundDayEndBalance 查询平台商户指定日期日终余额
	QueryPlatformFundDayEndBalance(ctx context.Context, accountType, date string) (*PlatformFundBalanceResponse, error)

	// CreateEcommerceWithdraw 发起二级商户提现
	CreateEcommerceWithdraw(ctx context.Context, req *EcommerceWithdrawRequest) (*EcommerceWithdrawResponse, error)

	// QueryEcommerceWithdrawByOutRequestNo 通过外部申请单号查询提现状态
	QueryEcommerceWithdrawByOutRequestNo(ctx context.Context, subMchID, outRequestNo string) (*EcommerceWithdrawResponse, error)

	// ==================== 通知解密 ====================
	// DecryptPartnerPaymentNotification 解密服务商模式单笔支付通知
	DecryptPartnerPaymentNotification(notification *PaymentNotification) (*PartnerPaymentNotificationResource, error)

	// DecryptCombinePaymentNotification 解密合单支付通知
	DecryptCombinePaymentNotification(notification *PaymentNotification) (*CombinePaymentNotification, error)

	// DecryptProfitSharingNotification 解密分账通知
	DecryptProfitSharingNotification(notification *PaymentNotification) (*ProfitSharingNotification, error)

	// DecryptEcommerceRefundNotification 解密电商退款通知
	DecryptEcommerceRefundNotification(notification *PaymentNotification) (*EcommerceRefundNotification, error)

	// DecryptNotificationRaw 解密通知原始数据（返回 JSON 字节）
	DecryptNotificationRaw(notification *PaymentNotification) ([]byte, error)

	// DecryptSettlementNotification 解密结算事件通知（trade_manage_order_settlement）
	DecryptSettlementNotification(notification *PaymentNotification) (*SettlementNotificationResource, error)

	// DecryptComplaintNotification 解密用户投诉通知
	DecryptComplaintNotification(notification *PaymentNotification) (*ComplaintNotification, error)

	// VerifyNotificationSignature 验证微信支付回调签名
	VerifyNotificationSignature(signature, timestamp, nonce, body string) error

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
	CreateSubsidy(ctx context.Context, req SubsidyRequest) (*SubsidyResponse, error)
	// ReturnSubsidy 退回补差（退款时回收平台补贴款）
	ReturnSubsidy(ctx context.Context, req SubsidyReturnRequest) (*SubsidyReturnResponse, error)
	// CancelSubsidy 取消补差（尚未分账前可取消）
	CancelSubsidy(ctx context.Context, req SubsidyCancelRequest) error
}

// 确保 *Client 实现了 WechatClient 接口
var _ WechatClient = (*Client)(nil)

// 确保 *PaymentClient 实现了 PaymentClientInterface 接口
var _ PaymentClientInterface = (*PaymentClient)(nil)

// 确保 *EcommerceClient 实现了 EcommerceClientInterface 接口
var _ EcommerceClientInterface = (*EcommerceClient)(nil)

// 确保 *EcommerceClient 实现了 BillClientInterface 接口
// （DownloadTradeBill / DownloadRefundBill 通过内嵌 *PaymentClient 自动满足）
var _ BillClientInterface = (*EcommerceClient)(nil)
