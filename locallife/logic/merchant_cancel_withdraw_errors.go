package logic

import (
	"errors"

	"github.com/merrydance/locallife/wechat"
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	"github.com/merrydance/locallife/wechat/errorcodes"
)

func MerchantCancelWithdrawSafeErrorMessage(err error) string {
	if err == nil {
		return ""
	}

	var wxErr *wechat.WechatPayError
	if errors.As(err, &wxErr) && wxErr != nil {
		switch errorcodes.CanonicalCancelWithdrawCode(wxErr.Code) {
		case errorcodes.CancelWithdrawCodeParamError:
			return "WeChat rejected the cancel-withdraw request: check sub_mchid, out_request_no, payee info, proof materials, and additional materials before retrying"
		case errorcodes.CancelWithdrawCodeInvalidRequest:
			return "WeChat rejected the cancel-withdraw request in the current state: verify merchant configuration, current cancel-withdraw state, and signed request inputs before retrying"
		case errorcodes.CancelWithdrawCodeNoAuth:
			return "WeChat rejected the cancel-withdraw request because the current merchant configuration has no permission to operate on this sub-merchant"
		case errorcodes.CancelWithdrawCodeSignError:
			return "WeChat rejected the cancel-withdraw request because signature verification failed: verify merchant credentials and signing inputs"
		case errorcodes.CancelWithdrawCodeAlreadyExists:
			return "cancel-withdraw application already exists for the given out_request_no"
		case errorcodes.CancelWithdrawCodeBizErrNeedRetry,
			errorcodes.CancelWithdrawCodeRateLimitExceeded,
			errorcodes.CancelWithdrawCodeFrequencyLimited,
			errorcodes.CancelWithdrawCodeFrequencyLimit:
			return "WeChat asked the cancel-withdraw request to retry later because the upstream business state is still processing"
		default:
			if wxErr.StatusCode >= 500 {
				return "WeChat cancel-withdraw service is temporarily unavailable; retry later"
			}
			return "WeChat rejected the cancel-withdraw request in the current state: verify merchant configuration, current cancel-withdraw state, and signed request inputs before retrying"
		}
	}

	var contractErr *wechatcontracts.CancelWithdrawQueryContractError
	if errors.As(err, &contractErr) {
		return "WeChat returned a cancel-withdraw response that does not match the documented contract"
	}

	var validationErr *wechatcontracts.CancelWithdrawRequestValidationError
	if errors.As(err, &validationErr) {
		return "WeChat cancel-withdraw service is temporarily unavailable; retry later"
	}

	return "WeChat cancel-withdraw service is temporarily unavailable; retry later"
}
