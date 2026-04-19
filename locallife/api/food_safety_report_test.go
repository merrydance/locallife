package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type recordingTaskDistributor struct {
	worker.NoopTaskDistributor
	sendNotifications []*worker.SendNotificationPayload
	reservationAlerts []*worker.PayloadReservationFoodSafetyAlert
}

func (d *recordingTaskDistributor) DistributeTaskSendNotification(_ context.Context, payload *worker.SendNotificationPayload, _ ...asynq.Option) error {
	clone := *payload
	d.sendNotifications = append(d.sendNotifications, &clone)
	return nil
}

func (d *recordingTaskDistributor) DistributeTaskReservationFoodSafetyAlert(_ context.Context, payload *worker.PayloadReservationFoodSafetyAlert, _ ...asynq.Option) error {
	clone := *payload
	d.reservationAlerts = append(d.reservationAlerts, &clone)
	return nil
}

func TestReportFoodSafety_UsesAuthenticatedUserAndWritesValidSnapshots(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	order := randomOrder(user.ID, merchant.ID)
	order.Status = OrderStatusCompleted
	order.AddressID = pgtype.Int8{Int64: 42, Valid: true}

	items := []db.ListOrderItemsWithDishByOrderRow{
		{
			ID:        1,
			OrderID:   order.ID,
			DishID:    pgtype.Int8{Int64: 301, Valid: true},
			Name:      "酸辣粉",
			Quantity:  1,
			Subtotal:  1800,
			CreatedAt: time.Now(),
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().ListOrderItemsWithDishByOrder(gomock.Any(), order.ID).Return(items, nil)
	store.EXPECT().GetMerchantRecentFoodSafetyReports(gomock.Any(), merchant.ID).Return([]db.GetMerchantRecentFoodSafetyReportsRow{}, nil)
	store.EXPECT().ReportFoodSafetyIncidentTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, arg db.ReportFoodSafetyIncidentTxParams) (db.ReportFoodSafetyIncidentTxResult, error) {
			require.Equal(t, user.ID, arg.CreateFoodSafetyIncidentParams.UserID)
			require.Equal(t, merchant.ID, arg.CreateFoodSafetyIncidentParams.MerchantID)
			require.Equal(t, order.ID, arg.CreateFoodSafetyIncidentParams.OrderID)
			require.Equal(t, "reported", arg.CreateFoodSafetyIncidentParams.Status)
			require.Equal(t, "dish:301", arg.ProductKey)
			require.Equal(t, "酸辣粉", arg.ProductLabel)

			var orderSnapshot map[string]any
			require.NoError(t, json.Unmarshal(arg.CreateFoodSafetyIncidentParams.OrderSnapshot, &orderSnapshot))
			require.EqualValues(t, order.ID, orderSnapshot["order_id"])
			require.EqualValues(t, user.ID, orderSnapshot["reporter_user_id"])
			require.EqualValues(t, merchant.ID, orderSnapshot["merchant_id"])

			itemsPayload, ok := orderSnapshot["items"].([]any)
			require.True(t, ok)
			require.Len(t, itemsPayload, 1)

			var merchantSnapshot map[string]any
			require.NoError(t, json.Unmarshal(arg.CreateFoodSafetyIncidentParams.MerchantSnapshot, &merchantSnapshot))
			require.EqualValues(t, merchant.ID, merchantSnapshot["merchant_id"])

			var riderSnapshot map[string]any
			require.NoError(t, json.Unmarshal(arg.CreateFoodSafetyIncidentParams.RiderSnapshot, &riderSnapshot))
			require.Empty(t, riderSnapshot)

			return db.ReportFoodSafetyIncidentTxResult{
				Incident: db.FoodSafetyIncident{ID: 81, MerchantID: merchant.ID, OrderID: order.ID, UserID: user.ID, Status: arg.CreateFoodSafetyIncidentParams.Status},
			}, nil
		},
	)

	server := newTestServer(t, store)

	body, err := json.Marshal(map[string]any{
		"merchant_id":    merchant.ID,
		"order_id":       order.ID,
		"incident_type":  "contamination",
		"description":    "午饭后出现明显腹泻症状，需要平台介入排查",
		"severity_level": 3,
	})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/food-safety/report", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp ReportFoodSafetyResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, int64(81), resp.IncidentID)
	require.False(t, resp.MerchantSuspended)
	require.Equal(t, "食安举报未达到熔断阈值，仅记录", resp.Message)
}

