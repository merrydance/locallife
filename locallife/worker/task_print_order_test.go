package worker

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/merrydance/locallife/cloudprint"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type printClientRecorder struct {
	inputs                []printInputSnapshot
	callbackEnabled       bool
	printOrderID          string
	printErr              error
	queryOrderStateCalls  []string
	queryOrderStateResult bool
	queryOrderStateErr    error
}

type printInputSnapshot struct {
	SN               string
	Content          string
	Copies           int
	ProviderOriginID string
}

func (r *printClientRecorder) AddPrinter(ctx context.Context, input cloudprint.AddPrinterInput) error {
	return nil
}

func (r *printClientRecorder) RemovePrinter(ctx context.Context, input cloudprint.RemovePrinterInput) error {
	return nil
}

func (r *printClientRecorder) Print(ctx context.Context, input cloudprint.PrintInput) (string, error) {
	r.inputs = append(r.inputs, printInputSnapshot{
		SN:               input.SN,
		Content:          input.Content,
		Copies:           input.Copies,
		ProviderOriginID: input.ProviderOriginID,
	})
	if r.printOrderID == "" && r.printErr == nil {
		r.printOrderID = "vendor-order-id"
	}
	return r.printOrderID, r.printErr
}

func (r *printClientRecorder) PrintResultCallbackEnabled() bool {
	return r.callbackEnabled
}

func (r *printClientRecorder) QueryOrderState(ctx context.Context, orderID string) (bool, error) {
	r.queryOrderStateCalls = append(r.queryOrderStateCalls, orderID)
	return r.queryOrderStateResult, r.queryOrderStateErr
}

func (r *printClientRecorder) QueryPrinterStatus(ctx context.Context, sn string) (string, error) {
	return "在线，工作状态正常", nil
}

func (r *printClientRecorder) GetPrinterInfo(ctx context.Context, sn string) (cloudprint.PrinterInfo, error) {
	return cloudprint.PrinterInfo{Model: "FEIE-80"}, nil
}

type printProviderManagerStub struct {
	providers map[string]cloudprint.Client
}

func (m printProviderManagerStub) Provider(providerType string) (cloudprint.Client, bool) {
	provider, ok := m.providers[providerType]
	return provider, ok
}

func (m printProviderManagerStub) Supported(providerType string) bool {
	_, ok := m.Provider(providerType)
	return ok
}

func (m printProviderManagerStub) Configured() bool {
	for _, provider := range m.providers {
		if provider != nil {
			return true
		}
	}
	return false
}

func TestBuildFeieReceipt_PreservesCurrentFullReceiptFormat(t *testing.T) {
	order := db.GetOrderWithDetailsRow{
		ID:                  100,
		UserID:              200,
		MerchantID:          300,
		OrderNo:             "ABC123",
		OrderType:           db.OrderTypeTakeout,
		Subtotal:            2800,
		DiscountAmount:      100,
		VoucherAmount:       50,
		Notes:               pgText("少辣"),
		PickupCode:          pgText("105"),
		CreatedAt:           time.Date(2026, 4, 1, 9, 30, 0, 0, time.Local),
		DeliveryContactName: "张三",
		DeliveryAddress:     "测试路 88 号",
	}
	items := []db.ListOrderItemsWithDishByOrderRow{
		{Name: "牛肉面", Quantity: 2, Subtotal: 2800},
	}

	content := buildFeieReceipt(order, items, db.User{ID: order.UserID, FullName: "张三"}, printSlipFull, nil)

	require.Equal(t,
		"<CB><B>105# 乐客来福</B></CB><BR>"+
			"<C>前台出单</C><BR>"+
			"订单号：ABC123<BR>"+
			"下单时间：2026-04-01 09:30:00<BR>"+
			"类型：外卖<BR>"+
			"--------------------------------<BR>"+
			"牛肉面 x2  28.00<BR>"+
			"--------------------------------<BR>"+
			"菜品小计：28.00<BR>"+
			"优惠：-1.00<BR>"+
			"券抵扣：-0.50<BR>"+
			"备注：少辣<BR>"+
			"顾客：张三<BR>"+
			"地址：测试路 88 号<BR>"+
			"<BR><BC128_A>ABC123</BC128_A><BR>"+
			"<CUT>",
		content,
	)
}

