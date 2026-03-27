package db

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
)

func createRandomMediaAsset(t *testing.T, userID int64) MediaAsset {
	store, ok := testStore.(*SQLStore)
	require.True(t, ok)

	asset := MediaAsset{}
	objectKey := fmt.Sprintf("test/ocr/%d-%s.jpg", time.Now().UnixNano(), util.RandomString(6))
	checksum := util.RandomString(32)
	bucketTypes := []string{"private", "public", "private_bucket", "public_bucket", "oss_private", "oss_public", "local"}
	var lastErr error

	for _, bucketType := range bucketTypes {
		err := store.connPool.QueryRow(context.Background(), `
			INSERT INTO media_assets (
				object_key,
				visibility,
				media_category,
				mime_type,
				file_size,
				checksum_sha256,
				upload_status,
				moderation_status,
				uploaded_by,
				source_client,
				bucket_type
			) VALUES (
				$1, $2, $3, $4, $5, $6,
				'pending', 'pending',
				$7, $8, $9
			)
			RETURNING id, object_key, visibility, media_category, mime_type, file_size, checksum_sha256, upload_status, moderation_status, uploaded_by, source_client, created_at, updated_at
		`, objectKey, "private", "document", "image/jpeg", int64(1024), checksum, userID, "test", bucketType).Scan(
			&asset.ID,
			&asset.ObjectKey,
			&asset.Visibility,
			&asset.MediaCategory,
			&asset.MimeType,
			&asset.FileSize,
			&asset.ChecksumSha256,
			&asset.UploadStatus,
			&asset.ModerationStatus,
			&asset.UploadedBy,
			&asset.SourceClient,
			&asset.CreatedAt,
			&asset.UpdatedAt,
		)
		if err == nil {
			return asset
		}
		lastErr = err
	}

	require.NoError(t, lastErr)
	return asset
}

func createRandomOCRJob(t *testing.T) OcrJob {
	user := createRandomUser(t)
	asset := createRandomMediaAsset(t, user.ID)
	job, err := testStore.UpsertOCRJob(context.Background(), UpsertOCRJobParams{
		IdempotencyKey: fmt.Sprintf("%d:business_license:merchant_application:%d:", asset.ID, user.ID),
		DocumentType:   "business_license",
		Provider:       "aliyun",
		MediaAssetID:   asset.ID,
		OwnerType:      "merchant_application",
		OwnerID:        user.ID,
		Side:           "",
		MaxAttempts:    3,
		RetentionUntil: pgtype.Timestamptz{},
		RequestedBy:    user.ID,
	})
	require.NoError(t, err)
	return job
}

func TestUpsertOCRJob_IsIdempotent(t *testing.T) {
	user := createRandomUser(t)
	asset := createRandomMediaAsset(t, user.ID)
	idempotencyKey := fmt.Sprintf("%d:food_permit:merchant_application:%d:", asset.ID, user.ID)

	job1, err := testStore.UpsertOCRJob(context.Background(), UpsertOCRJobParams{
		IdempotencyKey: idempotencyKey,
		DocumentType:   "food_permit",
		Provider:       "aliyun",
		MediaAssetID:   asset.ID,
		OwnerType:      "merchant_application",
		OwnerID:        user.ID,
		Side:           "",
		MaxAttempts:    3,
		RequestedBy:    user.ID,
	})
	require.NoError(t, err)

	job2, err := testStore.UpsertOCRJob(context.Background(), UpsertOCRJobParams{
		IdempotencyKey: idempotencyKey,
		DocumentType:   "food_permit",
		Provider:       "aliyun",
		MediaAssetID:   asset.ID,
		OwnerType:      "merchant_application",
		OwnerID:        user.ID,
		Side:           "",
		MaxAttempts:    3,
		RequestedBy:    user.ID,
	})
	require.NoError(t, err)

	require.Equal(t, job1.ID, job2.ID)
	require.Equal(t, idempotencyKey, job2.IdempotencyKey)
	require.Equal(t, int32(0), job2.AttemptCount)
	require.Equal(t, "pending", job2.Status)
}