func TestReportFoodSafety_NewCaseDispatchesFollowUps(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	order := randomOrder(user.ID, merchant.ID)
	order.Status = OrderStatusCompleted
	order.AddressID = pgtype.Int8{Int64: 1001, Valid: true}

	riderUser, _ := randomUser(t)
	rider := randomRider(riderUser.ID)
	delivery := randomDelivery(order.ID, rider.ID)

	reservationAt := time.Now().Add(5 * time.Hour).Round(time.Minute)
	reservation := db.TableReservation{
		ID:              301,
		MerchantID:      merchant.ID,
		UserID:          user.ID,
		Status:          "confirmed",
		ReservationDate: pgtype.Date{Time: reservationAt, Valid: true},
		ReservationTime: pgtype.Time{Microseconds: int64(reservationAt.Hour()*3600+reservationAt.Minute()*60) * 1000000, Valid: true},
	}

	priorUserA, _ := randomUser(t)
	priorUserB, _ := randomUser(t)
	priorOrderA := randomOrder(priorUserA.ID, merchant.ID)
	priorOrderA.ID = 2001
	priorOrderA.AddressID = pgtype.Int8{Int64: 2001, Valid: true}
	priorOrderB := randomOrder(priorUserB.ID, merchant.ID)
	priorOrderB.ID = 2002
	priorOrderB.AddressID = pgtype.Int8{Int64: 2002, Valid: true}

	items := []db.ListOrderItemsWithDishByOrderRow{{
		ID:        1,
		OrderID:   order.ID,
		DishID:    pgtype.Int8{Int64: 301, Valid: true},
		Name:      "酸辣粉",
		Quantity:  1,
		Subtotal:  1800,
		CreatedAt: time.Now(),
	}}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().ListOrderItemsWithDishByOrder(gomock.Any(), order.ID).Return(items, nil)
	store.EXPECT().GetMerchantRecentFoodSafetyReports(gomock.Any(), merchant.ID).Return([]db.GetMerchantRecentFoodSafetyReportsRow{
		{ID: 11, OrderID: priorOrderA.ID, UserID: priorUserA.ID},
		{ID: 12, OrderID: priorOrderB.ID, UserID: priorUserB.ID},
	}, nil)
	store.EXPECT().CountUserOrders(gomock.Any(), user.ID).Return(int32(2), nil)
	store.EXPECT().GetDevicesByUserID(gomock.Any(), user.ID).Return(nil, nil)
	store.EXPECT().GetDevicesByUserID(gomock.Any(), priorUserA.ID).Return(nil, nil)
	store.EXPECT().GetDevicesByUserID(gomock.Any(), priorUserB.ID).Return(nil, nil)
	store.EXPECT().GetOrder(gomock.Any(), priorOrderA.ID).Return(priorOrderA, nil)
	store.EXPECT().GetOrder(gomock.Any(), priorOrderB.ID).Return(priorOrderB, nil)
	store.EXPECT().ReportFoodSafetyIncidentTx(gomock.Any(), gomock.Any()).Return(db.ReportFoodSafetyIncidentTxResult{
		Incident:      db.FoodSafetyIncident{ID: 81, MerchantID: merchant.ID, OrderID: order.ID, UserID: user.ID, Status: "merchant-suspended"},
		Case:          &db.FoodSafetyCase{ID: 501, MerchantID: merchant.ID, Status: "open", TriggerReason: "threshold"},
		OpenedNewCase: true,
		AffectedReservations: []db.TableReservation{
			reservation,
		},
		AffectedTakeoutOrders: []db.Order{
			order,
		},
	}, nil)
	store.EXPECT().GetMerchant(gomock.Any(), merchant.ID).Return(merchant, nil)
	store.EXPECT().GetDeliveryByOrderID(gomock.Any(), order.ID).Return(delivery, nil)
	store.EXPECT().GetRider(gomock.Any(), rider.ID).Return(rider, nil)

	distributor := &recordingTaskDistributor{}
	server := newTestServerWithTaskDistributor(t, store, distributor)

	body, err := json.Marshal(map[string]any{
		"merchant_id":    merchant.ID,
		"order_id":       order.ID,
		"incident_type":  "contamination",
		"description":    "同商户多名顾客出现明显不适，需要立即停业核查",
		"severity_level": 3,
	})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/food-safety/report", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp ReportFoodSafetyResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, int64(81), resp.IncidentID)
	require.True(t, resp.MerchantSuspended)
	require.NotNil(t, resp.SuspendDuration)
	require.Equal(t, 48, *resp.SuspendDuration)
	require.Len(t, distributor.reservationAlerts, 1)
	require.Equal(t, reservation.ID, distributor.reservationAlerts[0].ReservationID)
	require.Len(t, distributor.sendNotifications, 3)

	notificationUserIDs := map[int64]struct{}{}
	for _, notification := range distributor.sendNotifications {
		notificationUserIDs[notification.UserID] = struct{}{}
	}
	_, ok := notificationUserIDs[merchant.OwnerUserID]
	require.True(t, ok)
	_, ok = notificationUserIDs[order.UserID]
	require.True(t, ok)
	_, ok = notificationUserIDs[rider.UserID]
	require.True(t, ok)
	for _, notification := range distributor.sendNotifications {
		if notification.RelatedType == "order" {
			require.Contains(t, notification.Content, "退款需由您或商家主动发起")
		}
	}
}

