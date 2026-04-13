package worker_test

import (
	"context"
	"testing"
	"time"

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

func TestApplymentSettlementVerificationSchedulerMarksVerifyingCandidate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().
		ListMerchantApplymentsPendingSettlementVerification(gomock.Any(), gomock.Any()).
		Return([]db.ListMerchantApplymentsPendingSettlementVerificationRow{{
			ID:                         11,
			SubjectID:                  21,
			SubMchID:                   pgtype.Text{String: "sub_mch_21", Valid: true},
			SettlementVerifyCheckCount: 0,
			FirstPaidAt:                time.Now().Add(-2 * time.Hour),
		}}, nil)

	ecommerceClient.EXPECT().
		QuerySubMerchantSettlement(gomock.Any(), "sub_mch_21", "").
		Return(&wechat.SubMerchantSettlementResponse{VerifyResult: "VERIFYING"}, nil)

	store.EXPECT().
		UpdateEcommerceApplymentSettlementVerification(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateEcommerceApplymentSettlementVerificationParams{})).
		DoAndReturn(func(_ context.Context, arg db.UpdateEcommerceApplymentSettlementVerificationParams) (db.EcommerceApplyment, error) {
			require.Equal(t, int64(11), arg.ID)
			require.True(t, arg.SettlementVerifyFirstTradeAt.Valid)
			require.True(t, arg.SettlementVerifyLastCheckedAt.Valid)
			require.Equal(t, int32(1), arg.SettlementVerifyCheckCount.Int32)
			require.Equal(t, "verifying", arg.SettlementVerifyStatus.String)
			return db.EcommerceApplyment{ID: 11}, nil
		})

	scheduler := worker.NewApplymentSettlementVerificationScheduler(store, distributor, ecommerceClient)
	scheduler.RunOnce()
}

func TestApplymentSettlementVerificationSchedulerNotifiesOperatorOnFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := mockwk.NewMockTaskDistributor(ctrl)
	ecommerceClient := mockwechat.NewMockEcommerceClientInterface(ctrl)

	store.EXPECT().
		ListMerchantApplymentsPendingSettlementVerification(gomock.Any(), gomock.Any()).
		Return([]db.ListMerchantApplymentsPendingSettlementVerificationRow{{
			ID:                         31,
			SubjectID:                  41,
			SubMchID:                   pgtype.Text{String: "sub_mch_41", Valid: true},
			SettlementVerifyCheckCount: 1,
			FirstPaidAt:                time.Now().Add(-24 * time.Hour),
		}}, nil)

	ecommerceClient.EXPECT().
		QuerySubMerchantSettlement(gomock.Any(), "sub_mch_41", "").
		Return(&wechat.SubMerchantSettlementResponse{
			VerifyResult:     "VERIFY_FAIL",
			VerifyFailReason: "银行卡户名不一致",
		}, nil)

	store.EXPECT().
		UpdateEcommerceApplymentSettlementVerification(gomock.Any(), gomock.AssignableToTypeOf(db.UpdateEcommerceApplymentSettlementVerificationParams{})).
		DoAndReturn(func(_ context.Context, arg db.UpdateEcommerceApplymentSettlementVerificationParams) (db.EcommerceApplyment, error) {
			require.Equal(t, "fail", arg.SettlementVerifyStatus.String)
			require.Equal(t, "银行卡户名不一致", arg.SettlementVerifyFailReason.String)
			return db.EcommerceApplyment{ID: 31}, nil
		})

	store.EXPECT().
		GetMerchant(gomock.Any(), int64(41)).
		Return(db.Merchant{ID: 41, Name: "南山店", RegionID: 51}, nil)

	store.EXPECT().
		GetActiveOperatorByRegion(gomock.Any(), int64(51)).
		Return(db.Operator{ID: 61, UserID: 71, Name: "南山区运营商", Status: "active"}, nil)

	distributor.EXPECT().
		DistributeTaskSendNotification(gomock.Any(), gomock.AssignableToTypeOf(&worker.SendNotificationPayload{}), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, payload *worker.SendNotificationPayload, _ ...asynq.Option) error {
			require.Equal(t, int64(71), payload.UserID)
			require.Equal(t, "system", payload.Type)
			require.Equal(t, "商户结算卡校验失败", payload.Title)
			require.Contains(t, payload.Content, "南山店")
			require.Contains(t, payload.Content, "银行卡户名不一致")
			require.Equal(t, "merchant", payload.RelatedType)
			require.Equal(t, int64(41), payload.RelatedID)
			require.True(t, payload.IgnorePreferences)
			return nil
		})

	store.EXPECT().
		MarkEcommerceApplymentSettlementVerifyFailedNotified(gomock.Any(), int64(31)).
		Return(db.EcommerceApplyment{ID: 31}, nil)

	scheduler := worker.NewApplymentSettlementVerificationScheduler(store, distributor, ecommerceClient)
	scheduler.RunOnce()
}
