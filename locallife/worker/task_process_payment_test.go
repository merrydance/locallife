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
	wechatcontracts "github.com/merrydance/locallife/wechat/contracts"
	mockwechat "github.com/merrydance/locallife/wechat/mock"
	"github.com/merrydance/locallife/worker"
	mockwk "github.com/merrydance/locallife/worker/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type profitSharingFactApplicationEnqueueRecorder struct {
	worker.NoopTaskDistributor
	applicationIDs []int64
}

func (r *profitSharingFactApplicationEnqueueRecorder) DistributeTaskProcessPaymentFactApplication(_ context.Context, payload *worker.PaymentFactApplicationPayload, _ ...asynq.Option) error {
	r.applicationIDs = append(r.applicationIDs, payload.ApplicationID)
	return nil
}

func TestProcessTaskRefundResult_RiderDepositReturnsSkipRetryBecauseMovedToFactApplication(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	processor := worker.NewTestTaskProcessor(store, nil, nil, nil)

	refundOrder := db.RefundOrder{ID: 702, PaymentOrderID: 551, OutRefundNo: "REFUND_702", RefundAmount: 30000}
	paymentOrder := db.PaymentOrder{ID: 551, UserID: 77, Amount: 30000, BusinessType: "rider_deposit"}

	store.EXPECT().GetRefundOrderByOutRefundNo(gomock.Any(), refundOrder.OutRefundNo).Return(refundOrder, nil)
	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)

	payload, err := json.Marshal(worker.RefundResultPayload{OutRefundNo: refundOrder.OutRefundNo, RefundStatus: "SUCCESS", RefundID: "WX_REFUND_702"})
	require.NoError(t, err)

	err = processor.ProcessTaskRefundResult(context.Background(), asynq.NewTask(worker.TaskProcessRefundResult, payload))
	require.Error(t, err)
	require.Contains(t, err.Error(), "rider deposit refund results must be applied via payment fact application")
	require.True(t, errors.Is(err, asynq.SkipRetry))
}

// ==================== ProcessTaskApplymentResult Tests ====================

