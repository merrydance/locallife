package api

import (
	"context"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
)

func (server *Server) ensureRiderBaofuSettlementReady(ctx context.Context, rider db.Rider) error {
	readiness, err := server.getRiderBaofuSettlementReadiness(ctx, rider)
	if err != nil {
		return err
	}
	if !readiness.PaymentReady {
		return ErrRiderBaofuAccountMissing
	}
	return nil
}

func (server *Server) getRiderBaofuSettlementReadiness(ctx context.Context, rider db.Rider) (logic.BaofuAccountReadiness, error) {
	binding, err := server.store.GetBaofuAccountBindingByOwner(ctx, db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: db.BaofuAccountOwnerTypeRider,
		OwnerID:   rider.ID,
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

func newBaofuSettlementReadinessResponse(readiness logic.BaofuAccountReadiness) *baofuSettlementReadinessResponse {
	return &baofuSettlementReadinessResponse{
		State:        readiness.State,
		Label:        readiness.Label,
		PaymentReady: readiness.PaymentReady,
	}
}
