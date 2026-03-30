package api

import (
	"context"

	db "github.com/merrydance/locallife/db/sqlc"
)

func (server *Server) getRegionActiveOperator(ctx context.Context, regionID int64) (db.Operator, error) {
	if regionID <= 0 {
		return db.Operator{}, db.ErrRecordNotFound
	}

	return server.store.GetActiveOperatorByRegion(ctx, regionID)
}