func TestReportFoodSafety_ReusedIncidentDoesNotReturnSuspendDuration(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	order := randomOrder(user.ID, merchant.ID)
	order.Status = OrderStatusCompleted
	order.AddressID = pgtype.Int8{Int64: 42, Valid: true}

	items := []db.ListOrderItemsWithDishByOrderRow{{
		ID:        1,
		OrderID:   order.ID,
		DishID:    pgtype.Int8{Int64: 301, Valid: true},
		Name:      "酸辣粉",
		Quantity:  1,
		Subtotal:  1800,
		CreatedAt: time.Now(),
	}}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)
	store.EXPECT().ListOrderItemsWithDishByOrder(gomock.Any(), order.ID).Return(items, nil)
	store.EXPECT().GetMerchantRecentFoodSafetyReports(gomock.Any(), merchant.ID).Return([]db.GetMerchantRecentFoodSafetyReportsRow{}, nil)
	store.EXPECT().ReportFoodSafetyIncidentTx(gomock.Any(), gomock.Any()).Return(db.ReportFoodSafetyIncidentTxResult{
		Incident:               db.FoodSafetyIncident{ID: 82, MerchantID: merchant.ID, OrderID: order.ID, UserID: user.ID, Status: "reported"},
		Case:                   &db.FoodSafetyCase{ID: 601, MerchantID: merchant.ID, Status: "open"},
		ReusedExistingIncident: true,
	}, nil)

	server := newTestServer(t, store)

	body, err := json.Marshal(map[string]any{
		"merchant_id":    merchant.ID,
		"order_id":       order.ID,
		"incident_type":  "contamination",
		"description":    "重复提交同一订单食安上报",
		"severity_level": 2,
	})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/food-safety/report", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)

	var resp ReportFoodSafetyResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.True(t, resp.MerchantSuspended)
	require.Nil(t, resp.SuspendDuration)
	require.Equal(t, "当前订单已有有效食安上报，已复用现有记录", resp.Message)
}

func TestReportFoodSafety_RejectsOrderNotOwned(t *testing.T) {
	user, _ := randomUser(t)
	otherUser, _ := randomUser(t)
	merchant := randomMerchant(otherUser.ID)
	order := randomOrder(otherUser.ID, merchant.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)

	server := newTestServer(t, store)

	body, err := json.Marshal(map[string]any{
		"merchant_id":    merchant.ID,
		"order_id":       order.ID,
		"incident_type":  "expired",
		"description":    "用户未消费该订单，但尝试伪造食安上报",
		"severity_level": 2,
	})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/food-safety/report", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestReportFoodSafety_RejectsUnfulfilledOrder(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	order := randomOrder(user.ID, merchant.ID)
	order.OrderType = OrderTypeTakeout
	order.Status = OrderStatusPaid

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().GetOrder(gomock.Any(), order.ID).Return(order, nil)

	server := newTestServer(t, store)

	body, err := json.Marshal(map[string]any{
		"merchant_id":    merchant.ID,
		"order_id":       order.ID,
		"incident_type":  "expired",
		"description":    "订单尚未履约完成，不能参与食安熔断链路",
		"severity_level": 2,
	})
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodPost, "/v1/food-safety/report", bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "fulfilled order")
}
