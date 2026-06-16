package db

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

type ProcessSelfCloudPrintCallbackTxParams struct {
	EventID        string
	PrintJobID     string
	PrintLogID     pgtype.Int8
	CallbackStatus string
	PrintLogStatus string
	ErrorMessage   pgtype.Text
	RawPayload     []byte
}

type ProcessSelfCloudPrintCallbackTxResult struct {
	Event           SelfCloudPrintCallbackEvent
	PrintLog        PrintLog
	Duplicate       bool
	AlreadyTerminal bool
}

func (store *SQLStore) ProcessSelfCloudPrintCallbackTx(ctx context.Context, arg ProcessSelfCloudPrintCallbackTxParams) (ProcessSelfCloudPrintCallbackTxResult, error) {
	var result ProcessSelfCloudPrintCallbackTxResult

	err := store.execTx(ctx, func(q *Queries) error {
		existing, err := q.GetSelfCloudPrintCallbackEventByEventID(ctx, strings.TrimSpace(arg.EventID))
		if err == nil {
			result.Event = existing
			result.Duplicate = true
			return nil
		}
		if !errors.Is(err, ErrRecordNotFound) {
			return fmt.Errorf("get self-cloud print callback event: %w", err)
		}

		printLog, err := findSelfCloudPrintLogForCallback(ctx, q, arg)
		if err != nil {
			return err
		}
		result.PrintLog = printLog

		event, err := q.CreateSelfCloudPrintCallbackEvent(ctx, CreateSelfCloudPrintCallbackEventParams{
			EventID:    strings.TrimSpace(arg.EventID),
			PrintJobID: strings.TrimSpace(arg.PrintJobID),
			PrintLogID: pgtype.Int8{Int64: printLog.ID, Valid: true},
			Status:     strings.TrimSpace(arg.CallbackStatus),
			RawPayload: arg.RawPayload,
		})
		if err != nil {
			if ErrorCode(err) == UniqueViolation {
				existing, getErr := q.GetSelfCloudPrintCallbackEventByEventID(ctx, strings.TrimSpace(arg.EventID))
				if getErr != nil {
					return fmt.Errorf("get duplicate self-cloud print callback event: %w", getErr)
				}
				result.Event = existing
				result.Duplicate = true
				return nil
			}
			return fmt.Errorf("create self-cloud print callback event: %w", err)
		}
		result.Event = event

		updated, err := q.MarkProviderStatusPrintLogTerminal(ctx, MarkProviderStatusPrintLogTerminalParams{
			ID:           printLog.ID,
			Status:       strings.TrimSpace(arg.PrintLogStatus),
			ErrorMessage: arg.ErrorMessage,
		})
		if err != nil {
			if errors.Is(err, ErrRecordNotFound) {
				current, getErr := findSelfCloudPrintLogForCallback(ctx, q, arg)
				if getErr != nil {
					return getErr
				}
				result.PrintLog = current
				result.AlreadyTerminal = true
				return nil
			}
			return fmt.Errorf("mark self-cloud print log terminal: %w", err)
		}
		result.PrintLog = updated
		return nil
	})

	return result, err
}

func findSelfCloudPrintLogForCallback(ctx context.Context, q *Queries, arg ProcessSelfCloudPrintCallbackTxParams) (PrintLog, error) {
	printJobID := pgtype.Text{String: strings.TrimSpace(arg.PrintJobID), Valid: strings.TrimSpace(arg.PrintJobID) != ""}
	if arg.PrintLogID.Valid {
		return q.GetPrintLogByIDProviderAndVendorOrderID(ctx, GetPrintLogByIDProviderAndVendorOrderIDParams{
			ID:            arg.PrintLogID.Int64,
			PrinterType:   CloudPrinterProviderSelfCloud,
			VendorOrderID: printJobID,
		})
	}
	return q.GetPrintLogByProviderAndVendorOrderID(ctx, GetPrintLogByProviderAndVendorOrderIDParams{
		PrinterType:   CloudPrinterProviderSelfCloud,
		VendorOrderID: printJobID,
	})
}
