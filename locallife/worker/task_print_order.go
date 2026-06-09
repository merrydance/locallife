package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/cloudprint"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
	"github.com/rs/zerolog/log"
)

const (
	TaskPrintOrder = "order:print"

	printerTypeFeieyun   = string(cloudprint.ProviderFeieyun)
	printerTypeYilianyun = string(cloudprint.ProviderYilianyun)
	printerTypeShangpeng = string(cloudprint.ProviderShangpeng)

	printerRoleFront   = "front"
	printerRoleKitchen = "kitchen"

	printDispatchModeSplit      = "split"
	printDispatchModeSingleFull = "single_full"

	printTriggerAccepted = "accepted"
	printTriggerReady    = "ready"
	printTriggerManual   = "manual"

	printSlipFull    = "full"
	printSlipKitchen = "kitchen"

	printLogStatusPending = db.PrintLogStatusPending
	printLogStatusSuccess = db.PrintLogStatusSuccess
	printLogStatusFailed  = db.PrintLogStatusFailed
)

type PrintOrderPayload struct {
	OrderID         int64  `json:"order_id"`
	Trigger         string `json:"trigger"`
	RetryPrintLogID int64  `json:"retry_print_log_id,omitempty"`
	TaskKey         string `json:"task_key,omitempty"`
}

type printJob struct {
	printer db.CloudPrinter
	slip    string
	content string
}

// DistributeTaskPrintOrder 分发订单打印任务。
func (distributor *RedisTaskDistributor) DistributeTaskPrintOrder(
	ctx context.Context,
	payload *PrintOrderPayload,
	opts ...asynq.Option,
) error {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	task := asynq.NewTask(TaskPrintOrder, jsonPayload, opts...)
	info, err := distributor.enqueueTask(ctx, task, opts...)
	if err != nil {
		return fmt.Errorf("enqueue task: %w", err)
	}

	log.Info().
		Str("type", task.Type()).
		Str("queue", info.Queue).
		Int64("order_id", payload.OrderID).
		Str("trigger", payload.Trigger).
		Msg("enqueued order print task")

	return nil
}

// ProcessTaskPrintOrder 处理订单打印任务。
func (processor *RedisTaskProcessor) ProcessTaskPrintOrder(ctx context.Context, task *asynq.Task) error {
	var payload PrintOrderPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", asynq.SkipRetry)
	}

	if !processor.hasAnyCloudPrinterProvider() {
		log.Warn().Int64("order_id", payload.OrderID).Msg("skip order print because cloud printer client is not configured")
		return nil
	}
	taskKey := resolvePrintTaskKey(payload)
	if payload.RetryPrintLogID > 0 {
		return processor.retryPrintLog(ctx, payload.RetryPrintLogID, taskKey)
	}

	order, err := processor.store.GetOrderWithDetails(ctx, payload.OrderID)
	if err != nil {
		if err == db.ErrRecordNotFound {
			return nil
		}
		return fmt.Errorf("get order with details: %w", err)
	}
	if order.Status == db.OrderStatusCancelled {
		return nil
	}

	config, err := processor.store.GetOrderDisplayConfigByMerchant(ctx, order.MerchantID)
	if err != nil {
		if err == db.ErrRecordNotFound {
			config = db.OrderDisplayConfig{
				EnablePrint:       true,
				PrintTakeout:      true,
				PrintDineIn:       true,
				PrintReservation:  true,
				PrintDispatchMode: printDispatchModeSingleFull,
				PrintTriggerMode:  printTriggerAccepted,
			}
		} else {
			return fmt.Errorf("get order display config: %w", err)
		}
	}
	if !config.EnablePrint || !displayConfigSupportsOrder(config, order.OrderType) || !printTriggerMatches(config.PrintTriggerMode, payload.Trigger) {
		return nil
	}

	printers, err := processor.store.ListActiveCloudPrintersByMerchant(ctx, order.MerchantID)
	if err != nil {
		return fmt.Errorf("list active cloud printers: %w", err)
	}
	jobs, err := processor.buildPrintJobs(ctx, order, printers, config)
	if err != nil {
		log.Error().Err(err).
			Int64("order_id", order.ID).
			Int64("merchant_id", order.MerchantID).
			Str("order_no", order.OrderNo).
			Str("status", order.Status).
			Msg("build order print jobs failed")
		return err
	}
	if len(jobs) == 0 {
		return nil
	}

	for _, job := range jobs {
		processor.executePrintAttempt(ctx, order.ID, job.printer, job.content, job.slip, taskKey)
	}

	return nil
}

