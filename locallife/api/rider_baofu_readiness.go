package api

import (
	"context"
	"errors"

	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
)

func (server *Server) ensureRiderBaofuSettlementReady(ctx context.Context, rider db.Rider) error {
	binding, err := server.store.GetBaofuAccountBindingByOwner(ctx, db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: db.BaofuAccountOwnerTypeRider,
		OwnerID:   rider.ID,
	})
	if err != nil {
		if isNotFoundError(err) {
			return ErrRiderBaofuAccountMissing
		}
		return err
	}
	if err := logic.NewBaofuAccountService(nil, nil).ValidatePaymentReady(binding); err != nil {
		if errors.Is(err, logic.ErrBaofuAccountInactive) || errors.Is(err, logic.ErrBaofuAccountReceiverRequired) {
			return ErrRiderBaofuAccountMissing
		}
		return err
	}
	return nil
}
