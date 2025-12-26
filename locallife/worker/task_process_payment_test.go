package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
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
			name: "骑手进件成功_添加分账接收方并发送通知",
			payload: worker.ApplymentResultPayload{
				ApplymentID:    2,
				OutRequestNo:   "APPLY_R_2_1234567890",
				ApplymentState: "APPLYMENT_STATE_FINISHED",
				SubMchID:       "9876543210",
				SubjectType:    "rider",
				SubjectID:      200,
			},
			buildStubs: func(store *mockdb.MockStore, ecommerceClient *mockwechat.MockEcommerceClientInterface, distributor *mockwk.MockTaskDistributor) {
				// 1. 添加分账接收方
				ecommerceClient.EXPECT().
					GetSpAppID().
					Return("wx1234567890")
				ecommerceClient.EXPECT().
					AddProfitSharingReceiver(gomock.Any(), gomock.Any()).
					Return(&wechat.AddReceiverResponse{}, nil)

				// 2. 获取骑手信息
				store.EXPECT().
					GetRider(gomock.Any(), int64(200)).
					Return(db.Rider{
						ID:     200,
						UserID: 2001,
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
					Return(db.Merchant{}, pgx.ErrNoRows)

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
			name: "骑手进件被驳回_发送通知",
			payload: worker.ApplymentResultPayload{
				ApplymentID:    6,
				OutRequestNo:   "APPLY_R_6_1234567890",
				ApplymentState: "APPLYMENT_STATE_REJECTED",
				SubjectType:    "rider",
				SubjectID:      200,
			},
			buildStubs: func(store *mockdb.MockStore, distributor *mockwk.MockTaskDistributor) {
				// 1. 获取进件记录
				store.EXPECT().
					GetEcommerceApplyment(gomock.Any(), int64(6)).
					Return(db.EcommerceApplyment{
						ID:           6,
						SubjectType:  "rider",
						SubjectID:    200,
						RejectReason: pgtype.Text{String: "身份证过期", Valid: true},
					}, nil)

				// 2. 获取骑手信息
				store.EXPECT().
					GetRider(gomock.Any(), int64(200)).
					Return(db.Rider{
						ID:     200,
						UserID: 2001,
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
			name: "骑手待签约_发送提醒通知",
			payload: worker.ApplymentResultPayload{
				ApplymentID:    9,
				OutRequestNo:   "APPLY_R_9_1234567890",
				ApplymentState: "APPLYMENT_STATE_TO_BE_SIGNED",
				SubjectType:    "rider",
				SubjectID:      200,
			},
			buildStubs: func(store *mockdb.MockStore, distributor *mockwk.MockTaskDistributor) {
				// 获取骑手信息
				store.EXPECT().
					GetRider(gomock.Any(), int64(200)).
					Return(db.Rider{
						ID:     200,
						UserID: 2001,
					}, nil)

				// 发送通知
				distributor.EXPECT().
					DistributeTaskSendNotification(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, payload *worker.SendNotificationPayload, opts ...asynq.Option) error {
						require.Contains(t, payload.Content, "需要签约")
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

func TestProcessTaskProfitSharingResult_MerchantNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	// 商户不存在
	store.EXPECT().
		GetMerchant(gomock.Any(), int64(999)).
		Return(db.Merchant{}, pgx.ErrNoRows)

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