func (processor *RedisTaskProcessor) retryPrintLog(ctx context.Context, printLogID int64, taskKey string) error {
	printLog, err := processor.store.GetPrintLog(ctx, printLogID)
	if err != nil {
		if err == db.ErrRecordNotFound {
			return nil
		}
		return fmt.Errorf("get print log: %w", err)
	}

	printer, err := processor.store.GetCloudPrinterIncludingDeleted(ctx, printLog.PrinterID)
	if err != nil {
		if err == db.ErrRecordNotFound {
			return nil
		}
		return fmt.Errorf("get cloud printer: %w", err)
	}
	if printer.DeletedAt.Valid {
		processor.recordSkippedPrintRetry(ctx, printLog, printer, taskKey, "printer is deleted")
		return nil
	}
	printerProvider, ok := processor.cloudPrinterProvider(printer.PrinterType)
	if !printer.IsActive || !ok {
		reason := "printer is inactive"
		if !ok {
			reason = "printer provider is not configured"
		}
		processor.recordSkippedPrintRetry(ctx, printLog, printer, taskKey, reason)
		return nil
	}

	processor.executePrintAttemptWithProvider(ctx, printLog.OrderID, printer, printerProvider, printLog.PrintContent, "retry", taskKey)
	return nil
}

func (processor *RedisTaskProcessor) recordSkippedPrintRetry(ctx context.Context, printLog db.PrintLog, printer db.CloudPrinter, taskKey string, reason string) {
	log.Warn().
		Int64("print_log_id", printLog.ID).
		Int64("printer_id", printer.ID).
		Str("printer_type", printer.PrinterType).
		Str("reason", reason).
		Msg("record skipped print retry")

	if taskKey != "" {
		if existingLog, err := processor.store.GetPrintLogByTaskKeyAndPrinter(ctx, db.GetPrintLogByTaskKeyAndPrinterParams{
			TaskKey:   pgtype.Text{String: taskKey, Valid: true},
			PrinterID: printer.ID,
		}); err == nil {
			log.Info().Int64("order_id", printLog.OrderID).Int64("printer_id", printer.ID).Int64("print_log_id", existingLog.ID).Str("task_key", taskKey).Msg("skip duplicate print retry failure record")
			return
		} else if err != db.ErrRecordNotFound {
			log.Error().Err(err).Int64("order_id", printLog.OrderID).Int64("printer_id", printer.ID).Str("task_key", taskKey).Msg("check print retry task key failed")
			return
		}
	}

	providerOriginID := newPrintProviderOriginID()
	failedLog, err := processor.store.CreatePrintLog(ctx, db.CreatePrintLogParams{
		OrderID:          printLog.OrderID,
		PrinterID:        printer.ID,
		PrintContent:     printLog.PrintContent,
		Status:           printLogStatusFailed,
		TaskKey:          pgtype.Text{String: taskKey, Valid: taskKey != ""},
		ProviderOriginID: pgtype.Text{String: providerOriginID, Valid: providerOriginID != ""},
	})
	if err != nil {
		log.Error().Err(err).Int64("order_id", printLog.OrderID).Int64("printer_id", printer.ID).Msg("create skipped print retry log failed")
		return
	}

	if _, err := processor.store.UpdatePrintLogStatus(ctx, db.UpdatePrintLogStatusParams{
		ID:           failedLog.ID,
		Status:       printLogStatusFailed,
		ErrorMessage: pgtype.Text{String: reason, Valid: true},
	}); err != nil {
		log.Error().Err(err).Int64("print_log_id", failedLog.ID).Msg("update skipped print retry log failed")
	}
}

func (processor *RedisTaskProcessor) cloudPrinterProvider(providerType string) (cloudprint.Client, bool) {
	if providerType == printerTypeYilianyun {
		provider := newYilianyunRuntimeClient(processor.config, processor.store, processor.dataEncryptor)
		return provider, provider != nil
	}
	if processor.cloudPrinterManager != nil {
		if provider, ok := processor.cloudPrinterManager.Provider(providerType); ok && provider != nil {
			return provider, true
		}
	}
	if providerType == printerTypeFeieyun && processor.printerClient != nil {
		return processor.printerClient, true
	}
	return nil, false
}

func (processor *RedisTaskProcessor) hasAnyCloudPrinterProvider() bool {
	if newYilianyunRuntimeClient(processor.config, processor.store, processor.dataEncryptor) != nil {
		return true
	}
	if checker, ok := processor.cloudPrinterManager.(interface{ Configured() bool }); ok {
		return checker.Configured()
	}
	return processor.printerClient != nil
}