func TestProcessTaskApplymentResult_Success(t *testing.T) {
	testCases := []struct {
		name        string
		payload     worker.ApplymentResultPayload
		buildStubs  func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface, distributor *mockwk.MockTaskDistributor)
		checkResult func(t *testing.T, err error)
	}{
		{
			name: "商户进件成功_发送通知并标记已处理",
			payload: worker.ApplymentResultPayload{
				ApplymentID:    1,
				OutRequestNo:   "APPLY_M_1_1234567890",
				ApplymentState: "FINISH",
				SubMchID:       "1234567890",
				SubjectType:    "merchant",
				SubjectID:      100,
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface, distributor *mockwk.MockTaskDistributor) {
				// 1. 获取商户信息
				store.EXPECT().
					GetMerchant(gomock.Any(), int64(100)).
					Return(db.Merchant{
						ID:          100,
						OwnerUserID: 1001,
						Name:        "测试商户",
					}, nil)

				// 2. 发送通知
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
				ApplymentState: "FINISH",
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
			name: "已移除的运营商进件成功结果被忽略",
			payload: worker.ApplymentResultPayload{
				ApplymentID:    12,
				OutRequestNo:   "APPLY_O_12_1234567890",
				ApplymentState: "FINISH",
				SubMchID:       "2234567890",
				SubjectType:    "operator",
				SubjectID:      300,
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface, distributor *mockwk.MockTaskDistributor) {
			},
			checkResult: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "进件成功_不依赖添加商户接收方",
			payload: worker.ApplymentResultPayload{
				ApplymentID:    3,
				OutRequestNo:   "APPLY_M_3_1234567890",
				ApplymentState: "FINISH",
				SubMchID:       "1234567890",
				SubjectType:    "merchant",
				SubjectID:      100,
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface, distributor *mockwk.MockTaskDistributor) {
				// 1. 获取商户信息
				store.EXPECT().
					GetMerchant(gomock.Any(), int64(100)).
					Return(db.Merchant{
						ID:          100,
						OwnerUserID: 1001,
						Name:        "测试商户",
					}, nil)

				// 2. 发送通知
				distributor.EXPECT().
					DistributeTaskSendNotification(gomock.Any(), gomock.Any()).
					Return(nil)
			},
			checkResult: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "进件成功_商户不存在",
			payload: worker.ApplymentResultPayload{
				ApplymentID:    4,
				OutRequestNo:   "APPLY_M_4_1234567890",
				ApplymentState: "FINISH",
				SubMchID:       "1234567890",
				SubjectType:    "merchant",
				SubjectID:      999,
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface, distributor *mockwk.MockTaskDistributor) {
				// 1. 商户不存在，流程提前结束
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

func TestProcessTaskApplymentResult_AuditingUnsignedStillTriggersPendingNotification(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)

	store.EXPECT().
		GetMerchant(gomock.Any(), int64(100)).
		Return(db.Merchant{
			ID:          100,
			OwnerUserID: 1001,
			Name:        "测试商户",
		}, nil)

	distributor.EXPECT().
		DistributeTaskSendNotification(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, payload *worker.SendNotificationPayload, _ ...asynq.Option) error {
			require.Equal(t, int64(1001), payload.UserID)
			require.Equal(t, "微信支付开户待处理", payload.Title)
			require.Contains(t, payload.Content, "需要签约")
			return nil
		})

	processor := worker.NewTestTaskProcessor(store, distributor, nil, nil)
	payload, err := json.Marshal(worker.ApplymentResultPayload{
		ApplymentID:     15,
		OutRequestNo:    "APPLY_M_15_1234567890",
		ApplymentStatus: "auditing",
		SignState:       "UNSIGNED",
		SubjectType:     "merchant",
		SubjectID:       100,
	})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessApplymentResult, payload)
	err = processor.ProcessTaskApplymentResult(context.Background(), task)
	require.NoError(t, err)
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
				ApplymentState: "REJECTED",
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
				ApplymentState: "REJECTED",
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
			name: "已移除的运营商进件驳回结果被忽略",
			payload: worker.ApplymentResultPayload{
				ApplymentID:    13,
				OutRequestNo:   "APPLY_O_13_1234567890",
				ApplymentState: "REJECTED",
				SubjectType:    "operator",
				SubjectID:      300,
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
				ApplymentState: "REJECTED",
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
				ApplymentID:     8,
				OutRequestNo:    "APPLY_M_8_1234567890",
				ApplymentStatus: "to_be_confirmed",
				SubjectType:     "merchant",
				SubjectID:       100,
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
				ApplymentID:     9,
				OutRequestNo:    "APPLY_R_9_1234567890",
				ApplymentStatus: "to_be_signed",
				SubjectType:     "rider",
				SubjectID:       200,
			},
			buildStubs: func(store *mockdb.MockStore, distributor *mockwk.MockTaskDistributor) {
			},
			checkResult: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "运营商待账户验证结果被忽略",
			payload: worker.ApplymentResultPayload{
				ApplymentID:     14,
				OutRequestNo:    "APPLY_O_14_1234567890",
				ApplymentStatus: "account_need_verify",
				SubjectType:     "operator",
				SubjectID:       300,
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
		buildStubs  func(store *mockdb.MockStore, distributor *mockwk.MockTaskDistributor)
		checkResult func(t *testing.T, err error)
	}{
		{
			name: "审核中状态_不处理",
			payload: worker.ApplymentResultPayload{
				ApplymentID:    10,
				ApplymentState: "AUDITING",
				SubjectType:    "merchant",
				SubjectID:      100,
			},
			buildStubs: func(store *mockdb.MockStore, distributor *mockwk.MockTaskDistributor) {},
			checkResult: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "冻结状态_发送通知",
			payload: worker.ApplymentResultPayload{
				ApplymentID:    11,
				ApplymentState: "FROZEN",
				SubjectType:    "merchant",
				SubjectID:      100,
			},
			buildStubs: func(store *mockdb.MockStore, distributor *mockwk.MockTaskDistributor) {
				store.EXPECT().
					GetMerchant(gomock.Any(), int64(100)).
					Return(db.Merchant{ID: 100, OwnerUserID: 1001, Name: "测试商户"}, nil)

				distributor.EXPECT().
					DistributeTaskSendNotification(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, payload *worker.SendNotificationPayload, _ ...asynq.Option) error {
						require.Equal(t, int64(1001), payload.UserID)
						require.Equal(t, "微信支付开户已冻结", payload.Title)
						require.Contains(t, payload.Content, "已被冻结")
						return nil
					})
			},
			checkResult: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "作废状态_发送通知",
			payload: worker.ApplymentResultPayload{
				ApplymentID:    12,
				ApplymentState: "CANCELED",
				SubjectType:    "merchant",
				SubjectID:      101,
			},
			buildStubs: func(store *mockdb.MockStore, distributor *mockwk.MockTaskDistributor) {
				store.EXPECT().
					GetMerchant(gomock.Any(), int64(101)).
					Return(db.Merchant{ID: 101, OwnerUserID: 1002, Name: "测试商户二"}, nil)

				distributor.EXPECT().
					DistributeTaskSendNotification(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, payload *worker.SendNotificationPayload, _ ...asynq.Option) error {
						require.Equal(t, int64(1002), payload.UserID)
						require.Equal(t, "微信支付开户已作废", payload.Title)
						require.Contains(t, payload.Content, "已作废")
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
			if tc.buildStubs != nil {
				tc.buildStubs(store, distributor)
			}

			processor := worker.NewTestTaskProcessor(store, distributor, nil, nil)

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
		ID:     44,
		UserID: 440,
		Name:   "区域运营商A",
	}
	operatorUser := db.User{ID: operator.UserID, WechatOpenid: "operator_openid_44", FullName: "区域运营商A"}

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
		GetUser(gomock.Any(), operator.UserID).
		Return(operatorUser, nil)
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
		EncryptSensitiveData(operator.Name).
		Return("encrypted_operator_44", nil)
	ecommerceClient.EXPECT().
		AddProfitSharingReceiver(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *wechatcontracts.AddReceiverRequest) (*wechatcontracts.AddReceiverResponse, error) {
			require.Equal(t, wechatcontracts.ReceiverTypePersonal, req.Type)
			require.Equal(t, operatorUser.WechatOpenid, req.Account)
			require.Equal(t, "encrypted_operator_44", req.EncryptedName)
			return &wechatcontracts.AddReceiverResponse{}, nil
		})
	ecommerceClient.EXPECT().
		CreateProfitSharing(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *wechatcontracts.ProfitSharingRequest) (*wechatcontracts.ProfitSharingResponse, error) {
			require.Equal(t, "sub_mch_15", req.SubMchID)
			require.Equal(t, "wx_txn_901", req.TransactionID)
			require.Len(t, req.Receivers, 1)
			require.Equal(t, wechatcontracts.ReceiverTypePersonal, req.Receivers[0].Type)
			require.Equal(t, operatorUser.WechatOpenid, req.Receivers[0].ReceiverAccount)
			require.Empty(t, req.Receivers[0].ReceiverName)
			require.Equal(t, int64(2000), req.Receivers[0].Amount)
			return &wechatcontracts.ProfitSharingResponse{OrderID: "ps_wx_3001", Status: wechatcontracts.ProfitSharingStatusProcessing}, nil
		})
	store.EXPECT().
		UpdateProfitSharingOrderToProcessing(gomock.Any(), db.UpdateProfitSharingOrderToProcessingParams{
			ID:             3001,
			SharingOrderID: pgtype.Text{String: "ps_wx_3001", Valid: true},
		}).
		Return(db.ProfitSharingOrder{ID: 3001, Status: "processing"}, nil)
	expectProfitSharingCommand(t, store, 3001, "PS90177", db.ExternalPaymentCommandTypeCreateProfitSharing, "ps_wx_3001", wechatcontracts.ProfitSharingStatusProcessing, 9921)

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

func TestProcessTaskProfitSharing_UsesPersonalOperatorOpenID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{
		ID:            904,
		Amount:        10000,
		TransactionID: pgtype.Text{String: "wx_txn_904", Valid: true},
	}
	order := db.Order{
		ID:          80,
		MerchantID:  18,
		TotalAmount: 10000,
		DeliveryFee: 0,
		OrderType:   "takeout",
		AddressID:   pgtype.Int8{Int64: 1002, Valid: true},
	}
	merchant := db.Merchant{ID: 18, RegionID: 15, Name: "商户个人运营商分账"}
	operator := db.Operator{
		ID:          47,
		UserID:      501,
		Name:        "个人运营商",
		ContactName: "个人运营商联系人",
	}
	operatorUser := db.User{ID: operator.UserID, WechatOpenid: "operator_openid_47", FullName: "个人运营商联系人"}

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
		Return(db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "sub_mch_18"}, nil)
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
			require.True(t, arg.OperatorID.Valid)
			require.Equal(t, operator.ID, arg.OperatorID.Int64)
			require.Equal(t, int64(2000), arg.OperatorCommission)
			return db.ProfitSharingOrder{ID: 3004, OutOrderNo: arg.OutOrderNo}, nil
		})
	store.EXPECT().
		GetUser(gomock.Any(), operator.UserID).
		Return(operatorUser, nil)
	ecommerceClient.EXPECT().
		GetSpAppID().
		Return("wx_sp_app_1")
	ecommerceClient.EXPECT().
		EncryptSensitiveData(operator.ContactName).
		Return("encrypted_operator_name", nil)
	ecommerceClient.EXPECT().
		AddProfitSharingReceiver(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *wechatcontracts.AddReceiverRequest) (*wechatcontracts.AddReceiverResponse, error) {
			require.Equal(t, wechatcontracts.ReceiverTypePersonal, req.Type)
			require.Equal(t, operatorUser.WechatOpenid, req.Account)
			require.Equal(t, "encrypted_operator_name", req.EncryptedName)
			return &wechatcontracts.AddReceiverResponse{}, nil
		})
	ecommerceClient.EXPECT().
		CreateProfitSharing(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *wechatcontracts.ProfitSharingRequest) (*wechatcontracts.ProfitSharingResponse, error) {
			require.Equal(t, "sub_mch_18", req.SubMchID)
			require.Len(t, req.Receivers, 1)
			require.Equal(t, wechatcontracts.ReceiverTypePersonal, req.Receivers[0].Type)
			require.Equal(t, operatorUser.WechatOpenid, req.Receivers[0].ReceiverAccount)
			require.Empty(t, req.Receivers[0].ReceiverName)
			require.Equal(t, int64(2000), req.Receivers[0].Amount)
			return &wechatcontracts.ProfitSharingResponse{OrderID: "ps_wx_3004", Status: wechatcontracts.ProfitSharingStatusProcessing}, nil
		})
	store.EXPECT().
		UpdateProfitSharingOrderToProcessing(gomock.Any(), db.UpdateProfitSharingOrderToProcessingParams{
			ID:             3004,
			SharingOrderID: pgtype.Text{String: "ps_wx_3004", Valid: true},
		}).
		Return(db.ProfitSharingOrder{ID: 3004, Status: "processing"}, nil)
	expectProfitSharingCommand(t, store, 3004, "PS90480", db.ExternalPaymentCommandTypeCreateProfitSharing, "ps_wx_3004", wechatcontracts.ProfitSharingStatusProcessing, 9922)

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

