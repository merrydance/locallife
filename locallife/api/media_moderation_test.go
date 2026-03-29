package api

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
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
	"github.com/merrydance/locallife/ocr"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	mockworker "github.com/merrydance/locallife/worker/mock"
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

func TestCompleteMediaUploadSkipsModerationForOwnerOnlyPrivateMedia(t *testing.T) {
	user, _ := randomUser(t)
	objectKey := "id_card/front/1/20260329/private-id-card.jpg"
	uploadID := "up_test_private_id_card"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wxClient := mockwechat.NewMockWechatClient(ctrl)

	session := randomUploadSession(user.ID, "id_card_front", "private", objectKey, false)
	session.ID = uploadID
	asset := randomMediaAsset(11, user.ID, "private", objectKey)
	asset.MediaCategory = "id_card_front"
	asset.ModerationStatus = "pending"
	approvedAsset := asset
	approvedAsset.ModerationStatus = "approved"

	store.EXPECT().GetUploadSession(gomock.Any(), uploadID).Times(1).Return(session, nil)
	store.EXPECT().CreateMediaAsset(gomock.Any(), gomock.Any()).Times(1).Return(asset, nil)
	store.EXPECT().CompleteUploadSession(gomock.Any(), gomock.Any()).Times(1).Return(session, nil)
	store.EXPECT().SetMediaAssetModerationStatus(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
		func(_ context.Context, arg db.SetMediaAssetModerationStatusParams) (db.MediaAsset, error) {
			require.Equal(t, asset.ID, arg.ID)
			require.Equal(t, "approved", arg.ModerationStatus)
			return approvedAsset, nil
		},
	)

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
	require.EqualValues(t, 11, resp.MediaID)
	require.Equal(t, "approved", resp.Status)
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
	store.EXPECT().ListPendingOCRJobsByMediaAsset(gomock.Any(), asset.ID).Return(nil, nil)

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

func TestMiniProgramMediaCheckNotify_ApprovedEnqueuesPendingOCRJobs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockworker.NewMockTaskDistributor(ctrl)
	asset := randomMediaAsset(13, 1, "public", "merchant/dish/1/20260318/moderation-approved.jpg")
	asset.ModerationStatus = "approved"
	asset.ModerationTraceID = pgtype.Text{String: "trace-approved", Valid: true}
	job := db.OcrJob{ID: 501, Status: string(ocr.JobStatusPending), DocumentType: string(ocr.DocumentTypeBusinessLicense), Provider: string(ocr.ProviderNameAliyun), MediaAssetID: asset.ID, OwnerType: string(ocr.OwnerTypeMerchantApplication), OwnerID: 88, CreatedAt: time.Now()}

	gomock.InOrder(
		store.EXPECT().SetMediaAssetModerationStatusByTraceID(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
			func(_ context.Context, arg db.SetMediaAssetModerationStatusByTraceIDParams) (db.MediaAsset, error) {
				require.Equal(t, "trace-approved", arg.ModerationTraceID.String)
				require.Equal(t, "approved", arg.ModerationStatus)
				return asset, nil
			},
		),
		store.EXPECT().ListPendingOCRJobsByMediaAsset(gomock.Any(), asset.ID).Return([]db.OcrJob{job}, nil),
	)
	distributor.EXPECT().DistributeTaskMerchantApplicationBusinessLicenseOCR(gomock.Any(), job.OwnerID, job.MediaAssetID, job.ID).Return(nil)

	server, _ := newTestServerForMedia(t, store)
	server.config.WechatMiniAppID = "wx-test-app"
	server.config.WechatMiniAppMessageToken = "mini-token"
	server.SetTaskDistributorForTest(distributor)

	timestamp := "1710000000"
	nonce := "nonce-approved"
	signature := signMiniProgramCallback(server.config.WechatMiniAppMessageToken, timestamp, nonce)
	body := `<xml>
<AppID><![CDATA[wx-test-app]]></AppID>
<MsgType><![CDATA[event]]></MsgType>
<Event><![CDATA[wxa_media_check]]></Event>
<trace_id><![CDATA[trace-approved]]></trace_id>
<result>
<suggest><![CDATA[pass]]></suggest>
<label><![CDATA[10001]]></label>
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

func TestMiniProgramMediaCheckNotify_DetailOnlyCallbackApprovesPendingOCRJobs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockworker.NewMockTaskDistributor(ctrl)
	asset := randomMediaAsset(15, 1, "public", "merchant/dish/1/20260318/moderation-detail-approved.jpg")
	asset.ModerationStatus = "approved"
	asset.ModerationTraceID = pgtype.Text{String: "trace-detail-approved", Valid: true}
	job := db.OcrJob{ID: 701, Status: string(ocr.JobStatusPending), DocumentType: string(ocr.DocumentTypeBusinessLicense), Provider: string(ocr.ProviderNameAliyun), MediaAssetID: asset.ID, OwnerType: string(ocr.OwnerTypeMerchantApplication), OwnerID: 108, CreatedAt: time.Now()}

	gomock.InOrder(
		store.EXPECT().SetMediaAssetModerationStatusByTraceID(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
			func(_ context.Context, arg db.SetMediaAssetModerationStatusByTraceIDParams) (db.MediaAsset, error) {
				require.Equal(t, "trace-detail-approved", arg.ModerationTraceID.String)
				require.Equal(t, "approved", arg.ModerationStatus)
				return asset, nil
			},
		),
		store.EXPECT().ListPendingOCRJobsByMediaAsset(gomock.Any(), asset.ID).Return([]db.OcrJob{job}, nil),
	)
	distributor.EXPECT().DistributeTaskMerchantApplicationBusinessLicenseOCR(gomock.Any(), job.OwnerID, job.MediaAssetID, job.ID).Return(nil)

	server, _ := newTestServerForMedia(t, store)
	server.config.WechatMiniAppID = "wx-test-app"
	server.config.WechatMiniAppMessageToken = "mini-token"
	server.SetTaskDistributorForTest(distributor)

	timestamp := "1710000002"
	nonce := "nonce-detail-approved"
	signature := signMiniProgramCallback(server.config.WechatMiniAppMessageToken, timestamp, nonce)
	body := `<xml>
<appid><![CDATA[wx-test-app]]></appid>
<MsgType><![CDATA[event]]></MsgType>
<Event><![CDATA[wxa_media_check]]></Event>
<trace_id><![CDATA[trace-detail-approved]]></trace_id>
<version>2</version>
<detail>
<strategy><![CDATA[img]]></strategy>
<errcode>0</errcode>
<suggest><![CDATA[pass]]></suggest>
<label><![CDATA[10001]]></label>
<prob>90</prob>
</detail>
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

func TestMiniProgramMediaCheckNotify_QuarantinedFailsPendingOCRJobs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	asset := randomMediaAsset(14, 1, "public", "merchant/dish/1/20260318/moderation-quarantined.jpg")
	asset.ModerationStatus = "quarantined"
	asset.ModerationTraceID = pgtype.Text{String: "trace-quarantined", Valid: true}
	job := db.OcrJob{ID: 601, Status: string(ocr.JobStatusPending), DocumentType: string(ocr.DocumentTypeBusinessLicense), Provider: string(ocr.ProviderNameAliyun), MediaAssetID: asset.ID, OwnerType: string(ocr.OwnerTypeMerchantApplication), OwnerID: 99, CreatedAt: time.Now()}
	failedJob := job
	failedJob.Status = string(ocr.JobStatusFailed)
	failedJob.ErrorCode = pgtype.Text{String: "ocr_bad_request", Valid: true}
	failedJob.ErrorMessage = pgtype.Text{String: "media moderation quarantined the uploaded image", Valid: true}

	gomock.InOrder(
		store.EXPECT().SetMediaAssetModerationStatusByTraceID(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
			func(_ context.Context, arg db.SetMediaAssetModerationStatusByTraceIDParams) (db.MediaAsset, error) {
				require.Equal(t, "trace-quarantined", arg.ModerationTraceID.String)
				require.Equal(t, "quarantined", arg.ModerationStatus)
				return asset, nil
			},
		),
		store.EXPECT().ListPendingOCRJobsByMediaAsset(gomock.Any(), asset.ID).Return([]db.OcrJob{job}, nil),
		store.EXPECT().FailPendingOCRJob(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.FailPendingOCRJobParams) (db.OcrJob, error) {
			require.Equal(t, job.ID, arg.ID)
			require.True(t, arg.ErrorCode.Valid)
			require.Equal(t, "ocr_bad_request", arg.ErrorCode.String)
			require.True(t, arg.ErrorMessage.Valid)
			require.Contains(t, arg.ErrorMessage.String, "moderation quarantined")
			return failedJob, nil
		}),
		store.EXPECT().UpdateMerchantApplicationBusinessLicense(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.UpdateMerchantApplicationBusinessLicenseParams) (db.MerchantApplication, error) {
			var payload BusinessLicenseOCRData
			require.NoError(t, json.Unmarshal(arg.BusinessLicenseOcr, &payload))
			require.Equal(t, string(ocr.JobStatusFailed), payload.Status)
			require.Equal(t, "ocr_bad_request", payload.ErrorCode)
			require.Contains(t, payload.Error, "moderation quarantined")
			require.NotNil(t, payload.OCRJobID)
			require.Equal(t, failedJob.ID, *payload.OCRJobID)
			return db.MerchantApplication{ID: arg.ID}, nil
		}),
	)

	server, _ := newTestServerForMedia(t, store)
	server.config.WechatMiniAppID = "wx-test-app"
	server.config.WechatMiniAppMessageToken = "mini-token"

	timestamp := "1710000001"
	nonce := "nonce-quarantined"
	signature := signMiniProgramCallback(server.config.WechatMiniAppMessageToken, timestamp, nonce)
	body := `<xml>
<AppID><![CDATA[wx-test-app]]></AppID>
<MsgType><![CDATA[event]]></MsgType>
<Event><![CDATA[wxa_media_check]]></Event>
<trace_id><![CDATA[trace-quarantined]]></trace_id>
<result>
<suggest><![CDATA[review]]></suggest>
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
