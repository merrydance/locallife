package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/util"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func buildMerchantReservationDateRow(reservation db.TableReservation, table db.Table) db.ListReservationsByMerchantAndDateRow {
	return db.ListReservationsByMerchantAndDateRow{
		ID:              reservation.ID,
		TableID:         reservation.TableID,
		UserID:          reservation.UserID,
		MerchantID:      reservation.MerchantID,
		ReservationDate: reservation.ReservationDate,
		ReservationTime: reservation.ReservationTime,
		GuestCount:      reservation.GuestCount,
		ContactName:     reservation.ContactName,
		ContactPhone:    reservation.ContactPhone,
		PaymentMode:     reservation.PaymentMode,
		DepositAmount:   reservation.DepositAmount,
		PrepaidAmount:   reservation.PrepaidAmount,
		RefundDeadline:  reservation.RefundDeadline,
		Status:          reservation.Status,
		PaymentDeadline: reservation.PaymentDeadline,
		CreatedAt:       reservation.CreatedAt,
		TableNo:         table.TableNo,
		TableType:       table.TableType,
	}
}

type reservationListEnvelope struct {
	Code    int                     `json:"code"`
	Message string                  `json:"message"`
	Data    reservationListResponse `json:"data"`
}

type reservationWorkbenchEnvelope struct {
	Code    int                                  `json:"code"`
	Message string                               `json:"message"`
	Data    merchantReservationWorkbenchResponse `json:"data"`
}

