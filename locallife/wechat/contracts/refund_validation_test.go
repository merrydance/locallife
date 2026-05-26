package contracts

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateDirectRefundRequest_RejectsMissingAmount(t *testing.T) {
	err := ValidateDirectRefundRequest(&DirectRefundRequest{
		TransactionID: "txn-1",
		OutRefundNo:   "refund-1",
	})

	require.EqualError(t, err, "create direct refund: amount is required")
}

func TestValidateDirectRefundCreateResponse_ReturnsTypedContractError(t *testing.T) {
	err := ValidateDirectRefundCreateResponse("create direct refund", &DirectRefundResponse{
		OutRefundNo: "refund-1",
	})

	var contractErr *RefundContractError
	require.ErrorAs(t, err, &contractErr)
	require.EqualError(t, err, "create direct refund: refund_id is required")
}

func TestValidateDirectRefundCreateResponse_AcceptsAmountWithoutTotal(t *testing.T) {
	err := ValidateDirectRefundCreateResponse("create direct refund", &DirectRefundResponse{
		RefundID:      "refund-id-1",
		OutRefundNo:   "refund-1",
		TransactionID: "transaction-1",
		OutTradeNo:    "order-1",
		CreateTime:    "2026-04-24T18:50:03+08:00",
		Status:        DirectRefundStatusProcessing,
		Amount: DirectRefundAmount{
			Refund:   100,
			Currency: DirectRefundCurrencyCNY,
		},
	})

	require.NoError(t, err)
}

func TestValidateDirectRefundQueryResponse_RequiresAmountTotal(t *testing.T) {
	err := ValidateDirectRefundQueryResponse("query direct refund", &DirectRefundResponse{
		RefundID:            "refund-id-1",
		OutRefundNo:         "refund-1",
		TransactionID:       "transaction-1",
		OutTradeNo:          "order-1",
		Channel:             DirectRefundChannelOriginal,
		UserReceivedAccount: "user",
		CreateTime:          "2026-04-24T18:50:03+08:00",
		Status:              DirectRefundStatusProcessing,
		FundsAccount:        DirectRefundFundsAccountAvailable,
		Amount: DirectRefundAmount{
			Refund:   100,
			Currency: DirectRefundCurrencyCNY,
		},
	})

	var contractErr *RefundContractError
	require.ErrorAs(t, err, &contractErr)
	require.EqualError(t, err, "query direct refund: amount.total must be positive")
}

func TestValidateDirectQueryRefundByOutRefundNoInput_RejectsBlankValue(t *testing.T) {
	_, err := ValidateDirectQueryRefundByOutRefundNoInput(" ")

	var validationErr *RefundValidationError
	require.ErrorAs(t, err, &validationErr)
	require.EqualError(t, err, "query direct refund: out_refund_no is required")
}

func TestValidateDirectRefundNotificationResource_ReturnsTypedContractError(t *testing.T) {
	err := ValidateDirectRefundNotificationResource("decrypt direct refund notification", &DirectRefundNotificationResource{
		MchID:               "1900000109",
		OutTradeNo:          "order-1",
		TransactionID:       "txn-1",
		OutRefundNo:         "refund-1",
		RefundID:            "refund-id-1",
		RefundStatus:        DirectRefundStatusSuccess,
		UserReceivedAccount: "user",
		SuccessTime:         "2026-04-16T10:00:00+08:00",
		Amount: DirectRefundNotificationAmount{
			Total:      100,
			Refund:     100,
			PayerTotal: 100,
		},
	})

	var contractErr *RefundContractError
	require.ErrorAs(t, err, &contractErr)
	require.EqualError(t, err, "decrypt direct refund notification: amount.payer_refund must be positive")
}
