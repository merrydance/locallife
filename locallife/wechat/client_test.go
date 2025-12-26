package wechat

import (
	"context"
	"testing"
	"time"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetAccessToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	client := NewClient("test_app_id", "test_app_secret", store)

	testCases := []struct {
		name        string
		appType     string
		buildStub   func(store *mockdb.MockStore)
		checkResult func(t *testing.T, token string, err error)
	}{
		{
			name:    "ValidCachedToken",
			appType: "miniprogram",
			buildStub: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetWechatAccessToken(gomock.Any(), gomock.Eq("miniprogram")).
					Times(1).
					Return(db.WechatAccessToken{
						AppType:     "miniprogram",
						AccessToken: "cached_token",
						ExpiresAt:   time.Now().Add(30 * time.Minute),
					}, nil)
			},
			checkResult: func(t *testing.T, token string, err error) {
				require.NoError(t, err)
				require.Equal(t, "cached_token", token)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.buildStub(store)

			token, err := client.GetAccessToken(context.Background(), tc.appType)
			tc.checkResult(t, token, err)
		})
	}
}
