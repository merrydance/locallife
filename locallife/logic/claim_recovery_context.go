package logic

import (
	"context"
	"errors"
	"net/http"

	db "github.com/merrydance/locallife/db/sqlc"
)

func getClaimRecoveryContextByID(ctx context.Context, store db.Store, recoveryID int64) (db.GetClaimRecoveryContextByIDRow, error) {
	recoveryCtx, err := store.GetClaimRecoveryContextByID(ctx, recoveryID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return db.GetClaimRecoveryContextByIDRow{}, NewRequestError(http.StatusNotFound, errors.New("claim recovery not found"))
		}
		return db.GetClaimRecoveryContextByIDRow{}, err
	}
	return recoveryCtx, nil
}

func claimRecoveryFromContextByID(recoveryCtx db.GetClaimRecoveryContextByIDRow) db.ClaimRecovery {
	return db.ClaimRecovery{
		ID:               recoveryCtx.ID,
		ClaimID:          recoveryCtx.ClaimID,
		OrderID:          recoveryCtx.OrderID,
		ResponsibleParty: recoveryCtx.ResponsibleParty,
		RecoveryTarget:   recoveryCtx.RecoveryTarget,
		RecoveryAmount:   recoveryCtx.RecoveryAmount,
		Status:           recoveryCtx.Status,
		DueAt:            recoveryCtx.DueAt,
		DecisionSnapshot: recoveryCtx.DecisionSnapshot,
		CreatedAt:        recoveryCtx.CreatedAt,
		UpdatedAt:        recoveryCtx.UpdatedAt,
		DecisionID:       recoveryCtx.DecisionID,
		RecoveryBasis:    recoveryCtx.RecoveryBasis,
	}
}
