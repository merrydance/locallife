package logic

import (
	"errors"
	"testing"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	"github.com/stretchr/testify/require"
)

func TestMerchantCancelWithdrawIsTerminal(t *testing.T) {
	require.True(t, MerchantCancelWithdrawIsTerminal(db.MerchantCancelStateRejected))
	require.True(t, MerchantCancelWithdrawIsTerminal(db.MerchantCancelStateRevoked))
	require.True(t, MerchantCancelWithdrawIsTerminal(db.MerchantCancelStateCanceled))
	require.True(t, MerchantCancelWithdrawIsTerminal(db.MerchantCancelStateFinish))
	require.False(t, MerchantCancelWithdrawIsTerminal(db.MerchantCancelStateWaitingMerchantConfirm))
	require.False(t, MerchantCancelWithdrawIsTerminal(db.MerchantCancelStateFundProcessing))
}

func TestMerchantCancelWithdrawSafeErrorMessage(t *testing.T) {
	require.Equal(t, "", MerchantCancelWithdrawSafeErrorMessage(nil))
	require.Equal(t,
		"WeChat rejected the cancel-withdraw request: check sub_mchid, out_request_no, payee info, proof materials, and additional materials before retrying",
		MerchantCancelWithdrawSafeErrorMessage(&wechat.WechatPayError{StatusCode: 400, Code: "PARAM_ERROR"}),
	)
	require.Equal(t,
		"WeChat returned a cancel-withdraw response that does not match the documented contract",
		MerchantCancelWithdrawSafeErrorMessage(&wechat.MerchantCancelWithdrawContractError{Message: "invalid response"}),
	)
	require.Equal(t,
		"WeChat cancel-withdraw service is temporarily unavailable; retry later",
		MerchantCancelWithdrawSafeErrorMessage(errors.New("network down")),
	)
}