func TestProcessTaskProfitSharing_FailsWhenOperatorOpenIDMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{
		ID:            905,
		Amount:        10000,
		TransactionID: pgtype.Text{String: "wx_txn_905", Valid: true},
	}
	order := db.Order{
		ID:          81,
		MerchantID:  19,
		TotalAmount: 10000,
		DeliveryFee: 0,
		OrderType:   "takeout",
		AddressID:   pgtype.Int8{Int64: 1003, Valid: true},
	}
	merchant := db.Merchant{ID: 19, RegionID: 16, Name: "商户企业运营商待开户"}
	operator := db.Operator{
		ID:          48,
		UserID:      502,
		Name:        "企业运营商待开户",
		ContactName: "企业运营商联系人",
	}
	operatorUser := db.User{ID: operator.UserID}

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
		Return(db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "sub_mch_19"}, nil)
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
			require.True(t, arg.OperatorID.Valid)
			require.Equal(t, operator.ID, arg.OperatorID.Int64)
			require.Equal(t, int64(2000), arg.OperatorCommission)
			return db.ProfitSharingOrder{ID: 3005, OutOrderNo: arg.OutOrderNo}, nil
		})
	store.EXPECT().
		GetUser(gomock.Any(), operator.UserID).
		Return(operatorUser, nil)

	processor := worker.NewTestTaskProcessor(store, nil, nil, ecommerceClient)
	payloadBytes, err := json.Marshal(worker.ProfitSharingPayload{
		PaymentOrderID: paymentOrder.ID,
		OrderID:        order.ID,
	})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessProfitSharing, payloadBytes)
	err = processor.ProcessTaskProfitSharing(context.Background(), task)
	require.Error(t, err)
	require.Contains(t, err.Error(), "resolve operator receiver: operator wechat openid not configured")
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
		ID:     45,
		UserID: 450,
		Name:   "区域运营商B",
	}
	operatorUser := db.User{ID: operator.UserID, WechatOpenid: "operator_openid_45", FullName: "区域运营商B"}

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
		GetUser(gomock.Any(), operator.UserID).
		Return(operatorUser, nil)
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
		EncryptSensitiveData(operator.Name).
		Return("encrypted_operator_45", nil)
	ecommerceClient.EXPECT().
		AddProfitSharingReceiver(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *wechatcontracts.AddReceiverRequest) (*wechatcontracts.AddReceiverResponse, error) {
			require.Equal(t, wechatcontracts.ReceiverTypePersonal, req.Type)
			require.Equal(t, operatorUser.WechatOpenid, req.Account)
			require.Equal(t, "encrypted_operator_45", req.EncryptedName)
			return &wechatcontracts.AddReceiverResponse{}, nil
		})
	ecommerceClient.EXPECT().
		CreateProfitSharing(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *wechatcontracts.ProfitSharingRequest) (*wechatcontracts.ProfitSharingResponse, error) {
			require.Equal(t, wechatcontracts.ReceiverTypePersonal, req.Receivers[0].Type)
			require.Equal(t, operatorUser.WechatOpenid, req.Receivers[0].ReceiverAccount)
			require.Empty(t, req.Receivers[0].ReceiverName)
			require.Equal(t, int64(1400), req.Receivers[0].Amount)
			return &wechatcontracts.ProfitSharingResponse{OrderID: "ps_wx_3002", Status: wechatcontracts.ProfitSharingStatusProcessing}, nil
		})
	store.EXPECT().
		UpdateProfitSharingOrderToProcessing(gomock.Any(), db.UpdateProfitSharingOrderToProcessingParams{
			ID:             3002,
			SharingOrderID: pgtype.Text{String: "ps_wx_3002", Valid: true},
		}).
		Return(db.ProfitSharingOrder{ID: 3002, Status: "processing"}, nil)
	expectProfitSharingCommand(t, store, 3002, "PS90278", db.ExternalPaymentCommandTypeCreateProfitSharing, "ps_wx_3002", wechatcontracts.ProfitSharingStatusProcessing, 9923)

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

func TestProcessTaskProfitSharing_UsesServiceProviderNameForPlatformReceiver(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{
		ID:            903,
		Amount:        10000,
		TransactionID: pgtype.Text{String: "wx_txn_903", Valid: true},
	}
	order := db.Order{
		ID:          79,
		MerchantID:  17,
		TotalAmount: 10000,
		DeliveryFee: 0,
		OrderType:   "takeout",
		AddressID:   pgtype.Int8{Int64: 1001, Valid: true},
	}
	merchant := db.Merchant{ID: 17, RegionID: 14, Name: "商户平台分账"}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetMerchant(gomock.Any(), merchant.ID).Return(merchant, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Return(db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "sub_mch_17"}, nil)
	store.EXPECT().GetActiveProfitSharingConfig(gomock.Any(), db.GetActiveProfitSharingConfigParams{
		OrderSource: order.OrderType,
		MerchantID:  pgtype.Int8{Int64: merchant.ID, Valid: true},
		RegionID:    pgtype.Int8{Int64: merchant.RegionID, Valid: true},
	}).Return(db.ProfitSharingConfig{PlatformRate: 10, OperatorRate: 0, RiderEnabled: false}, nil)
	store.EXPECT().GetActiveOperatorByRegion(gomock.Any(), merchant.RegionID).Return(db.Operator{}, db.ErrRecordNotFound)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(db.ProfitSharingOrder{}, db.ErrRecordNotFound)
	store.EXPECT().CreateProfitSharingOrder(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateProfitSharingOrderParams) (db.ProfitSharingOrder, error) {
		require.Equal(t, int64(1000), arg.PlatformCommission)
		require.Equal(t, int64(9000), arg.MerchantAmount)
		return db.ProfitSharingOrder{ID: 3003, OutOrderNo: arg.OutOrderNo}, nil
	})
	ecommerceClient.EXPECT().GetSpMchID().Return("sp_mch_1")
	ecommerceClient.EXPECT().GetSpMchName().Return("平台服务商")
	ecommerceClient.EXPECT().CreateProfitSharing(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *wechatcontracts.ProfitSharingRequest) (*wechatcontracts.ProfitSharingResponse, error) {
		require.Len(t, req.Receivers, 1)
		require.Equal(t, "sp_mch_1", req.Receivers[0].ReceiverAccount)
		require.Equal(t, "平台服务商", req.Receivers[0].ReceiverName)
		require.Equal(t, int64(1000), req.Receivers[0].Amount)
		return &wechatcontracts.ProfitSharingResponse{OrderID: "ps_wx_3003", Status: wechatcontracts.ProfitSharingStatusProcessing}, nil
	})
	store.EXPECT().UpdateProfitSharingOrderToProcessing(gomock.Any(), db.UpdateProfitSharingOrderToProcessingParams{
		ID:             3003,
		SharingOrderID: pgtype.Text{String: "ps_wx_3003", Valid: true},
	}).Return(db.ProfitSharingOrder{ID: 3003, Status: "processing"}, nil)
	expectProfitSharingCommand(t, store, 3003, "PS90379", db.ExternalPaymentCommandTypeCreateProfitSharing, "ps_wx_3003", wechatcontracts.ProfitSharingStatusProcessing, 9924)

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

