package api

import (
	"context"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
)

func (server *Server) getOperatorBaofuSettlementReadiness(ctx context.Context, operator db.Operator) (logic.BaofuAccountReadiness, error) {
	binding, err := server.store.GetBaofuAccountBindingByOwner(ctx, db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: db.BaofuAccountOwnerTypeOperator,
		OwnerID:   operator.ID,
	})
	service := logic.NewBaofuAccountService(nil, nil)
	if err != nil {
		if isNotFoundError(err) {
			return service.ReadinessFromBinding(db.BaofuAccountBinding{}, false), nil
		}
		return logic.BaofuAccountReadiness{}, err
	}
	return service.ReadinessFromBinding(binding, true), nil
}
