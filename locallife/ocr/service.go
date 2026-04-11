package ocr

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
)

const defaultJobLeaseTTL = 15 * time.Minute

// JobStore is the subset of persistence operations required by OCR service.
type JobStore interface {
	UpsertOCRJob(ctx context.Context, arg db.UpsertOCRJobParams) (db.OcrJob, error)
	GetOCRJob(ctx context.Context, id int64) (db.OcrJob, error)
	MarkOCRJobProcessing(ctx context.Context, arg db.MarkOCRJobProcessingParams) (db.OcrJob, error)
	CompleteOCRJob(ctx context.Context, arg db.CompleteOCRJobParams) (db.OcrJob, error)
	FailOCRJob(ctx context.Context, arg db.FailOCRJobParams) (db.OcrJob, error)
}

// CreateJobParams contains the inputs for creating an OCR job.
type CreateJobParams struct {
	IdempotencyKey string
	DocumentType   DocumentType
	MediaAssetID   int64
	OwnerType      OwnerType
	OwnerID        int64
	Side           DocumentSide
	RequestedBy    int64
	MaxAttempts    int32
	RetentionUntil *time.Time
}

// ExecuteJobParams contains the provider input needed to execute an OCR job.
type ExecuteJobParams struct {
	JobID        int64
	LeaseOwner   string
	ContentType  string
	Data         []byte
	NextRetryAt  *time.Time
	FailureState JobStatus
}

// JobResult returns job state plus decoded normalized OCR output.
type JobResult struct {
	Job              db.OcrJob
	NormalizedResult NormalizedResult
}

// Service coordinates OCR job creation, execution and result retrieval.
type Service struct {
	store  JobStore
	router Router
	reader BinaryReader
}

// NewService creates an OCR service with explicit dependencies.
func NewService(store JobStore, router Router, reader BinaryReader) *Service {
	return &Service{store: store, router: router, reader: reader}
}

// CreateJob inserts or reuses an OCR job using the idempotency key.
func (s *Service) CreateJob(ctx context.Context, arg CreateJobParams) (db.OcrJob, error) {
	if err := arg.DocumentType.Validate(); err != nil {
		return db.OcrJob{}, err
	}
	if err := arg.OwnerType.Validate(); err != nil {
		return db.OcrJob{}, err
	}
	if err := arg.Side.Validate(); err != nil {
		return db.OcrJob{}, err
	}
	if arg.IdempotencyKey == "" {
		arg.IdempotencyKey = BuildIdempotencyKey(arg.MediaAssetID, arg.DocumentType, arg.OwnerType, arg.OwnerID, arg.Side)
	}
	if arg.RetentionUntil == nil {
		arg.RetentionUntil = DefaultRetentionUntil(arg.DocumentType, time.Now())
	}
	route, err := s.router.Route(arg.DocumentType)
	if err != nil {
		return db.OcrJob{}, err
	}
	maxAttempts := arg.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	params := db.UpsertOCRJobParams{
		IdempotencyKey: arg.IdempotencyKey,
		DocumentType:   string(arg.DocumentType),
		Provider:       string(route.Provider.Name()),
		MediaAssetID:   arg.MediaAssetID,
		OwnerType:      string(arg.OwnerType),
		OwnerID:        arg.OwnerID,
		Side:           string(arg.Side),
		MaxAttempts:    maxAttempts,
		RetentionUntil: toPgTimestamp(arg.RetentionUntil),
		RequestedBy:    arg.RequestedBy,
	}
	return s.store.UpsertOCRJob(ctx, params)
}

// ExecuteJob marks a job processing, calls the routed provider and persists the result.
func (s *Service) ExecuteJob(ctx context.Context, arg ExecuteJobParams) (db.OcrJob, error) {
	job, err := s.store.GetOCRJob(ctx, arg.JobID)
	if err != nil {
		return db.OcrJob{}, err
	}
	route, err := s.router.Route(DocumentType(job.DocumentType))
	if err != nil {
		return db.OcrJob{}, err
	}
	processingJob, err := s.store.MarkOCRJobProcessing(ctx, db.MarkOCRJobProcessingParams{
		ID:                 job.ID,
		LeaseOwner:         nullableText(arg.LeaseOwner),
		LeaseExpiresBefore: toPgTimestamp(timePtr(time.Now().Add(-defaultJobLeaseTTL))),
	})
	if err != nil {
		return db.OcrJob{}, err
	}
	contentType := arg.ContentType
	data := arg.Data
	if len(data) == 0 || contentType == "" {
		if s.reader == nil {
			return db.OcrJob{}, fmt.Errorf("ocr binary reader not configured")
		}
		data, contentType, err = s.reader.ReadMediaAsset(ctx, processingJob.MediaAssetID)
		if err != nil {
			return db.OcrJob{}, err
		}
	}
	resp, err := route.Provider.Recognize(ctx, route.Capability, RecognizeRequest{
		DocumentType: DocumentType(processingJob.DocumentType),
		Side:         DocumentSide(processingJob.Side),
		MediaAssetID: processingJob.MediaAssetID,
		ContentType:  contentType,
		Data:         data,
	})
	if err != nil {
		status := arg.FailureState
		if status == "" {
			status = JobStatusFailed
		}
		failedJob, updateErr := s.store.FailOCRJob(ctx, db.FailOCRJobParams{
			ID:           processingJob.ID,
			Status:       string(status),
			ErrorCode:    nullableText(errorCode(err)),
			ErrorMessage: nullableText(err.Error()),
			RawResult:    nil,
			NextRetryAt:  toPgTimestamp(arg.NextRetryAt),
		})
		if updateErr != nil {
			return db.OcrJob{}, fmt.Errorf("ocr provider error: %w; fail update error: %v", err, updateErr)
		}
		return failedJob, err
	}
	normalized, err := MarshalNormalizedResult(resp.Normalized)
	if err != nil {
		return db.OcrJob{}, err
	}
	rawResult := SanitizeRawResultForStorage(DocumentType(processingJob.DocumentType), resp.RawResult)
	return s.store.CompleteOCRJob(ctx, db.CompleteOCRJobParams{
		ID:               processingJob.ID,
		ProviderTaskID:   nullableText(resp.ProviderTaskID),
		RawResult:        rawResult,
		NormalizedResult: normalized,
		ResultVersion:    1,
	})
}

// GetJobResult loads a persisted OCR job and decodes its normalized result.
func (s *Service) GetJobResult(ctx context.Context, jobID int64) (JobResult, error) {
	job, err := s.store.GetOCRJob(ctx, jobID)
	if err != nil {
		return JobResult{}, err
	}
	decoded, err := UnmarshalNormalizedResult(job.NormalizedResult)
	if err != nil {
		return JobResult{}, err
	}
	return JobResult{Job: job, NormalizedResult: decoded}, nil
}

// BuildIdempotencyKey constructs the default OCR job idempotency key.
func BuildIdempotencyKey(mediaAssetID int64, documentType DocumentType, ownerType OwnerType, ownerID int64, side DocumentSide) string {
	return fmt.Sprintf("%d:%s:%s:%d:%s", mediaAssetID, documentType, ownerType, ownerID, side)
}

func toPgTimestamp(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *value, Valid: true}
}

func nullableText(value string) pgtype.Text {
	if value == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: value, Valid: true}
}

func timePtr(value time.Time) *time.Time {
	return &value
}
