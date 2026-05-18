package contracts

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWechatCategoryAllowlistSource(t *testing.T) {
	require.Equal(t, "c521b7b15397a5aa63be9a3d8297c8a8c207e68e7d7fea7a26f8450945b4793f", WechatCategorySourceSHA256)
	require.Len(t, WechatCategories, 110)
	require.True(t, IsValidWechatCategory(WechatCategories[0].Value))
	require.True(t, IsValidWechatCategory("758-2"))
	require.False(t, IsValidWechatCategory("INVALID_CATEGORY"))
}
