package ocr

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

type stubJobStore struct {
	upsertJob      db.OcrJob
	getJob         db.OcrJob
	markJob        db.OcrJob
	completeJob    db.OcrJob
	failJob        db.OcrJob
	upsertParams   db.UpsertOCRJobParams
	markParams     db.MarkOCRJobProcessingParams
	completeParams db.CompleteOCRJobParams
	failParams     db.FailOCRJobParams
	upsertErr      error
	getErr         error
	markErr        error
	completeErr    error
	failErr        error
}

func (s *stubJobStore) UpsertOCRJob(ctx context.Context, arg db.UpsertOCRJobParams) (db.OcrJob, error) {
	_ = ctx
	s.upsertParams = arg
	return s.upsertJob, s.upsertErr
}

func (s *stubJobStore) GetOCRJob(ctx context.Context, id int64) (db.OcrJob, error) {
	_ = ctx
	_ = id
	return s.getJob, s.getErr
}

func (s *stubJobStore) MarkOCRJobProcessing(ctx context.Context, arg db.MarkOCRJobProcessingParams) (db.OcrJob, error) {
	_ = ctx
	s.markParams = arg
	return s.markJob, s.markErr
}

func (s *stubJobStore) CompleteOCRJob(ctx context.Context, arg db.CompleteOCRJobParams) (db.OcrJob, error) {
	_ = ctx
	s.completeParams = arg
	return s.completeJob, s.completeErr
}

func (s *stubJobStore) FailOCRJob(ctx context.Context, arg db.FailOCRJobParams) (db.OcrJob, error) {
	_ = ctx
	s.failParams = arg
	return s.failJob, s.failErr
}

type stubRouter struct {
	route Route
	err   error
}

func (r stubRouter) Route(documentType DocumentType) (Route, error) {
	_ = documentType
	return r.route, r.err
}

type stubRecognizeProvider struct {
	name     ProviderName
	response RecognizeResponse
	err      error
}

func (p stubRecognizeProvider) Name() ProviderName {
	return p.name
}

func (p stubRecognizeProvider) Recognize(ctx context.Context, capability Capability, req RecognizeRequest) (RecognizeResponse, error) {
	_ = ctx
	_ = capability
	_ = req
	return p.response, p.err
}

type stubBinaryReader struct {
	data        []byte
	contentType string
	err         error
	assetID     int64
}

func (r *stubBinaryReader) ReadMediaAsset(ctx context.Context, mediaAssetID int64) ([]byte, string, error) {
	_ = ctx
	r.assetID = mediaAssetID
	return r.data, r.contentType, r.err
}

func TestServiceCreateJobBuildsIdempotencyKeyAndRoutesProvider(t *testing.T) {
	store := &stubJobStore{upsertJob: db.OcrJob{ID: 7}}
	provider := stubRecognizeProvider{name: ProviderNameAliyun}
	service := NewService(store, stubRouter{route: Route{Provider: provider, Capability: CapabilityAliyunFoodPermit}}, nil)
	_, err := service.CreateJob(context.Background(), CreateJobParams{
		DocumentType: DocumentTypeFoodPermit,
		MediaAssetID: 88,
		OwnerType:    OwnerTypeMerchantApplication,
		OwnerID:      9,
		RequestedBy:  100,
	})
	if err != nil {
		t.Fatalf("CreateJob error = %v", err)
	}
	if store.upsertParams.IdempotencyKey != "88:food_permit:merchant_application:9:" {
		t.Fatalf("idempotency key = %s", store.upsertParams.IdempotencyKey)
	}
	if store.upsertParams.Provider != string(ProviderNameAliyun) {
		t.Fatalf("provider = %s, want %s", store.upsertParams.Provider, ProviderNameAliyun)
	}
	if store.upsertParams.MaxAttempts != 3 {
		t.Fatalf("max attempts = %d, want 3", store.upsertParams.MaxAttempts)
	}
}

func TestServiceCreateJob_DefaultsIDCardRetention(t *testing.T) {
	store := &stubJobStore{upsertJob: db.OcrJob{ID: 8}}
	provider := stubRecognizeProvider{name: ProviderNameAliyun}
	service := NewService(store, stubRouter{route: Route{Provider: provider, Capability: CapabilityAliyunIDCard}}, nil)

	_, err := service.CreateJob(context.Background(), CreateJobParams{
		DocumentType: DocumentTypeIDCard,
		MediaAssetID: 99,
		OwnerType:    OwnerTypeMerchantApplication,
		OwnerID:      10,
		Side:         DocumentSideFront,
		RequestedBy:  101,
	})
	if err != nil {
		t.Fatalf("CreateJob error = %v", err)
	}
	if !store.upsertParams.RetentionUntil.Valid {
		t.Fatal("expected retention until for id card jobs")
	}
	retention := time.Until(store.upsertParams.RetentionUntil.Time)
	if retention <= 6*24*time.Hour || retention >= 8*24*time.Hour {
		t.Fatalf("retention duration = %v, want about 7d", retention)
	}
}

