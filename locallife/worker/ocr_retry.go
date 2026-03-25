package worker

import (
	"fmt"

	"github.com/hibiken/asynq"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/ocr"
)

const defaultOCRTaskMaxRetry = 2

func withDefaultOCRTaskOptions(opts ...asynq.Option) []asynq.Option {
	merged := make([]asynq.Option, 0, len(opts)+1)
	merged = append(merged, asynq.MaxRetry(defaultOCRTaskMaxRetry))
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
