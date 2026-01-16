package api

import (
	"database/sql"
	"errors"

	db "github.com/merrydance/locallife/db/sqlc"
)

func isNotFoundError(err error) bool {
	return errors.Is(err, db.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows)
}