func TestNewPrintProviderOriginID_IsProviderSafe(t *testing.T) {
	seen := make(map[string]struct{})
	for i := 0; i < 20; i++ {
		originID := newPrintProviderOriginID()
		require.Regexp(t, regexp.MustCompile(`^[A-Za-z0-9]{32}$`), originID)
		require.NotContains(t, seen, originID)
		seen[originID] = struct{}{}
	}
}

func TestProcessTaskPrintOrder_SkipsBeforeStoreWhenNoProviderConfigured(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := NewTestTaskProcessor(store, NewNoopTaskDistributor(), nil, nil)
	processor.SetCloudPrinterManagerForTest(printProviderManagerStub{providers: map[string]cloudprint.Client{}})

	payload, err := json.Marshal(PrintOrderPayload{OrderID: 1009, Trigger: "accepted", TaskKey: "order:1009:accepted"})
	require.NoError(t, err)

	err = processor.ProcessTaskPrintOrder(context.Background(), asynq.NewTask(TaskPrintOrder, payload))
	require.NoError(t, err)
}

func TestExecutePrintAttempt_KeepsPendingWhenStatusQueryProviderAccepts(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := NewTestTaskProcessor(store, NewNoopTaskDistributor(), nil, nil)
	shangpengClient := &printClientRecorder{printOrderID: "sp-order-1"}
	processor.SetCloudPrinterManagerForTest(printProviderManagerStub{providers: map[string]cloudprint.Client{
		string(cloudprint.ProviderShangpeng): shangpengClient,
	}})
	printer := db.CloudPrinter{ID: 21, PrinterSn: "sp-sn", PrinterType: string(cloudprint.ProviderShangpeng), IsActive: true}
	var createdProviderOriginID string

	store.EXPECT().CreatePrintLog(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(_ context.Context, arg db.CreatePrintLogParams) (db.PrintLog, error) {
		require.Equal(t, int64(2001), arg.OrderID)
		require.Equal(t, printer.ID, arg.PrinterID)
		require.Equal(t, "pending", arg.Status)
		require.True(t, arg.ProviderOriginID.Valid)
		require.Regexp(t, regexp.MustCompile(`^[A-Za-z0-9]{32}$`), arg.ProviderOriginID.String)
		createdProviderOriginID = arg.ProviderOriginID.String
		return db.PrintLog{ID: 903, OrderID: arg.OrderID, PrinterID: arg.PrinterID, Status: arg.Status, ProviderOriginID: arg.ProviderOriginID}, nil
	})
	store.EXPECT().UpdatePrintLogStatus(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(_ context.Context, arg db.UpdatePrintLogStatusParams) (db.PrintLog, error) {
		require.Equal(t, int64(903), arg.ID)
		require.Equal(t, "pending", arg.Status)
		require.True(t, arg.VendorOrderID.Valid)
		require.Equal(t, "sp-order-1", arg.VendorOrderID.String)
		require.False(t, arg.ErrorMessage.Valid)
		return db.PrintLog{}, nil
	})

	processor.executePrintAttempt(context.Background(), 2001, printer, "商鹏测试\n订单号：SP001\n", "full", "")

	require.Len(t, shangpengClient.inputs, 1)
	require.Equal(t, printer.PrinterSn, shangpengClient.inputs[0].SN)
	require.Equal(t, createdProviderOriginID, shangpengClient.inputs[0].ProviderOriginID)
}

