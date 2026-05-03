package api

import (
	"context"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
)

const platformBaofuAccountOwnerID int64 = 0

func (server *Server) getPlatformBaofuSettlementReadiness(ctx context.Context) (logic.BaofuAccountReadiness, error) {
	binding, err := server.store.GetBaofuAccountBindingByOwner(ctx, db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: db.BaofuAccountOwnerTypePlatform,
		OwnerID:   platformBaofuAccountOwnerID,
	})
	service := logic.NewBaofuAccountService(nil, nil)
	if err != nil {
		if isNotFoundError(err) {
			return service.ReadinessFromBinding(db.BaofuAccountBinding{}, false, false), nil
		}
		return logic.BaofuAccountReadiness{}, err
	}
	return service.ReadinessFromBinding(binding, true, false), nil
}