func TestProcessTaskProfitSharing_RedirectsOperatorCommissionToPlatformWhenNoActiveOperator(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{
		ID:            906,
		Amount:        10000,
		TransactionID: pgtype.Text{String: "wx_txn_906", Valid: true},
	}
	order := db.Order{
		ID:          82,
		MerchantID:  20,
		TotalAmount: 10000,
		DeliveryFee: 0,
		OrderType:   "takeout",
		AddressID:   pgtype.Int8{Int64: 1004, Valid: true},
	}
	merchant := db.Merchant{ID: 20, RegionID: 17, Name: "商户无运营商区域"}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetMerchant(gomock.Any(), merchant.ID).Return(merchant, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Return(db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "sub_mch_20"}, nil)
	store.EXPECT().GetActiveProfitSharingConfig(gomock.Any(), db.GetActiveProfitSharingConfigParams{
		OrderSource: order.OrderType,
		MerchantID:  pgtype.Int8{Int64: merchant.ID, Valid: true},
		RegionID:    pgtype.Int8{Int64: merchant.RegionID, Valid: true},
	}).Return(db.ProfitSharingConfig{PlatformRate: 10, OperatorRate: 5, RiderEnabled: false}, nil)
	store.EXPECT().GetActiveOperatorByRegion(gomock.Any(), merchant.RegionID).Return(db.Operator{}, db.ErrRecordNotFound)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(db.ProfitSharingOrder{}, db.ErrRecordNotFound)
	store.EXPECT().CreateProfitSharingOrder(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateProfitSharingOrderParams) (db.ProfitSharingOrder, error) {
		require.False(t, arg.OperatorID.Valid)
		require.Equal(t, int64(1500), arg.PlatformCommission)
		require.Equal(t, int64(0), arg.OperatorCommission)
		require.Equal(t, int64(8500), arg.MerchantAmount)
		return db.ProfitSharingOrder{ID: 3006, OutOrderNo: arg.OutOrderNo}, nil
	})
	ecommerceClient.EXPECT().GetSpMchID().Return("sp_mch_1")
	ecommerceClient.EXPECT().GetSpMchName().Return("平台服务商")
	ecommerceClient.EXPECT().CreateProfitSharing(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *wechatcontracts.ProfitSharingRequest) (*wechatcontracts.ProfitSharingResponse, error) {
		require.Len(t, req.Receivers, 1)
		require.Equal(t, "sp_mch_1", req.Receivers[0].ReceiverAccount)
		require.Equal(t, "平台服务商", req.Receivers[0].ReceiverName)
		require.Equal(t, int64(1500), req.Receivers[0].Amount)
		return &wechatcontracts.ProfitSharingResponse{OrderID: "ps_wx_3006", Status: wechatcontracts.ProfitSharingStatusProcessing}, nil
	})
	store.EXPECT().UpdateProfitSharingOrderToProcessing(gomock.Any(), db.UpdateProfitSharingOrderToProcessingParams{
		ID:             3006,
		SharingOrderID: pgtype.Text{String: "ps_wx_3006", Valid: true},
	}).Return(db.ProfitSharingOrder{ID: 3006, Status: "processing"}, nil)
	expectProfitSharingCommand(t, store, 3006, "PS90682", db.ExternalPaymentCommandTypeCreateProfitSharing, "ps_wx_3006", wechatcontracts.ProfitSharingStatusProcessing, 9925)

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

func TestProcessTaskProfitSharing_DineInRequestsFinishOrderWithoutSharing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

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
	ecommerceClient.EXPECT().
		FinishProfitSharing(gomock.Any(), "sub_mch_17", paymentOrder.TransactionID.String, "PS90379", "无需分账，解冻剩余资金").
		Return(&wechatcontracts.ProfitSharingResponse{OrderID: "ps_finish_3003", Status: wechatcontracts.ProfitSharingStatusProcessing}, nil)
	store.EXPECT().
		UpdateProfitSharingOrderToProcessing(gomock.Any(), db.UpdateProfitSharingOrderToProcessingParams{
			ID:             3003,
			SharingOrderID: pgtype.Text{String: "ps_finish_3003", Valid: true},
		}).
		Return(db.ProfitSharingOrder{ID: 3003, Status: "processing"}, nil)
	expectProfitSharingCommand(t, store, 3003, "PS90379", db.ExternalPaymentCommandTypeFinishProfitSharing, "ps_finish_3003", wechatcontracts.ProfitSharingStatusProcessing, 9928)

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