func TestExecutePrintAttempt_KeepsPendingWhenYilianyunAuthorizationPrintIsAccepted(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var receivedAccessToken string
	var receivedMachineCode string
	var receivedOriginID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/print/index", r.URL.Path)
		require.NoError(t, r.ParseForm())
		receivedAccessToken = r.PostForm.Get("access_token")
		receivedMachineCode = r.PostForm.Get("machine_code")
		receivedOriginID = r.PostForm.Get("origin_id")
		_, _ = w.Write([]byte(`{"error":"0","body":{"id":"yl-order-1"}}`))
	}))
	defer server.Close()

	encryptor, err := util.NewAESEncryptor("12345678901234567890123456789012")
	require.NoError(t, err)
	accessTokenCiphertext, err := util.EncryptSensitiveField(encryptor, "access-token-plain")
	require.NoError(t, err)

	store := mockdb.NewMockStore(ctrl)
	processor := NewTestTaskProcessor(store, NewNoopTaskDistributor(), nil, nil)
	processor.config = util.Config{
		YilianyunEnabled:     true,
		YilianyunAPIBaseURL:  server.URL,
		YilianyunAppID:       "client-001",
		YilianyunAppSecret:   "secret-001",
		YilianyunHTTPTimeout: time.Second,
	}
	processor.dataEncryptor = encryptor
	printer := db.CloudPrinter{
		ID:          71,
		MerchantID:  300,
		PrinterSn:   "YL-SN-001",
		PrinterType: string(cloudprint.ProviderYilianyun),
		IsActive:    true,
	}
	var createdProviderOriginID string

	store.EXPECT().CreatePrintLog(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(_ context.Context, arg db.CreatePrintLogParams) (db.PrintLog, error) {
		require.Equal(t, int64(3001), arg.OrderID)
		require.Equal(t, printer.ID, arg.PrinterID)
		require.Equal(t, printLogStatusPending, arg.Status)
		require.True(t, arg.ProviderOriginID.Valid)
		createdProviderOriginID = arg.ProviderOriginID.String
		return db.PrintLog{ID: 917, OrderID: arg.OrderID, PrinterID: arg.PrinterID, Status: arg.Status, ProviderOriginID: arg.ProviderOriginID}, nil
	})
	store.EXPECT().GetActiveCloudPrinterProviderAuthorizationByPrinter(gomock.Any(), db.GetActiveCloudPrinterProviderAuthorizationByPrinterParams{
		AuthorizedCloudPrinterID: pgtype.Int8{Int64: printer.ID, Valid: true},
		ProviderType:             db.CloudPrinterProviderYilianyun,
		MachineCode:              printer.PrinterSn,
	}).Times(1).Return(db.CloudPrinterProviderAuthorization{
		ID:                       801,
		MerchantID:               printer.MerchantID,
		ProviderType:             db.CloudPrinterProviderYilianyun,
		MachineCode:              printer.PrinterSn,
		AuthorizedCloudPrinterID: pgtype.Int8{Int64: printer.ID, Valid: true},
		AccessTokenCiphertext:    accessTokenCiphertext,
		AccessTokenExpiresAt:     time.Now().Add(time.Hour),
		Status:                   db.CloudPrinterAuthorizationStatusActive,
	}, nil)
	store.EXPECT().UpdatePrintLogStatus(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(_ context.Context, arg db.UpdatePrintLogStatusParams) (db.PrintLog, error) {
		require.Equal(t, int64(917), arg.ID)
		require.Equal(t, printLogStatusPending, arg.Status)
		require.True(t, arg.VendorOrderID.Valid)
		require.Equal(t, "yl-order-1", arg.VendorOrderID.String)
		require.False(t, arg.ErrorMessage.Valid)
		return db.PrintLog{}, nil
	})

	processor.executePrintAttempt(context.Background(), 3001, printer, "<CB>易联云测试</CB>", "full", "")

	require.Equal(t, "access-token-plain", receivedAccessToken)
	require.Equal(t, printer.PrinterSn, receivedMachineCode)
	require.Equal(t, createdProviderOriginID, receivedOriginID)
}