func TestListMerchantReservationsDateScopedFilters(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	room := randomRoom(merchant.ID)
	reservationDate := time.Now().Add(24 * time.Hour).Truncate(24 * time.Hour)

	checkedInReservation := randomReservation(room.ID, user.ID+1, merchant.ID)
	checkedInReservation.Status = "checked_in"
	checkedInReservation.ReservationDate = pgtype.Date{Time: reservationDate, Valid: true}

	confirmedReservation := randomReservation(room.ID, user.ID+2, merchant.ID)
	confirmedReservation.Status = "confirmed"
	confirmedReservation.ReservationDate = pgtype.Date{Time: reservationDate, Valid: true}

	cancelledReservation := randomReservation(room.ID, user.ID+3, merchant.ID)
	cancelledReservation.Status = "cancelled"
	cancelledReservation.ReservationDate = pgtype.Date{Time: reservationDate, Valid: true}

	noShowReservation := randomReservation(room.ID, user.ID+4, merchant.ID)
	noShowReservation.Status = "no_show"
	noShowReservation.ReservationDate = pgtype.Date{Time: reservationDate, Valid: true}

	testCases := []struct {
		name          string
		query         string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "DateWithCheckedInStatus",
			query: "date=" + reservationDate.Format("2006-01-02") + "&status=checked_in&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
				store.EXPECT().
					ListReservationsByMerchantAndDate(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListReservationsByMerchantAndDateRow{
						buildMerchantReservationDateRow(checkedInReservation, room),
						buildMerchantReservationDateRow(confirmedReservation, room),
					}, nil)
				store.EXPECT().
					ListReservationItems(gomock.Any(), gomock.Eq(checkedInReservation.ID)).
					Times(1).
					Return([]db.ListReservationItemsRow{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp reservationListEnvelope
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Equal(t, 0, resp.Code)
				require.Equal(t, int64(1), resp.Data.Total)
				require.Len(t, resp.Data.Reservations, 1)
				require.Equal(t, "checked_in", resp.Data.Reservations[0].Status)
			},
		},
		{
			name:  "DateWithExceptionStatus",
			query: "date=" + reservationDate.Format("2006-01-02") + "&status=exception&page_id=1&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, user.ID, merchant)
				store.EXPECT().
					ListReservationsByMerchantAndDate(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListReservationsByMerchantAndDateRow{
						buildMerchantReservationDateRow(cancelledReservation, room),
						buildMerchantReservationDateRow(noShowReservation, room),
						buildMerchantReservationDateRow(confirmedReservation, room),
					}, nil)
				store.EXPECT().
					ListReservationItems(gomock.Any(), gomock.Eq(cancelledReservation.ID)).
					Times(1).
					Return([]db.ListReservationItemsRow{}, nil)
				store.EXPECT().
					ListReservationItems(gomock.Any(), gomock.Eq(noShowReservation.ID)).
					Times(1).
					Return([]db.ListReservationItemsRow{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var resp reservationListEnvelope
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Equal(t, 0, resp.Code)
				require.Equal(t, int64(2), resp.Data.Total)
				require.Len(t, resp.Data.Reservations, 2)
				require.Equal(t, "cancelled", resp.Data.Reservations[0].Status)
				require.Equal(t, "no_show", resp.Data.Reservations[1].Status)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			req, err := http.NewRequest(http.MethodGet, "/v1/reservations/merchant?"+tc.query, nil)
			require.NoError(t, err)
			addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

			server.router.ServeHTTP(recorder, req)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestListMerchantReservationsExceptionRequiresDate(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	req, err := http.NewRequest(http.MethodGet, "/v1/reservations/merchant?status=exception&page_id=1&page_size=10", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestGetMerchantReservationWorkbenchAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)

	tableA := randomRoom(merchant.ID)
	tableA.TableNo = "A01"
	tableB := randomRoom(merchant.ID)
	tableB.ID = tableA.ID + 1
	tableB.TableNo = "B02"
	tableC := randomRoom(merchant.ID)
	tableC.ID = tableB.ID + 1
	tableC.TableNo = "C03"

	reservationDate := time.Now().Add(24 * time.Hour).Truncate(24 * time.Hour)

	confirmedReservation := randomReservation(tableA.ID, user.ID+1, merchant.ID)
	confirmedReservation.Status = "confirmed"
	confirmedReservation.ReservationDate = pgtype.Date{Time: reservationDate, Valid: true}
	confirmedReservation.ReservationTime = pgtype.Time{Microseconds: int64(18*3600) * 1000000, Valid: true}
	confirmedReservation.ContactName = "张三"

	checkedInReservation := randomReservation(tableB.ID, user.ID+2, merchant.ID)
	checkedInReservation.Status = "checked_in"
	checkedInReservation.ReservationDate = pgtype.Date{Time: reservationDate, Valid: true}
	checkedInReservation.ReservationTime = pgtype.Time{Microseconds: int64(19*3600+30*60) * 1000000, Valid: true}
	checkedInReservation.ContactName = "李四"

	cancelledReservation := randomReservation(tableC.ID, user.ID+3, merchant.ID)
	cancelledReservation.Status = "cancelled"
	cancelledReservation.ReservationDate = pgtype.Date{Time: reservationDate, Valid: true}

	dishID := util.RandomInt(1, 999)
	comboID := util.RandomInt(1000, 1999)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().
		GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		ListReservationsByMerchantAndDate(gomock.Any(), gomock.Any()).
		Times(1).
		Return([]db.ListReservationsByMerchantAndDateRow{
			buildMerchantReservationDateRow(confirmedReservation, tableA),
			buildMerchantReservationDateRow(checkedInReservation, tableB),
			buildMerchantReservationDateRow(cancelledReservation, tableC),
		}, nil)

	store.EXPECT().
		ListReservationItems(gomock.Any(), gomock.Eq(confirmedReservation.ID)).
		Times(1).
		Return([]db.ListReservationItemsRow{
			{
				ID:            util.RandomInt(1, 999),
				ReservationID: confirmedReservation.ID,
				DishID:        pgtype.Int8{Int64: dishID, Valid: true},
				Quantity:      2,
				UnitPrice:     3800,
				TotalPrice:    7600,
				CreatedAt:     time.Now(),
				DishName:      pgtype.Text{String: "招牌鱼", Valid: true},
			},
		}, nil)
	store.EXPECT().
		ListReservationItems(gomock.Any(), gomock.Eq(checkedInReservation.ID)).
		Times(1).
		Return([]db.ListReservationItemsRow{
			{
				ID:            util.RandomInt(1, 999),
				ReservationID: checkedInReservation.ID,
				ComboID:       pgtype.Int8{Int64: comboID, Valid: true},
				Quantity:      1,
				UnitPrice:     9800,
				TotalPrice:    9800,
				CreatedAt:     time.Now(),
				ComboName:     pgtype.Text{String: "双人套餐", Valid: true},
			},
		}, nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	req, err := http.NewRequest(http.MethodGet, "/v1/reservations/merchant/workbench?date="+reservationDate.Format("2006-01-02"), nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp reservationWorkbenchEnvelope
	err = json.Unmarshal(recorder.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Equal(t, 0, resp.Code)
	require.Equal(t, reservationDate.Format("2006-01-02"), resp.Data.Date)
	require.Equal(t, int64(3), resp.Data.Summary.ReservationCount)
	require.Equal(t, int64(2), resp.Data.Summary.ActiveTableCount)
	require.Equal(t, int64(1), resp.Data.StatusTotals.Confirmed)
	require.Equal(t, int64(1), resp.Data.StatusTotals.CheckedIn)
	require.Equal(t, int64(1), resp.Data.StatusTotals.Cancelled)
	require.Equal(t, int64(1), resp.Data.StatusTotals.Exception)
	require.Equal(t, int64(2), resp.Data.PrepSummary.TableCount)
	require.Equal(t, int64(2), resp.Data.PrepSummary.DishKinds)
	require.Equal(t, int64(3), resp.Data.PrepSummary.TotalQuantity)
	require.Len(t, resp.Data.PrepSummary.Items, 2)
	require.Equal(t, "招牌鱼", resp.Data.PrepSummary.Items[0].Name)
	require.Equal(t, int64(2), resp.Data.PrepSummary.Items[0].TotalQuantity)
}

func TestGetMerchantReservationWorkbenchNotMerchant(t *testing.T) {
	user, _ := randomUser(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveNoAccessibleMerchants(store, user.ID)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	req, err := http.NewRequest(http.MethodGet, "/v1/reservations/merchant/workbench?date=2025-01-01", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusForbidden, recorder.Code)
}
