package wechatdoc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuditMerchantTransferAlignment_CoversCreateQueryCancelAndNotify(t *testing.T) {
	extraction := &Extraction{
		Sections: []Section{
			{
				Heading:   "接口说明",
				Path:      []string{"直连支付组", "商家转账", "发起转账", "接口说明"},
				Endpoints: []Endpoint{{Method: "POST", Path: "/v3/fund-app/mch-transfer/transfer-bills"}},
			},
			{
				Heading: "Body 参数",
				Path:    []string{"直连支付组", "商家转账", "发起转账", "Body 参数"},
				Fields: []Field{
					{Name: "appid"},
					{Name: "out_bill_no"},
					{Name: "transfer_scene_id", EnumValues: []string{"1011"}},
					{Name: "openid"},
					{Name: "user_name"},
					{Name: "transfer_amount", Description: "转账金额，单位为分"},
					{Name: "transfer_remark"},
					{Name: "notify_url"},
					{Name: "user_recv_perception", EnumValues: []string{"退款", "商家赔付"}},
					{Name: "transfer_scene_report_infos.info_type", EnumValues: []string{"赔付原因"}},
					{Name: "transfer_scene_report_infos.info_content"},
				},
			},
			{
				Heading: "应答参数",
				Path:    []string{"直连支付组", "商家转账", "发起转账", "应答参数"},
				Fields: []Field{
					{Name: "out_bill_no"},
					{Name: "transfer_bill_no"},
					{Name: "create_time", Description: "单据创建时间，遵循 RFC3339 标准格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE"},
					{Name: "state", EnumValues: []string{"ACCEPTED", "PROCESSING", "WAIT_USER_CONFIRM", "TRANSFERING", "SUCCESS", "FAIL", "CANCELING", "CANCELLED"}},
					{Name: "package_info"},
				},
			},
			{
				Heading:    "错误码",
				Path:       []string{"直连支付组", "商家转账", "发起转账", "错误码"},
				ErrorCodes: []ErrorCode{{Code: "PARAM_ERROR"}, {Code: "INVALID_REQUEST"}, {Code: "NO_AUTH"}, {Code: "SIGN_ERROR"}, {Code: "SYSTEM_ERROR"}, {Code: "NOT_ENOUGH"}, {Code: "FREQUENCY_LIMIT_EXCEED"}, {Code: "RATELIMIT_EXCEEDED"}, {Code: "FREQUENCY_LIMIT"}, {Code: "ALREADY_EXISTS"}},
			},
			{
				Heading:   "接口说明",
				Path:      []string{"直连支付组", "商家转账", "商户单号查询转账单", "接口说明"},
				Endpoints: []Endpoint{{Method: "GET", Path: "/v3/fund-app/mch-transfer/transfer-bills/out-bill-no/{out_bill_no}"}},
			},
			{
				Heading: "Path 参数",
				Path:    []string{"直连支付组", "商家转账", "商户单号查询转账单", "Path 参数"},
				Fields:  []Field{{Name: "out_bill_no"}},
			},
			{
				Heading: "应答参数",
				Path:    []string{"直连支付组", "商家转账", "商户单号查询转账单", "应答参数"},
				Fields: []Field{
					{Name: "mch_id"},
					{Name: "out_bill_no"},
					{Name: "transfer_bill_no"},
					{Name: "appid"},
					{Name: "state", EnumValues: []string{"ACCEPTED", "PROCESSING", "WAIT_USER_CONFIRM", "TRANSFERING", "SUCCESS", "FAIL", "CANCELING", "CANCELLED"}},
					{Name: "transfer_amount", Description: "转账金额单位为分"},
					{Name: "transfer_remark"},
					{Name: "fail_reason", EnumValues: []string{"ACCOUNT_FROZEN", "ACCOUNT_NOT_EXIST", "PAYEE_ACCOUNT_ABNORMAL", "PAYER_ACCOUNT_ABNORMAL", "TRANSFER_SCENE_INVALID", "TRANSFER_SCENE_UNAVAILABLE", "TRANSFER_RISK", "OVERDUE_CLOSE"}},
					{Name: "openid"},
					{Name: "user_name"},
					{Name: "create_time", Description: "单据创建时间，遵循 RFC3339 标准格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE"},
					{Name: "update_time", Description: "最后一次状态变更时间，遵循 RFC3339 标准格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE"},
				},
			},
			{
				Heading:    "错误码",
				Path:       []string{"直连支付组", "商家转账", "商户单号查询转账单", "错误码"},
				ErrorCodes: []ErrorCode{{Code: "PARAM_ERROR"}, {Code: "INVALID_REQUEST"}, {Code: "NO_AUTH"}, {Code: "SIGN_ERROR"}, {Code: "NOT_FOUND"}, {Code: "FREQUENCY_LIMITED"}, {Code: "SYSTEM_ERROR"}, {Code: "FREQUENCY_LIMIT_EXCEED"}, {Code: "RATELIMIT_EXCEEDED"}},
			},
			{
				Heading:   "接口说明",
				Path:      []string{"直连支付组", "商家转账", "微信单号查询转账单", "接口说明"},
				Endpoints: []Endpoint{{Method: "GET", Path: "/v3/fund-app/mch-transfer/transfer-bills/transfer-bill-no/{transfer_bill_no}"}},
			},
			{
				Heading: "Path 参数",
				Path:    []string{"直连支付组", "商家转账", "微信单号查询转账单", "Path 参数"},
				Fields:  []Field{{Name: "transfer_bill_no"}},
			},
			{
				Heading: "应答参数",
				Path:    []string{"直连支付组", "商家转账", "微信单号查询转账单", "应答参数"},
				Fields: []Field{
					{Name: "mch_id"},
					{Name: "out_bill_no"},
					{Name: "transfer_bill_no"},
					{Name: "appid"},
					{Name: "state", EnumValues: []string{"ACCEPTED", "PROCESSING", "WAIT_USER_CONFIRM", "TRANSFERING", "SUCCESS", "FAIL", "CANCELING", "CANCELLED"}},
					{Name: "transfer_amount", Description: "转账金额单位为分"},
					{Name: "transfer_remark"},
					{Name: "fail_reason", EnumValues: []string{"ACCOUNT_FROZEN", "ACCOUNT_NOT_EXIST", "PAYEE_ACCOUNT_ABNORMAL", "PAYER_ACCOUNT_ABNORMAL", "TRANSFER_SCENE_INVALID", "TRANSFER_SCENE_UNAVAILABLE", "TRANSFER_RISK", "OVERDUE_CLOSE"}},
					{Name: "openid"},
					{Name: "user_name"},
					{Name: "create_time", Description: "单据创建时间，遵循 RFC3339 标准格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE"},
					{Name: "update_time", Description: "最后一次状态变更时间，遵循 RFC3339 标准格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE"},
				},
			},
			{
				Heading:    "错误码",
				Path:       []string{"直连支付组", "商家转账", "微信单号查询转账单", "错误码"},
				ErrorCodes: []ErrorCode{{Code: "PARAM_ERROR"}, {Code: "INVALID_REQUEST"}, {Code: "NO_AUTH"}, {Code: "SIGN_ERROR"}, {Code: "NOT_FOUND"}, {Code: "FREQUENCY_LIMITED"}, {Code: "SYSTEM_ERROR"}, {Code: "FREQUENCY_LIMIT_EXCEED"}, {Code: "RATELIMIT_EXCEEDED"}},
			},
			{
				Heading:   "接口说明",
				Path:      []string{"直连支付组", "商家转账", "撤销转账", "接口说明"},
				Endpoints: []Endpoint{{Method: "POST", Path: "/v3/fund-app/mch-transfer/transfer-bills/out-bill-no/{out_bill_no}/cancel"}},
			},
			{
				Heading: "Path 参数",
				Path:    []string{"直连支付组", "商家转账", "撤销转账", "Path 参数"},
				Fields:  []Field{{Name: "out_bill_no"}},
			},
			{
				Heading: "应答参数",
				Path:    []string{"直连支付组", "商家转账", "撤销转账", "应答参数"},
				Fields:  []Field{{Name: "out_bill_no"}, {Name: "transfer_bill_no"}, {Name: "state", EnumValues: []string{"CANCELING", "CANCELLED"}}, {Name: "update_time", Description: "最后一次单据状态变更时间，遵循 RFC3339 标准格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE"}},
			},
			{
				Heading:    "错误码",
				Path:       []string{"直连支付组", "商家转账", "撤销转账", "错误码"},
				ErrorCodes: []ErrorCode{{Code: "PARAM_ERROR"}, {Code: "INVALID_REQUEST"}, {Code: "SIGN_ERROR"}, {Code: "SYSTEM_ERROR"}, {Code: "FREQUENCY_LIMIT_EXCEED"}, {Code: "RATELIMIT_EXCEEDED"}},
			},
			{
				Heading:   "接口说明",
				Path:      []string{"直连支付组", "商家转账", "商家转账回调通知", "接口说明"},
				Endpoints: []Endpoint{{Method: "POST", Path: "/v1/webhooks/wechat-pay/merchant-transfer-notify"}},
			},
			{
				Heading: "Body 参数",
				Path:    []string{"直连支付组", "商家转账", "商家转账回调通知", "Body 参数"},
				Fields:  []Field{{Name: "out_bill_no"}, {Name: "transfer_bill_no"}, {Name: "state", EnumValues: []string{"SUCCESS", "FAIL", "CANCELLED"}}, {Name: "mch_id"}, {Name: "transfer_amount", Description: "转账总金额，单位为分"}, {Name: "openid"}, {Name: "fail_reason", EnumValues: []string{"ACCOUNT_FROZEN", "ACCOUNT_NOT_EXIST", "PAYEE_ACCOUNT_ABNORMAL", "PAYER_ACCOUNT_ABNORMAL", "TRANSFER_SCENE_INVALID", "TRANSFER_SCENE_UNAVAILABLE", "TRANSFER_RISK", "OVERDUE_CLOSE"}}, {Name: "create_time", Description: "遵循 RFC3339 标准格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE"}, {Name: "update_time", Description: "遵循 RFC3339 标准格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE"}, {Name: "payment_method_type", EnumValues: []string{"CFT", "WPHK"}}},
			},
		},
	}

	report := AuditMerchantTransferAlignment(extraction)
	require.Equal(t, "merchant_transfer", report.Scope)
	require.Equal(t, 5, report.Summary.DocumentedEndpointCount)
	require.Equal(t, 5, report.Summary.AuditedEndpointCount)
	require.Empty(t, report.Endpoints)
}