func TestExecutePrintAttempt_FailsYilianyunSafelyWhenAuthorizationIsMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := NewTestTaskProcessor(store, NewNoopTaskDistributor(), nil, nil)
	processor.config = util.Config{
		YilianyunEnabled:     true,
		YilianyunAPIBaseURL:  "https://open-api.10ss.net/v2",
		YilianyunAppID:       "client-001",
		YilianyunAppSecret:   "secret-001",
		YilianyunHTTPTimeout: time.Second,
	}
	printer := db.CloudPrinter{
		ID:          72,
		MerchantID:  300,
		PrinterSn:   "YL-SN-002",
		PrinterType: string(cloudprint.ProviderYilianyun),
		IsActive:    true,
	}

	store.EXPECT().CreatePrintLog(gomock.Any(), gomock.Any()).Times(1).Return(db.PrintLog{ID: 918, OrderID: 3002, PrinterID: printer.ID, Status: printLogStatusPending}, nil)
	store.EXPECT().GetActiveCloudPrinterProviderAuthorizationByPrinter(gomock.Any(), db.GetActiveCloudPrinterProviderAuthorizationByPrinterParams{
		AuthorizedCloudPrinterID: pgtype.Int8{Int64: printer.ID, Valid: true},
		ProviderType:             db.CloudPrinterProviderYilianyun,
		MachineCode:              printer.PrinterSn,
	}).Times(1).Return(db.CloudPrinterProviderAuthorization{}, db.ErrRecordNotFound)
	store.EXPECT().UpdatePrintLogStatus(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(_ context.Context, arg db.UpdatePrintLogStatusParams) (db.PrintLog, error) {
		require.Equal(t, int64(918), arg.ID)
		require.Equal(t, printLogStatusFailed, arg.Status)
		require.True(t, arg.ErrorMessage.Valid)
		require.Contains(t, arg.ErrorMessage.String, "yilianyun")
		require.Contains(t, arg.ErrorMessage.String, "authorization")
		require.NotContains(t, arg.ErrorMessage.String, "access_token")
		require.NotContains(t, arg.ErrorMessage.String, "refresh_token")
		return db.PrintLog{}, nil
	})

	processor.executePrintAttempt(context.Background(), 3002, printer, "<CB>易联云测试</CB>", "full", "")
}

func TestExecutePrintAttempt_KeepsPendingWhenPrintResultCallbackEnabled(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := NewTestTaskProcessor(store, NewNoopTaskDistributor(), nil, nil)
	printerClient := &printClientRecorder{callbackEnabled: true, printOrderID: "vendor-callback-1"}
	processor.SetPrinterClientForTest(printerClient)
	printer := db.CloudPrinter{ID: 11, PrinterSn: "front-sn", PrinterType: "feieyun", IsActive: true}

	store.EXPECT().CreatePrintLog(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(_ context.Context, arg db.CreatePrintLogParams) (db.PrintLog, error) {
		require.Equal(t, int64(1001), arg.OrderID)
		require.Equal(t, printer.ID, arg.PrinterID)
		require.Equal(t, "pending", arg.Status)
		return db.PrintLog{ID: 901, OrderID: arg.OrderID, PrinterID: arg.PrinterID, Status: arg.Status}, nil
	})
	store.EXPECT().UpdatePrintLogStatus(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(_ context.Context, arg db.UpdatePrintLogStatusParams) (db.PrintLog, error) {
		require.Equal(t, int64(901), arg.ID)
		require.Equal(t, "pending", arg.Status)
		require.True(t, arg.VendorOrderID.Valid)
		require.Equal(t, "vendor-callback-1", arg.VendorOrderID.String)
		require.False(t, arg.ErrorMessage.Valid)
		return db.PrintLog{}, nil
	})

	processor.executePrintAttempt(context.Background(), 1001, printer, "<CB>测试</CB>", "full", "")

	require.Len(t, printerClient.inputs, 1)
}

func TestExecutePrintAttempt_SanitizesFailedProviderErrorBeforePersistence(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := NewTestTaskProcessor(store, NewNoopTaskDistributor(), nil, nil)
	printerClient := &printClientRecorder{printErr: errors.New(`provider failed access_token=token-secret {"appsecret":"secret-001"}`)}
	processor.SetPrinterClientForTest(printerClient)
	printer := db.CloudPrinter{ID: 13, PrinterSn: "front-sn", PrinterType: "feieyun", IsActive: true}

	store.EXPECT().CreatePrintLog(gomock.Any(), gomock.Any()).Times(1).Return(db.PrintLog{ID: 904, OrderID: 1003, PrinterID: printer.ID, Status: printLogStatusPending}, nil)
	store.EXPECT().UpdatePrintLogStatus(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(_ context.Context, arg db.UpdatePrintLogStatusParams) (db.PrintLog, error) {
		require.Equal(t, int64(904), arg.ID)
		require.Equal(t, printLogStatusFailed, arg.Status)
		require.True(t, arg.ErrorMessage.Valid)
		require.NotContains(t, arg.ErrorMessage.String, "token-secret")
		require.NotContains(t, arg.ErrorMessage.String, "secret-001")
		require.Contains(t, arg.ErrorMessage.String, "[redacted]")
		return db.PrintLog{}, nil
	})

	processor.executePrintAttempt(context.Background(), 1003, printer, "<CB>测试</CB>", "full", "")
}

