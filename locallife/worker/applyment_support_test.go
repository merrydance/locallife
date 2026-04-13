package worker

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMapWechatApplymentStateToStatus(t *testing.T) {
	require.Equal(t, "to_be_signed", mapWechatApplymentStateToStatus("NEED_SIGN"))
	require.Equal(t, "", mapWechatApplymentStateToStatus("NEW_UPSTREAM_STATE"))
}

func TestResolveApplymentResultStatus(t *testing.T) {
	require.Equal(t, "account_need_verify", resolveApplymentResultStatus(ApplymentResultPayload{
		ApplymentStatus: "account_need_verify",
	}))
	require.Equal(t, "to_be_signed", resolveApplymentResultStatus(ApplymentResultPayload{
		ApplymentStatus: "auditing",
		SignState:       "UNSIGNED",
	}))
	require.Equal(t, "", resolveApplymentResultStatus(ApplymentResultPayload{
		ApplymentState: "NEW_UPSTREAM_STATE",
	}))
}

func TestApplymentStatusNeedsAsyncFollowUp(t *testing.T) {
	require.True(t, applymentStatusNeedsAsyncFollowUp("auditing", "UNSIGNED"))
	require.False(t, applymentStatusNeedsAsyncFollowUp("auditing", "SIGNED"))
}