func printProviderAcceptanceRequiresStatusQuery(providerType string) bool {
	switch providerType {
	case printerTypeShangpeng:
		return true
	default:
		return false
	}
}

func resolvePrintTaskKey(payload PrintOrderPayload) string {
	if payload.TaskKey != "" {
		return payload.TaskKey
	}
	if payload.RetryPrintLogID > 0 {
		return fmt.Sprintf("retry-legacy:%d", payload.RetryPrintLogID)
	}
	if payload.OrderID > 0 && payload.Trigger != "" && payload.Trigger != printTriggerManual {
		return fmt.Sprintf("order-legacy:%d:%s", payload.OrderID, payload.Trigger)
	}
	return ""
}

func (processor *RedisTaskProcessor) buildPrintJobs(
	ctx context.Context,
	order db.GetOrderWithDetailsRow,
	printers []db.CloudPrinter,
	config db.OrderDisplayConfig,
) ([]printJob, error) {
	eligible := make([]db.CloudPrinter, 0, len(printers))
	frontPrinters := make([]db.CloudPrinter, 0, len(printers))
	kitchenPrinters := make([]db.CloudPrinter, 0, len(printers))
	for _, printer := range printers {
		if _, ok := processor.cloudPrinterProvider(printer.PrinterType); !ok {
			log.Warn().
				Int64("printer_id", printer.ID).
				Str("printer_type", printer.PrinterType).
				Msg("skip unsupported printer type for order print")
			continue
		}
		if !printerSupportsOrder(printer, order.OrderType) {
			continue
		}
		eligible = append(eligible, printer)
		switch printer.PrinterRole {
		case printerRoleFront:
			frontPrinters = append(frontPrinters, printer)
		case printerRoleKitchen:
			kitchenPrinters = append(kitchenPrinters, printer)
		default:
			frontPrinters = append(frontPrinters, printer)
		}
	}
	if len(eligible) == 0 {
		return nil, nil
	}

	items, err := processor.store.ListOrderItemsWithDishByOrder(ctx, order.ID)
	if err != nil {
		return nil, fmt.Errorf("list order items with dish: %w", err)
	}

	var user db.User
	if len(frontPrinters) > 0 || (len(eligible) == 1 && len(kitchenPrinters) == 1) {
		user, err = processor.store.GetUser(ctx, order.UserID)
		if err != nil && err != db.ErrRecordNotFound {
			return nil, fmt.Errorf("get user for print: %w", err)
		}
	}

	settlementBill, err := processor.loadPrintSettlementBill(ctx, order)
	if err != nil {
		return nil, fmt.Errorf("load print settlement bill: %w", err)
	}

	jobs := make([]printJob, 0, len(eligible))
	splitEnabled := config.PrintDispatchMode == printDispatchModeSplit && len(eligible) > 1 && len(frontPrinters) > 0 && len(kitchenPrinters) > 0
	if !splitEnabled {
		for _, printer := range eligible {
			jobs = append(jobs, printJob{printer: printer, slip: printSlipFull, content: buildReceiptForProvider(printer.PrinterType, order, items, user, printSlipFull, settlementBill)})
		}
		return jobs, nil
	}

	for _, printer := range frontPrinters {
		jobs = append(jobs, printJob{printer: printer, slip: printSlipFull, content: buildReceiptForProvider(printer.PrinterType, order, items, user, printSlipFull, settlementBill)})
	}
	for _, printer := range kitchenPrinters {
		jobs = append(jobs, printJob{printer: printer, slip: printSlipKitchen, content: buildReceiptForProvider(printer.PrinterType, order, items, user, printSlipKitchen, nil)})
	}
	return jobs, nil
}

