package worker

import (
	"context"
	"encoding/json"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/rs/zerolog/log"
)

// SavePlatformAlertEvent persists a platform alert for later review without
// blocking realtime delivery if persistence fails.
func SavePlatformAlertEvent(
	ctx context.Context,
	store db.Store,
	alertType string,
	level string,
	title string,
	message string,
	relatedID int64,
	relatedType string,
	extra map[string]any,
	emittedAt time.Time,
) error {
	if store == nil {
		return nil
	}
	if emittedAt.IsZero() {
		emittedAt = time.Now()
	}

	extraJSON, err := json.Marshal(extra)
	if err != nil {
		return err
	}

	_, err = store.CreatePlatformAlertEvent(ctx, db.CreatePlatformAlertEventParams{
		AlertType:   alertType,
		Level:       level,
		Title:       title,
		Message:     message,
		RelatedID:   relatedID,
		RelatedType: relatedType,
		Extra:       extraJSON,
		EmittedAt:   emittedAt,
	})
	if err != nil {
		log.Error().Err(err).Str("alert_type", alertType).Msg("failed to persist platform alert event")
	}
	return err
}
