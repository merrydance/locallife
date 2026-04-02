package worker

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/cloudprint"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type printClientRecorder struct {
	inputs []printInputSnapshot
}

type printInputSnapshot struct {
	SN      string
	Content string
	Copies  int
}

func (r *printClientRecorder) AddPrinter(ctx context.Context, input cloudprint.AddPrinterInput) error {
	return nil
}

func (r *printClientRecorder) RemovePrinter(ctx context.Context, input cloudprint.RemovePrinterInput) error {
	return nil
}

func (r *printClientRecorder) Print(ctx context.Context, input cloudprint.PrintInput) (string, error) {
	r.inputs = append(r.inputs, printInputSnapshot{SN: input.SN, Content: input.Content, Copies: input.Copies})
	return "vendor-order-id", nil
}

func (r *printClientRecorder) QueryOrderState(ctx context.Context, orderID string) (bool, error) {
	return false, nil
}

func (r *printClientRecorder) QueryPrinterStatus(ctx context.Context, sn string) (string, error) {
	return "在线，工作状态正常", nil
}

func (r *printClientRecorder) GetPrinterInfo(ctx context.Context, sn string) (cloudprint.PrinterInfo, error) {
	return cloudprint.PrinterInfo{Model: "FEIE-80"}, nil
}

func TestProcessTaskPrintOrder_SplitFrontAndKitchenReceipts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := NewTestTaskProcessor(store, NewNoopTaskDistributor(), nil, nil)
	printerClient := &printClientRecorder{}
	processor.SetPrinterClientForTest(printerClient)

	order := db.GetOrderWithDetailsRow{
		ID:                  100,
		UserID:              200,
		MerchantID:          300,
		OrderNo:             "20260401093000ab12cd",
		OrderType:           db.OrderTypeTakeout,
		Status:              db.OrderStatusPreparing,
		Subtotal:            2800,
		DeliveryFee:         300,
		TotalAmount:         3100,
		Notes:               pgText("少辣"),
		PickupCode:          pgText("105"),
		CreatedAt:           time.Date(2026, 4, 1, 9, 30, 0, 0, time.Local),
		DeliveryContactName: "张三",
		DeliveryAddress:     "测试路 88 号",
	}
	config := db.OrderDisplayConfig{
		MerchantID:        order.MerchantID,
		EnablePrint:       true,
		PrintTakeout:      true,
		PrintDineIn:       true,
		PrintReservation:  true,
		PrintDispatchMode: "split",
		PrintTriggerMode:  "accepted",
	}
	frontPrinter := db.CloudPrinter{ID: 1, MerchantID: order.MerchantID, PrinterName: "前台", PrinterSn: "front-sn", PrinterType: "feieyun", PrinterRole: "front", PrintTakeout: true, IsActive: true}
	kitchenPrinter := db.CloudPrinter{ID: 2, MerchantID: order.MerchantID, PrinterName: "后厨", PrinterSn: "kitchen-sn", PrinterType: "feieyun", PrinterRole: "kitchen", PrintTakeout: true, IsActive: true}
	items := []db.ListOrderItemsWithDishByOrderRow{{Name: "牛肉面", Quantity: 2, Subtotal: 2800}}

	store.EXPECT().GetOrderWithDetails(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetOrderDisplayConfigByMerchant(gomock.Any(), order.MerchantID).Return(config, nil)
	store.EXPECT().ListActiveCloudPrintersByMerchant(gomock.Any(), order.MerchantID).Return([]db.CloudPrinter{frontPrinter, kitchenPrinter}, nil)
	store.EXPECT().ListOrderItemsWithDishByOrder(gomock.Any(), order.ID).Return(items, nil)
	store.EXPECT().GetUser(gomock.Any(), order.UserID).Return(db.User{ID: order.UserID, FullName: "张三"}, nil)
	store.EXPECT().GetPrintLogByTaskKeyAndPrinter(gomock.Any(), gomock.Any()).Times(2).Return(db.PrintLog{}, db.ErrRecordNotFound)
	store.EXPECT().CreatePrintLog(gomock.Any(), gomock.Any()).Times(2).DoAndReturn(func(_ context.Context, arg db.CreatePrintLogParams) (db.PrintLog, error) {
		require.True(t, arg.TaskKey.Valid)
		return db.PrintLog{ID: time.Now().UnixNano(), OrderID: arg.OrderID, PrinterID: arg.PrinterID, Status: arg.Status}, nil
	})
	store.EXPECT().UpdatePrintLogStatus(gomock.Any(), gomock.Any()).Times(2).DoAndReturn(func(_ context.Context, arg db.UpdatePrintLogStatusParams) (db.PrintLog, error) {
		require.True(t, arg.VendorOrderID.Valid)
		require.Equal(t, "vendor-order-id", arg.VendorOrderID.String)
		return db.PrintLog{}, nil
	})

	payload, err := json.Marshal(PrintOrderPayload{OrderID: order.ID, Trigger: "accepted", TaskKey: "order:100:accepted"})
	require.NoError(t, err)

	err = processor.ProcessTaskPrintOrder(context.Background(), asynq.NewTask(TaskPrintOrder, payload))
	require.NoError(t, err)
	require.Len(t, printerClient.inputs, 2)
	require.Equal(t, "front-sn", printerClient.inputs[0].SN)
	require.Contains(t, printerClient.inputs[0].Content, "顾客：张三")
	require.Contains(t, printerClient.inputs[0].Content, "地址：测试路 88 号")
	require.Equal(t, "kitchen-sn", printerClient.inputs[1].SN)
	require.Contains(t, printerClient.inputs[1].Content, "后厨单")
	require.NotContains(t, printerClient.inputs[1].Content, "顾客：")
	require.NotContains(t, printerClient.inputs[1].Content, "地址：")
	require.True(t, strings.Contains(printerClient.inputs[0].Content, "<QR>") || strings.Contains(printerClient.inputs[0].Content, "<BC128_A>"))
}

