package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/wechat"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/merrydance/locallife/worker"
	mockwk "github.com/merrydance/locallife/worker/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ==================== ProcessTaskApplymentResult Tests ====================

func TestProcessTaskApplymentResult_Success(t *testing.T) {
	testCases := []struct {
		name        string
		payload     worker.ApplymentResultPayload
		buildStubs  func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface, distributor *mockwk.MockTaskDistributor)
		checkResult func(t *testing.T, err error)
	}{
		{
			name: "商户进件成功_添加分账接收方并发送通知",
			payload: worker.ApplymentResultPayload{
				ApplymentID:    1,
				OutRequestNo:   "APPLY_M_1_1234567890",
				ApplymentState: "APPLYMENT_STATE_FINISHED",
				SubMchID:       "1234567890",
				SubjectType:    "merchant",
				SubjectID:      100,
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface, distributor *mockwk.MockTaskDistributor) {
				// 1. 添加分账接收方
				ecommerceClient.EXPECT().
					GetSpAppID().
					Return("wx1234567890")
				ecommerceClient.EXPECT().
					AddProfitSharingReceiver(gomock.Any(), gomock.Any()).
					Return(&wechat.AddReceiverResponse{}, nil)

				// 2. 获取商户信息
				store.EXPECT().
					GetMerchant(gomock.Any(), int64(100)).
					Return(db.Merchant{
						ID:          100,
						OwnerUserID: 1001,
						Name:        "测试商户",
					}, nil)

				// 3. 发送通知
				distributor.EXPECT().
					DistributeTaskSendNotification(gomock.Any(), gomock.Any()).
					Return(nil)
			},
			checkResult: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "不支持的骑手主体被直接忽略",
			payload: worker.ApplymentResultPayload{
				ApplymentID:    2,
				OutRequestNo:   "APPLY_R_2_1234567890",
				ApplymentState: "APPLYMENT_STATE_FINISHED",
				SubMchID:       "9876543210",
				SubjectType:    "rider",
				SubjectID:      200,
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface, distributor *mockwk.MockTaskDistributor) {
			},
			checkResult: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "进件成功_添加接收方失败但不影响流程",
			payload: worker.ApplymentResultPayload{
				ApplymentID:    3,
				OutRequestNo:   "APPLY_M_3_1234567890",
				ApplymentState: "APPLYMENT_STATE_FINISHED",
				SubMchID:       "1234567890",
				SubjectType:    "merchant",
				SubjectID:      100,
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface, distributor *mockwk.MockTaskDistributor) {
				// 1. 添加分账接收方失败
				ecommerceClient.EXPECT().
					GetSpAppID().
					Return("wx1234567890")
				ecommerceClient.EXPECT().
					AddProfitSharingReceiver(gomock.Any(), gomock.Any()).
					Return(nil, errors.New("wechat api error"))

				// 2. 仍然获取商户信息并发送通知
				store.EXPECT().
					GetMerchant(gomock.Any(), int64(100)).
					Return(db.Merchant{
						ID:          100,
						OwnerUserID: 1001,
						Name:        "测试商户",
					}, nil)

				// 3. 发送通知
				distributor.EXPECT().
					DistributeTaskSendNotification(gomock.Any(), gomock.Any()).
					Return(nil)
			},
			checkResult: func(t *testing.T, err error) {
				require.NoError(t, err) // 添加接收方失败不影响流程
			},
		},
		{
			name: "进件成功_商户不存在",
			payload: worker.ApplymentResultPayload{
				ApplymentID:    4,
				OutRequestNo:   "APPLY_M_4_1234567890",
				ApplymentState: "APPLYMENT_STATE_FINISHED",
				SubMchID:       "1234567890",
				SubjectType:    "merchant",
				SubjectID:      999,
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface, distributor *mockwk.MockTaskDistributor) {
				// 1. 添加分账接收方
				ecommerceClient.EXPECT().
					GetSpAppID().
					Return("wx1234567890")
				ecommerceClient.EXPECT().
					AddProfitSharingReceiver(gomock.Any(), gomock.Any()).
					Return(&wechat.AddReceiverResponse{}, nil)

				// 2. 商户不存在
				store.EXPECT().
					GetMerchant(gomock.Any(), int64(999)).
					Return(db.Merchant{}, db.ErrRecordNotFound)

				// 不发送通知
			},
			checkResult: func(t *testing.T, err error) {
				require.NoError(t, err) // 商户不存在不影响流程，只是不发通知
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)
			distributor := mockwk.NewMockTaskDistributor(ctrl)

			tc.buildStubs(store, ecommerceClient, distributor)

			processor := worker.NewTestTaskProcessor(store, distributor, nil, ecommerceClient)

			payload, err := json.Marshal(tc.payload)
			require.NoError(t, err)

			task := asynq.NewTask(worker.TaskProcessApplymentResult, payload)
			err = processor.ProcessTaskApplymentResult(context.Background(), task)
			tc.checkResult(t, err)
		})
	}
}

