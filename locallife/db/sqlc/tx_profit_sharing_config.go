package db

import (
	"context"
	"fmt"
	"strconv"
)

// CreateProfitSharingConfigTxParams contains the input parameters for creating a profit sharing config with audit context.
type CreateProfitSharingConfigTxParams struct {
	ActorID   int64
	ActorRole string
	Params    CreateProfitSharingConfigParams
}

// CreateProfitSharingConfigTxResult contains the result of create transaction.
type CreateProfitSharingConfigTxResult struct {
	Config ProfitSharingConfig
}

// CreateProfitSharingConfigTx creates a profit sharing config within a transaction and sets audit actor.
func (store *SQLStore) CreateProfitSharingConfigTx(ctx context.Context, arg CreateProfitSharingConfigTxParams) (CreateProfitSharingConfigTxResult, error) {
	var result CreateProfitSharingConfigTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		if err := q.SetProfitSharingAuditActor(ctx, SetProfitSharingAuditActorParams{
			Column1:     strconv.FormatInt(arg.ActorID, 10),
			SetConfig:   arg.ActorRole,
			SetConfig_2: "",
		}); err != nil {
			return fmt.Errorf("set audit actor: %w", err)
		}

		var err error
		result.Config, err = q.CreateProfitSharingConfig(ctx, arg.Params)
		if err != nil {
			return fmt.Errorf("create profit sharing config: %w", err)
		}
		return nil
	})

	return result, err
}

// UpdateProfitSharingConfigTxParams contains the input parameters for updating a profit sharing config with audit context.
type UpdateProfitSharingConfigTxParams struct {
	ActorID   int64
	ActorRole string
	Params    UpdateProfitSharingConfigParams
}

// UpdateProfitSharingConfigTxResult contains the result of update transaction.
type UpdateProfitSharingConfigTxResult struct {
	Config ProfitSharingConfig
}

// UpdateProfitSharingConfigTx updates a profit sharing config within a transaction and sets audit actor.
func (store *SQLStore) UpdateProfitSharingConfigTx(ctx context.Context, arg UpdateProfitSharingConfigTxParams) (UpdateProfitSharingConfigTxResult, error) {
	var result UpdateProfitSharingConfigTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		if err := q.SetProfitSharingAuditActor(ctx, SetProfitSharingAuditActorParams{
			Column1:     strconv.FormatInt(arg.ActorID, 10),
			SetConfig:   arg.ActorRole,
			SetConfig_2: "",
		}); err != nil {
			return fmt.Errorf("set audit actor: %w", err)
		}

		var err error
		result.Config, err = q.UpdateProfitSharingConfig(ctx, arg.Params)
		if err != nil {
			return fmt.Errorf("update profit sharing config: %w", err)
		}
		return nil
	})

	return result, err
}

// UpdateProfitSharingConfigStatusTxParams contains the input parameters for updating config status with audit context.
type UpdateProfitSharingConfigStatusTxParams struct {
	ActorID   int64
	ActorRole string
	Note      string
	Params    UpdateProfitSharingConfigStatusParams
}

// UpdateProfitSharingConfigStatusTxResult contains the result of update status transaction.
type UpdateProfitSharingConfigStatusTxResult struct {
	Config ProfitSharingConfig
}

// UpdateProfitSharingConfigStatusTx updates a profit sharing config status within a transaction and sets audit actor.
func (store *SQLStore) UpdateProfitSharingConfigStatusTx(ctx context.Context, arg UpdateProfitSharingConfigStatusTxParams) (UpdateProfitSharingConfigStatusTxResult, error) {
	var result UpdateProfitSharingConfigStatusTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		if err := q.SetProfitSharingAuditActor(ctx, SetProfitSharingAuditActorParams{
			Column1:     strconv.FormatInt(arg.ActorID, 10),
			SetConfig:   arg.ActorRole,
			SetConfig_2: arg.Note,
		}); err != nil {
			return fmt.Errorf("set audit actor: %w", err)
		}

		var err error
		result.Config, err = q.UpdateProfitSharingConfigStatus(ctx, arg.Params)
		if err != nil {
			return fmt.Errorf("update profit sharing config status: %w", err)
		}
		return nil
	})

	return result, err
}
