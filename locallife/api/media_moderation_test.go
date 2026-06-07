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
	"github.com/merrydance/locallife/media"
	"github.com/merrydance/locallife/ocr"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	mockworker "github.com/merrydance/locallife/worker/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCompleteMediaUploadTriggersAsyncModeration(t *testing.T) {
	user, _ := randomUser(t)
	objectKey := "user/review/1/20260318/moderation.jpg"
	uploadID := "up_test_moderation"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wxClient := mockwechat.NewMockWechatClient(ctrl)

	session := randomUploadSession(user.ID, string(media.CategoryReviewImage), "public", objectKey, false)
	session.ID = uploadID
	asset := randomMediaAsset(10, user.ID, "public", objectKey)
	asset.MediaCategory = string(media.CategoryReviewImage)
	asset.ModerationStatus = "pending"
	updatedAsset := asset
	updatedAsset.ModerationTraceID = pgtype.Text{String: "trace-123", Valid: true}

	store.EXPECT().GetUploadSession(gomock.Any(), uploadID).Times(1).Return(session, nil)
	store.EXPECT().CreateMediaAsset(gomock.Any(), gomock.Any()).Times(1).Return(asset, nil)
	store.EXPECT().ConfirmMediaAssetUploaded(gomock.Any(), gomock.Any()).Times(1).Return(asset, nil)
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

func TestCompleteMediaUploadAutoApprovesNonReviewImagesWithoutWechatModeration(t *testing.T) {
	user, _ := randomUser(t)

	testCases := []struct {
		name       string
		category   media.Category
		visibility string
		objectKey  string
		mediaID    int64
	}{
		{
			name:       "merchant business license",
			category:   media.CategoryBusinessLicense,
			visibility: string(media.VisibilityPublic),
			objectKey:  "merchant/license/business/1/20260318/license.jpg",
			mediaID:    21,
		},
		{
			name:       "dish image",
			category:   media.CategoryDishImage,
			visibility: string(media.VisibilityPublic),
			objectKey:  "merchant/dish/1/20260318/dish.jpg",
			mediaID:    22,
		},
		{
			name:       "table image",
			category:   media.CategoryTableImage,
			visibility: string(media.VisibilityPublic),
			objectKey:  "merchant/table/1/20260318/table.jpg",
			mediaID:    23,
		},
		{
			name:       "rider health cert",
			category:   media.CategoryHealthCert,
			visibility: string(media.VisibilityPrivate),
			objectKey:  "rider/health_cert/1/20260318/health-cert.jpg",
			mediaID:    24,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uploadID := "up_test_skip_" + strings.ReplaceAll(tc.name, " ", "_")

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			wxClient := mockwechat.NewMockWechatClient(ctrl)

			session := randomUploadSession(user.ID, string(tc.category), tc.visibility, tc.objectKey, false)
			session.ID = uploadID
			asset := randomMediaAsset(tc.mediaID, user.ID, tc.visibility, tc.objectKey)
			asset.MediaCategory = string(tc.category)
			asset.ModerationStatus = "pending"
			approvedAsset := asset
			approvedAsset.ModerationStatus = "approved"

			store.EXPECT().GetUploadSession(gomock.Any(), uploadID).Times(1).Return(session, nil)
			store.EXPECT().CreateMediaAsset(gomock.Any(), gomock.Any()).Times(1).Return(asset, nil)
			store.EXPECT().ConfirmMediaAssetUploaded(gomock.Any(), gomock.Any()).Times(1).Return(asset, nil)
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
			writeLocalFile(t, tempDir, tc.objectKey)

			recorder := httptest.NewRecorder()
			req, err := http.NewRequest(http.MethodPost, "/v1/media/complete", marshalBody(t, completeUploadRequest{
				UploadID:  uploadID,
				ObjectKey: tc.objectKey,
			}))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

			server.router.ServeHTTP(recorder, req)

			require.Equal(t, http.StatusOK, recorder.Code)
			var resp completeUploadResponse
			requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
			require.EqualValues(t, tc.mediaID, resp.MediaID)
			require.Equal(t, "approved", resp.Status)
			if tc.visibility == string(media.VisibilityPublic) {
				require.NotEmpty(t, resp.Variants["original"])
			} else {
				require.Empty(t, resp.Variants)
			}
		})
	}
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
	store.EXPECT().ConfirmMediaAssetUploaded(gomock.Any(), gomock.Any()).Times(1).Return(asset, nil)
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