func (processor *RedisTaskProcessor) loadPrintSettlementBill(ctx context.Context, order db.GetOrderWithDetailsRow) (*printSettlementBill, error) {
	paymentOrder, err := processor.store.GetLatestPaymentOrderByOrder(ctx, db.GetLatestPaymentOrderByOrderParams{
		OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
		BusinessType: db.ExternalPaymentBusinessOwnerOrder,
	})
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get latest payment order by order: %w", err)
	}
	if !paymentOrder.OrderID.Valid || paymentOrder.OrderID.Int64 != order.ID || paymentOrder.BusinessType != db.ExternalPaymentBusinessOwnerOrder {
		return nil, fmt.Errorf("%w: payment order mismatch for order_id=%d payment_order_id=%d", logic.ErrMerchantFeeBreakdownInconsistent, order.ID, paymentOrder.ID)
	}
	if !db.PaymentOrderRequiresProfitSharing(paymentOrder) {
		return nil, nil
	}
	if paymentOrder.Status != "paid" {
		return nil, fmt.Errorf("baofu profit sharing receipt requires paid payment order: order_id=%d payment_order_id=%d status=%s", order.ID, paymentOrder.ID, paymentOrder.Status)
	}
	if paymentOrder.Amount != order.TotalAmount {
		return nil, fmt.Errorf("%w: payment amount mismatch for order_id=%d payment_order_id=%d", logic.ErrMerchantFeeBreakdownInconsistent, order.ID, paymentOrder.ID)
	}

	profitSharingOrder, err := processor.store.GetProfitSharingOrderByPaymentOrder(ctx, paymentOrder.ID)
	if err != nil {
		return nil, fmt.Errorf("get profit sharing order by payment order: %w", err)
	}
	if profitSharingOrder.PaymentOrderID != paymentOrder.ID ||
		profitSharingOrder.MerchantID != order.MerchantID ||
		profitSharingOrder.Provider != db.ExternalPaymentProviderBaofu ||
		profitSharingOrder.Channel != db.PaymentChannelBaofuAggregate {
		return nil, fmt.Errorf("%w: profit sharing bill mismatch for order_id=%d payment_order_id=%d profit_sharing_order_id=%d", logic.ErrMerchantFeeBreakdownInconsistent, order.ID, paymentOrder.ID, profitSharingOrder.ID)
	}

	breakdown, err := logic.BuildMerchantOrderFeeBreakdown(logic.BuildMerchantOrderFeeBreakdownInput{
		Order:              orderDetailsRowToOrder(order),
		ProfitSharingOrder: &profitSharingOrder,
	})
	if err != nil {
		return nil, err
	}
	return &printSettlementBill{
		breakdown:          breakdown,
		profitSharingOrder: profitSharingOrder,
	}, nil
}

func orderDetailsRowToOrder(order db.GetOrderWithDetailsRow) db.Order {
	return db.Order{
		ID:                  order.ID,
		OrderNo:             order.OrderNo,
		UserID:              order.UserID,
		MerchantID:          order.MerchantID,
		OrderType:           order.OrderType,
		AddressID:           order.AddressID,
		DeliveryFee:         order.DeliveryFee,
		DeliveryDistance:    order.DeliveryDistance,
		TableID:             order.TableID,
		ReservationID:       order.ReservationID,
		Subtotal:            order.Subtotal,
		DiscountAmount:      order.DiscountAmount,
		DeliveryFeeDiscount: order.DeliveryFeeDiscount,
		TotalAmount:         order.TotalAmount,
		Status:              order.Status,
		PaymentMethod:       order.PaymentMethod,
		PaidAt:              order.PaidAt,
		Notes:               order.Notes,
		CreatedAt:           order.CreatedAt,
		UpdatedAt:           order.UpdatedAt,
		CompletedAt:         order.CompletedAt,
		CancelledAt:         order.CancelledAt,
		CancelReason:        order.CancelReason,
		FinalAmount:         order.FinalAmount,
		PlatformCommission:  order.PlatformCommission,
		UserVoucherID:       order.UserVoucherID,
		VoucherAmount:       order.VoucherAmount,
		BalancePaid:         order.BalancePaid,
		MembershipID:        order.MembershipID,
	}
}

func displayConfigSupportsOrder(config db.OrderDisplayConfig, orderType string) bool {
	switch orderType {
	case db.OrderTypeTakeout, "takeaway":
		return config.PrintTakeout
	case "dine_in":
		return config.PrintDineIn
	case db.OrderTypeReservation:
		return config.PrintReservation
	default:
		return false
	}
}

func printTriggerMatches(configTrigger string, trigger string) bool {
	switch configTrigger {
	case "", printTriggerAccepted:
		return trigger == printTriggerAccepted
	case printTriggerReady:
		return trigger == printTriggerReady
	case printTriggerManual:
		return trigger == printTriggerManual
	default:
		return false
	}
}

func printerSupportsOrder(printer db.CloudPrinter, orderType string) bool {
	switch orderType {
	case db.OrderTypeTakeout, "takeaway":
		return printer.PrintTakeout
	case "dine_in":
		return printer.PrintDineIn
	case db.OrderTypeReservation:
		return printer.PrintReservation
	default:
		return false
	}
}

func truncatePrintError(message string) string {
	message = strings.TrimSpace(message)
	if len(message) <= 500 {
		return message
	}
	return message[:500]
}

type printClientStub interface {
	cloudprint.Client
}

var _ printClientStub = (*cloudprint.FeieyunClient)(nil)