func TestProcessTaskPrintOrder_ManualTriggerRequiresManualConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := NewTestTaskProcessor(store, NewNoopTaskDistributor(), nil, nil)
	printerClient := &printClientRecorder{}
	processor.SetPrinterClientForTest(printerClient)

	order := db.GetOrderWithDetailsRow{
		ID:         101,
		UserID:     201,
		MerchantID: 301,
		OrderNo:    "20260401101500ABCD",
		OrderType:  db.OrderTypeTakeout,
		Status:     db.OrderStatusPreparing,
	}

	store.EXPECT().GetOrderWithDetails(gomock.Any(), order.ID).Times(2).Return(order, nil)
	store.EXPECT().GetOrderDisplayConfigByMerchant(gomock.Any(), order.MerchantID).Return(db.OrderDisplayConfig{
		MerchantID:        order.MerchantID,
		EnablePrint:       true,
		PrintTakeout:      true,
		PrintDineIn:       true,
		PrintReservation:  true,
		PrintDispatchMode: "single_full",
		PrintTriggerMode:  "accepted",
	}, nil)

	payload, err := json.Marshal(PrintOrderPayload{OrderID: order.ID, Trigger: "manual"})
	require.NoError(t, err)
	err = processor.ProcessTaskPrintOrder(context.Background(), asynq.NewTask(TaskPrintOrder, payload))
	require.NoError(t, err)
	require.Empty(t, printerClient.inputs)

	store.EXPECT().GetOrderDisplayConfigByMerchant(gomock.Any(), order.MerchantID).Return(db.OrderDisplayConfig{
		MerchantID:        order.MerchantID,
		EnablePrint:       true,
		PrintTakeout:      true,
		PrintDineIn:       true,
		PrintReservation:  true,
		PrintDispatchMode: "single_full",
		PrintTriggerMode:  "manual",
	}, nil)
	store.EXPECT().ListActiveCloudPrintersByMerchant(gomock.Any(), order.MerchantID).Return([]db.CloudPrinter{{
		ID:           1,
		MerchantID:   order.MerchantID,
		PrinterName:  "前台",
		PrinterSn:    "front-sn",
		PrinterType:  "feieyun",
		PrinterRole:  "front",
		PrintTakeout: true,
		IsActive:     true,
	}}, nil)
	store.EXPECT().ListOrderItemsWithDishByOrder(gomock.Any(), order.ID).Return([]db.ListOrderItemsWithDishByOrderRow{{Name: "牛肉面", Quantity: 1, Subtotal: 1800}}, nil)
	store.EXPECT().GetUser(gomock.Any(), order.UserID).Return(db.User{ID: order.UserID, FullName: "张三"}, nil)
	store.EXPECT().GetPrintLogByTaskKeyAndPrinter(gomock.Any(), gomock.Any()).Times(1).Return(db.PrintLog{}, db.ErrRecordNotFound)
	store.EXPECT().CreatePrintLog(gomock.Any(), gomock.Any()).Return(db.PrintLog{ID: 1, OrderID: order.ID, PrinterID: 1, Status: "pending"}, nil)
	store.EXPECT().UpdatePrintLogStatus(gomock.Any(), gomock.Any()).Return(db.PrintLog{}, nil)

	payload, err = json.Marshal(PrintOrderPayload{OrderID: order.ID, Trigger: "manual", TaskKey: "manual:101"})
	require.NoError(t, err)
	err = processor.ProcessTaskPrintOrder(context.Background(), asynq.NewTask(TaskPrintOrder, payload))
	require.NoError(t, err)
	require.Len(t, printerClient.inputs, 1)
}