func TestServiceExecuteJobCompletesResult(t *testing.T) {
	normalized := NormalizedResult{
		DocumentType: DocumentTypeBusinessLicense,
		BusinessLicense: &BusinessLicenseResult{
			CreditCode: "123",
		},
		RecognizedAt: time.Now().UTC(),
	}
	rawResult := json.RawMessage(`{"ok":true}`)
	store := &stubJobStore{
		getJob:      db.OcrJob{ID: 1, DocumentType: string(DocumentTypeBusinessLicense), Side: "", MediaAssetID: 99},
		markJob:     db.OcrJob{ID: 1, DocumentType: string(DocumentTypeBusinessLicense), Side: "", MediaAssetID: 99},
		completeJob: db.OcrJob{ID: 1, Status: string(JobStatusSucceeded)},
	}
	provider := stubRecognizeProvider{
		name: ProviderNameAliyun,
		response: RecognizeResponse{
			Provider:       ProviderNameAliyun,
			ProviderTaskID: "task-1",
			RawResult:      rawResult,
			Normalized:     normalized,
		},
	}
	service := NewService(store, stubRouter{route: Route{Provider: provider, Capability: CapabilityAliyunBusinessLicense}}, nil)
	job, err := service.ExecuteJob(context.Background(), ExecuteJobParams{
		JobID:       1,
		LeaseOwner:  "worker-1",
		ContentType: "image/jpeg",
		Data:        []byte("image"),
	})
	if err != nil {
		t.Fatalf("ExecuteJob error = %v", err)
	}
	if job.Status != string(JobStatusSucceeded) {
		t.Fatalf("job status = %s, want %s", job.Status, JobStatusSucceeded)
	}
	if store.markParams.LeaseOwner != (pgtype.Text{String: "worker-1", Valid: true}) {
		t.Fatalf("lease owner = %+v, want worker-1", store.markParams.LeaseOwner)
	}
	if !store.markParams.LeaseExpiresBefore.Valid {
		t.Fatal("expected lease expiry cutoff to be populated")
	}
	leaseAge := time.Until(store.markParams.LeaseExpiresBefore.Time)
	if leaseAge > -14*time.Minute || leaseAge < -16*time.Minute {
		t.Fatalf("lease expiry cutoff offset = %v, want about -15m", leaseAge)
	}
	if store.completeParams.ProviderTaskID != (pgtype.Text{String: "task-1", Valid: true}) {
		t.Fatalf("provider task id = %+v", store.completeParams.ProviderTaskID)
	}
	if string(store.completeParams.RawResult) != string(rawResult) {
		t.Fatalf("raw result = %s, want %s", string(store.completeParams.RawResult), string(rawResult))
	}
}

func TestServiceExecuteJobMarksFailureOnProviderError(t *testing.T) {
	store := &stubJobStore{
		getJob:  db.OcrJob{ID: 2, DocumentType: string(DocumentTypeIDCard), Side: string(DocumentSideFront), MediaAssetID: 101},
		markJob: db.OcrJob{ID: 2, DocumentType: string(DocumentTypeIDCard), Side: string(DocumentSideFront), MediaAssetID: 101},
		failJob: db.OcrJob{ID: 2, Status: string(JobStatusFailed)},
	}
	provider := stubRecognizeProvider{name: ProviderNameAliyun, err: errors.New("provider down")}
	service := NewService(store, stubRouter{route: Route{Provider: provider, Capability: CapabilityAliyunIDCard}}, nil)
	_, err := service.ExecuteJob(context.Background(), ExecuteJobParams{JobID: 2, LeaseOwner: "worker-2", ContentType: "image/jpeg", Data: []byte("image")})
	if err == nil {
		t.Fatalf("ExecuteJob error = nil, want provider error")
	}
	if store.failParams.Status != string(JobStatusFailed) {
		t.Fatalf("failure status = %s, want %s", store.failParams.Status, JobStatusFailed)
	}
	if store.failParams.ErrorCode != (pgtype.Text{String: "ocr_execution_failed", Valid: true}) {
		t.Fatalf("error code = %+v, want ocr_execution_failed", store.failParams.ErrorCode)
	}
	if store.failParams.ErrorMessage != (pgtype.Text{String: "provider down", Valid: true}) {
		t.Fatalf("error message = %+v, want provider down", store.failParams.ErrorMessage)
	}
}