func TestProcessTaskProfitSharing_ReservationLinkedDineInUsesReservationSource(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{
		ID:            905,
		Amount:        8800,
		TransactionID: pgtype.Text{String: "wx_txn_905", Valid: true},
	}
	order := db.Order{
		ID:            88,
		MerchantID:    18,
		TotalAmount:   8800,
		DeliveryFee:   0,
		OrderType:     "dine_in",
		ReservationID: pgtype.Int8{Int64: 901, Valid: true},
	}
	merchant := db.Merchant{ID: 18, RegionID: 15, Name: "预订到店商户"}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetMerchant(gomock.Any(), merchant.ID).Return(merchant, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Return(db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "sub_mch_18"}, nil)
	store.EXPECT().GetActiveProfitSharingConfig(gomock.Any(), db.GetActiveProfitSharingConfigParams{
		OrderSource: db.OrderTypeReservation,
		MerchantID:  pgtype.Int8{Int64: merchant.ID, Valid: true},
		RegionID:    pgtype.Int8{Int64: merchant.RegionID, Valid: true},
	}).Return(db.ProfitSharingConfig{PlatformRate: 10, OperatorRate: 0, RiderEnabled: false}, nil)
	store.EXPECT().GetActiveOperatorByRegion(gomock.Any(), merchant.RegionID).Return(db.Operator{}, db.ErrRecordNotFound)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(db.ProfitSharingOrder{}, db.ErrRecordNotFound)
	store.EXPECT().CreateProfitSharingOrder(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateProfitSharingOrderParams) (db.ProfitSharingOrder, error) {
		require.Equal(t, db.OrderTypeReservation, arg.OrderSource)
		require.Equal(t, int32(10), arg.PlatformRate)
		require.Equal(t, int64(880), arg.PlatformCommission)
		require.Equal(t, int64(7920), arg.MerchantAmount)
		return db.ProfitSharingOrder{ID: 3005, OutOrderNo: arg.OutOrderNo}, nil
	})
	ecommerceClient.EXPECT().GetSpMchID().Return("sp_mch_1")
	ecommerceClient.EXPECT().GetSpMchName().Return("平台服务商")
	ecommerceClient.EXPECT().CreateProfitSharing(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *wechatcontracts.ProfitSharingRequest) (*wechatcontracts.ProfitSharingResponse, error) {
		require.Equal(t, "sub_mch_18", req.SubMchID)
		require.Equal(t, "wx_txn_905", req.TransactionID)
		require.Len(t, req.Receivers, 1)
		require.Equal(t, int64(880), req.Receivers[0].Amount)
		return &wechatcontracts.ProfitSharingResponse{OrderID: "ps_wx_3005", Status: wechatcontracts.ProfitSharingStatusProcessing}, nil
	})
	store.EXPECT().UpdateProfitSharingOrderToProcessing(gomock.Any(), db.UpdateProfitSharingOrderToProcessingParams{
		ID:             3005,
		SharingOrderID: pgtype.Text{String: "ps_wx_3005", Valid: true},
	}).Return(db.ProfitSharingOrder{ID: 3005, Status: "processing"}, nil)
	expectProfitSharingCommand(t, store, 3005, "PS90588", db.ExternalPaymentCommandTypeCreateProfitSharing, "ps_wx_3005", wechatcontracts.ProfitSharingStatusProcessing, 9926)

	processor := worker.NewTestTaskProcessor(store, nil, nil, ecommerceClient)
	payloadBytes, err := json.Marshal(worker.ProfitSharingPayload{PaymentOrderID: paymentOrder.ID, OrderID: order.ID})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessProfitSharing, payloadBytes)
	err = processor.ProcessTaskProfitSharing(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskProfitSharing_SubmitFinishedResponseRecordsCommandFactOnly(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &profitSharingFactApplicationEnqueueRecorder{}
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{
		ID:            911,
		Amount:        10000,
		TransactionID: pgtype.Text{String: "wx_txn_911", Valid: true},
	}
	order := db.Order{
		ID:          89,
		MerchantID:  25,
		TotalAmount: 10000,
		DeliveryFee: 0,
		OrderType:   "takeout",
	}
	merchant := db.Merchant{ID: 25, RegionID: 19, Name: "同步完成分账商户"}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetMerchant(gomock.Any(), merchant.ID).Return(merchant, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Return(db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "sub_mch_25"}, nil)
	store.EXPECT().GetActiveProfitSharingConfig(gomock.Any(), db.GetActiveProfitSharingConfigParams{
		OrderSource: order.OrderType,
		MerchantID:  pgtype.Int8{Int64: merchant.ID, Valid: true},
		RegionID:    pgtype.Int8{Int64: merchant.RegionID, Valid: true},
	}).Return(db.ProfitSharingConfig{PlatformRate: 10, OperatorRate: 0, RiderEnabled: false}, nil)
	store.EXPECT().GetActiveOperatorByRegion(gomock.Any(), merchant.RegionID).Return(db.Operator{}, db.ErrRecordNotFound)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(db.ProfitSharingOrder{}, db.ErrRecordNotFound)
	store.EXPECT().CreateProfitSharingOrder(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateProfitSharingOrderParams) (db.ProfitSharingOrder, error) {
		require.Equal(t, int64(1000), arg.PlatformCommission)
		require.Equal(t, int64(9000), arg.MerchantAmount)
		return db.ProfitSharingOrder{ID: 3011, OutOrderNo: arg.OutOrderNo, PlatformCommission: 1000}, nil
	})
	ecommerceClient.EXPECT().GetSpMchID().Return("sp_mch_1")
	ecommerceClient.EXPECT().GetSpMchName().Return("平台服务商")
	ecommerceClient.EXPECT().CreateProfitSharing(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *wechatcontracts.ProfitSharingRequest) (*wechatcontracts.ProfitSharingResponse, error) {
		require.Len(t, req.Receivers, 1)
		require.Equal(t, int64(1000), req.Receivers[0].Amount)
		return &wechatcontracts.ProfitSharingResponse{OrderID: "ps_wx_3011", Status: wechatcontracts.ProfitSharingStatusFinished}, nil
	})
	store.EXPECT().UpdateProfitSharingOrderToProcessing(gomock.Any(), db.UpdateProfitSharingOrderToProcessingParams{
		ID:             3011,
		SharingOrderID: pgtype.Text{String: "ps_wx_3011", Valid: true},
	}).Return(db.ProfitSharingOrder{ID: 3011, Status: "processing"}, nil)
	expectProfitSharingCommand(t, store, 3011, "PS91189", db.ExternalPaymentCommandTypeCreateProfitSharing, "ps_wx_3011", wechatcontracts.ProfitSharingStatusFinished, 9929)
	expectProfitSharingCommandResponseFact(t, store, 3011, "PS91189", db.ExternalPaymentCommandTypeCreateProfitSharing, "ps_wx_3011", 1000, wechatcontracts.ProfitSharingStatusFinished, db.ExternalPaymentTerminalStatusUnknown, false, nil)

	processor := worker.NewTestTaskProcessor(store, distributor, nil, ecommerceClient)
	payloadBytes, err := json.Marshal(worker.ProfitSharingPayload{PaymentOrderID: paymentOrder.ID, OrderID: order.ID})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessProfitSharing, payloadBytes)
	err = processor.ProcessTaskProfitSharing(context.Background(), task)
	require.NoError(t, err)
	require.Empty(t, distributor.applicationIDs)
}

func TestProcessTaskProfitSharing_ProcessingOrderQueriesAndFinishes(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &profitSharingFactApplicationEnqueueRecorder{}
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
		Return(&wechatcontracts.ProfitSharingQueryResponse{
			SubMchID:  "sub_mch_18",
			OrderID:   "wx_profit_sharing_3004",
			Status:    wechatcontracts.ProfitSharingStatusFinished,
			Receivers: []wechatcontracts.ProfitSharingReceiverResult{{Type: wechatcontracts.ReceiverTypeMerchant, Result: wechatcontracts.ProfitSharingResultSuccess, Amount: 100, DetailID: "detail_3004"}},
		}, nil)
	expectProfitSharingQueryFact(t, store, existingOrder.ID, existingOrder.OutOrderNo, "wx_profit_sharing_3004", 100, "SUCCESS", db.ExternalPaymentTerminalStatusSuccess, true, nil)

	processor := worker.NewTestTaskProcessor(store, distributor, nil, ecommerceClient)
	payloadBytes, err := json.Marshal(worker.ProfitSharingPayload{PaymentOrderID: paymentOrder.ID, OrderID: 80})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessProfitSharing, payloadBytes)
	err = processor.ProcessTaskProfitSharing(context.Background(), task)
	require.NoError(t, err)
	require.Equal(t, []int64{12304}, distributor.applicationIDs)
}