func TestProcessTaskPrintOrder_SkipsUnsupportedPrinterType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := NewTestTaskProcessor(store, NewNoopTaskDistributor(), nil, nil)
	printerClient := &printClientRecorder{}
	processor.SetPrinterClientForTest(printerClient)

	order := db.GetOrderWithDetailsRow{
		ID:         102,
		UserID:     202,
		MerchantID: 302,
		OrderNo:    "20260401113000WXYZ",
		OrderType:  db.OrderTypeTakeout,
		Status:     db.OrderStatusPreparing,
	}
	config := db.OrderDisplayConfig{
		MerchantID:        order.MerchantID,
		EnablePrint:       true,
		PrintTakeout:      true,
		PrintDineIn:       true,
		PrintReservation:  true,
		PrintDispatchMode: "single_full",
		PrintTriggerMode:  "accepted",
	}
	legacyPrinter := db.CloudPrinter{ID: 3, MerchantID: order.MerchantID, PrinterName: "旧设备", PrinterSn: "legacy-sn", PrinterType: "other", PrinterRole: "front", PrintTakeout: true, IsActive: true}
	supportedPrinter := db.CloudPrinter{ID: 4, MerchantID: order.MerchantID, PrinterName: "前台", PrinterSn: "front-sn", PrinterType: "feieyun", PrinterRole: "front", PrintTakeout: true, IsActive: true}

	store.EXPECT().GetOrderWithDetails(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetOrderDisplayConfigByMerchant(gomock.Any(), order.MerchantID).Return(config, nil)
	store.EXPECT().ListActiveCloudPrintersByMerchant(gomock.Any(), order.MerchantID).Return([]db.CloudPrinter{legacyPrinter, supportedPrinter}, nil)
	store.EXPECT().ListOrderItemsWithDishByOrder(gomock.Any(), order.ID).Return([]db.ListOrderItemsWithDishByOrderRow{{Name: "牛肉面", Quantity: 1, Subtotal: 1800}}, nil)
	store.EXPECT().GetUser(gomock.Any(), order.UserID).Return(db.User{ID: order.UserID, FullName: "张三"}, nil)
	store.EXPECT().GetPrintLogByTaskKeyAndPrinter(gomock.Any(), gomock.Any()).Times(1).Return(db.PrintLog{}, db.ErrRecordNotFound)
	store.EXPECT().CreatePrintLog(gomock.Any(), gomock.Any()).Times(1).Return(db.PrintLog{ID: 1, OrderID: order.ID, PrinterID: supportedPrinter.ID, Status: "pending"}, nil)
	store.EXPECT().UpdatePrintLogStatus(gomock.Any(), gomock.Any()).Times(1).Return(db.PrintLog{}, nil)

	payload, err := json.Marshal(PrintOrderPayload{OrderID: order.ID, Trigger: "accepted", TaskKey: "order:102:accepted"})
	require.NoError(t, err)

	err = processor.ProcessTaskPrintOrder(context.Background(), asynq.NewTask(TaskPrintOrder, payload))
	require.NoError(t, err)
	require.Len(t, printerClient.inputs, 1)
	require.Equal(t, supportedPrinter.PrinterSn, printerClient.inputs[0].SN)
}

