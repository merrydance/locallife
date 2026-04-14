package logic

import (
	"errors"
	"strings"

	"github.com/merrydance/locallife/wechat"
)

func MerchantCancelWithdrawSafeErrorMessage(err error) string {
	if err == nil {
		return ""
	}

	var wxErr *wechat.WechatPayError
	if errors.As(err, &wxErr) && wxErr != nil {
		switch strings.TrimSpace(wxErr.Code) {
		case "PARAM_ERROR":
			return "WeChat rejected the cancel-withdraw request: check sub_mchid, out_request_no, payee info, proof materials, and additional materials before retrying"
		case "INVALID_REQUEST":
			return "WeChat rejected the cancel-withdraw request in the current state: verify merchant configuration, current cancel-withdraw state, and signed request inputs before retrying"
		case "NO_AUTH":
			return "WeChat rejected the cancel-withdraw request because the current merchant configuration has no permission to operate on this sub-merchant"
		case "SIGN_ERROR":
			return "WeChat rejected the cancel-withdraw request because signature verification failed: verify merchant credentials and signing inputs"
		default:
			if wxErr.StatusCode >= 500 {
				return "WeChat cancel-withdraw service is temporarily unavailable; retry later"
			}
			return "WeChat rejected the cancel-withdraw request in the current state: verify merchant configuration, current cancel-withdraw state, and signed request inputs before retrying"
		}
	}

	var contractErr *wechat.MerchantCancelWithdrawContractError
	if errors.As(err, &contractErr) {
		return "WeChat returned a cancel-withdraw response that does not match the documented contract"
	}

	var validationErr *wechat.MerchantCancelWithdrawValidationError
	if errors.As(err, &validationErr) {
		return "WeChat cancel-withdraw service is temporarily unavailable; retry later"
	}

	return "WeChat cancel-withdraw service is temporarily unavailable; retry later"
}