func TestProcessTaskProfitSharing_ProcessingOrderFactFailureDoesNotFinish(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{
		ID:            908,
		Amount:        10000,
		TransactionID: pgtype.Text{String: "wx_txn_908", Valid: true},
		OrderID:       pgtype.Int8{Int64: 84, Valid: true},
	}
	order := db.Order{ID: 84, MerchantID: 22, TotalAmount: 10000, DeliveryFee: 0, OrderType: "takeout"}
	merchant := db.Merchant{ID: 22, RegionID: 16, Name: "商户D"}
	existingOrder := db.ProfitSharingOrder{
		ID:         3008,
		MerchantID: merchant.ID,
		OutOrderNo: "PS90884",
		Status:     "processing",
	}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), int64(84)).Return(order, nil)
	store.EXPECT().GetMerchant(gomock.Any(), merchant.ID).Return(merchant, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Return(db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "sub_mch_22"}, nil)
	store.EXPECT().GetActiveProfitSharingConfig(gomock.Any(), gomock.Any()).Return(db.ProfitSharingConfig{PlatformRate: 0, OperatorRate: 10, RiderEnabled: false}, nil)
	store.EXPECT().GetActiveOperatorByRegion(gomock.Any(), merchant.RegionID).Return(db.Operator{}, db.ErrRecordNotFound)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(existingOrder, nil)
	ecommerceClient.EXPECT().QueryProfitSharing(gomock.Any(), "sub_mch_22", "wx_txn_908", existingOrder.OutOrderNo).
		Return(&wechatcontracts.ProfitSharingQueryResponse{
			SubMchID:  "sub_mch_22",
			OrderID:   "wx_profit_sharing_3008",
			Status:    wechatcontracts.ProfitSharingStatusFinished,
			Receivers: []wechatcontracts.ProfitSharingReceiverResult{{Type: wechatcontracts.ReceiverTypeMerchant, Result: wechatcontracts.ProfitSharingResultSuccess, Amount: 100, DetailID: "detail_3008"}},
		}, nil)
	expectProfitSharingQueryFact(t, store, existingOrder.ID, existingOrder.OutOrderNo, "wx_profit_sharing_3008", 100, "SUCCESS", db.ExternalPaymentTerminalStatusSuccess, true, errors.New("fact store unavailable"))

	processor := worker.NewTestTaskProcessor(store, distributor, nil, ecommerceClient)
	payloadBytes, err := json.Marshal(worker.ProfitSharingPayload{PaymentOrderID: paymentOrder.ID, OrderID: 84})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessProfitSharing, payloadBytes)
	err = processor.ProcessTaskProfitSharing(context.Background(), task)
	require.Error(t, err)
	require.Contains(t, err.Error(), "record profit sharing query fact")
}

func TestProcessTaskProfitSharing_ProcessingOrderQueryStillProcessingRecordsFactOnly(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{
		ID:            909,
		Amount:        10000,
		TransactionID: pgtype.Text{String: "wx_txn_909", Valid: true},
		OrderID:       pgtype.Int8{Int64: 85, Valid: true},
	}
	order := db.Order{ID: 85, MerchantID: 23, TotalAmount: 10000, DeliveryFee: 0, OrderType: "takeout"}
	merchant := db.Merchant{ID: 23, RegionID: 17, Name: "商户E"}
	existingOrder := db.ProfitSharingOrder{
		ID:             3009,
		MerchantID:     merchant.ID,
		OutOrderNo:     "PS90985",
		Status:         "processing",
		SharingOrderID: pgtype.Text{String: "wx_profit_sharing_3009", Valid: true},
	}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), int64(85)).Return(order, nil)
	store.EXPECT().GetMerchant(gomock.Any(), merchant.ID).Return(merchant, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Return(db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "sub_mch_23"}, nil)
	store.EXPECT().GetActiveProfitSharingConfig(gomock.Any(), gomock.Any()).Return(db.ProfitSharingConfig{PlatformRate: 0, OperatorRate: 10, RiderEnabled: false}, nil)
	store.EXPECT().GetActiveOperatorByRegion(gomock.Any(), merchant.RegionID).Return(db.Operator{}, db.ErrRecordNotFound)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(existingOrder, nil)
	ecommerceClient.EXPECT().QueryProfitSharing(gomock.Any(), "sub_mch_23", "wx_txn_909", existingOrder.OutOrderNo).
		Return(&wechatcontracts.ProfitSharingQueryResponse{
			SubMchID: "sub_mch_23",
			Status:   "PROCESSING",
		}, nil)
	expectProfitSharingQueryFact(t, store, existingOrder.ID, existingOrder.OutOrderNo, "wx_profit_sharing_3009", 0, "PROCESSING", db.ExternalPaymentTerminalStatusProcessing, false, nil)

	processor := worker.NewTestTaskProcessor(store, distributor, nil, ecommerceClient)
	payloadBytes, err := json.Marshal(worker.ProfitSharingPayload{PaymentOrderID: paymentOrder.ID, OrderID: 85})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessProfitSharing, payloadBytes)
	err = processor.ProcessTaskProfitSharing(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskProfitSharing_ProcessingOrderUnsupportedReceiverResultFallsBackToProcessingFactOnly(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{
		ID:            910,
		Amount:        10000,
		TransactionID: pgtype.Text{String: "wx_txn_910", Valid: true},
		OrderID:       pgtype.Int8{Int64: 86, Valid: true},
	}
	order := db.Order{ID: 86, MerchantID: 24, TotalAmount: 10000, DeliveryFee: 0, OrderType: "takeout"}
	merchant := db.Merchant{ID: 24, RegionID: 18, Name: "商户F"}
	existingOrder := db.ProfitSharingOrder{
		ID:             3010,
		MerchantID:     merchant.ID,
		OutOrderNo:     "PS91086",
		Status:         "processing",
		SharingOrderID: pgtype.Text{String: "wx_profit_sharing_3010", Valid: true},
	}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), int64(86)).Return(order, nil)
	store.EXPECT().GetMerchant(gomock.Any(), merchant.ID).Return(merchant, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Return(db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "sub_mch_24"}, nil)
	store.EXPECT().GetActiveProfitSharingConfig(gomock.Any(), gomock.Any()).Return(db.ProfitSharingConfig{PlatformRate: 0, OperatorRate: 10, RiderEnabled: false}, nil)
	store.EXPECT().GetActiveOperatorByRegion(gomock.Any(), merchant.RegionID).Return(db.Operator{}, db.ErrRecordNotFound)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(existingOrder, nil)
	ecommerceClient.EXPECT().QueryProfitSharing(gomock.Any(), "sub_mch_24", "wx_txn_910", existingOrder.OutOrderNo).
		Return(&wechatcontracts.ProfitSharingQueryResponse{
			SubMchID:  "sub_mch_24",
			OrderID:   "wx_profit_sharing_3010",
			Status:    wechatcontracts.ProfitSharingStatusFinished,
			Receivers: []wechatcontracts.ProfitSharingReceiverResult{{Type: wechatcontracts.ReceiverTypeMerchant, Result: "UNSUPPORTED_RESULT", Amount: 100, DetailID: "detail_3010"}},
		}, nil)
	expectProfitSharingQueryFact(t, store, existingOrder.ID, existingOrder.OutOrderNo, "wx_profit_sharing_3010", 100, "PROCESSING", db.ExternalPaymentTerminalStatusProcessing, false, nil)

	processor := worker.NewTestTaskProcessor(store, distributor, nil, ecommerceClient)
	payloadBytes, err := json.Marshal(worker.ProfitSharingPayload{PaymentOrderID: paymentOrder.ID, OrderID: 86})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessProfitSharing, payloadBytes)
	err = processor.ProcessTaskProfitSharing(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskProfitSharing_FinishOrderRecordsAcceptedCommand(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{
		ID:            907,
		Amount:        10000,
		TransactionID: pgtype.Text{String: "wx_txn_907", Valid: true},
		OrderID:       pgtype.Int8{Int64: 83, Valid: true},
	}
	order := db.Order{ID: 83, MerchantID: 21, TotalAmount: 10000, DeliveryFee: 1000, OrderType: "takeout"}
	merchant := db.Merchant{ID: 21, RegionID: 18, Name: "待解冻商户"}
	existingOrder := db.ProfitSharingOrder{
		ID:             3007,
		MerchantID:     merchant.ID,
		OutOrderNo:     "PS90783",
		Status:         "pending",
		RiderID:        pgtype.Int8{Int64: 701, Valid: true},
		RiderAmount:    1000,
		MerchantAmount: 9000,
	}
	rider := db.Rider{ID: 701, UserID: 1701, RealName: "骑手待补实名"}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetMerchant(gomock.Any(), merchant.ID).Return(merchant, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Return(db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "sub_mch_21"}, nil)
	store.EXPECT().GetActiveProfitSharingConfig(gomock.Any(), db.GetActiveProfitSharingConfigParams{
		OrderSource: order.OrderType,
		MerchantID:  pgtype.Int8{Int64: merchant.ID, Valid: true},
		RegionID:    pgtype.Int8{Int64: merchant.RegionID, Valid: true},
	}).Return(db.ProfitSharingConfig{PlatformRate: 0, OperatorRate: 0, RiderEnabled: false}, nil)
	store.EXPECT().GetActiveOperatorByRegion(gomock.Any(), merchant.RegionID).Return(db.Operator{}, db.ErrRecordNotFound)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(existingOrder, nil)
	store.EXPECT().GetRider(gomock.Any(), existingOrder.RiderID.Int64).Return(rider, nil)
	store.EXPECT().GetUser(gomock.Any(), rider.UserID).Return(db.User{ID: rider.UserID}, nil)
	ecommerceClient.EXPECT().FinishProfitSharing(gomock.Any(), "sub_mch_21", paymentOrder.TransactionID.String, existingOrder.OutOrderNo, "无可用分账接收方，解冻剩余资金").Return(&wechatcontracts.ProfitSharingResponse{OrderID: "ps_finish_3007", Status: wechatcontracts.ProfitSharingStatusProcessing}, nil)
	store.EXPECT().UpdateProfitSharingOrderToProcessing(gomock.Any(), db.UpdateProfitSharingOrderToProcessingParams{
		ID:             existingOrder.ID,
		SharingOrderID: pgtype.Text{String: "ps_finish_3007", Valid: true},
	}).Return(db.ProfitSharingOrder{ID: existingOrder.ID, Status: "processing"}, nil)
	expectProfitSharingCommand(t, store, existingOrder.ID, existingOrder.OutOrderNo, db.ExternalPaymentCommandTypeFinishProfitSharing, "ps_finish_3007", wechatcontracts.ProfitSharingStatusProcessing, 9927)

	processor := worker.NewTestTaskProcessor(store, nil, nil, ecommerceClient)
	payloadBytes, err := json.Marshal(worker.ProfitSharingPayload{PaymentOrderID: paymentOrder.ID, OrderID: order.ID})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessProfitSharing, payloadBytes)
	err = processor.ProcessTaskProfitSharing(context.Background(), task)
	require.NoError(t, err)
}

