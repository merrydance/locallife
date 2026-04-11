package worker

import (
	"context"
	"encoding/json"
	"testing"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/ocr"
	"github.com/stretchr/testify/require"
)

func TestPublishOCRFailureAlert_PublishesPermanentFailure(t *testing.T) {
	publisher := &testPublisher{}
	processor := &RedisTaskProcessor{pubSubPublisher: publisher}
	job := db.OcrJob{ID: 901, OwnerType: "merchant_application", OwnerID: 88, DocumentType: "business_license", Provider: "aliyun", MediaAssetID: 5001, AttemptCount: 3, MaxAttempts: 3}

	emittedAt := processor.publishOCRFailureAlert(context.Background(), job, ocr.ErrAliyunOCRRateLimited)
	require.NotNil(t, emittedAt)
	require.Equal(t, AlertChannel, publisher.channel)

	var envelope map[string]any
	require.NoError(t, json.Unmarshal(publisher.payload, &envelope))
	data := envelope["data"].(map[string]any)
	require.Equal(t, string(AlertTypeOCRRetryExhausted), data["alert_type"])
	require.Equal(t, string(AlertLevelCritical), data["level"])
	extra := data["extra"].(map[string]any)
	require.Equal(t, "ocr_rate_limited", extra["error_code"])
	require.Equal(t, true, extra["retryable"])
	require.Equal(t, "attempts_exhausted", extra["reason"])
}

func TestPublishOCRFailureAlert_SkipsRetryableFailureBeforeExhaustion(t *testing.T) {
	publisher := &testPublisher{}
	processor := &RedisTaskProcessor{pubSubPublisher: publisher}
	job := db.OcrJob{ID: 902, OwnerType: "merchant_application", OwnerID: 99, DocumentType: "business_license", Provider: "aliyun", MediaAssetID: 5002, AttemptCount: 1, MaxAttempts: 3}

	emittedAt := processor.publishOCRFailureAlert(context.Background(), job, ocr.ErrAliyunOCRRateLimited)
	require.Nil(t, emittedAt)
	require.Empty(t, publisher.channel)
	require.Empty(t, publisher.payload)
}

func TestPublishOCRFailureAlert_PublishesPermanentProviderFailure(t *testing.T) {
	publisher := &testPublisher{}
	processor := &RedisTaskProcessor{pubSubPublisher: publisher}
	job := db.OcrJob{ID: 903, OwnerType: "operator_application", OwnerID: 66, DocumentType: "id_card", Provider: "aliyun", MediaAssetID: 5003, AttemptCount: 1, MaxAttempts: 3}

	emittedAt := processor.publishOCRFailureAlert(context.Background(), job, ocr.ErrAliyunOCRForbidden)
	require.NotNil(t, emittedAt)
	require.Equal(t, AlertChannel, publisher.channel)

	var envelope map[string]any
	require.NoError(t, json.Unmarshal(publisher.payload, &envelope))
	data := envelope["data"].(map[string]any)
	require.Equal(t, string(AlertTypeOCRJobFailed), data["alert_type"])
	require.Equal(t, string(AlertLevelCritical), data["level"])
	extra := data["extra"].(map[string]any)
	require.Equal(t, "ocr_provider_forbidden", extra["error_code"])
	require.Equal(t, false, extra["retryable"])
	require.Equal(t, "permanent_error", extra["reason"])
}