func TestProcessTaskApplymentResult_Rejected(t *testing.T) {
	testCases := []struct {
		name        string
		payload     worker.ApplymentResultPayload
		buildStubs  func(store *mockdb.MockStore, distributor *mockwk.MockTaskDistributor)
		checkResult func(t *testing.T, err error)
	}{
		{
			name: "商户进件被驳回_发送通知",
			payload: worker.ApplymentResultPayload{
				ApplymentID:    5,
				OutRequestNo:   "APPLY_M_5_1234567890",
				ApplymentState: "APPLYMENT_STATE_REJECTED",
				SubjectType:    "merchant",
				SubjectID:      100,
			},
			buildStubs: func(store *mockdb.MockStore, distributor *mockwk.MockTaskDistributor) {
				// 1. 获取进件记录（含驳回原因）
				store.EXPECT().
					GetEcommerceApplyment(gomock.Any(), int64(5)).
					Return(db.EcommerceApplyment{
						ID:           5,
						SubjectType:  "merchant",
						SubjectID:    100,
						RejectReason: pgtype.Text{String: "资料不完整", Valid: true},
					}, nil)

				// 2. 获取商户信息
				store.EXPECT().
					GetMerchant(gomock.Any(), int64(100)).
					Return(db.Merchant{
						ID:          100,
						OwnerUserID: 1001,
						Name:        "测试商户",
					}, nil)

				// 3. 发送通知
				distributor.EXPECT().
					DistributeTaskSendNotification(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, payload *worker.SendNotificationPayload, opts ...asynq.Option) error {
						require.Equal(t, int64(1001), payload.UserID)
						require.Equal(t, "微信支付开户被驳回", payload.Title)
						require.Contains(t, payload.Content, "资料不完整")
						return nil
					})
			},
			checkResult: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "骑手主体驳回结果被忽略",
			payload: worker.ApplymentResultPayload{
				ApplymentID:    6,
				OutRequestNo:   "APPLY_R_6_1234567890",
				ApplymentState: "APPLYMENT_STATE_REJECTED",
				SubjectType:    "rider",
				SubjectID:      200,
			},
			buildStubs: func(store *mockdb.MockStore, distributor *mockwk.MockTaskDistributor) {
			},
			checkResult: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "进件被驳回_无驳回原因",
			payload: worker.ApplymentResultPayload{
				ApplymentID:    7,
				OutRequestNo:   "APPLY_M_7_1234567890",
				ApplymentState: "APPLYMENT_STATE_REJECTED",
				SubjectType:    "merchant",
				SubjectID:      100,
			},
			buildStubs: func(store *mockdb.MockStore, distributor *mockwk.MockTaskDistributor) {
				// 1. 获取进件记录（无驳回原因）
				store.EXPECT().
					GetEcommerceApplyment(gomock.Any(), int64(7)).
					Return(db.EcommerceApplyment{
						ID:          7,
						SubjectType: "merchant",
						SubjectID:   100,
					}, nil)

				// 2. 获取商户信息
				store.EXPECT().
					GetMerchant(gomock.Any(), int64(100)).
					Return(db.Merchant{
						ID:          100,
						OwnerUserID: 1001,
						Name:        "测试商户",
					}, nil)

				// 3. 发送通知（使用默认原因）
				distributor.EXPECT().
					DistributeTaskSendNotification(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, payload *worker.SendNotificationPayload, opts ...asynq.Option) error {
						require.Contains(t, payload.Content, "请登录后台查看详情")
						return nil
					})
			},
			checkResult: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			distributor := mockwk.NewMockTaskDistributor(ctrl)

			tc.buildStubs(store, distributor)

			processor := worker.NewTestTaskProcessor(store, distributor, nil, nil)

			payload, err := json.Marshal(tc.payload)
			require.NoError(t, err)

			task := asynq.NewTask(worker.TaskProcessApplymentResult, payload)
			err = processor.ProcessTaskApplymentResult(context.Background(), task)
			tc.checkResult(t, err)
		})
	}
}