func TestExecutePrintAttempt_FailsWhenCallbackEnabledWithoutVendorOrderID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := NewTestTaskProcessor(store, NewNoopTaskDistributor(), nil, nil)
	printerClient := &printClientRecorder{callbackEnabled: true, printOrderID: " "}
	processor.SetPrinterClientForTest(printerClient)
	printer := db.CloudPrinter{ID: 12, PrinterSn: "front-sn", PrinterType: "feieyun", IsActive: true}

	store.EXPECT().CreatePrintLog(gomock.Any(), gomock.Any()).Times(1).Return(db.PrintLog{ID: 902, OrderID: 1002, PrinterID: printer.ID, Status: "pending"}, nil)
	store.EXPECT().UpdatePrintLogStatus(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(_ context.Context, arg db.UpdatePrintLogStatusParams) (db.PrintLog, error) {
		require.Equal(t, int64(902), arg.ID)
		require.Equal(t, "failed", arg.Status)
		require.True(t, arg.ErrorMessage.Valid)
		require.Contains(t, arg.ErrorMessage.String, "vendor order id")
		require.False(t, arg.VendorOrderID.Valid)
		return db.PrintLog{}, nil
	})

	processor.executePrintAttempt(context.Background(), 1002, printer, "<CB>测试</CB>", "full", "")

	require.Len(t, printerClient.inputs, 1)
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
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
			BusinessType: db.ExternalPaymentBusinessOwnerOrder,
		}).
		Return(db.PaymentOrder{}, db.ErrRecordNotFound)
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

