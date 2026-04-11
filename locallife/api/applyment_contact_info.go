package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

func firstNonEmptyTrimmed(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func pgTextValue(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func (server *Server) resolveApplymentContactPhone(ctx context.Context, userID int64, candidates ...string) (string, error) {
	if phone := firstNonEmptyTrimmed(candidates...); phone != "" {
		return phone, nil
	}

	user, err := server.store.GetUser(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("resolve applyment contact phone: %w", err)
	}

	if phone := firstNonEmptyTrimmed(pgTextValue(user.Phone)); phone != "" {
		return phone, nil
	}

	return "", fmt.Errorf("contact phone is required")
}