func TestProcessTaskProfitSharing_FinishOrderFinishedResponseRecordsCommandFactOnly(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &profitSharingFactApplicationEnqueueRecorder{}
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	paymentOrder := db.PaymentOrder{
		ID:            912,
		Amount:        10000,
		TransactionID: pgtype.Text{String: "wx_txn_912", Valid: true},
	}
	order := db.Order{ID: 84, MerchantID: 26, TotalAmount: 10000, DeliveryFee: 1000, OrderType: "takeout"}
	merchant := db.Merchant{ID: 26, RegionID: 20, Name: "finish同步完成商户"}
	existingOrder := db.ProfitSharingOrder{
		ID:             3012,
		MerchantID:     merchant.ID,
		OutOrderNo:     "PS91284",
		Status:         "pending",
		MerchantAmount: 10000,
	}

	store.EXPECT().GetPaymentOrder(gomock.Any(), paymentOrder.ID).Return(paymentOrder, nil)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().GetMerchant(gomock.Any(), merchant.ID).Return(merchant, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Return(db.MerchantPaymentConfig{MerchantID: merchant.ID, SubMchID: "sub_mch_26"}, nil)
	store.EXPECT().GetActiveProfitSharingConfig(gomock.Any(), db.GetActiveProfitSharingConfigParams{
		OrderSource: order.OrderType,
		MerchantID:  pgtype.Int8{Int64: merchant.ID, Valid: true},
		RegionID:    pgtype.Int8{Int64: merchant.RegionID, Valid: true},
	}).Return(db.ProfitSharingConfig{PlatformRate: 0, OperatorRate: 0, RiderEnabled: false}, nil)
	store.EXPECT().GetActiveOperatorByRegion(gomock.Any(), merchant.RegionID).Return(db.Operator{}, db.ErrRecordNotFound)
	store.EXPECT().GetProfitSharingOrderByPaymentOrder(gomock.Any(), paymentOrder.ID).Return(existingOrder, nil)
	ecommerceClient.EXPECT().FinishProfitSharing(gomock.Any(), "sub_mch_26", paymentOrder.TransactionID.String, existingOrder.OutOrderNo, "无需分账，解冻剩余资金").Return(&wechatcontracts.ProfitSharingResponse{OrderID: "ps_finish_3012", Status: wechatcontracts.ProfitSharingStatusFinished}, nil)
	store.EXPECT().UpdateProfitSharingOrderToProcessing(gomock.Any(), db.UpdateProfitSharingOrderToProcessingParams{
		ID:             existingOrder.ID,
		SharingOrderID: pgtype.Text{String: "ps_finish_3012", Valid: true},
	}).Return(db.ProfitSharingOrder{ID: existingOrder.ID, Status: "processing"}, nil)
	expectProfitSharingCommand(t, store, existingOrder.ID, existingOrder.OutOrderNo, db.ExternalPaymentCommandTypeFinishProfitSharing, "ps_finish_3012", wechatcontracts.ProfitSharingStatusFinished, 9930)
	expectProfitSharingCommandResponseFact(t, store, existingOrder.ID, existingOrder.OutOrderNo, db.ExternalPaymentCommandTypeFinishProfitSharing, "ps_finish_3012", 0, wechatcontracts.ProfitSharingStatusFinished, db.ExternalPaymentTerminalStatusUnknown, false, nil)

	processor := worker.NewTestTaskProcessor(store, distributor, nil, ecommerceClient)
	payloadBytes, err := json.Marshal(worker.ProfitSharingPayload{PaymentOrderID: paymentOrder.ID, OrderID: order.ID})
	require.NoError(t, err)

	task := asynq.NewTask(worker.TaskProcessProfitSharing, payloadBytes)
	err = processor.ProcessTaskProfitSharing(context.Background(), task)
	require.NoError(t, err)
	require.Empty(t, distributor.applicationIDs)
}

func expectProfitSharingCommand(t *testing.T, store *mockdb.MockStore, profitSharingOrderID int64, outOrderNo string, commandType string, sharingOrderID string, status string, commandID int64) {
	t.Helper()

	store.EXPECT().CreateExternalPaymentCommand(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentCommandParams) (db.ExternalPaymentCommand, error) {
		require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
		require.Equal(t, db.PaymentChannelEcommerce, arg.Channel)
		require.Equal(t, db.ExternalPaymentCapabilityProfitSharing, arg.Capability)
		require.Equal(t, commandType, arg.CommandType)
		require.Equal(t, db.ExternalPaymentBusinessOwnerProfitSharing, arg.BusinessOwner)
		require.True(t, arg.BusinessObjectType.Valid)
		require.Equal(t, "profit_sharing_order", arg.BusinessObjectType.String)
		require.True(t, arg.BusinessObjectID.Valid)
		require.Equal(t, profitSharingOrderID, arg.BusinessObjectID.Int64)
		require.Equal(t, db.ExternalPaymentObjectProfitSharing, arg.ExternalObjectType)
		require.Equal(t, outOrderNo, arg.ExternalObjectKey)
		require.Equal(t, db.ExternalPaymentCommandStatusAccepted, arg.CommandStatus)
		require.True(t, arg.ExternalSecondaryKey.Valid)
		require.Equal(t, sharingOrderID, arg.ExternalSecondaryKey.String)
		require.False(t, arg.LastErrorCode.Valid)
		require.False(t, arg.LastErrorMessage.Valid)
		snapshot := string(arg.ResponseSnapshot)
		require.Contains(t, snapshot, outOrderNo)
		require.Contains(t, snapshot, sharingOrderID)
		if status != "" {
			require.Contains(t, snapshot, status)
		}
		require.NotContains(t, snapshot, "ReceiverAccount")
		require.NotContains(t, snapshot, "receiver_account")
		require.NotContains(t, snapshot, "ReceiverName")
		require.NotContains(t, snapshot, "receiver_name")
		require.NotContains(t, snapshot, "encrypted")
		return db.ExternalPaymentCommand{ID: commandID}, nil
	})
}

