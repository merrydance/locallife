package worker

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type reservationAlertTestDistributor struct {
	NoopTaskDistributor
	notifications []*SendNotificationPayload
}

func (d *reservationAlertTestDistributor) DistributeTaskSendNotification(_ context.Context, payload *SendNotificationPayload, _ ...asynq.Option) error {
	clone := *payload
	d.notifications = append(d.notifications, &clone)
	return nil
}

func TestProcessTaskReservationFoodSafetyAlert_SendsNotificationWhenStillSuspended(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &reservationAlertTestDistributor{}
	processor := &RedisTaskProcessor{store: store, distributor: distributor}

	reservationAt := time.Now().Add(4 * time.Hour).Round(time.Minute)
	reservation := db.TableReservation{
		ID:              101,
		MerchantID:      201,
		UserID:          301,
		Status:          "confirmed",
		ReservationDate: pgtype.Date{Time: reservationAt, Valid: true},
		ReservationTime: pgtype.Time{Microseconds: int64(reservationAt.Hour()*3600+reservationAt.Minute()*60) * 1000000, Valid: true},
	}
	merchant := db.Merchant{ID: reservation.MerchantID, Name: "样例商户"}
	profile := db.GetMerchantProfileRow{
		MerchantID:    reservation.MerchantID,
		IsSuspended:   true,
		SuspendReason: pgtype.Text{String: "同商户1小时内多名顾客食安举报触发熔断（订单ID: 88）", Valid: true},
	}

	store.EXPECT().GetTableReservation(gomock.Any(), reservation.ID).Return(reservation, nil)
	store.EXPECT().GetMerchantProfile(gomock.Any(), reservation.MerchantID).Return(profile, nil)
	store.EXPECT().GetMerchant(gomock.Any(), reservation.MerchantID).Return(merchant, nil)

	payload, err := json.Marshal(&PayloadReservationFoodSafetyAlert{ReservationID: reservation.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskReservationFoodSafetyAlert(context.Background(), asynq.NewTask(TaskReservationFoodSafetyAlert, payload))
	require.NoError(t, err)
	require.Len(t, distributor.notifications, 1)
	require.Equal(t, reservation.UserID, distributor.notifications[0].UserID)
	require.Contains(t, distributor.notifications[0].Content, "平台不会代您自动退款")
}

func TestProcessTaskReservationFoodSafetyAlert_SkipsWhenMerchantRecovered(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	distributor := &reservationAlertTestDistributor{}
	processor := &RedisTaskProcessor{store: store, distributor: distributor}

	reservation := db.TableReservation{
		ID:         102,
		MerchantID: 202,
		UserID:     302,
		Status:     "confirmed",
	}

	store.EXPECT().GetTableReservation(gomock.Any(), reservation.ID).Return(reservation, nil)
	store.EXPECT().GetMerchantProfile(gomock.Any(), reservation.MerchantID).Return(db.GetMerchantProfileRow{
		MerchantID:    reservation.MerchantID,
		IsSuspended:   false,
		SuspendReason: pgtype.Text{},
	}, nil)

	payload, err := json.Marshal(&PayloadReservationFoodSafetyAlert{ReservationID: reservation.ID})
	require.NoError(t, err)

	err = processor.ProcessTaskReservationFoodSafetyAlert(context.Background(), asynq.NewTask(TaskReservationFoodSafetyAlert, payload))
	require.NoError(t, err)
	require.Empty(t, distributor.notifications)
}