func TestProcessTaskPrintOrder_FullReceiptIncludesBaofuProfitSharingBill(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := NewTestTaskProcessor(store, NewNoopTaskDistributor(), nil, nil)
	printerClient := &printClientRecorder{}
	processor.SetPrinterClientForTest(printerClient)

	order := db.GetOrderWithDetailsRow{
		ID:                  104,
		UserID:              204,
		MerchantID:          304,
		OrderNo:             "20260401115000BILL",
		OrderType:           db.OrderTypeTakeout,
		Status:              db.OrderStatusPreparing,
		Subtotal:            10000,
		DiscountAmount:      300,
		VoucherAmount:       200,
		DeliveryFee:         800,
		TotalAmount:         10300,
		CreatedAt:           time.Date(2026, 4, 1, 11, 50, 0, 0, time.Local),
		DeliveryContactName: "张三",
		DeliveryAddress:     "测试路 88 号",
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
	printer := db.CloudPrinter{ID: 6, MerchantID: order.MerchantID, PrinterName: "前台", PrinterSn: "front-sn", PrinterType: "feieyun", PrinterRole: "front", PrintTakeout: true, IsActive: true}
	paymentOrder := db.PaymentOrder{
		ID:                    704,
		OrderID:               pgtype.Int8{Int64: order.ID, Valid: true},
		BusinessType:          db.ExternalPaymentBusinessOwnerOrder,
		Amount:                order.TotalAmount,
		Status:                "paid",
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
	}
	profitSharingOrder := db.ProfitSharingOrder{
		ID:                 804,
		PaymentOrderID:     paymentOrder.ID,
		MerchantID:         order.MerchantID,
		OrderSource:        order.OrderType,
		TotalAmount:        order.TotalAmount,
		PlatformCommission: 190,
		OperatorCommission: 285,
		PaymentFee:         31,
		Provider:           db.ExternalPaymentProviderBaofu,
		Channel:            db.PaymentChannelBaofuAggregate,
		MerchantPaymentFee: 57,
		MerchantAmount:     8968,
		CalculationVersion: "baofu_fee_v2",
		RiderGrossAmount:   800,
		RiderPaymentFee:    5,
		RiderAmount:        795,
	}

	store.EXPECT().GetOrderWithDetails(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetOrderDisplayConfigByMerchant(gomock.Any(), order.MerchantID).Return(config, nil)
	store.EXPECT().ListActiveCloudPrintersByMerchant(gomock.Any(), order.MerchantID).Return([]db.CloudPrinter{printer}, nil)
	store.EXPECT().ListOrderItemsWithDishByOrder(gomock.Any(), order.ID).Return([]db.ListOrderItemsWithDishByOrderRow{{Name: "牛肉面", Quantity: 2, Subtotal: 5600}}, nil)
	store.EXPECT().GetUser(gomock.Any(), order.UserID).Return(db.User{ID: order.UserID, FullName: "张三"}, nil)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
			BusinessType: db.ExternalPaymentBusinessOwnerOrder,
		}).
		Return(paymentOrder, nil)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(profitSharingOrder, nil)
	store.EXPECT().GetPrintLogByTaskKeyAndPrinter(gomock.Any(), gomock.Any()).Times(1).Return(db.PrintLog{}, db.ErrRecordNotFound)
	store.EXPECT().CreatePrintLog(gomock.Any(), gomock.Any()).Times(1).Return(db.PrintLog{ID: 1, OrderID: order.ID, PrinterID: printer.ID, Status: "pending"}, nil)
	store.EXPECT().UpdatePrintLogStatus(gomock.Any(), gomock.Any()).Times(1).Return(db.PrintLog{}, nil)

	payload, err := json.Marshal(PrintOrderPayload{OrderID: order.ID, Trigger: "accepted", TaskKey: "order:104:accepted"})
	require.NoError(t, err)

	err = processor.ProcessTaskPrintOrder(context.Background(), asynq.NewTask(TaskPrintOrder, payload))
	require.NoError(t, err)
	require.Len(t, printerClient.inputs, 1)
	content := printerClient.inputs[0].Content
	require.Contains(t, content, "菜品小计：100.00")
	require.NotContains(t, content, "跑腿费")
	require.NotContains(t, content, "<BOLD>实付：")
	require.Contains(t, content, "用户实付：103.00")
	require.Contains(t, content, "商户账单")
	require.Contains(t, content, "菜品合计：95.00")
	require.Contains(t, content, "- 平台服务费：-4.75")
	require.Contains(t, content, "- 支付通道费：-0.57")
	require.Contains(t, content, "商户实收：89.68")
	require.Contains(t, content, "骑手账单")
	require.Contains(t, content, "代取费：8.00")
	require.Contains(t, content, "- 支付通道费：-0.05")
	require.NotContains(t, content, "骑手通道费")
	require.Contains(t, content, "骑手实收：7.95")
}

