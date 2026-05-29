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

	printerTypeFeieyun = "feieyun"

	printerRoleFront   = "front"
	printerRoleKitchen = "kitchen"

	printDispatchModeSplit      = "split"
	printDispatchModeSingleFull = "single_full"

	printTriggerAccepted = "accepted"
	printTriggerReady    = "ready"
	printTriggerManual   = "manual"

	printSlipFull    = "full"
	printSlipKitchen = "kitchen"
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

type printSettlementBill struct {
	breakdown          logic.MerchantOrderFeeBreakdown
	profitSharingOrder db.ProfitSharingOrder
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

	if processor.printerClient == nil {
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

	printer, err := processor.store.GetCloudPrinter(ctx, printLog.PrinterID)
	if err != nil {
		if err == db.ErrRecordNotFound {
			return nil
		}
		return fmt.Errorf("get cloud printer: %w", err)
	}
	if !printer.IsActive || printer.PrinterType != printerTypeFeieyun {
		log.Warn().Int64("print_log_id", printLogID).Int64("printer_id", printer.ID).Msg("skip print retry because printer is inactive or unsupported")
		return nil
	}

	processor.executePrintAttempt(ctx, printLog.OrderID, printer, printLog.PrintContent, "retry", taskKey)
	return nil
}

func (processor *RedisTaskProcessor) executePrintAttempt(ctx context.Context, orderID int64, printer db.CloudPrinter, content string, slip string, taskKey string) {
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

	printLog, err := processor.store.CreatePrintLog(ctx, db.CreatePrintLogParams{
		OrderID:      orderID,
		PrinterID:    printer.ID,
		PrintContent: content,
		Status:       "pending",
		TaskKey:      pgtype.Text{String: taskKey, Valid: taskKey != ""},
	})
	if err != nil {
		log.Error().Err(err).Int64("order_id", orderID).Int64("printer_id", printer.ID).Msg("create print log failed")
		return
	}

	vendorOrderID, printErr := processor.printerClient.Print(ctx, cloudprint.PrintInput{
		SN:      printer.PrinterSn,
		Content: content,
		Copies:  1,
	})

	updateParams := db.UpdatePrintLogStatusParams{
		ID:     printLog.ID,
		Status: "success",
	}
	trimmedVendorOrderID := strings.TrimSpace(vendorOrderID)
	if trimmedVendorOrderID != "" {
		updateParams.VendorOrderID = pgtype.Text{String: trimmedVendorOrderID, Valid: true}
	}
	if printErr != nil {
		updateParams.Status = "failed"
		updateParams.ErrorMessage = pgtype.Text{String: truncatePrintError(printErr.Error()), Valid: true}
		log.Error().Err(printErr).
			Int64("order_id", orderID).
			Int64("printer_id", printer.ID).
			Str("slip", slip).
			Msg("print order failed")
	} else if processor.printerClient.PrintResultCallbackEnabled() {
		if trimmedVendorOrderID == "" {
			updateParams.Status = "failed"
			updateParams.ErrorMessage = pgtype.Text{String: "feieyun print accepted without vendor order id; callback cannot be matched", Valid: true}
			log.Error().
				Int64("order_id", orderID).
				Int64("printer_id", printer.ID).
				Str("slip", slip).
				Msg("feieyun print accepted without vendor order id")
		} else {
			updateParams.Status = "pending"
			log.Info().
				Int64("order_id", orderID).
				Int64("printer_id", printer.ID).
				Str("vendor_order_id", trimmedVendorOrderID).
				Str("slip", slip).
				Msg("feieyun accepted print job; waiting for print result callback")
		}
	} else {
		log.Info().
			Int64("order_id", orderID).
			Int64("printer_id", printer.ID).
			Str("vendor_order_id", trimmedVendorOrderID).
			Str("slip", slip).
			Msg("printed order successfully")
	}

	if _, err := processor.store.UpdatePrintLogStatus(ctx, updateParams); err != nil {
		log.Error().Err(err).Int64("print_log_id", printLog.ID).Msg("update print log status failed")
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
		if printer.PrinterType != printerTypeFeieyun {
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

	fullContent := buildFeieReceipt(order, items, user, printSlipFull, settlementBill)
	jobs := make([]printJob, 0, len(eligible))
	splitEnabled := config.PrintDispatchMode == printDispatchModeSplit && len(eligible) > 1 && len(frontPrinters) > 0 && len(kitchenPrinters) > 0
	if !splitEnabled {
		for _, printer := range eligible {
			jobs = append(jobs, printJob{printer: printer, slip: printSlipFull, content: fullContent})
		}
		return jobs, nil
	}

	kitchenContent := buildFeieReceipt(order, items, user, printSlipKitchen, nil)
	for _, printer := range frontPrinters {
		jobs = append(jobs, printJob{printer: printer, slip: printSlipFull, content: fullContent})
	}
	for _, printer := range kitchenPrinters {
		jobs = append(jobs, printJob{printer: printer, slip: printSlipKitchen, content: kitchenContent})
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

func buildFeieReceipt(order db.GetOrderWithDetailsRow, items []db.ListOrderItemsWithDishByOrderRow, user db.User, slip string, settlementBill *printSettlementBill) string {
	var builder strings.Builder
	title := "乐客来福"
	if order.PickupCode.Valid && order.PickupCode.String != "" {
		title = order.PickupCode.String + "# 乐客来福"
	}

	builder.WriteString("<CB><B>" + title + "</B></CB><BR>")
	if slip == printSlipKitchen {
		builder.WriteString("<C>后厨单</C><BR>")
	} else {
		builder.WriteString("<C>前台出单</C><BR>")
	}
	builder.WriteString("订单号：" + order.OrderNo + "<BR>")
	builder.WriteString("下单时间：" + order.CreatedAt.Format("2006-01-02 15:04:05") + "<BR>")
	builder.WriteString("类型：" + orderTypeLabel(order.OrderType) + "<BR>")
	builder.WriteString("--------------------------------<BR>")

	for _, item := range items {
		builder.WriteString(formatPrintItemLine(item.Name, item.Quantity, item.Subtotal))
		builder.WriteString("<BR>")
	}

	builder.WriteString("--------------------------------<BR>")
	builder.WriteString("菜品小计：" + fenToYuan(order.Subtotal) + "<BR>")
	if order.DiscountAmount > 0 {
		builder.WriteString("优惠：-" + fenToYuan(order.DiscountAmount) + "<BR>")
	}
	if order.VoucherAmount > 0 {
		builder.WriteString("券抵扣：-" + fenToYuan(order.VoucherAmount) + "<BR>")
	}
	if slip == printSlipFull && settlementBill != nil {
		writeFeieSettlementBill(&builder, settlementBill)
	}

	if order.Notes.Valid && order.Notes.String != "" {
		builder.WriteString("备注：" + order.Notes.String + "<BR>")
	}

	if slip == printSlipFull {
		if customerName := resolvePrintCustomerName(order, user); customerName != "" {
			builder.WriteString("顾客：" + customerName + "<BR>")
		}
		if order.OrderType == db.OrderTypeTakeout && strings.TrimSpace(order.DeliveryAddress) != "" {
			builder.WriteString("地址：" + order.DeliveryAddress + "<BR>")
		}
	}

	builder.WriteString(ticketCodeBlock(order.OrderNo))
	builder.WriteString("<CUT>")
	return builder.String()
}

func writeFeieSettlementBill(builder *strings.Builder, settlementBill *printSettlementBill) {
	breakdown := settlementBill.breakdown
	profitSharingOrder := settlementBill.profitSharingOrder

	builder.WriteString("--------------------------------<BR>")
	builder.WriteString("用户实付：" + fenToYuan(breakdown.CustomerPayableAmount) + "<BR>")
	builder.WriteString("商户账单<BR>")
	builder.WriteString("菜品合计：" + fenToYuan(breakdown.FoodPayableAmount) + "<BR>")
	builder.WriteString(formatPrintDeductionLine("平台服务费", breakdown.PlatformServiceFeeAmount))
	builder.WriteString(formatPrintDeductionLine("支付通道费", breakdown.PaymentChannelFeeAmount))
	builder.WriteString("<BOLD>商户实收：" + fenToYuan(breakdown.MerchantReceivableAmount) + "</BOLD><BR>")

	if profitSharingOrder.RiderGrossAmount > 0 || profitSharingOrder.RiderPaymentFee > 0 || profitSharingOrder.RiderAmount > 0 {
		builder.WriteString("骑手账单<BR>")
		builder.WriteString("代取费：" + fenToYuan(profitSharingOrder.RiderGrossAmount) + "<BR>")
		builder.WriteString(formatPrintDeductionLine("支付通道费", profitSharingOrder.RiderPaymentFee))
		builder.WriteString("骑手实收：" + fenToYuan(profitSharingOrder.RiderAmount) + "<BR>")
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

func resolvePrintCustomerName(order db.GetOrderWithDetailsRow, user db.User) string {
	if name := strings.TrimSpace(order.DeliveryContactName); name != "" {
		return name
	}
	return strings.TrimSpace(user.FullName)
}

func formatPrintItemLine(name string, quantity int16, subtotal int64) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		trimmed = "未命名商品"
	}
	return fmt.Sprintf("%s x%d  %s", trimmed, quantity, fenToYuan(subtotal))
}

func formatPrintDeductionLine(label string, amount int64) string {
	return "- " + label + "：-" + fenToYuan(amount) + "<BR>"
}

func fenToYuan(amount int64) string {
	return fmt.Sprintf("%.2f", float64(amount)/100)
}

func orderTypeLabel(orderType string) string {
	switch orderType {
	case db.OrderTypeTakeout:
		return "外卖"
	case "takeaway":
		return "自取"
	case "dine_in":
		return "堂食"
	case db.OrderTypeReservation:
		return "预订"
	default:
		return orderType
	}
}

func ticketCodeBlock(orderNo string) string {
	upper := strings.ToUpper(orderNo)
	if canUseFeieBarcode(upper) {
		return "<BR><BC128_A>" + upper + "</BC128_A><BR>"
	}
	return "<BR><QR>" + orderNo + "</QR><BR>"
}

func canUseFeieBarcode(value string) bool {
	if len(value) == 0 || len(value) > 14 {
		return false
	}
	for _, ch := range value {
		if (ch < '0' || ch > '9') && (ch < 'A' || ch > 'Z') {
			return false
		}
	}
	return true
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