func TestServiceExecuteJobInjectsRetryableFailureState(t *testing.T) {
	nextRetryAt := time.Now().Add(15 * time.Minute).UTC()
	store := &stubJobStore{
		getJob:  db.OcrJob{ID: 5, DocumentType: string(DocumentTypeBusinessLicense), MediaAssetID: 202},
		markJob: db.OcrJob{ID: 5, DocumentType: string(DocumentTypeBusinessLicense), MediaAssetID: 202},
		failJob: db.OcrJob{ID: 5, Status: string(JobStatusPending)},
	}
	provider := stubRecognizeProvider{name: ProviderNameAliyun, err: ErrAliyunOCRRateLimited}
	service := NewService(store, stubRouter{route: Route{Provider: provider, Capability: CapabilityAliyunBusinessLicense}}, nil)

	_, err := service.ExecuteJob(context.Background(), ExecuteJobParams{
		JobID:        5,
		LeaseOwner:   "worker-5",
		ContentType:  "image/jpeg",
		Data:         []byte("image"),
		FailureState: JobStatusPending,
		NextRetryAt:  &nextRetryAt,
	})
	if err == nil {
		t.Fatal("ExecuteJob error = nil, want rate limited error")
	}
	if !errors.Is(err, ErrAliyunOCRRateLimited) {
		t.Fatalf("ExecuteJob error = %v, want ErrAliyunOCRRateLimited", err)
	}
	if store.failParams.Status != string(JobStatusPending) {
		t.Fatalf("failure status = %s, want %s", store.failParams.Status, JobStatusPending)
	}
	if store.failParams.ErrorCode != (pgtype.Text{String: "ocr_rate_limited", Valid: true}) {
		t.Fatalf("error code = %+v, want ocr_rate_limited", store.failParams.ErrorCode)
	}
	if !store.failParams.NextRetryAt.Valid {
		t.Fatal("expected next retry at to be set")
	}
	if !store.failParams.NextRetryAt.Time.Equal(nextRetryAt) {
		t.Fatalf("next retry at = %v, want %v", store.failParams.NextRetryAt.Time, nextRetryAt)
	}
}

func TestServiceExecuteJob_SanitizesIDCardRawResult(t *testing.T) {
	rawResult := json.RawMessage(`{"name":"张三","id":"110101199001011234","addr":"北京市朝阳区测试路1号","meta":{"gender":"男"}}`)
	store := &stubJobStore{
		getJob:      db.OcrJob{ID: 4, DocumentType: string(DocumentTypeIDCard), Side: string(DocumentSideFront), MediaAssetID: 404},
		markJob:     db.OcrJob{ID: 4, DocumentType: string(DocumentTypeIDCard), Side: string(DocumentSideFront), MediaAssetID: 404},
		completeJob: db.OcrJob{ID: 4, Status: string(JobStatusSucceeded)},
	}
	provider := stubRecognizeProvider{
		name: ProviderNameAliyun,
		response: RecognizeResponse{
			Provider:   ProviderNameAliyun,
			RawResult:  rawResult,
			Normalized: NormalizedResult{DocumentType: DocumentTypeIDCard, Side: DocumentSideFront, RecognizedAt: time.Now().UTC()},
		},
	}
	service := NewService(store, stubRouter{route: Route{Provider: provider, Capability: CapabilityAliyunIDCard}}, nil)

	_, err := service.ExecuteJob(context.Background(), ExecuteJobParams{JobID: 4, LeaseOwner: "worker-4", ContentType: "image/jpeg", Data: []byte("image")})
	if err != nil {
		t.Fatalf("ExecuteJob error = %v", err)
	}

	var sanitized map[string]any
	if err := json.Unmarshal(store.completeParams.RawResult, &sanitized); err != nil {
		t.Fatalf("unmarshal sanitized raw result: %v", err)
	}
	if sanitized["name"] == "张三" {
		t.Fatalf("expected name to be masked, got %v", sanitized["name"])
	}
	if sanitized["id"] == "110101199001011234" {
		t.Fatalf("expected id number to be masked, got %v", sanitized["id"])
	}
	if sanitized["addr"] == "北京市朝阳区测试路1号" {
		t.Fatalf("expected address to be masked, got %v", sanitized["addr"])
	}
	meta, ok := sanitized["meta"].(map[string]any)
	if !ok || meta["gender"] != "[REDACTED]" {
		t.Fatalf("expected nested gender to be redacted, got %+v", sanitized["meta"])
	}
}

func TestServiceExecuteJobReadsMediaWhenPayloadMissing(t *testing.T) {
	normalized := NormalizedResult{DocumentType: DocumentTypeFoodPermit, RecognizedAt: time.Now().UTC()}
	store := &stubJobStore{
		getJob:      db.OcrJob{ID: 3, DocumentType: string(DocumentTypeFoodPermit), MediaAssetID: 303},
		markJob:     db.OcrJob{ID: 3, DocumentType: string(DocumentTypeFoodPermit), MediaAssetID: 303},
		completeJob: db.OcrJob{ID: 3, Status: string(JobStatusSucceeded)},
	}
	reader := &stubBinaryReader{data: []byte("asset-bytes"), contentType: "image/png"}
	provider := stubRecognizeProvider{
		name: ProviderNameAliyun,
		response: RecognizeResponse{
			Provider:   ProviderNameAliyun,
			RawResult:  json.RawMessage(`{"ok":true}`),
			Normalized: normalized,
		},
	}
	service := NewService(store, stubRouter{route: Route{Provider: provider, Capability: CapabilityAliyunFoodPermit}}, reader)
	_, err := service.ExecuteJob(context.Background(), ExecuteJobParams{JobID: 3, LeaseOwner: "worker-3"})
	if err != nil {
		t.Fatalf("ExecuteJob error = %v", err)
	}
	if reader.assetID != 303 {
		t.Fatalf("reader asset id = %d, want 303", reader.assetID)
	}
}
