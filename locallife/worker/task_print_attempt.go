package worker

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/cloudprint"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/rs/zerolog/log"
)

func (processor *RedisTaskProcessor) executePrintAttempt(ctx context.Context, orderID int64, printer db.CloudPrinter, content string, slip string, taskKey string) {
	printerProvider, ok := processor.cloudPrinterProvider(printer.PrinterType)
	if !ok {
		log.Warn().
			Int64("order_id", orderID).
			Int64("printer_id", printer.ID).
			Str("printer_type", printer.PrinterType).
			Msg("skip print because cloud printer provider is not configured")
		return
	}
	processor.executePrintAttemptWithProvider(ctx, orderID, printer, printerProvider, content, slip, taskKey)
}

func (processor *RedisTaskProcessor) executePrintAttemptWithProvider(ctx context.Context, orderID int64, printer db.CloudPrinter, printerProvider cloudprint.Client, content string, slip string, taskKey string) {
	if taskKey != "" {
		if existingLog, err := processor.store.GetPrintLogByTaskKeyAndPrinter(ctx, db.GetPrintLogByTaskKeyAndPrinterParams{
			TaskKey:   pgtype.Text{String: taskKey, Valid: true},
			PrinterID: printer.ID,
		}); err == nil {
			log.Info().Int64("order_id", orderID).Int64("printer_id", printer.ID).Int64("print_log_id", existingLog.ID).Str("task_key", taskKey).Msg("skip duplicate order print task re-entry")
			return
		} else if err != db.ErrRecordNotFound {
			log.Error().Err(err).Int64("order_id", orderID).Int64("printer_id", printer.ID).Str("task_key", taskKey).Msg("check print task key failed")
			return
		}
	}

	providerOriginID := newPrintProviderOriginID()
	printLog, err := processor.store.CreatePrintLog(ctx, db.CreatePrintLogParams{
		OrderID:          orderID,
		PrinterID:        printer.ID,
		PrintContent:     content,
		Status:           printLogStatusPending,
		TaskKey:          pgtype.Text{String: taskKey, Valid: taskKey != ""},
		ProviderOriginID: pgtype.Text{String: providerOriginID, Valid: providerOriginID != ""},
	})
	if err != nil {
		log.Error().Err(err).Int64("order_id", orderID).Int64("printer_id", printer.ID).Msg("create print log failed")
		return
	}

	vendorOrderID, printErr := printerProvider.Print(ctx, cloudprint.PrintInput{
		PrinterID:        printer.ID,
		MerchantID:       printer.MerchantID,
		SN:               printer.PrinterSn,
		Content:          content,
		Copies:           1,
		ProviderOriginID: providerOriginID,
	})

	updateParams := db.UpdatePrintLogStatusParams{
		ID:     printLog.ID,
		Status: printLogStatusSuccess,
	}
	trimmedVendorOrderID := strings.TrimSpace(vendorOrderID)
	if trimmedVendorOrderID != "" {
		updateParams.VendorOrderID = pgtype.Text{String: trimmedVendorOrderID, Valid: true}
	}
	if printErr != nil {
		updateParams.Status = printLogStatusFailed
		updateParams.ErrorMessage = pgtype.Text{String: sanitizePrintProviderError(printErr.Error()), Valid: true}
		log.Error().Err(printErr).
			Int64("order_id", orderID).
			Int64("printer_id", printer.ID).
			Str("printer_type", printer.PrinterType).
			Str("slip", slip).
			Msg("print order failed")
	} else if printerProvider.PrintResultCallbackEnabled() {
		if trimmedVendorOrderID == "" {
			updateParams.Status = printLogStatusFailed
			updateParams.ErrorMessage = pgtype.Text{String: printer.PrinterType + " print accepted without vendor order id; callback cannot be matched", Valid: true}
			log.Error().
				Int64("order_id", orderID).
				Int64("printer_id", printer.ID).
				Str("printer_type", printer.PrinterType).
				Str("slip", slip).
				Msg("cloud printer accepted print job without vendor order id")
		} else {
			updateParams.Status = printLogStatusPending
			log.Info().
				Int64("order_id", orderID).
				Int64("printer_id", printer.ID).
				Str("printer_type", printer.PrinterType).
				Str("vendor_order_id", trimmedVendorOrderID).
				Str("slip", slip).
				Msg("cloud printer accepted print job; waiting for print result callback")
		}
	} else if printProviderAcceptanceRequiresStatusQuery(printer.PrinterType) {
		if trimmedVendorOrderID == "" {
			updateParams.Status = printLogStatusFailed
			updateParams.ErrorMessage = pgtype.Text{String: printer.PrinterType + " print accepted without vendor order id; status query cannot be matched", Valid: true}
			log.Error().
				Int64("order_id", orderID).
				Int64("printer_id", printer.ID).
				Str("printer_type", printer.PrinterType).
				Str("slip", slip).
				Msg("cloud printer accepted print job without vendor order id")
		} else {
			updateParams.Status = printLogStatusPending
			log.Info().
				Int64("order_id", orderID).
				Int64("printer_id", printer.ID).
				Str("printer_type", printer.PrinterType).
				Str("vendor_order_id", trimmedVendorOrderID).
				Str("slip", slip).
				Msg("cloud printer accepted print job; waiting for status query")
		}
	} else {
		log.Info().
			Int64("order_id", orderID).
			Int64("printer_id", printer.ID).
			Str("printer_type", printer.PrinterType).
			Str("vendor_order_id", trimmedVendorOrderID).
			Str("slip", slip).
			Msg("printed order successfully")
	}

	if _, err := processor.store.UpdatePrintLogStatus(ctx, updateParams); err != nil {
		log.Error().Err(err).Int64("print_log_id", printLog.ID).Msg("update print log status failed")
	}
}