func TestProcessTaskApplymentResult_Pending(t *testing.T) {
	testCases := []struct {
		name        string
		payload     worker.ApplymentResultPayload
		buildStubs  func(store *mockdb.MockStore, distributor *mockwk.MockTaskDistributor)
		checkResult func(t *testing.T, err error)
	}{
		{
			name: "商户待确认_发送提醒通知",
			payload: worker.ApplymentResultPayload{
				ApplymentID:    8,
				OutRequestNo:   "APPLY_M_8_1234567890",
				ApplymentState: "APPLYMENT_STATE_TO_BE_CONFIRMED",
				SubjectType:    "merchant",
				SubjectID:      100,
			},
			buildStubs: func(store *mockdb.MockStore, distributor *mockwk.MockTaskDistributor) {
				// 获取商户信息
				store.EXPECT().
					GetMerchant(gomock.Any(), int64(100)).
					Return(db.Merchant{
						ID:          100,
						OwnerUserID: 1001,
						Name:        "测试商户",
					}, nil)

				// 发送通知
				distributor.EXPECT().
					DistributeTaskSendNotification(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, payload *worker.SendNotificationPayload, opts ...asynq.Option) error {
						require.Equal(t, "微信支付开户待处理", payload.Title)
						require.Contains(t, payload.Content, "需要确认")
						return nil
					})
			},
			checkResult: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "骑手待签约结果被忽略",
			payload: worker.ApplymentResultPayload{
				ApplymentID:    9,
				OutRequestNo:   "APPLY_R_9_1234567890",
				ApplymentState: "APPLYMENT_STATE_TO_BE_SIGNED",
				SubjectType:    "rider",
				SubjectID:      200,
			},
			buildStubs: func(store *mockdb.MockStore, distributor *mockwk.MockTaskDistributor) {
			},
			checkResult: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			distributor := mockwk.NewMockTaskDistributor(ctrl)

			tc.buildStubs(store, distributor)

			processor := worker.NewTestTaskProcessor(store, distributor, nil, nil)

			payload, err := json.Marshal(tc.payload)
			require.NoError(t, err)

			task := asynq.NewTask(worker.TaskProcessApplymentResult, payload)
			err = processor.ProcessTaskApplymentResult(context.Background(), task)
			tc.checkResult(t, err)
		})
	}
}

func TestProcessTaskApplymentResult_OtherStates(t *testing.T) {
	testCases := []struct {
		name        string
		payload     worker.ApplymentResultPayload
		checkResult func(t *testing.T, err error)
	}{
		{
			name: "审核中状态_不处理",
			payload: worker.ApplymentResultPayload{
				ApplymentID:    10,
				ApplymentState: "APPLYMENT_STATE_AUDITING",
				SubjectType:    "merchant",
				SubjectID:      100,
			},
			checkResult: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "冻结状态_不处理",
			payload: worker.ApplymentResultPayload{
				ApplymentID:    11,
				ApplymentState: "APPLYMENT_STATE_FROZEN",
				SubjectType:    "merchant",
				SubjectID:      100,
			},
			checkResult: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// 不需要任何 mock，因为这些状态不需要处理
			processor := worker.NewTestTaskProcessor(nil, nil, nil, nil)

			payload, err := json.Marshal(tc.payload)
			require.NoError(t, err)

			task := asynq.NewTask(worker.TaskProcessApplymentResult, payload)
			err = processor.ProcessTaskApplymentResult(context.Background(), task)
			tc.checkResult(t, err)
		})
	}
}

func TestProcessTaskApplymentResult_InvalidPayload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	processor := worker.NewTestTaskProcessor(nil, nil, nil, nil)

	// 无效的 JSON payload
	task := asynq.NewTask(worker.TaskProcessApplymentResult, []byte("invalid json"))
	err := processor.ProcessTaskApplymentResult(context.Background(), task)
	require.Error(t, err)
	require.ErrorIs(t, err, asynq.SkipRetry)
}

