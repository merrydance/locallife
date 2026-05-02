package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMerchantApplymentSubmitCapability(t *testing.T) {
	testCases := []struct {
		name              string
		merchantStatus    string
		applymentStatus   string
		expectedCanSubmit bool
		expectedReason    string
	}{
		{
			name:              "ApprovedCanSubmitWhenNotApplied",
			merchantStatus:    "approved",
			applymentStatus:   "not_applied",
			expectedCanSubmit: true,
		},
		{
			name:              "PendingBindbankCanResubmitRejected",
			merchantStatus:    "pending_bindbank",
			applymentStatus:   "rejected",
			expectedCanSubmit: true,
		},
		{
			name:              "AuditingCannotSubmit",
			merchantStatus:    "pending_bindbank",
			applymentStatus:   "auditing",
			expectedCanSubmit: false,
			expectedReason:    "当前资料正在审核中，暂不支持重复提交。",
		},
		{
			name:              "AccountNeedVerifyCannotSubmit",
			merchantStatus:    "pending_bindbank",
			applymentStatus:   "account_need_verify",
			expectedCanSubmit: false,
			expectedReason:    "当前申请待账户验证，请先完成验证后再刷新状态。",
		},
		{
			name:              "ToBeConfirmedCannotSubmit",
			merchantStatus:    "pending_bindbank",
			applymentStatus:   "to_be_confirmed",
			expectedCanSubmit: false,
			expectedReason:    "当前申请待确认，请先完成确认后再刷新状态。",
		},
		{
			name:              "SignedMerchantCannotSubmit",
			merchantStatus:    "active",
			applymentStatus:   "active",
			expectedCanSubmit: false,
			expectedReason:    "当前账户已开通，无需重复提交进件资料。",
		},
		{
			name:              "SuspendedMerchantCannotSubmit",
			merchantStatus:    "suspended",
			applymentStatus:   "not_applied",
			expectedCanSubmit: false,
			expectedReason:    "当前商户状态不可用，暂不支持提交普通服务商进件。",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			canSubmit, blockReason := getMerchantApplymentSubmitCapability(tc.merchantStatus, tc.applymentStatus)
			require.Equal(t, tc.expectedCanSubmit, canSubmit)
			require.Equal(t, tc.expectedReason, blockReason)
		})
	}
}