func TestProcessTaskPrintOrder_BlocksBaofuProfitSharingReceiptWhenBillMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := NewTestTaskProcessor(store, NewNoopTaskDistributor(), nil, nil)
	printerClient := &printClientRecorder{}
	processor.SetPrinterClientForTest(printerClient)

	order := db.GetOrderWithDetailsRow{
		ID:          105,
		UserID:      205,
		MerchantID:  305,
		OrderNo:     "20260401115500MISS",
		OrderType:   db.OrderTypeTakeout,
		Status:      db.OrderStatusPreparing,
		Subtotal:    10000,
		DeliveryFee: 800,
		TotalAmount: 10800,
		CreatedAt:   time.Date(2026, 4, 1, 11, 55, 0, 0, time.Local),
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
	printer := db.CloudPrinter{ID: 7, MerchantID: order.MerchantID, PrinterName: "前台", PrinterSn: "front-sn", PrinterType: "feieyun", PrinterRole: "front", PrintTakeout: true, IsActive: true}
	paymentOrder := db.PaymentOrder{
		ID:                    705,
		OrderID:               pgtype.Int8{Int64: order.ID, Valid: true},
		BusinessType:          db.ExternalPaymentBusinessOwnerOrder,
		Amount:                order.TotalAmount,
		Status:                "paid",
		PaymentChannel:        db.PaymentChannelBaofuAggregate,
		RequiresProfitSharing: true,
	}

	store.EXPECT().GetOrderWithDetails(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetOrderDisplayConfigByMerchant(gomock.Any(), order.MerchantID).Return(config, nil)
	store.EXPECT().ListActiveCloudPrintersByMerchant(gomock.Any(), order.MerchantID).Return([]db.CloudPrinter{printer}, nil)
	store.EXPECT().ListOrderItemsWithDishByOrder(gomock.Any(), order.ID).Return([]db.ListOrderItemsWithDishByOrderRow{{Name: "牛肉面", Quantity: 1, Subtotal: 10000}}, nil)
	store.EXPECT().GetUser(gomock.Any(), order.UserID).Return(db.User{ID: order.UserID, FullName: "张三"}, nil)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
			BusinessType: db.ExternalPaymentBusinessOwnerOrder,
		}).
		Return(paymentOrder, nil)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(db.ProfitSharingOrder{}, db.ErrRecordNotFound)
	store.EXPECT().CreatePrintLog(gomock.Any(), gomock.Any()).Times(0)

	payload, err := json.Marshal(PrintOrderPayload{OrderID: order.ID, Trigger: "accepted", TaskKey: "order:105:accepted"})
	require.NoError(t, err)

	err = processor.ProcessTaskPrintOrder(context.Background(), asynq.NewTask(TaskPrintOrder, payload))
	require.Error(t, err)
	require.True(t, errors.Is(err, db.ErrRecordNotFound))
	require.Empty(t, printerClient.inputs)
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
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
			BusinessType: db.ExternalPaymentBusinessOwnerOrder,
		}).
		Return(db.PaymentOrder{}, db.ErrRecordNotFound)
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
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
			BusinessType: db.ExternalPaymentBusinessOwnerOrder,
		}).
		Return(db.PaymentOrder{}, db.ErrRecordNotFound)
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