func TestProcessTaskProfitSharing_UsesMerchantRegionActiveOperator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{
		ID:            901,
		Amount:        10000,
		TransactionID: pgtype.Text{String: "wx_txn_901", Valid: true},
	}
	order := db.Order{
		ID:          77,
		MerchantID:  15,
		TotalAmount: 10000,
		DeliveryFee: 0,
		OrderType:   "takeout",
		AddressID:   pgtype.Int8{Int64: 999, Valid: true},
	}
	merchant := db.Merchant{ID: 15, RegionID: 12, Name: "商户A"}
	operator := db.Operator{
		ID:          44,
		WechatMchID: pgtype.Text{String: "op_mch_44", Valid: true},
	}

	store.EXPECT().
		GetPaymentOrder(gomock.Any(), paymentOrder.ID).
		Return(paymentOrder, nil)
	store.EXPECT().
		GetOrder(gomock.Any(), order.ID).
		Return(order, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchant.ID).
		Return(merchant, nil)
	store.EXPECT().
		GetMerchantPaymentConfig(gomock.Any(), merchant.ID).
		Return(db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "sub_mch_15"}, nil)
	store.EXPECT().
		GetActiveProfitSharingConfig(gomock.Any(), db.GetActiveProfitSharingConfigParams{
			OrderSource: order.OrderType,
			MerchantID:  pgtype.Int8{Int64: merchant.ID, Valid: true},
			RegionID:    pgtype.Int8{Int64: merchant.RegionID, Valid: true},
		}).
		Return(db.ProfitSharingConfig{PlatformRate: 0, OperatorRate: 20, RiderEnabled: false}, nil)
	store.EXPECT().
		GetActiveOperatorByRegion(gomock.Any(), merchant.RegionID).
		Return(operator, nil)
	store.EXPECT().
		GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).
		Return(db.ProfitSharingOrder{}, db.ErrRecordNotFound)
	store.EXPECT().
		CreateProfitSharingOrder(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateProfitSharingOrderParams) (db.ProfitSharingOrder, error) {
			require.Equal(t, paymentOrder.ID, arg.PaymentOrderID)
			require.Equal(t, merchant.ID, arg.MerchantID)
			require.True(t, arg.OperatorID.Valid)
			require.Equal(t, operator.ID, arg.OperatorID.Int64)
			require.Equal(t, int64(2000), arg.OperatorCommission)
			require.Equal(t, int64(8000), arg.MerchantAmount)
			return db.ProfitSharingOrder{ID: 3001, OutOrderNo: arg.OutOrderNo}, nil
		})
	ecommerceClient.EXPECT().
		GetSpAppID().
		Return("wx_sp_app_1")
	ecommerceClient.EXPECT().
		AddProfitSharingReceiver(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *wechat.AddReceiverRequest) (*wechat.AddReceiverResponse, error) {
			require.Equal(t, wechat.ReceiverTypeMerchant, req.Type)
			require.Equal(t, "op_mch_44", req.Account)
			return &wechat.AddReceiverResponse{}, nil
		})
	ecommerceClient.EXPECT().
		CreateProfitSharing(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *wechat.ProfitSharingRequest) (*wechat.ProfitSharingResponse, error) {
			require.Equal(t, "sub_mch_15", req.SubMchID)
			require.Equal(t, "wx_txn_901", req.TransactionID)
			require.Len(t, req.Receivers, 1)
			require.Equal(t, "op_mch_44", req.Receivers[0].ReceiverAccount)
			require.Equal(t, int64(2000), req.Receivers[0].Amount)
			return &wechat.ProfitSharingResponse{OrderID: "ps_wx_3001", Status: "PROCESSING"}, nil
		})
	store.EXPECT().
		UpdateProfitSharingOrderToProcessing(gomock.Any(), db.UpdateProfitSharingOrderToProcessingParams{
			ID:             3001,
			SharingOrderID: pgtype.Text{String: "ps_wx_3001", Valid: true},
		}).
		Return(db.ProfitSharingOrder{ID: 3001, Status: "processing"}, nil)

	processor := worker.NewTestTaskProcessor(store, nil, nil, ecommerceClient)
	payloadBytes, err := json.Marshal(worker.ProfitSharingPayload{
		PaymentOrderID: paymentOrder.ID,
		OrderID:        order.ID,
	})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessProfitSharing, payloadBytes)
	err = processor.ProcessTaskProfitSharing(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskProfitSharing_UsesPaymentOrderAmountAsProfitSharingBase(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{
		ID:            902,
		Amount:        7000,
		TransactionID: pgtype.Text{String: "wx_txn_902", Valid: true},
	}
	order := db.Order{
		ID:          78,
		MerchantID:  16,
		TotalAmount: 10000,
		DeliveryFee: 0,
		OrderType:   "takeout",
		AddressID:   pgtype.Int8{Int64: 1000, Valid: true},
	}
	merchant := db.Merchant{ID: 16, RegionID: 13, Name: "商户B"}
	operator := db.Operator{
		ID:          45,
		WechatMchID: pgtype.Text{String: "op_mch_45", Valid: true},
	}

	store.EXPECT().
		GetPaymentOrder(gomock.Any(), paymentOrder.ID).
		Return(paymentOrder, nil)
	store.EXPECT().
		GetOrder(gomock.Any(), order.ID).
		Return(order, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchant.ID).
		Return(merchant, nil)
	store.EXPECT().
		GetMerchantPaymentConfig(gomock.Any(), merchant.ID).
		Return(db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "sub_mch_16"}, nil)
	store.EXPECT().
		GetActiveProfitSharingConfig(gomock.Any(), db.GetActiveProfitSharingConfigParams{
			OrderSource: order.OrderType,
			MerchantID:  pgtype.Int8{Int64: merchant.ID, Valid: true},
			RegionID:    pgtype.Int8{Int64: merchant.RegionID, Valid: true},
		}).
		Return(db.ProfitSharingConfig{PlatformRate: 0, OperatorRate: 20, RiderEnabled: false}, nil)
	store.EXPECT().
		GetActiveOperatorByRegion(gomock.Any(), merchant.RegionID).
		Return(operator, nil)
	store.EXPECT().
		GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).
		Return(db.ProfitSharingOrder{}, db.ErrRecordNotFound)
	store.EXPECT().
		CreateProfitSharingOrder(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateProfitSharingOrderParams) (db.ProfitSharingOrder, error) {
			require.Equal(t, paymentOrder.Amount, arg.TotalAmount)
			require.Equal(t, int64(1400), arg.OperatorCommission)
			require.Equal(t, int64(5600), arg.MerchantAmount)
			return db.ProfitSharingOrder{ID: 3002, OutOrderNo: arg.OutOrderNo}, nil
		})
	ecommerceClient.EXPECT().
		GetSpAppID().
		Return("wx_sp_app_1")
	ecommerceClient.EXPECT().
		AddProfitSharingReceiver(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *wechat.AddReceiverRequest) (*wechat.AddReceiverResponse, error) {
			require.Equal(t, "op_mch_45", req.Account)
			return &wechat.AddReceiverResponse{}, nil
		})
	ecommerceClient.EXPECT().
		CreateProfitSharing(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *wechat.ProfitSharingRequest) (*wechat.ProfitSharingResponse, error) {
			require.Equal(t, int64(1400), req.Receivers[0].Amount)
			return &wechat.ProfitSharingResponse{OrderID: "ps_wx_3002", Status: "PROCESSING"}, nil
		})
	store.EXPECT().
		UpdateProfitSharingOrderToProcessing(gomock.Any(), db.UpdateProfitSharingOrderToProcessingParams{
			ID:             3002,
			SharingOrderID: pgtype.Text{String: "ps_wx_3002", Valid: true},
		}).
		Return(db.ProfitSharingOrder{ID: 3002, Status: "processing"}, nil)

	processor := worker.NewTestTaskProcessor(store, nil, nil, ecommerceClient)
	payloadBytes, err := json.Marshal(worker.ProfitSharingPayload{
		PaymentOrderID: paymentOrder.ID,
		OrderID:        order.ID,
	})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessProfitSharing, payloadBytes)
	err = processor.ProcessTaskProfitSharing(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskProfitSharing_DineInMarksFinishedWithoutSharing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	paymentOrder := db.PaymentOrder{
		ID:            903,
		Amount:        8800,
		TransactionID: pgtype.Text{String: "wx_txn_903", Valid: true},
	}
	order := db.Order{
		ID:          79,
		MerchantID:  17,
		TotalAmount: 8800,
		DeliveryFee: 600,
		OrderType:   "dine_in",
	}
	merchant := db.Merchant{ID: 17, RegionID: 14, Name: "堂食商户"}
	operator := db.Operator{ID: 46, WechatMchID: pgtype.Text{String: "op_mch_46", Valid: true}}

	store.EXPECT().
		GetPaymentOrder(gomock.Any(), paymentOrder.ID).
		Return(paymentOrder, nil)
	store.EXPECT().
		GetOrder(gomock.Any(), order.ID).
		Return(order, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchant.ID).
		Return(merchant, nil)
	store.EXPECT().
		GetMerchantPaymentConfig(gomock.Any(), merchant.ID).
		Return(db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "sub_mch_17"}, nil)
	store.EXPECT().
		GetActiveProfitSharingConfig(gomock.Any(), db.GetActiveProfitSharingConfigParams{
			OrderSource: order.OrderType,
			MerchantID:  pgtype.Int8{Int64: merchant.ID, Valid: true},
			RegionID:    pgtype.Int8{Int64: merchant.RegionID, Valid: true},
		}).
		Return(db.ProfitSharingConfig{PlatformRate: 10, OperatorRate: 20, RiderEnabled: true}, nil)
	store.EXPECT().
		GetActiveOperatorByRegion(gomock.Any(), merchant.RegionID).
		Return(operator, nil)
	store.EXPECT().
		GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).
		Return(db.ProfitSharingOrder{}, db.ErrRecordNotFound)
	store.EXPECT().
		CreateProfitSharingOrder(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, arg db.CreateProfitSharingOrderParams) (db.ProfitSharingOrder, error) {
			require.Equal(t, int32(0), arg.PlatformRate)
			require.Equal(t, int32(0), arg.OperatorRate)
			require.Equal(t, int64(0), arg.PlatformCommission)
			require.Equal(t, int64(0), arg.OperatorCommission)
			require.Equal(t, int64(0), arg.RiderAmount)
			require.Equal(t, paymentOrder.Amount, arg.MerchantAmount)
			return db.ProfitSharingOrder{ID: 3003, OutOrderNo: arg.OutOrderNo}, nil
		})
	store.EXPECT().
		UpdateProfitSharingOrderToFinished(gomock.Any(), int64(3003)).
		Return(db.ProfitSharingOrder{ID: 3003, Status: "finished"}, nil)

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)
	payloadBytes, err := json.Marshal(worker.ProfitSharingPayload{
		PaymentOrderID: paymentOrder.ID,
		OrderID:        order.ID,
	})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessProfitSharing, payloadBytes)
	err = processor.ProcessTaskProfitSharing(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskProfitSharing_ProcessingOrderQueriesAndFinishes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{
		ID:            904,
		Amount:        10000,
		TransactionID: pgtype.Text{String: "wx_txn_904", Valid: true},
		OrderID:       pgtype.Int8{Int64: 80, Valid: true},
	}
	existingOrder := db.ProfitSharingOrder{
		ID:         3004,
		MerchantID: 18,
		OutOrderNo: "PS90480",
		Status:     "processing",
	}
	merchant := db.Merchant{ID: 18, RegionID: 15, Name: "商户C"}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), int64(80)).Return(db.Order{ID: 80, MerchantID: merchant.ID, TotalAmount: 10000, DeliveryFee: 0, OrderType: "takeout"}, nil)
	store.EXPECT().GetMerchant(gomock.Any(), merchant.ID).Return(merchant, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Return(db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "sub_mch_18"}, nil)
	store.EXPECT().GetActiveProfitSharingConfig(gomock.Any(), gomock.Any()).Return(db.ProfitSharingConfig{PlatformRate: 0, OperatorRate: 10, RiderEnabled: false}, nil)
	store.EXPECT().GetActiveOperatorByRegion(gomock.Any(), merchant.RegionID).Return(db.Operator{}, db.ErrRecordNotFound)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(existingOrder, nil)
	ecommerceClient.EXPECT().QueryProfitSharing(gomock.Any(), "sub_mch_18", "wx_txn_904", existingOrder.OutOrderNo).
		Return(&wechat.ProfitSharingQueryResponse{
			Status:    "FINISHED",
			Receivers: []wechat.ProfitSharingReceiverResult{{Result: "SUCCESS"}},
		}, nil)
	store.EXPECT().UpdateProfitSharingOrderToFinished(gomock.Any(), existingOrder.ID).Return(db.ProfitSharingOrder{ID: existingOrder.ID, Status: "finished"}, nil)
	distributor.EXPECT().DistributeTaskProcessProfitSharingResult(gomock.Any(), gomock.AssignableToTypeOf(&worker.ProfitSharingResultPayload{}), gomock.Any()).
		DoAndReturn(func(_ context.Context, payload *worker.ProfitSharingResultPayload, _ ...asynq.Option) error {
			require.Equal(t, existingOrder.ID, payload.ProfitSharingOrderID)
			require.Equal(t, existingOrder.OutOrderNo, payload.OutOrderNo)
			require.Equal(t, "SUCCESS", payload.Result)
			require.Equal(t, merchant.ID, payload.MerchantID)
			return nil
		})

	processor := worker.NewTestTaskProcessor(store, distributor, nil, ecommerceClient)
	payloadBytes, err := json.Marshal(worker.ProfitSharingPayload{PaymentOrderID: paymentOrder.ID, OrderID: 80})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessProfitSharing, payloadBytes)
	err = processor.ProcessTaskProfitSharing(context.Background(), task)
	require.NoError(t, err)
}

