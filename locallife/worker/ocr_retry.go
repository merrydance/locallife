package worker

import (
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/ocr"
)

const defaultOCRTaskMaxRetry = 2

// ocrTaskEnqueueDedupWindow suppresses duplicate enqueue attempts for the same OCR job payload,
// which can happen when upstream moderation callbacks are retried before a worker leases the job.
const ocrTaskEnqueueDedupWindow = 12 * time.Minute

func withDefaultOCRTaskOptions(opts ...asynq.Option) []asynq.Option {
	merged := make([]asynq.Option, 0, len(opts)+2)
	merged = append(merged, asynq.MaxRetry(defaultOCRTaskMaxRetry))
	merged = append(merged, asynq.Unique(ocrTaskEnqueueDedupWindow))
	merged = append(merged, opts...)
	return merged
}

func shouldSkipOCRError(job db.OcrJob, err error) bool {
	if err == nil {
		return false
	}
	if !ocr.IsRetryableError(err) {
		return true
	}
	return job.MaxAttempts > 0 && job.AttemptCount >= job.MaxAttempts
}

func asynqOCRTaskError(job db.OcrJob, err error) error {
	if shouldSkipOCRError(job, err) {
		return fmt.Errorf("ocr task permanent failure: %v: %w", err, asynq.SkipRetry)
	}
	return err
}
