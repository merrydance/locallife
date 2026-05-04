package api

import (
	"context"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
)

const platformBaofuAccountOwnerID int64 = 0

func (server *Server) getPlatformBaofuSettlementReadiness(ctx context.Context) (logic.BaofuAccountReadiness, error) {
	binding, found, err := server.getPlatformBaofuSettlementBinding(ctx)
	service := logic.NewBaofuAccountService(nil, nil)
	if err != nil {
		return logic.BaofuAccountReadiness{}, err
	}
	return service.ReadinessFromBinding(binding, found), nil
}

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