// ==================== ProcessTaskProfitSharingResult Tests ====================

func TestProcessTaskProfitSharingResult_Success(t *testing.T) {
	testCases := []struct {
		name        string
		payload     worker.ProfitSharingResultPayload
		buildStubs  func(store *mockdb.MockStore, distributor *mockwk.MockTaskDistributor)
		checkResult func(t *testing.T, err error)
	}{
		{
			name: "分账成功_通知商户收入到账",
			payload: worker.ProfitSharingResultPayload{
				ProfitSharingOrderID: 1,
				OutOrderNo:           "PS123456",
				Result:               "SUCCESS",
				MerchantID:           100,
			},
			buildStubs: func(store *mockdb.MockStore, distributor *mockwk.MockTaskDistributor) {
				// 获取商户信息
				store.EXPECT().
					GetMerchant(gomock.Any(), int64(100)).
					Return(db.Merchant{
						ID:          100,
						OwnerUserID: 1001,
						Name:        "测试商户",
					}, nil)

				// 获取分账订单
				store.EXPECT().
					GetProfitSharingOrderByOutOrderNo(gomock.Any(), "PS123456").
					Return(db.ProfitSharingOrder{
						ID:                 1,
						MerchantID:         100,
						MerchantAmount:     9500, // 95元
						PlatformCommission: 200,  // 2元
						OperatorCommission: 300,  // 3元
						TotalAmount:        10000,
					}, nil)

				// 发送通知
				distributor.EXPECT().
					DistributeTaskSendNotification(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, payload *worker.SendNotificationPayload, opts ...asynq.Option) error {
						require.Equal(t, int64(1001), payload.UserID)
						require.Equal(t, "finance", payload.Type)
						require.Equal(t, "订单收入已到账", payload.Title)
						require.Contains(t, payload.Content, "95.00")
						return nil
					})
			},
			checkResult: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "分账成功_小额订单",
			payload: worker.ProfitSharingResultPayload{
				ProfitSharingOrderID: 2,
				OutOrderNo:           "PS789012",
				Result:               "SUCCESS",
				MerchantID:           200,
			},
			buildStubs: func(store *mockdb.MockStore, distributor *mockwk.MockTaskDistributor) {
				// 获取商户信息
				store.EXPECT().
					GetMerchant(gomock.Any(), int64(200)).
					Return(db.Merchant{
						ID:          200,
						OwnerUserID: 2001,
						Name:        "小商户",
					}, nil)

				// 获取分账订单
				store.EXPECT().
					GetProfitSharingOrderByOutOrderNo(gomock.Any(), "PS789012").
					Return(db.ProfitSharingOrder{
						ID:                 2,
						MerchantID:         200,
						MerchantAmount:     950, // 9.5元
						PlatformCommission: 20,
						OperatorCommission: 30,
						TotalAmount:        1000,
					}, nil)

				// 发送通知
				distributor.EXPECT().
					DistributeTaskSendNotification(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, payload *worker.SendNotificationPayload, opts ...asynq.Option) error {
						require.Contains(t, payload.Content, "9.50")
						return nil
					})
			},
			checkResult: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			distributor := mockwk.NewMockTaskDistributor(ctrl)

			tc.buildStubs(store, distributor)

			processor := worker.NewTestTaskProcessor(store, distributor, nil, nil)

			payload, err := json.Marshal(tc.payload)
			require.NoError(t, err)

			task := asynq.NewTask(worker.TaskProcessProfitSharingResult, payload)
			err = processor.ProcessTaskProfitSharingResult(context.Background(), task)
			tc.checkResult(t, err)
		})
	}
}