func TestCompleteMediaUploadSkipsModerationForPrivateHealthCert(t *testing.T) {
	user, _ := randomUser(t)
	objectKey := "rider/health_cert/1/20260329/private-health-cert.jpg"
	uploadID := "up_test_private_health_cert"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	wxClient := mockwechat.NewMockWechatClient(ctrl)

	session := randomUploadSession(user.ID, "health_cert", "private", objectKey, false)
	session.ID = uploadID
	asset := randomMediaAsset(16, user.ID, "private", objectKey)
	asset.MediaCategory = string(media.CategoryHealthCert)
	asset.ModerationStatus = "pending"
	approvedAsset := asset
	approvedAsset.ModerationStatus = "approved"

	store.EXPECT().GetUploadSession(gomock.Any(), uploadID).Times(1).Return(session, nil)
	store.EXPECT().CreateMediaAsset(gomock.Any(), gomock.Any()).Times(1).Return(asset, nil)
	store.EXPECT().ConfirmMediaAssetUploaded(gomock.Any(), gomock.Any()).Times(1).Return(asset, nil)
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
	require.EqualValues(t, 16, resp.MediaID)
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

func TestMiniProgramMediaCheckNotify_AcknowledgesOrdinaryMiniProgramMessages(t *testing.T) {
	testCases := []struct {
		name string
		body string
	}{
		{
			name: "text message",
			body: `<xml>
<ToUserName><![CDATA[gh_50bd8db2c6be]]></ToUserName>
<FromUserName><![CDATA[openid-user]]></FromUserName>
<CreateTime>1780828301</CreateTime>
<MsgType><![CDATA[text]]></MsgType>
<Content><![CDATA[超市能入住吗]]></Content>
<MsgId>25495331247649341</MsgId>
</xml>`,
		},
		{
			name: "customer service temporary session event",
			body: `<xml>
<ToUserName><![CDATA[gh_50bd8db2c6be]]></ToUserName>
<FromUserName><![CDATA[openid-user]]></FromUserName>
<CreateTime>1780828313</CreateTime>
<MsgType><![CDATA[event]]></MsgType>
<Event><![CDATA[user_enter_tempsession]]></Event>
<SessionFrom><![CDATA[]]></SessionFrom>
</xml>`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			server, _ := newTestServerForMedia(t, store)
			server.config.WechatMiniAppID = "wx-test-app"
			server.config.WechatMiniAppMessageToken = "mini-token"

			timestamp := "1780828302"
			nonce := "nonce-ordinary-message"
			signature := signMiniProgramCallback(server.config.WechatMiniAppMessageToken, timestamp, nonce)

			recorder := httptest.NewRecorder()
			url := fmt.Sprintf("/v1/webhooks/wechat-miniprogram/media-check?signature=%s&timestamp=%s&nonce=%s", signature, timestamp, nonce)
			req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(tc.body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/xml")

			server.router.ServeHTTP(recorder, req)

			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, "success", recorder.Body.String())
		})
	}
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
	app := db.MerchantApplication{
		ID:                          job.OwnerID,
		Status:                      "draft",
		BusinessLicenseMediaAssetID: pgtype.Int8{Int64: asset.ID, Valid: true},
		BusinessLicenseOcr:          []byte(`{"status":"pending","ocr_job_id":601}`),
	}

	gomock.InOrder(
		store.EXPECT().SetMediaAssetModerationStatusByTraceID(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
			func(_ context.Context, arg db.SetMediaAssetModerationStatusByTraceIDParams) (db.MediaAsset, error) {
				require.Equal(t, "trace-quarantined", arg.ModerationTraceID.String)
				require.Equal(t, "quarantined", arg.ModerationStatus)
				return asset, nil
			},
		),
		store.EXPECT().ListPendingOCRJobsByMediaAsset(gomock.Any(), asset.ID).Return([]db.OcrJob{job}, nil),
		store.EXPECT().GetMerchantApplication(gomock.Any(), job.OwnerID).Return(app, nil),
		store.EXPECT().FailPendingOCRJob(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.FailPendingOCRJobParams) (db.OcrJob, error) {
			require.Equal(t, job.ID, arg.ID)
			require.True(t, arg.ErrorCode.Valid)
			require.Equal(t, "ocr_bad_request", arg.ErrorCode.String)
			require.True(t, arg.ErrorMessage.Valid)
			require.Contains(t, arg.ErrorMessage.String, "moderation quarantined")
			return failedJob, nil
		}),
		store.EXPECT().GetMerchantApplication(gomock.Any(), failedJob.OwnerID).Return(app, nil),
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

func TestMiniProgramMediaCheckNotify_QuarantinedSkipsReboundPendingOCRJob(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	asset := randomMediaAsset(18, 1, "public", "merchant/dish/1/20260318/moderation-stale.jpg")
	asset.ModerationStatus = "quarantined"
	asset.ModerationTraceID = pgtype.Text{String: "trace-stale-quarantined", Valid: true}
	job := db.OcrJob{ID: 602, Status: string(ocr.JobStatusPending), DocumentType: string(ocr.DocumentTypeBusinessLicense), Provider: string(ocr.ProviderNameAliyun), MediaAssetID: asset.ID, OwnerType: string(ocr.OwnerTypeMerchantApplication), OwnerID: 100, CreatedAt: time.Now()}
	app := db.MerchantApplication{
		ID:                          job.OwnerID,
		Status:                      "draft",
		BusinessLicenseMediaAssetID: pgtype.Int8{Int64: asset.ID + 1, Valid: true},
		BusinessLicenseOcr:          []byte(`{"status":"pending","ocr_job_id":602}`),
	}

	gomock.InOrder(
		store.EXPECT().SetMediaAssetModerationStatusByTraceID(gomock.Any(), gomock.Any()).DoAndReturn(
			func(_ context.Context, arg db.SetMediaAssetModerationStatusByTraceIDParams) (db.MediaAsset, error) {
				require.Equal(t, "trace-stale-quarantined", arg.ModerationTraceID.String)
				require.Equal(t, "quarantined", arg.ModerationStatus)
				return asset, nil
			},
		),
		store.EXPECT().ListPendingOCRJobsByMediaAsset(gomock.Any(), asset.ID).Return([]db.OcrJob{job}, nil),
		store.EXPECT().GetMerchantApplication(gomock.Any(), job.OwnerID).Return(app, nil),
	)

	server, _ := newTestServerForMedia(t, store)
	server.config.WechatMiniAppID = "wx-test-app"
	server.config.WechatMiniAppMessageToken = "mini-token"

	timestamp := "1710000005"
	nonce := "nonce-stale-quarantined"
	signature := signMiniProgramCallback(server.config.WechatMiniAppMessageToken, timestamp, nonce)
	body := `<xml>
<AppID><![CDATA[wx-test-app]]></AppID>
<MsgType><![CDATA[event]]></MsgType>
<Event><![CDATA[wxa_media_check]]></Event>
<trace_id><![CDATA[trace-stale-quarantined]]></trace_id>
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

func TestMiniProgramMediaCheckNotify_TopLevelErrcodeStillQuarantines(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	asset := randomMediaAsset(17, 1, "private", "rider/health_cert/1/20260329/moderation-download-failed.jpg")
	asset.ModerationStatus = "quarantined"
	asset.ModerationTraceID = pgtype.Text{String: "trace-download-error", Valid: true}
	store.EXPECT().SetMediaAssetModerationStatusByTraceID(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(
		func(_ context.Context, arg db.SetMediaAssetModerationStatusByTraceIDParams) (db.MediaAsset, error) {
			require.Equal(t, "trace-download-error", arg.ModerationTraceID.String)
			require.Equal(t, "quarantined", arg.ModerationStatus)
			return asset, nil
		},
	)
	store.EXPECT().ListPendingOCRJobsByMediaAsset(gomock.Any(), asset.ID).Return(nil, nil)

	server, _ := newTestServerForMedia(t, store)
	server.config.WechatMiniAppID = "wx-test-app"
	server.config.WechatMiniAppMessageToken = "mini-token"

	timestamp := "1710000004"
	nonce := "nonce-top-level-errcode"
	signature := signMiniProgramCallback(server.config.WechatMiniAppMessageToken, timestamp, nonce)
	body := `<xml>
<appid><![CDATA[wx-test-app]]></appid>
<MsgType><![CDATA[event]]></MsgType>
<Event><![CDATA[wxa_media_check]]></Event>
<trace_id><![CDATA[trace-download-error]]></trace_id>
<version>2</version>
<errcode>-1008</errcode>
<errmsg><![CDATA[下载错误，请检查媒体链接是否有效]]></errmsg>
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

func TestMiniProgramMediaCheckCallbackReason(t *testing.T) {
	testCases := []struct {
		name    string
		payload miniProgramMediaCheckXML
		want    string
	}{
		{
			name:    "top level download failed",
			payload: miniProgramMediaCheckXML{ErrCode: "-1008", ErrMsg: "下载错误"},
			want:    "download_failed",
		},
		{
			name:    "top level upstream error fallback",
			payload: miniProgramMediaCheckXML{ErrCode: "-1", ErrMsg: "system error"},
			want:    "upstream_callback_error",
		},
		{
			name: "review result",
			payload: miniProgramMediaCheckXML{Result: struct {
				Suggest string `xml:"suggest"`
				Label   string `xml:"label"`
			}{Suggest: "review", Label: "20001"}},
			want: "manual_review",
		},
		{
			name: "risky result",
			payload: miniProgramMediaCheckXML{Result: struct {
				Suggest string `xml:"suggest"`
				Label   string `xml:"label"`
			}{Suggest: "risky", Label: "20001"}},
			want: "risky_content",
		},
		{
			name: "pass result",
			payload: miniProgramMediaCheckXML{Result: struct {
				Suggest string `xml:"suggest"`
				Label   string `xml:"label"`
			}{Suggest: "pass", Label: "10001"}},
			want: "passed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.payload.callbackReason())
		})
	}
}

func TestMediaModerationSourceLogFields(t *testing.T) {
	host, path := mediaModerationSourceLogFields("https://oss-cn-beijing.aliyuncs.com/private/object.jpg?X-Amz-Signature=secret&X-Amz-Expires=900")
	require.Equal(t, "oss-cn-beijing.aliyuncs.com", host)
	require.Equal(t, "/private/object.jpg", path)

	host, path = mediaModerationSourceLogFields("://bad-url")
	require.Equal(t, "", host)
	require.Equal(t, "", path)
}

func signMiniProgramCallback(token, timestamp, nonce string) string {
	parts := []string{token, timestamp, nonce}
	sort.Strings(parts)
	sum := sha1.Sum([]byte(strings.Join(parts, "")))
	return fmt.Sprintf("%x", sum)
}
