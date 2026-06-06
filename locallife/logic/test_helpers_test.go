package logic

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
)

func assertRequestError(t *testing.T, err error) *RequestError {
	require.Error(t, err)
	var reqErr *RequestError
	require.ErrorAs(t, err, &reqErr)
	return reqErr
}

func comboDishOrderabilityRow(dishID int64, name string, dishExists bool, isOnline bool, isAvailable bool) db.ListComboDishOrderabilityRow {
	return db.ListComboDishOrderabilityRow{
		DishID:      dishID,
		DishName:    name,
		DishExists:  pgtype.Bool{Bool: dishExists, Valid: true},
		IsOnline:    isOnline,
		IsAvailable: isAvailable,
	}
}
