package api

import (
	"bytes"
	"context"
	"crypto/sha1"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCompleteMediaUploadTriggersAsyncModeration(t *testing.T) {
	user, _ := randomUser(t)
	objectKey := "merchant/dish/1/20260318/moderation.jpg"
	uploadID := "up_test_moderation"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wxClient := mockwechat.NewMockWechatClient(ctrl)

	session := randomUploadSession(user.ID, "dish", "public", objectKey, false)
	session.ID = uploadID
	asset := randomMediaAsset(10, user.ID, "public", objectKey)
	asset.ModerationStatus = "pending"
	updatedAsset := asset
	updatedAsset.ModerationTraceID = pgtype.Text{String: "trace-123", Valid: true}

	store.EXPECT().GetUploadSession(gomock.Any(), uploadID).Times(1).Return(session, nil)
	store.EXPECT().CreateMediaAsset(gomock.Any(), gomock.Any()).Times(1).Return(asset, nil)
	store.EXPECT().CompleteUploadSession(gomock.Any(), gomock.Any()).Times(1).Return(session, nil)
	store.EXPECT().GetUser(gomock.Any(), user.ID).Times(1).Return(user, nil)
	wxClient.EXPECT().MediaCheckAsync(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
		func(_ context.Context, req wechat.MediaCheckAsyncRequest) (*wechat.MediaCheckAsyncResponse, error) {
			require.Equal(t, wechat.SecCheckMediaTypeImage, req.MediaType)
			require.Equal(t, 2, req.Version)
			require.Equal(t, user.WechatOpenid, req.OpenID)
			require.Contains(t, req.MediaURL, objectKey)
			return &wechat.MediaCheckAsyncResponse{TraceID: "trace-123"}, nil
		},
	)
	store.EXPECT().SetMediaAssetModerationTraceID(gomock.Any(), gomock.Any()).Times(1).Return(updatedAsset, nil)

	server, tempDir := newTestServerForMedia(t, store)
	server.config.WechatMiniAppID = "wx-test-app"
	server.config.WechatMiniAppSecret = "secret"
	server.wechatClient = wxClient
	writeLocalFile(t, tempDir, objectKey)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPost, "/v1/media/complete", marshalBody(t, completeUploadRequest{
		UploadID:  uploadID,
		ObjectKey: objectKey,
	}))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	var resp completeUploadResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.EqualValues(t, 10, resp.MediaID)
	require.Equal(t, "pending", resp.Status)
	require.Len(t, resp.Variants, 0)
}

func TestMiniProgramMediaCheckNotify(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	asset := randomMediaAsset(12, 1, "public", "merchant/dish/1/20260318/moderation.jpg")
	asset.ModerationStatus = "rejected"
	asset.ModerationTraceID = pgtype.Text{String: "trace-risky", Valid: true}
	store.EXPECT().SetMediaAssetModerationStatusByTraceID(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
		func(_ context.Context, arg db.SetMediaAssetModerationStatusByTraceIDParams) (db.MediaAsset, error) {
			require.Equal(t, "trace-risky", arg.ModerationTraceID.String)
			require.Equal(t, "rejected", arg.ModerationStatus)
			return asset, nil
		},
	)

	server, _ := newTestServerForMedia(t, store)
	server.config.WechatMiniAppID = "wx-test-app"
	server.config.WechatMiniAppMessageToken = "mini-token"

	timestamp := "1710000000"
	nonce := "nonce-1"
	signature := signMiniProgramCallback(server.config.WechatMiniAppMessageToken, timestamp, nonce)
	body := `<xml>
<AppID><![CDATA[wx-test-app]]></AppID>
<MsgType><![CDATA[event]]></MsgType>
<Event><![CDATA[wxa_media_check]]></Event>
<trace_id><![CDATA[trace-risky]]></trace_id>
<result>
<suggest><![CDATA[risky]]></suggest>
<label><![CDATA[20001]]></label>
</result>
</xml>`

	recorder := httptest.NewRecorder()
	url := fmt.Sprintf("/v1/webhooks/wechat-miniprogram/media-check?signature=%s&timestamp=%s&nonce=%s", signature, timestamp, nonce)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/xml")

	server.router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "success", recorder.Body.String())
}

func signMiniProgramCallback(token, timestamp, nonce string) string {
	parts := []string{token, timestamp, nonce}
	sort.Strings(parts)
	sum := sha1.Sum([]byte(strings.Join(parts, "")))
	return fmt.Sprintf("%x", sum)
}