func TestProcessTaskProfitSharingResult_Failed(t *testing.T) {
	testCases := []struct {
		name        string
		payload     worker.ProfitSharingResultPayload
		buildStubs  func(store *mockdb.MockStore)
		checkResult func(t *testing.T, err error)
	}{
		{
			name: "分账失败_记录告警日志",
			payload: worker.ProfitSharingResultPayload{
				ProfitSharingOrderID: 3,
				OutOrderNo:           "PS345678",
				Result:               "FAILED",
				FailReason:           "余额不足",
				MerchantID:           100,
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 获取商户信息
				store.EXPECT().
					GetMerchant(gomock.Any(), int64(100)).
					Return(db.Merchant{
						ID:          100,
						OwnerUserID: 1001,
						Name:        "测试商户",
					}, nil)

				// 获取分账订单
				store.EXPECT().
					GetProfitSharingOrderByOutOrderNo(gomock.Any(), "PS345678").
					Return(db.ProfitSharingOrder{
						ID:             3,
						MerchantID:     100,
						MerchantAmount: 9500,
						TotalAmount:    10000,
					}, nil)
			},
			checkResult: func(t *testing.T, err error) {
				require.NoError(t, err) // 分账失败只记日志，不返回错误
			},
		},
		{
			name: "分账关闭_记录告警日志",
			payload: worker.ProfitSharingResultPayload{
				ProfitSharingOrderID: 4,
				OutOrderNo:           "PS901234",
				Result:               "CLOSED",
				MerchantID:           200,
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 获取商户信息
				store.EXPECT().
					GetMerchant(gomock.Any(), int64(200)).
					Return(db.Merchant{
						ID:          200,
						OwnerUserID: 2001,
						Name:        "测试商户2",
					}, nil)

				// 获取分账订单
				store.EXPECT().
					GetProfitSharingOrderByOutOrderNo(gomock.Any(), "PS901234").
					Return(db.ProfitSharingOrder{
						ID:             4,
						MerchantID:     200,
						MerchantAmount: 8000,
						TotalAmount:    10000,
					}, nil)
			},
			checkResult: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			processor := worker.NewTestTaskProcessor(store, nil, nil, nil)

			payload, err := json.Marshal(tc.payload)
			require.NoError(t, err)

			task := asynq.NewTask(worker.TaskProcessProfitSharingResult, payload)
			err = processor.ProcessTaskProfitSharingResult(context.Background(), task)
			tc.checkResult(t, err)
		})
	}
}