func TestProcessTaskPrintOrder_PrintsShangpengReceiptWithoutFeieTagsAndKeepsPending(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := NewTestTaskProcessor(store, NewNoopTaskDistributor(), nil, nil)
	shangpengClient := &printClientRecorder{printOrderID: "sp-vendor-100"}
	processor.SetCloudPrinterManagerForTest(printProviderManagerStub{providers: map[string]cloudprint.Client{
		string(cloudprint.ProviderShangpeng): shangpengClient,
	}})

	order := db.GetOrderWithDetailsRow{
		ID:                  106,
		UserID:              206,
		MerchantID:          306,
		OrderNo:             "20260401123000SP",
		OrderType:           db.OrderTypeTakeout,
		Status:              db.OrderStatusPreparing,
		Subtotal:            3600,
		DiscountAmount:      100,
		TotalAmount:         3500,
		Notes:               pgText("不要香菜"),
		PickupCode:          pgText("208"),
		CreatedAt:           time.Date(2026, 4, 1, 12, 30, 0, 0, time.Local),
		DeliveryContactName: "李四",
		DeliveryAddress:     "测试路 99 号",
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
	printer := db.CloudPrinter{
		ID:           8,
		MerchantID:   order.MerchantID,
		PrinterName:  "商鹏前台",
		PrinterSn:    "sp-front-sn",
		PrinterType:  string(cloudprint.ProviderShangpeng),
		PrinterRole:  "front",
		PrintTakeout: true,
		IsActive:     true,
	}

	store.EXPECT().GetOrderWithDetails(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetOrderDisplayConfigByMerchant(gomock.Any(), order.MerchantID).Return(config, nil)
	store.EXPECT().ListActiveCloudPrintersByMerchant(gomock.Any(), order.MerchantID).Return([]db.CloudPrinter{printer}, nil)
	store.EXPECT().ListOrderItemsWithDishByOrder(gomock.Any(), order.ID).Return([]db.ListOrderItemsWithDishByOrderRow{{Name: "鸡肉饭", Quantity: 1, Subtotal: 3600}}, nil)
	store.EXPECT().GetUser(gomock.Any(), order.UserID).Return(db.User{ID: order.UserID, FullName: "李四"}, nil)
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
			BusinessType: db.ExternalPaymentBusinessOwnerOrder,
		}).
		Return(db.PaymentOrder{}, db.ErrRecordNotFound)
	store.EXPECT().GetPrintLogByTaskKeyAndPrinter(gomock.Any(), gomock.Any()).Times(1).Return(db.PrintLog{}, db.ErrRecordNotFound)
	store.EXPECT().CreatePrintLog(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(_ context.Context, arg db.CreatePrintLogParams) (db.PrintLog, error) {
		require.Equal(t, printer.ID, arg.PrinterID)
		require.Equal(t, "pending", arg.Status)
		require.True(t, arg.ProviderOriginID.Valid)
		require.NotContains(t, arg.PrintContent, "<CB>")
		require.NotContains(t, arg.PrintContent, "<BR>")
		require.NotContains(t, arg.PrintContent, "<CUT>")
		require.NotContains(t, arg.PrintContent, "<QR>")
		require.NotContains(t, arg.PrintContent, "<BC128_A>")
		return db.PrintLog{
			ID:               904,
			OrderID:          arg.OrderID,
			PrinterID:        arg.PrinterID,
			PrintContent:     arg.PrintContent,
			Status:           arg.Status,
			ProviderOriginID: arg.ProviderOriginID,
		}, nil
	})
	store.EXPECT().UpdatePrintLogStatus(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(_ context.Context, arg db.UpdatePrintLogStatusParams) (db.PrintLog, error) {
		require.Equal(t, int64(904), arg.ID)
		require.Equal(t, "pending", arg.Status)
		require.True(t, arg.VendorOrderID.Valid)
		require.Equal(t, "sp-vendor-100", arg.VendorOrderID.String)
		return db.PrintLog{}, nil
	})

	payload, err := json.Marshal(PrintOrderPayload{OrderID: order.ID, Trigger: "accepted", TaskKey: "order:106:accepted"})
	require.NoError(t, err)

	err = processor.ProcessTaskPrintOrder(context.Background(), asynq.NewTask(TaskPrintOrder, payload))
	require.NoError(t, err)
	require.Len(t, shangpengClient.inputs, 1)
	require.Equal(t, printer.PrinterSn, shangpengClient.inputs[0].SN)

	content := shangpengClient.inputs[0].Content
	require.Contains(t, content, "208# 乐客来福")
	require.Contains(t, content, "前台出单")
	require.Contains(t, content, "订单号：20260401123000SP")
	require.Contains(t, content, "鸡肉饭 x1  36.00")
	require.Contains(t, content, "优惠：-1.00")
	require.Contains(t, content, "备注：不要香菜")
	require.Contains(t, content, "顾客：李四")
	require.Contains(t, content, "地址：测试路 99 号")
	for _, unsupported := range []string{"<CB>", "<B>", "<BR>", "<CUT>", "<QR>", "<BC128_A>", "<BOLD>"} {
		require.NotContains(t, content, unsupported)
	}
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
	store.EXPECT().CreatePrintLog(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(_ context.Context, arg db.CreatePrintLogParams) (db.PrintLog, error) {
		require.Equal(t, originalPrintLog.OrderID, arg.OrderID)
		require.Equal(t, printer.ID, arg.PrinterID)
		require.Equal(t, originalPrintLog.PrintContent, arg.PrintContent)
		require.Equal(t, "pending", arg.Status)
		require.Equal(t, pgtype.Text{String: "retry:401:301", Valid: true}, arg.TaskKey)
		require.True(t, arg.ProviderOriginID.Valid)
		require.Regexp(t, regexp.MustCompile(`^[A-Za-z0-9]{32}$`), arg.ProviderOriginID.String)
		return db.PrintLog{ID: 302, OrderID: originalPrintLog.OrderID, PrinterID: printer.ID, Status: "pending", ProviderOriginID: arg.ProviderOriginID}, nil
	})
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
	store.EXPECT().
		GetLatestPaymentOrderByOrder(gomock.Any(), db.GetLatestPaymentOrderByOrderParams{
			OrderID:      pgtype.Int8{Int64: order.ID, Valid: true},
			BusinessType: db.ExternalPaymentBusinessOwnerOrder,
		}).
		Return(db.PaymentOrder{}, db.ErrRecordNotFound)
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