func TestProcessTaskPrintOrder_RetryPrintLogReplaysOriginalContent(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := NewTestTaskProcessor(store, NewNoopTaskDistributor(), nil, nil)
	printerClient := &printClientRecorder{}
	processor.SetPrinterClientForTest(printerClient)

	originalPrintLog := db.PrintLog{
		ID:           301,
		OrderID:      401,
		PrinterID:    501,
		PrintContent: "<CB>补打测试</CB>",
		Status:       "failed",
	}
	printer := db.CloudPrinter{
		ID:          originalPrintLog.PrinterID,
		MerchantID:  601,
		PrinterName: "前台",
		PrinterSn:   "front-sn",
		PrinterType: "feieyun",
		PrinterRole: "front",
		IsActive:    true,
	}

	store.EXPECT().GetPrintLog(gomock.Any(), originalPrintLog.ID).Times(1).Return(originalPrintLog, nil)
	store.EXPECT().GetCloudPrinter(gomock.Any(), printer.ID).Times(1).Return(printer, nil)
	store.EXPECT().GetPrintLogByTaskKeyAndPrinter(gomock.Any(), gomock.Any()).Times(1).Return(db.PrintLog{}, db.ErrRecordNotFound)
	store.EXPECT().CreatePrintLog(gomock.Any(), gomock.Eq(db.CreatePrintLogParams{
		OrderID:      originalPrintLog.OrderID,
		PrinterID:    printer.ID,
		PrintContent: originalPrintLog.PrintContent,
		Status:       "pending",
		TaskKey:      pgtype.Text{String: "retry:401:301", Valid: true},
	})).Times(1).Return(db.PrintLog{ID: 302, OrderID: originalPrintLog.OrderID, PrinterID: printer.ID, Status: "pending"}, nil)
	store.EXPECT().UpdatePrintLogStatus(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(_ context.Context, arg db.UpdatePrintLogStatusParams) (db.PrintLog, error) {
		require.Equal(t, int64(302), arg.ID)
		require.Equal(t, "success", arg.Status)
		require.True(t, arg.VendorOrderID.Valid)
		return db.PrintLog{}, nil
	})

	payload, err := json.Marshal(PrintOrderPayload{OrderID: originalPrintLog.OrderID, Trigger: "retry", RetryPrintLogID: originalPrintLog.ID, TaskKey: "retry:401:301"})
	require.NoError(t, err)

	err = processor.ProcessTaskPrintOrder(context.Background(), asynq.NewTask(TaskPrintOrder, payload))
	require.NoError(t, err)
	require.Len(t, printerClient.inputs, 1)
	require.Equal(t, printer.PrinterSn, printerClient.inputs[0].SN)
	require.Equal(t, originalPrintLog.PrintContent, printerClient.inputs[0].Content)
}

func TestProcessTaskPrintOrder_SkipsDuplicateTaskKeyReentry(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := NewTestTaskProcessor(store, NewNoopTaskDistributor(), nil, nil)
	printerClient := &printClientRecorder{}
	processor.SetPrinterClientForTest(printerClient)

	order := db.GetOrderWithDetailsRow{
		ID:         103,
		UserID:     203,
		MerchantID: 303,
		OrderNo:    "20260401114000DUPL",
		OrderType:  db.OrderTypeTakeout,
		Status:     db.OrderStatusPreparing,
	}
	config := db.OrderDisplayConfig{
		MerchantID:        order.MerchantID,
		EnablePrint:       true,
		PrintTakeout:      true,
		PrintDineIn:       true,
		PrintReservation:  true,
		PrintDispatchMode: "single_full",
		PrintTriggerMode:  "accepted",
	}
	printer := db.CloudPrinter{ID: 5, MerchantID: order.MerchantID, PrinterName: "前台", PrinterSn: "front-sn", PrinterType: "feieyun", PrinterRole: "front", PrintTakeout: true, IsActive: true}

	store.EXPECT().GetOrderWithDetails(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetOrderDisplayConfigByMerchant(gomock.Any(), order.MerchantID).Return(config, nil)
	store.EXPECT().ListActiveCloudPrintersByMerchant(gomock.Any(), order.MerchantID).Return([]db.CloudPrinter{printer}, nil)
	store.EXPECT().ListOrderItemsWithDishByOrder(gomock.Any(), order.ID).Return([]db.ListOrderItemsWithDishByOrderRow{{Name: "牛肉面", Quantity: 1, Subtotal: 1800}}, nil)
	store.EXPECT().GetUser(gomock.Any(), order.UserID).Return(db.User{ID: order.UserID, FullName: "张三"}, nil)
	store.EXPECT().GetPrintLogByTaskKeyAndPrinter(gomock.Any(), gomock.Any()).Times(1).Return(db.PrintLog{ID: 88, OrderID: order.ID, PrinterID: printer.ID, Status: "success"}, nil)
	store.EXPECT().CreatePrintLog(gomock.Any(), gomock.Any()).Times(0)

	payload, err := json.Marshal(PrintOrderPayload{OrderID: order.ID, Trigger: "accepted", TaskKey: "order:103:accepted"})
	require.NoError(t, err)

	err = processor.ProcessTaskPrintOrder(context.Background(), asynq.NewTask(TaskPrintOrder, payload))
	require.NoError(t, err)
	require.Empty(t, printerClient.inputs)
}

func pgText(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: true}
}
