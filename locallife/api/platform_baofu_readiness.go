package api

import (
	"context"

	db "github.com/merrydance/locallife/db/sqlc"
)

const platformBaofuAccountOwnerID int64 = 0

func (server *Server) getPlatformBaofuSettlementBinding(ctx context.Context) (db.BaofuAccountBinding, bool, error) {
	binding, err := server.store.GetBaofuAccountBindingByOwner(ctx, db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: db.BaofuAccountOwnerTypePlatform,
		OwnerID:   platformBaofuAccountOwnerID,
	})
	if err != nil {
		if isNotFoundError(err) {
			return db.BaofuAccountBinding{}, false, nil
		}
		return db.BaofuAccountBinding{}, false, err
	}
	return binding, true, nil
}
