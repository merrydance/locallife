package db

import (
	"context"
	"errors"
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

type AuthorizeYilianyunCloudPrinterWithDeviceTxParams struct {
	State         string
	Authorization UpsertCloudPrinterProviderAuthorizationParams
	Printer       CreateCloudPrinterParams
	ConsumedAt    time.Time
}

type AuthorizeYilianyunCloudPrinterWithDeviceTxResult struct {
	Session       CloudPrinterAuthorizationSession
	Printer       CloudPrinter
	Authorization CloudPrinterProviderAuthorization
}

type CreateAuthorizedYilianyunCloudPrinterTxParams struct {
	Authorization UpsertCloudPrinterProviderAuthorizationParams
	Printer       CreateCloudPrinterParams
}

type CreateAuthorizedYilianyunCloudPrinterTxResult struct {
	Printer       CloudPrinter
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

func (store *SQLStore) AuthorizeYilianyunCloudPrinterWithDeviceTx(ctx context.Context, arg AuthorizeYilianyunCloudPrinterWithDeviceTxParams) (AuthorizeYilianyunCloudPrinterWithDeviceTxResult, error) {
	var result AuthorizeYilianyunCloudPrinterWithDeviceTxResult
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
		device, err := createOrAttachAuthorizedYilianyunCloudPrinter(ctx, q, arg.Authorization, arg.Printer)
		if err != nil {
			return err
		}

		result.Session = consumed
		result.Printer = device.Printer
		result.Authorization = device.Authorization
		return nil
	})
	return result, err
}

func (store *SQLStore) CreateAuthorizedYilianyunCloudPrinterTx(ctx context.Context, arg CreateAuthorizedYilianyunCloudPrinterTxParams) (CreateAuthorizedYilianyunCloudPrinterTxResult, error) {
	var result CreateAuthorizedYilianyunCloudPrinterTxResult
	err := store.execTx(ctx, func(q *Queries) error {
		device, err := createOrAttachAuthorizedYilianyunCloudPrinter(ctx, q, arg.Authorization, arg.Printer)
		if err != nil {
			return err
		}

		result = device
		return nil
	})
	return result, err
}

func createOrAttachAuthorizedYilianyunCloudPrinter(ctx context.Context, q *Queries, authorizationParams UpsertCloudPrinterProviderAuthorizationParams, printerParams CreateCloudPrinterParams) (CreateAuthorizedYilianyunCloudPrinterTxResult, error) {
	var result CreateAuthorizedYilianyunCloudPrinterTxResult
	if authorizationParams.ProviderType != CloudPrinterProviderYilianyun ||
		printerParams.PrinterType != CloudPrinterProviderYilianyun ||
		authorizationParams.MerchantID != printerParams.MerchantID ||
		authorizationParams.MachineCode != printerParams.PrinterSn ||
		authorizationParams.AuthorizedCloudPrinterID.Valid ||
		printerParams.PrinterKey != "" {
		return result, ErrRecordNotFound
	}

	authorization, err := q.UpsertCloudPrinterProviderAuthorization(ctx, authorizationParams)
	if err != nil {
		return result, err
	}
	if authorization.AuthorizedCloudPrinterID.Valid {
		printer, err := q.GetCloudPrinter(ctx, authorization.AuthorizedCloudPrinterID.Int64)
		if err != nil {
			return result, err
		}
		if printer.MerchantID != authorization.MerchantID ||
			printer.PrinterType != CloudPrinterProviderYilianyun ||
			printer.PrinterSn != authorization.MachineCode ||
			printer.PrinterKey != "" {
			return result, ErrRecordNotFound
		}
		return CreateAuthorizedYilianyunCloudPrinterTxResult{
			Printer:       printer,
			Authorization: authorization,
		}, nil
	}

	printer, err := q.GetCloudPrinterBySN(ctx, authorization.MachineCode)
	if err != nil {
		if !errors.Is(err, ErrRecordNotFound) {
			return result, err
		}
		printer, err = q.CreateCloudPrinter(ctx, printerParams)
		if err != nil {
			return result, err
		}
	} else if printer.MerchantID != authorization.MerchantID ||
		printer.PrinterType != CloudPrinterProviderYilianyun ||
		printer.PrinterSn != authorization.MachineCode ||
		printer.PrinterKey != "" {
		return result, ErrRecordNotFound
	}

	authorization, err = q.AttachCloudPrinterProviderAuthorizationToPrinter(ctx, AttachCloudPrinterProviderAuthorizationToPrinterParams{
		MerchantID:   authorization.MerchantID,
		ProviderType: authorization.ProviderType,
		MachineCode:  authorization.MachineCode,
		AuthorizedCloudPrinterID: pgtype.Int8{
			Int64: printer.ID,
			Valid: true,
		},
	})
	if err != nil {
		return result, err
	}

	return CreateAuthorizedYilianyunCloudPrinterTxResult{
		Printer:       printer,
		Authorization: authorization,
	}, nil
}
