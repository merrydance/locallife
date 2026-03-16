package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rs/zerolog/log"
)

const (
	execTxMaxAttempts = 3
	execTxRetryDelay  = 20 * time.Millisecond
)

func isDeadlockError(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "40P01"
	}
	return false
}

// ExecTx executes a function within a database transaction.
// It automatically retries on deadlock errors (PostgreSQL code 40P01) up to
// execTxMaxAttempts times with exponential backoff, while still honouring
// context cancellation between retries.
func (store *SQLStore) execTx(ctx context.Context, fn func(*Queries) error) error {
	var lastErr error
	for attempt := 1; attempt <= execTxMaxAttempts; attempt++ {
		tx, err := store.connPool.Begin(ctx)
		if err != nil {
			return err
		}

		q := New(tx)
		err = fn(q)
		if err != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				return fmt.Errorf("tx err: %v, rb err: %v", err, rbErr)
			}
			if isDeadlockError(err) && attempt < execTxMaxAttempts {
				log.Warn().
					Int("attempt", attempt).
					Int("max_attempts", execTxMaxAttempts).
					Dur("retry_delay", execTxRetryDelay*time.Duration(attempt)).
					Msg("tx deadlock detected, retrying")
				lastErr = err
				select {
				case <-time.After(execTxRetryDelay * time.Duration(attempt)):
				case <-ctx.Done():
					return ctx.Err()
				}
				continue
			}
			if isDeadlockError(err) {
				log.Error().
					Int("attempt", attempt).
					Int("max_attempts", execTxMaxAttempts).
					Err(err).
					Msg("tx deadlock retry exhausted")
			}
			return err
		}

		commitErr := tx.Commit(ctx)
		if isDeadlockError(commitErr) && attempt < execTxMaxAttempts {
			log.Warn().
				Int("attempt", attempt).
				Int("max_attempts", execTxMaxAttempts).
				Dur("retry_delay", execTxRetryDelay*time.Duration(attempt)).
				Msg("tx commit deadlock detected, retrying")
			lastErr = commitErr
			select {
			case <-time.After(execTxRetryDelay * time.Duration(attempt)):
			case <-ctx.Done():
				return ctx.Err()
			}
			continue
		}
		if isDeadlockError(commitErr) {
			log.Error().
				Int("attempt", attempt).
				Int("max_attempts", execTxMaxAttempts).
				Err(commitErr).
				Msg("tx commit deadlock retry exhausted")
		}
		return commitErr
	}

	// All retry attempts exhausted due to repeated deadlocks.
	if lastErr != nil {
		return fmt.Errorf("execTx: all %d attempts exhausted due to deadlock: %w", execTxMaxAttempts, lastErr)
	}
	return fmt.Errorf("execTx: all %d attempts exhausted", execTxMaxAttempts)
}
