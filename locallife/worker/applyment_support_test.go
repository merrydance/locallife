package worker

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMapWechatApplymentStateToStatus(t *testing.T) {
	require.Equal(t, "to_be_signed", mapWechatApplymentStateToStatus("NEED_SIGN"))
	require.Equal(t, "to_be_signed", mapWechatApplymentStateToStatus("APPLYMENT_STATE_TO_BE_SIGNED"))
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
	require.Equal(t, "frozen", resolveApplymentResultStatus(ApplymentResultPayload{
		ApplymentState: "FROZEN",
	}))
	require.Equal(t, "canceled", resolveApplymentResultStatus(ApplymentResultPayload{
		ApplymentState: "CANCELED",
	}))
	require.Equal(t, "", resolveApplymentResultStatus(ApplymentResultPayload{
		ApplymentState: "NEW_UPSTREAM_STATE",
	}))
}

func TestApplymentStatusNeedsAsyncFollowUp(t *testing.T) {
	require.True(t, applymentStatusNeedsAsyncFollowUp("auditing", "UNSIGNED"))
	require.True(t, applymentStatusNeedsAsyncFollowUp("frozen", ""))
	require.True(t, applymentStatusNeedsAsyncFollowUp("canceled", ""))
	require.False(t, applymentStatusNeedsAsyncFollowUp("auditing", "SIGNED"))
}