func expectProfitSharingQueryFact(t *testing.T, store *mockdb.MockStore, profitSharingOrderID int64, outOrderNo string, sharingOrderID string, amount int64, upstreamState string, terminalStatus string, isTerminal bool, factErr error) {
	t.Helper()

	factID := int64(8300) + profitSharingOrderID
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
		require.Equal(t, db.PaymentChannelEcommerce, arg.Channel)
		require.Equal(t, db.ExternalPaymentCapabilityProfitSharing, arg.Capability)
		require.Equal(t, db.ExternalPaymentFactSourceQuery, arg.FactSource)
		require.Equal(t, db.ExternalPaymentObjectProfitSharing, arg.ExternalObjectType)
		require.Equal(t, outOrderNo, arg.ExternalObjectKey)
		require.True(t, arg.ExternalSecondaryKey.Valid)
		require.Equal(t, sharingOrderID, arg.ExternalSecondaryKey.String)
		require.Equal(t, db.ExternalPaymentBusinessOwnerProfitSharing, arg.BusinessOwner.String)
		require.Equal(t, "profit_sharing_order", arg.BusinessObjectType.String)
		require.Equal(t, profitSharingOrderID, arg.BusinessObjectID.Int64)
		require.Equal(t, upstreamState, arg.UpstreamState)
		require.Equal(t, terminalStatus, arg.TerminalStatus)
		if amount > 0 {
			require.True(t, arg.Amount.Valid)
			require.Equal(t, amount, arg.Amount.Int64)
		} else {
			require.False(t, arg.Amount.Valid)
		}
		require.Equal(t, "CNY", arg.Currency)
		require.Equal(t, "wechat:query:ecommerce:profit_sharing:"+outOrderNo+":"+terminalStatus, arg.DedupeKey)
		raw := string(arg.RawResource)
		require.Contains(t, raw, sharingOrderID)
		require.Contains(t, raw, "receiver_results")
		if amount > 0 {
			require.Contains(t, raw, "detail_3")
		}
		require.NotContains(t, raw, "receiver_account")
		require.NotContains(t, raw, "ReceiverAccount")
		require.NotContains(t, raw, "receiver_name")
		require.NotContains(t, raw, "ReceiverName")
		if factErr != nil {
			return db.ExternalPaymentFact{}, factErr
		}
		return db.ExternalPaymentFact{ID: factID, DedupeKey: arg.DedupeKey, IsTerminal: isTerminal}, nil
	})

	if factErr != nil || !isTerminal {
		return
	}

	store.EXPECT().CreateExternalPaymentFactApplication(gomock.Any(), db.CreateExternalPaymentFactApplicationParams{
		FactID:             factID,
		Consumer:           "profit_sharing_domain",
		BusinessObjectType: "profit_sharing_order",
		BusinessObjectID:   profitSharingOrderID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}).Return(db.ExternalPaymentFactApplication{
		ID:                 factID + 1000,
		FactID:             factID,
		Consumer:           "profit_sharing_domain",
		BusinessObjectType: "profit_sharing_order",
		BusinessObjectID:   profitSharingOrderID,
		Status:             db.ExternalPaymentFactApplicationStatusPending,
	}, nil)
}

func expectProfitSharingCommandResponseFact(t *testing.T, store *mockdb.MockStore, profitSharingOrderID int64, outOrderNo string, commandType string, sharingOrderID string, amount int64, upstreamState string, terminalStatus string, isTerminal bool, factErr error) {
	t.Helper()

	factID := int64(8300) + profitSharingOrderID
	store.EXPECT().CreateExternalPaymentFact(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, arg db.CreateExternalPaymentFactParams) (db.ExternalPaymentFact, error) {
		require.Equal(t, db.ExternalPaymentProviderWechat, arg.Provider)
		require.Equal(t, db.PaymentChannelEcommerce, arg.Channel)
		require.Equal(t, db.ExternalPaymentCapabilityProfitSharing, arg.Capability)
		require.Equal(t, db.ExternalPaymentFactSourceCommandResponse, arg.FactSource)
		require.Equal(t, db.ExternalPaymentObjectProfitSharing, arg.ExternalObjectType)
		require.Equal(t, outOrderNo, arg.ExternalObjectKey)
		require.True(t, arg.ExternalSecondaryKey.Valid)
		require.Equal(t, sharingOrderID, arg.ExternalSecondaryKey.String)
		require.Equal(t, db.ExternalPaymentBusinessOwnerProfitSharing, arg.BusinessOwner.String)
		require.Equal(t, "profit_sharing_order", arg.BusinessObjectType.String)
		require.Equal(t, profitSharingOrderID, arg.BusinessObjectID.Int64)
		require.Equal(t, upstreamState, arg.UpstreamState)
		require.Equal(t, terminalStatus, arg.TerminalStatus)
		if amount > 0 {
			require.True(t, arg.Amount.Valid)
			require.Equal(t, amount, arg.Amount.Int64)
		} else {
			require.False(t, arg.Amount.Valid)
		}
		require.Equal(t, "CNY", arg.Currency)
		require.Equal(t, "wechat:command_response:ecommerce:profit_sharing:"+commandType+":"+outOrderNo+":"+upstreamState, arg.DedupeKey)
		raw := string(arg.RawResource)
		require.Contains(t, raw, sharingOrderID)
		require.Contains(t, raw, commandType)
		require.Contains(t, raw, upstreamState)
		if factErr != nil {
			return db.ExternalPaymentFact{}, factErr
		}
		return db.ExternalPaymentFact{ID: factID, DedupeKey: arg.DedupeKey, IsTerminal: isTerminal}, nil
	})

}
