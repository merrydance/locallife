package contracts

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateProfitSharingRequest_UsesFallbackAppIDForPersonalReceiver(t *testing.T) {
	err := ValidateProfitSharingRequest(&ProfitSharingRequest{
		SubMchID:      "1900000209",
		TransactionID: "4200000000001",
		OutOrderNo:    "ps-1",
		Receivers: []ProfitSharingReceiver{
			{
				Type:            ReceiverTypePersonal,
				ReceiverAccount: "openid",
				Amount:          1,
				Description:     "desc",
			},
		},
	}, "wx123")
	require.NoError(t, err)
}

func TestValidateProfitSharingRequest_DetectsDuplicateReceiver(t *testing.T) {
	err := ValidateProfitSharingRequest(&ProfitSharingRequest{
		SubMchID:      "1900000209",
		TransactionID: "4200000000001",
		OutOrderNo:    "ps-1",
		Receivers: []ProfitSharingReceiver{
			{Type: ReceiverTypeMerchant, ReceiverAccount: "1900000209", Amount: 1, Description: "a"},
			{Type: ReceiverTypeMerchant, ReceiverAccount: "1900000209", Amount: 2, Description: "b"},
		},
	}, "")
	require.EqualError(t, err, "create profit sharing: duplicate receiver MERCHANT_ID:1900000209")
}

func TestValidateAddReceiverRequest_RejectsUnsupportedRelationType(t *testing.T) {
	err := ValidateAddReceiverRequest(&AddReceiverRequest{
		Type:         ReceiverTypeMerchant,
		Account:      "1900000209",
		RelationType: "INVALID",
	})
	require.EqualError(t, err, "add profit sharing receiver: unsupported relation_type \"INVALID\"")
}

func TestValidateProfitSharingReturnRequest_RequiresReturnMchID(t *testing.T) {
	err := ValidateProfitSharingReturnRequest(&ProfitSharingReturnRequest{
		SubMchID:    "1900000209",
		OutOrderNo:  "ps-1",
		OutReturnNo: "return-1",
		Amount:      1,
		Description: "desc",
	})
	require.EqualError(t, err, "create profit sharing return: return_mchid is required")
}

func TestValidateProfitSharingQueryResponse_ReturnsTypedContractError(t *testing.T) {
	err := ValidateProfitSharingQueryResponse("query profit sharing", &ProfitSharingQueryResponse{
		SubMchID:   "1900000209",
		OutOrderNo: "ps-1",
		OrderID:    "wx-ps-1",
		Status:     "UNKNOWN",
	})

	var contractErr *ProfitSharingContractError
	require.ErrorAs(t, err, &contractErr)
	require.EqualError(t, err, "query profit sharing: unsupported status \"UNKNOWN\"")
}

func TestValidateProfitSharingAmountsResponse_UsesFallbackTransactionID(t *testing.T) {
	resp := &ProfitSharingAmountsResponse{UnsplitAmount: 10}
	err := ValidateProfitSharingAmountsResponse("query profit sharing amounts", resp, "wx-transaction-1")
	require.NoError(t, err)
	require.Equal(t, "wx-transaction-1", resp.TransactionID)
}

func TestValidateProfitSharingReturnResponse_FillsFallbackOutReturnNo(t *testing.T) {
	resp := &ProfitSharingReturnResponse{Result: ProfitSharingReturnResultProcessing}
	err := ValidateProfitSharingReturnResponse("query profit sharing return", resp, "return-1")
	require.NoError(t, err)
	require.Equal(t, "return-1", resp.OutReturnNo)
}

func TestValidateProfitSharingNotification_RequiresServiceProviderMchID(t *testing.T) {
	err := ValidateProfitSharingNotification("decrypt profit sharing notification", &ProfitSharingNotification{
		SubMchID:      "1900000209",
		TransactionID: "4200000000001",
		OrderID:       "wx-ps-1",
		OutOrderNo:    "ps-1",
		Receiver: ProfitSharingNotificationReceiver{
			Type:        ReceiverTypeMerchant,
			Account:     "1900000209",
			Amount:      1,
			Description: "desc",
		},
	})
	require.EqualError(t, err, "decrypt profit sharing notification: wechat response missing sp_mchid")
}
