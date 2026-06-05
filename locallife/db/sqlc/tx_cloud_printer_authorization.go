package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type AuthorizeYilianyunCloudPrinterTxParams struct {
	State         string
	Authorization UpsertCloudPrinterProviderAuthorizationParams
	ConsumedAt    time.Time
}

type AuthorizeYilianyunCloudPrinterTxResult struct {
	Session       CloudPrinterAuthorizationSession
	Authorization CloudPrinterProviderAuthorization
}

func (store *SQLStore) AuthorizeYilianyunCloudPrinterTx(ctx context.Context, arg AuthorizeYilianyunCloudPrinterTxParams) (AuthorizeYilianyunCloudPrinterTxResult, error) {
	var result AuthorizeYilianyunCloudPrinterTxResult
	consumedAt := arg.ConsumedAt
	if consumedAt.IsZero() {
		consumedAt = time.Now().UTC()
	}

	err := store.execTx(ctx, func(q *Queries) error {
		session, err := q.GetActiveCloudPrinterAuthorizationSessionForUpdate(ctx, arg.State)
		if err != nil {
			return err
		}
		if session.MerchantID != arg.Authorization.MerchantID || session.ProviderType != arg.Authorization.ProviderType {
			return ErrRecordNotFound
		}

		consumed, err := q.ConsumeCloudPrinterAuthorizationSession(ctx, ConsumeCloudPrinterAuthorizationSessionParams{
			ID:         session.ID,
			ConsumedAt: pgtype.Timestamptz{Time: consumedAt, Valid: true},
		})
		if err != nil {
			return err
		}
		authorization, err := q.UpsertCloudPrinterProviderAuthorization(ctx, arg.Authorization)
		if err != nil {
			return err
		}

		result.Session = consumed
		result.Authorization = authorization
		return nil
	})
	return result, err
}
