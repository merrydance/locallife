package contracts

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateDirectMerchantTransferCreateRequest_RejectsInvalidEnterpriseCompensationPayload(t *testing.T) {
	err := ValidateDirectMerchantTransferCreateRequest(&DirectMerchantTransferCreateRequest{
		AppID:           "wx123",
		OutBillNo:       "bill-1",
		TransferSceneID: DirectMerchantTransferSceneEnterpriseCompensation,
		OpenID:          "openid-1",
		TransferAmount:  200000,
		TransferRemark:  "赔付",
		TransferSceneReportInfos: []DirectMerchantTransferSceneReportInfo{{
			InfoType:    DirectMerchantTransferReportInfoTypeCompensationReason,
			InfoContent: "商品质量问题退款",
		}},
	})
	require.EqualError(t, err, "create direct merchant transfer: user_name is required when transfer_amount is at least 200000 fen")
}

func TestValidateDirectMerchantTransferCreateResponse_ReturnsTypedContractError(t *testing.T) {
	err := ValidateDirectMerchantTransferCreateResponse("create direct merchant transfer", &DirectMerchantTransferCreateResponse{
		OutBillNo:  "bill-1",
		CreateTime: "2026-04-16T10:00:00+08:00",
		State:      DirectMerchantTransferStateAccepted,
	})

	var contractErr *DirectMerchantTransferContractError
	require.ErrorAs(t, err, &contractErr)
	require.EqualError(t, err, "create direct merchant transfer: wechat response missing transfer_bill_no")
}

func TestValidateDirectMerchantTransferQueryResponse_ReturnsTypedContractError(t *testing.T) {
	err := ValidateDirectMerchantTransferQueryResponse("query direct merchant transfer by out_bill_no", &DirectMerchantTransferQueryResponse{
		MchID:          "1900000109",
		OutBillNo:      "bill-1",
		TransferBillNo: "wx-bill-1",
		AppID:          "wx123",
		State:          "UNKNOWN",
		TransferAmount: 100,
		TransferRemark: "赔付",
		CreateTime:     "2026-04-16T10:00:00+08:00",
		UpdateTime:     "2026-04-16T10:00:00+08:00",
	})

	var contractErr *DirectMerchantTransferContractError
	require.ErrorAs(t, err, &contractErr)
	require.EqualError(t, err, "query direct merchant transfer by out_bill_no: unsupported state \"UNKNOWN\"")
}

func TestValidateDirectMerchantTransferNotificationResource_ReturnsTypedContractError(t *testing.T) {
	err := ValidateDirectMerchantTransferNotificationResource("decrypt direct merchant transfer notification", &DirectMerchantTransferNotificationResource{
		OutBillNo:         "bill-1",
		State:             DirectMerchantTransferStateSuccess,
		MchID:             "1900000109",
		TransferAmount:    100,
		OpenID:            "openid-1",
		CreateTime:        "2026-04-16T10:00:00+08:00",
		UpdateTime:        "2026-04-16T10:00:00+08:00",
		PaymentMethodType: DirectMerchantTransferPaymentMethodTypeWallet,
	})

	var contractErr *DirectMerchantTransferNotificationContractError
	require.ErrorAs(t, err, &contractErr)
	require.EqualError(t, err, "decrypt direct merchant transfer notification: wechat response missing transfer_bill_no")
}

func TestValidateDirectMerchantTransferNotificationResource_RequiresPaymentMethodTypeOnSuccess(t *testing.T) {
	err := ValidateDirectMerchantTransferNotificationResource("decrypt direct merchant transfer notification", &DirectMerchantTransferNotificationResource{
		OutBillNo:      "bill-1",
		TransferBillNo: "wx-bill-1",
		State:          DirectMerchantTransferStateSuccess,
		MchID:          "1900000109",
		TransferAmount: 100,
		OpenID:         "openid-1",
		CreateTime:     "2026-04-16T10:00:00+08:00",
		UpdateTime:     "2026-04-16T10:00:00+08:00",
	})

	require.EqualError(t, err, "decrypt direct merchant transfer notification: payment_method_type is required when state=SUCCESS")
}