func TestProcessTaskProfitSharingResult_FailedReenqueuesReservationPayload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)

	store.EXPECT().
		GetMerchant(gomock.Any(), int64(300)).
		Return(db.Merchant{ID: 300, OwnerUserID: 3001, Name: "预订商户"}, nil)
	store.EXPECT().
		GetProfitSharingOrderByOutOrderNo(gomock.Any(), "PSR300").
		Return(db.ProfitSharingOrder{ID: 31, MerchantID: 300, PaymentOrderID: 701, MerchantAmount: 9000, TotalAmount: 10000}, nil)
	store.EXPECT().
		GetPaymentOrder(gomock.Any(), int64(701)).
		Return(db.PaymentOrder{ID: 701, ReservationID: pgtype.Int8{Int64: 801, Valid: true}}, nil)
	distributor.EXPECT().
		DistributeTaskProcessProfitSharing(gomock.Any(), gomock.AssignableToTypeOf(&worker.ProfitSharingPayload{}), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, payload *worker.ProfitSharingPayload, _ ...asynq.Option) error {
			require.Equal(t, int64(701), payload.PaymentOrderID)
			require.Equal(t, int64(801), payload.ReservationID)
			require.Zero(t, payload.OrderID)
			return nil
		})

	processor := worker.NewTestTaskProcessor(store, distributor, nil, nil)
	payloadBytes, err := json.Marshal(worker.ProfitSharingResultPayload{
		ProfitSharingOrderID: 31,
		OutOrderNo:           "PSR300",
		Result:               "FAILED",
		FailReason:           "system busy",
		MerchantID:           300,
	})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessProfitSharingResult, payloadBytes)
	err = processor.ProcessTaskProfitSharingResult(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskProfitSharingResult_MerchantNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	// 商户不存在
	store.EXPECT().
		GetMerchant(gomock.Any(), int64(999)).
		Return(db.Merchant{}, db.ErrRecordNotFound)

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)

	payload := worker.ProfitSharingResultPayload{
		ProfitSharingOrderID: 5,
		OutOrderNo:           "PS567890",
		Result:               "SUCCESS",
		MerchantID:           999,
	}

	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessProfitSharingResult, payloadBytes)
	err = processor.ProcessTaskProfitSharingResult(context.Background(), task)
	require.NoError(t, err) // 商户不存在不返回错误，只记日志
}

func TestProcessTaskProfitSharingResult_InvalidPayload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	processor := worker.NewTestTaskProcessor(nil, nil, nil, nil)

	// 无效的 JSON payload
	task := asynq.NewTask(worker.TaskProcessProfitSharingResult, []byte("invalid json"))
	err := processor.ProcessTaskProfitSharingResult(context.Background(), task)
	require.Error(t, err)
	require.ErrorIs(t, err, asynq.SkipRetry)
}

func TestProcessTaskProfitSharingResult_DBError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	// 数据库错误（非 NotFound）
	store.EXPECT().
		GetMerchant(gomock.Any(), int64(100)).
		Return(db.Merchant{}, errors.New("database connection error"))

	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)

	payload := worker.ProfitSharingResultPayload{
		ProfitSharingOrderID: 6,
		OutOrderNo:           "PS111222",
		Result:               "SUCCESS",
		MerchantID:           100,
	}

	payloadBytes, err := json.Marshal(payload)
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessProfitSharingResult, payloadBytes)
	err = processor.ProcessTaskProfitSharingResult(context.Background(), task)
	require.Error(t, err) // 数据库连接错误应该重试
	require.Contains(t, err.Error(), "get merchant")
}