func TestOCRJobStateTransition_ProcessingToSucceeded(t *testing.T) {
	job := createRandomOCRJob(t)

	processing, err := testStore.MarkOCRJobProcessing(context.Background(), MarkOCRJobProcessingParams{
		ID:         job.ID,
		LeaseOwner: pgtype.Text{String: "worker-a", Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "processing", processing.Status)
	require.Equal(t, int32(1), processing.AttemptCount)
	require.True(t, processing.LeasedAt.Valid)
	require.True(t, processing.LeaseOwner.Valid)
	require.Equal(t, "worker-a", processing.LeaseOwner.String)
	require.True(t, processing.StartedAt.Valid)
	require.False(t, processing.NextRetryAt.Valid)

	completed, err := testStore.CompleteOCRJob(context.Background(), CompleteOCRJobParams{
		ID:               job.ID,
		ProviderTaskID:   pgtype.Text{String: "provider-1", Valid: true},
		RawResult:        []byte(`{"raw":true}`),
		NormalizedResult: []byte(`{"document_type":"business_license"}`),
		ResultVersion:    1,
	})
	require.NoError(t, err)
	require.Equal(t, "succeeded", completed.Status)
	require.False(t, completed.LeasedAt.Valid)
	require.False(t, completed.LeaseOwner.Valid)
	require.False(t, completed.NextRetryAt.Valid)
	require.True(t, completed.FinishedAt.Valid)
	require.True(t, completed.StartedAt.Valid)
	require.Equal(t, int32(1), completed.AttemptCount)

	_, err = testStore.MarkOCRJobProcessing(context.Background(), MarkOCRJobProcessingParams{
		ID:         job.ID,
		LeaseOwner: pgtype.Text{String: "worker-b", Valid: true},
	})
	require.Error(t, err)
	require.ErrorIs(t, err, pgx.ErrNoRows)
}

func TestOCRJobStateTransition_ProcessingToPendingThenFailed(t *testing.T) {
	job := createRandomOCRJob(t)

	_, err := testStore.MarkOCRJobProcessing(context.Background(), MarkOCRJobProcessingParams{
		ID:         job.ID,
		LeaseOwner: pgtype.Text{String: "worker-a", Valid: true},
	})
	require.NoError(t, err)

	nextRetryAt := time.Now().Add(10 * time.Minute)
	retried, err := testStore.FailOCRJob(context.Background(), FailOCRJobParams{
		ID:           job.ID,
		Status:       "pending",
		ErrorCode:    pgtype.Text{String: "ocr_rate_limited", Valid: true},
		ErrorMessage: pgtype.Text{String: "retry later", Valid: true},
		NextRetryAt:  pgtype.Timestamptz{Time: nextRetryAt, Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "pending", retried.Status)
	require.True(t, retried.NextRetryAt.Valid)
	require.False(t, retried.LeasedAt.Valid)
	require.False(t, retried.LeaseOwner.Valid)
	require.False(t, retried.FinishedAt.Valid)

	processingAgain, err := testStore.MarkOCRJobProcessing(context.Background(), MarkOCRJobProcessingParams{
		ID:         job.ID,
		LeaseOwner: pgtype.Text{String: "worker-b", Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, int32(2), processingAgain.AttemptCount)
	require.False(t, processingAgain.NextRetryAt.Valid)

	failed, err := testStore.FailOCRJob(context.Background(), FailOCRJobParams{
		ID:           job.ID,
		Status:       "failed",
		ErrorCode:    pgtype.Text{String: "ocr_provider_forbidden", Valid: true},
		ErrorMessage: pgtype.Text{String: "forbidden", Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "failed", failed.Status)
	require.True(t, failed.FinishedAt.Valid)
	require.False(t, failed.LeasedAt.Valid)
	require.False(t, failed.LeaseOwner.Valid)

	_, err = testStore.CompleteOCRJob(context.Background(), CompleteOCRJobParams{
		ID:               job.ID,
		ProviderTaskID:   pgtype.Text{String: "provider-2", Valid: true},
		RawResult:        []byte(`{"raw":true}`),
		NormalizedResult: []byte(`{"document_type":"business_license"}`),
		ResultVersion:    1,
	})
	require.Error(t, err)
	require.ErrorIs(t, err, pgx.ErrNoRows)
}

func TestMarkOCRJobProcessing_ConcurrentSingleLease(t *testing.T) {
	job := createRandomOCRJob(t)

	type attemptResult struct {
		job   OcrJob
		err   error
		owner string
	}

	results := make(chan attemptResult, 2)
	start := make(chan struct{})
	var waitGroup sync.WaitGroup

	owners := []string{"worker-a", "worker-b"}
	for _, owner := range owners {
		waitGroup.Add(1)
		go func(leaseOwner string) {
			defer waitGroup.Done()
			<-start
			processing, err := testStore.MarkOCRJobProcessing(context.Background(), MarkOCRJobProcessingParams{
				ID:         job.ID,
				LeaseOwner: pgtype.Text{String: leaseOwner, Valid: true},
			})
			results <- attemptResult{job: processing, err: err, owner: leaseOwner}
		}(owner)
	}

	close(start)
	waitGroup.Wait()
	close(results)

	var successCount int
	var failedCount int
	var winner attemptResult
	for result := range results {
		if result.err == nil {
			successCount++
			winner = result
			continue
		}
		failedCount++
		require.ErrorIs(t, result.err, pgx.ErrNoRows)
	}

	require.Equal(t, 1, successCount, "only one concurrent worker should acquire the OCR lease")
	require.Equal(t, 1, failedCount, "the losing worker should not re-enter processing")
	require.Equal(t, "processing", winner.job.Status)
	require.Equal(t, int32(1), winner.job.AttemptCount)
	require.True(t, winner.job.LeaseOwner.Valid)
	require.Equal(t, winner.owner, winner.job.LeaseOwner.String)
	require.True(t, winner.job.LeasedAt.Valid)

	persisted, err := testStore.GetOCRJob(context.Background(), job.ID)
	require.NoError(t, err)
	require.Equal(t, "processing", persisted.Status)
	require.Equal(t, int32(1), persisted.AttemptCount)
	require.True(t, persisted.LeaseOwner.Valid)
	require.Equal(t, winner.owner, persisted.LeaseOwner.String)
	require.True(t, persisted.LeasedAt.Valid)
	if persisted.StartedAt.Valid {
		require.False(t, persisted.StartedAt.Time.Before(persisted.CreatedAt))
	}
	_, err = testStore.MarkOCRJobProcessing(context.Background(), MarkOCRJobProcessingParams{
		ID:         job.ID,
		LeaseOwner: pgtype.Text{String: "worker-c", Valid: true},
	})
	require.Error(t, err)
	require.ErrorIs(t, err, pgx.ErrNoRows)
}

func TestMarkOCRJobProcessing_ReclaimsExpiredLease(t *testing.T) {
	store, ok := testStore.(*SQLStore)
	require.True(t, ok)

	job := createRandomOCRJob(t)

	initial, err := testStore.MarkOCRJobProcessing(context.Background(), MarkOCRJobProcessingParams{
		ID:         job.ID,
		LeaseOwner: pgtype.Text{String: "worker-a", Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, int32(1), initial.AttemptCount)

	expiredLeasedAt := time.Now().Add(-20 * time.Minute)
	_, err = store.connPool.Exec(context.Background(), `
		UPDATE ocr_jobs
		SET leased_at = $2,
		    updated_at = now()
		WHERE id = $1
	`, job.ID, expiredLeasedAt)
	require.NoError(t, err)

	reclaimed, err := testStore.MarkOCRJobProcessing(context.Background(), MarkOCRJobProcessingParams{
		ID:                 job.ID,
		LeaseOwner:         pgtype.Text{String: "worker-b", Valid: true},
		LeaseExpiresBefore: pgtype.Timestamptz{Time: time.Now().Add(-15 * time.Minute), Valid: true},
	})
	require.NoError(t, err)
	require.Equal(t, "processing", reclaimed.Status)
	require.Equal(t, int32(2), reclaimed.AttemptCount)
	require.True(t, reclaimed.LeaseOwner.Valid)
	require.Equal(t, "worker-b", reclaimed.LeaseOwner.String)
	require.True(t, reclaimed.LeasedAt.Valid)
	require.True(t, reclaimed.LeasedAt.Time.After(expiredLeasedAt))

	persisted, err := testStore.GetOCRJob(context.Background(), job.ID)
	require.NoError(t, err)
	require.Equal(t, int32(2), persisted.AttemptCount)
	require.True(t, persisted.LeaseOwner.Valid)
	require.Equal(t, "worker-b", persisted.LeaseOwner.String)
	require.True(t, persisted.LeasedAt.Valid)
	require.True(t, persisted.LeasedAt.Time.After(expiredLeasedAt))
}

func TestListOCRDeadLetterJobs_AppliesFilterBeforePagination(t *testing.T) {
	store, ok := testStore.(*SQLStore)
	require.True(t, ok)

	_, err := store.connPool.Exec(context.Background(), `
		DELETE FROM ocr_jobs
		WHERE owner_type = 'merchant_application'
		  AND document_type IN ('business_license', 'food_permit')
	`)
	require.NoError(t, err)

	jobNewest := createRandomOCRJob(t)
	jobMiddle := createRandomOCRJob(t)
	jobOldest := createRandomOCRJob(t)

	otherUser := createRandomUser(t)
	otherAsset := createRandomMediaAsset(t, otherUser.ID)
	otherDocJob, err := testStore.UpsertOCRJob(context.Background(), UpsertOCRJobParams{
		IdempotencyKey: fmt.Sprintf("%d:food_permit:merchant_application:%d:", otherAsset.ID, otherUser.ID),
		DocumentType:   "food_permit",
		Provider:       "aliyun",
		MediaAssetID:   otherAsset.ID,
		OwnerType:      "merchant_application",
		OwnerID:        otherUser.ID,
		Side:           "",
		MaxAttempts:    3,
		RequestedBy:    otherUser.ID,
	})
	require.NoError(t, err)

	newestFinishedAt := time.Now().Add(-1 * time.Minute)
	middleFinishedAt := time.Now().Add(-2 * time.Minute)
	oldestFinishedAt := time.Now().Add(-3 * time.Minute)
	filteredOutFinishedAt := time.Now().Add(-30 * time.Second)

	for _, item := range []struct {
		job        OcrJob
		finishedAt time.Time
	}{
		{job: jobNewest, finishedAt: newestFinishedAt},
		{job: jobMiddle, finishedAt: middleFinishedAt},
		{job: jobOldest, finishedAt: oldestFinishedAt},
		{job: otherDocJob, finishedAt: filteredOutFinishedAt},
	} {
		_, execErr := store.connPool.Exec(context.Background(), `
			UPDATE ocr_jobs
			SET status = 'failed',
			    attempt_count = max_attempts,
			    next_retry_at = NULL,
			    error_code = 'ocr_execution_failed',
			    error_message = 'dead letter',
			    finished_at = $2,
			    updated_at = $2
			WHERE id = $1
		`, item.job.ID, item.finishedAt)
		require.NoError(t, execErr)
	}

	allJobs, err := testStore.ListOCRDeadLetterJobs(context.Background(), ListOCRDeadLetterJobsParams{
		OwnerType:    "merchant_application",
		DocumentType: "business_license",
		PageOffset:   0,
		PageLimit:    10,
	})
	require.NoError(t, err)
	require.Len(t, allJobs, 3)
	require.NotContains(t, []int64{allJobs[0].ID, allJobs[1].ID, allJobs[2].ID}, otherDocJob.ID)
	require.Equal(t, "business_license", allJobs[0].DocumentType)
	require.Equal(t, "business_license", allJobs[1].DocumentType)
	require.Equal(t, "business_license", allJobs[2].DocumentType)

	jobs, err := testStore.ListOCRDeadLetterJobs(context.Background(), ListOCRDeadLetterJobsParams{
		OwnerType:    "merchant_application",
		DocumentType: "business_license",
		PageOffset:   1,
		PageLimit:    2,
	})
	require.NoError(t, err)
	require.Len(t, jobs, 2)
	require.Equal(t, allJobs[1].ID, jobs[0].ID)
	require.Equal(t, allJobs[2].ID, jobs[1].ID)
	require.Contains(t, []int64{jobNewest.ID, jobMiddle.ID, jobOldest.ID}, jobs[0].ID)
	require.Contains(t, []int64{jobNewest.ID, jobMiddle.ID, jobOldest.ID}, jobs[1].ID)
}
