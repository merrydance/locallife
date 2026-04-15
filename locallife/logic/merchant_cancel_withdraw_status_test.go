package logic

import (
	"errors"
	"testing"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
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
		MerchantCancelWithdrawSafeErrorMessage(&wechatcontracts.CancelWithdrawQueryContractError{Message: "invalid response"}),
	)
	require.Equal(t,
		"cancel-withdraw application already exists for the given out_request_no",
		MerchantCancelWithdrawSafeErrorMessage(&wechat.WechatPayError{StatusCode: 409, Code: "ALREADY_EXISTS"}),
	)
	require.Equal(t,
		"WeChat asked the cancel-withdraw request to retry later because the upstream business state is still processing",
		MerchantCancelWithdrawSafeErrorMessage(&wechat.WechatPayError{StatusCode: 503, Code: "BIZ_ERR_NEED_RETRY"}),
	)
	require.Equal(t,
		"WeChat cancel-withdraw service is temporarily unavailable; retry later",
		MerchantCancelWithdrawSafeErrorMessage(errors.New("network down")),
	)
}
