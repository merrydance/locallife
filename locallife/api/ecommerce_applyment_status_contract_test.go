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
			expectedReason:    "当前商户状态不可用，暂不支持提交收付通进件。",
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

func TestOperatorApplymentSubmitCapability(t *testing.T) {
	testCases := []struct {
		name              string
		operatorStatus    string
		applymentStatus   string
		expectedCanSubmit bool
		expectedReason    string
	}{
		{
			name:              "ActiveOperatorCanSubmit",
			operatorStatus:    "active",
			applymentStatus:   "active",
			expectedCanSubmit: true,
		},
		{
			name:              "RejectedOperatorCanResubmit",
			operatorStatus:    "active",
			applymentStatus:   "rejected",
			expectedCanSubmit: true,
		},
		{
			name:              "ReviewingOperatorCannotSubmit",
			operatorStatus:    "bindbank_submitted",
			applymentStatus:   "submitted",
			expectedCanSubmit: false,
			expectedReason:    "微信支付正在审核开户信息，审核期间无需重复提交。",
		},
		{
			name:              "SigningOperatorCannotSubmit",
			operatorStatus:    "bindbank_submitted",
			applymentStatus:   "to_be_signed",
			expectedCanSubmit: false,
			expectedReason:    "微信支付已进入签约阶段，请先完成签约确认。",
		},
		{
			name:              "SuspendedOperatorCannotSubmit",
			operatorStatus:    "suspended",
			applymentStatus:   "active",
			expectedCanSubmit: false,
			expectedReason:    "当前运营商状态不可用，暂不支持提交微信支付开户。",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			canSubmit, blockReason := getOperatorApplymentSubmitCapability(tc.operatorStatus, tc.applymentStatus)
			require.Equal(t, tc.expectedCanSubmit, canSubmit)
			require.Equal(t, tc.expectedReason, blockReason)
		})
	}
}

func TestOperatorApplymentStatusDesc(t *testing.T) {
	testCases := []struct {
		name         string
		status       string
		canSubmit    bool
		expectedDesc string
	}{
		{
			name:         "ActiveAndCanSubmit",
			status:       "active",
			canSubmit:    true,
			expectedDesc: "可提交开户信息",
		},
		{
			name:         "ActiveButCompleted",
			status:       "active",
			canSubmit:    false,
			expectedDesc: "账户已开通",
		},
		{
			name:         "FrozenAccount",
			status:       "frozen",
			canSubmit:    false,
			expectedDesc: "当前账号状态不可用",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expectedDesc, getOperatorApplymentStatusDesc(tc.status, tc.canSubmit))
		})
	}
}
