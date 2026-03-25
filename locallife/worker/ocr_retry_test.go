package worker

import (
	"errors"
	"testing"

	"github.com/hibiken/asynq"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/ocr"
)

func TestShouldSkipOCRError(t *testing.T) {
	tests := []struct {
		name string
		job  db.OcrJob
		err  error
		want bool
	}{
		{name: "non retryable", job: db.OcrJob{AttemptCount: 1, MaxAttempts: 3}, err: ocr.ErrAliyunOCRForbidden, want: true},
		{name: "retryable and attempts remain", job: db.OcrJob{AttemptCount: 1, MaxAttempts: 3}, err: ocr.ErrAliyunOCRRateLimited, want: false},
		{name: "retryable but exhausted", job: db.OcrJob{AttemptCount: 3, MaxAttempts: 3}, err: ocr.ErrAliyunOCRRateLimited, want: true},
		{name: "unknown error", job: db.OcrJob{}, err: errors.New("boom"), want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldSkipOCRError(tc.job, tc.err); got != tc.want {
				t.Fatalf("shouldSkipOCRError() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAsynqOCRTaskError(t *testing.T) {
	err := asynqOCRTaskError(db.OcrJob{AttemptCount: 3, MaxAttempts: 3}, ocr.ErrAliyunOCRRateLimited)
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("expected SkipRetry wrapper, got %v", err)
	}
}
