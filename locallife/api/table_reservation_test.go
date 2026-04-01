package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ==================== 测试数据生成 ====================

func randomReservation(tableID, userID, merchantID int64) db.TableReservation {
	now := time.Now()
	reservationDate := now.Add(24 * time.Hour)

	return db.TableReservation{
		ID:              util.RandomInt(1, 1000),
		TableID:         tableID,
		UserID:          userID,
		MerchantID:      merchantID,
		ReservationDate: pgtype.Date{Time: reservationDate, Valid: true},
		ReservationTime: pgtype.Time{Microseconds: int64(18*3600) * 1000000, Valid: true},
		GuestCount:      int16(util.RandomInt(2, 8)),
		ContactName:     util.RandomString(6),
		ContactPhone:    "13800138000",
		PaymentMode:     "deposit",
		DepositAmount:   10000,
		PrepaidAmount:   0,
		RefundDeadline:  reservationDate.Add(-2 * time.Hour),
		PaymentDeadline: now.Add(30 * time.Minute),
		Status:          "pending",
		CreatedAt:       now,
	}
}

// ==================== 创建预定测试 ====================

func TestCreateReservationAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID + 1) // 不同的用户拥有商户
	room := randomRoom(merchant.ID)
	reservation := randomReservation(room.ID, user.ID, merchant.ID)

	tomorrow := time.Now().Add(24 * time.Hour).Format("2006-01-02")

	testCases := []struct {
		name          string
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: gin.H{
				"table_id":      room.ID,
				"date":          tomorrow,
				"time":          "18:00",
				"guest_count":   4,
				"contact_name":  "张三",
				"contact_phone": "13800138000",
				"payment_mode":  "deposit",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(room.ID)).
					Times(1).
					Return(room, nil)

				store.EXPECT().
					ListReservationsByTableAndDate(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.TableReservation{}, nil)

				store.EXPECT().
					ListMerchantBusinessHours(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.MerchantBusinessHour{}, nil)

				store.EXPECT().
					CreateReservationTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateReservationTxResult{Reservation: reservation}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
			},
		},
		{
			name: "NoAuthorization",
			body: gin.H{
				"table_id":      room.ID,
				"date":          tomorrow,
				"time":          "18:00",
				"guest_count":   4,
				"contact_name":  "张三",
				"contact_phone": "13800138000",
				"payment_mode":  "deposit",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				// No auth
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTable(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "TableNotFound",
			body: gin.H{
				"table_id":      room.ID,
				"date":          tomorrow,
				"time":          "18:00",
				"guest_count":   4,
				"contact_name":  "张三",
				"contact_phone": "13800138000",
				"payment_mode":  "deposit",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(room.ID)).
					Times(1).
					Return(db.Table{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "OnlyRoomsCanBeReserved",
			body: gin.H{
				"table_id":      room.ID,
				"date":          tomorrow,
				"time":          "18:00",
				"guest_count":   4,
				"contact_name":  "张三",
				"contact_phone": "13800138000",
				"payment_mode":  "deposit",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				table := room
				table.TableType = "table" // 普通桌台

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(room.ID)).
					Times(1).
					Return(table, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "TableNotAvailable",
			body: gin.H{
				"table_id":      room.ID,
				"date":          tomorrow,
				"time":          "18:00",
				"guest_count":   4,
				"contact_name":  "张三",
				"contact_phone": "13800138000",
				"payment_mode":  "deposit",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				unavailableRoom := room
				unavailableRoom.Status = "disabled"

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(room.ID)).
					Times(1).
					Return(unavailableRoom, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
			},
		},
		{
			name: "TimeSlotAlreadyReserved",
			body: gin.H{
				"table_id":      room.ID,
				"date":          tomorrow,
				"time":          "18:00",
				"guest_count":   4,
				"contact_name":  "张三",
				"contact_phone": "13800138000",
				"payment_mode":  "deposit",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(room.ID)).
					Times(1).
					Return(room, nil)

				store.EXPECT().
					ListReservationsByTableAndDate(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.TableReservation{reservation}, nil)

				store.EXPECT().
					ListMerchantBusinessHours(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.MerchantBusinessHour{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
			},
		},
		{
			name: "GuestCountExceedsCapacity",
			body: gin.H{
				"table_id":      room.ID,
				"date":          tomorrow,
				"time":          "18:00",
				"guest_count":   30, // 超过包间容量(4-20)但不超过binding限制(50)
				"contact_name":  "张三",
				"contact_phone": "13800138000",
				"payment_mode":  "deposit",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				// 返回容量为10的包间
				smallRoom := room
				smallRoom.Capacity = 10
				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(room.ID)).
					Times(1).
					Return(smallRoom, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "InvalidPaymentMode",
			body: gin.H{
				"table_id":      room.ID,
				"date":          tomorrow,
				"time":          "18:00",
				"guest_count":   4,
				"contact_name":  "张三",
				"contact_phone": "13800138000",
				"payment_mode":  "invalid",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTable(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/reservations"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 获取预定详情测试 ====================

func TestGetReservationAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	room := randomRoom(merchant.ID)
	reservation := randomReservation(room.ID, user.ID, merchant.ID)

	testCases := []struct {
		name          string
		reservationID int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:          "OK",
			reservationID: reservation.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTableReservationWithTable(gomock.Any(), gomock.Eq(reservation.ID)).
					Times(1).
					Return(db.GetTableReservationWithTableRow{
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
						PaymentDeadline: reservation.PaymentDeadline,
						Status:          reservation.Status,
						CreatedAt:       reservation.CreatedAt,
						TableNo:         room.TableNo,
						TableType:       room.TableType,
						Capacity:        room.Capacity,
					}, nil)

				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Eq(reservation.MerchantID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					ListOrdersByUserWithFilters(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListOrdersByUserWithFiltersRow{}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:          "NotFound",
			reservationID: reservation.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTableReservationWithTable(gomock.Any(), gomock.Eq(reservation.ID)).
					Times(1).
					Return(db.GetTableReservationWithTableRow{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:          "InvalidID",
			reservationID: 0,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTableReservationWithTable(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:          "Forbidden",
			reservationID: reservation.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				otherUserID := user.ID + 999 // 既不是预定用户也不是商户
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, otherUserID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTableReservationWithTable(gomock.Any(), gomock.Eq(reservation.ID)).
					Times(1).
					Return(db.GetTableReservationWithTableRow{
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
						PaymentDeadline: reservation.PaymentDeadline,
						Status:          reservation.Status,
						CreatedAt:       reservation.CreatedAt,
						TableNo:         room.TableNo,
						TableType:       room.TableType,
						Capacity:        room.Capacity,
					}, nil)

				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Eq(reservation.MerchantID)).
					Times(1).
					Return(merchant, nil)

				// 检查是否为商户 - 不是商户
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Merchant{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/reservations/%d", tc.reservationID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 用户预定列表测试 ====================

func TestListUserReservationsAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID + 1)
	room := randomRoom(merchant.ID)

	n := 5
	reservations := make([]db.ListReservationsByUserWithStatusRow, n)
	for i := 0; i < n; i++ {
		r := randomReservation(room.ID, user.ID, merchant.ID)
		reservations[i] = db.ListReservationsByUserWithStatusRow{
			ID:              r.ID,
			TableID:         r.TableID,
			UserID:          r.UserID,
			MerchantID:      r.MerchantID,
			ReservationDate: r.ReservationDate,
			ReservationTime: r.ReservationTime,
			GuestCount:      r.GuestCount,
			ContactName:     r.ContactName,
			ContactPhone:    r.ContactPhone,
			PaymentMode:     r.PaymentMode,
			DepositAmount:   r.DepositAmount,
			PrepaidAmount:   r.PrepaidAmount,
			RefundDeadline:  r.RefundDeadline,
			PaymentDeadline: r.PaymentDeadline,
			Status:          r.Status,
			CreatedAt:       r.CreatedAt,
			TableNo:         room.TableNo,
			TableType:       room.TableType,
		}
	}

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListReservationsByUserWithStatus(gomock.Any(), gomock.Any()).
					Times(1).
					Return(reservations, nil)

				store.EXPECT().
					CountReservationsByUserAndStatus(gomock.Any(), gomock.Any()).
					Times(8).
					Return(int64(0), nil)

				store.EXPECT().
					GetMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(merchant, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "InvalidPageSize",
			query: "page_id=1&page_size=100", // 超过限制
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListReservationsByUserWithStatus(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := "/v1/reservations/me?" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 确认预定测试 ====================

func TestConfirmReservationAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	room := randomRoom(merchant.ID)

	paidReservation := randomReservation(room.ID, user.ID+1, merchant.ID)
	paidReservation.Status = "paid"

	confirmedReservation := paidReservation
	confirmedReservation.Status = "confirmed"

	testCases := []struct {
		name          string
		reservationID int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:          "OK",
			reservationID: paidReservation.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(2).
					Return(merchant, nil)

				store.EXPECT().
					GetTableReservationForUpdate(gomock.Any(), gomock.Eq(paidReservation.ID)).
					Times(1).
					Return(paidReservation, nil)

				store.EXPECT().
					GetMerchantProfile(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(db.GetMerchantProfileRow{MerchantID: merchant.ID, IsTakeoutSuspended: false}, nil)

				store.EXPECT().
					ConfirmReservationTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.ConfirmReservationTxResult{Reservation: confirmedReservation, Table: room}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:          "NotMerchant",
			reservationID: paidReservation.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.Merchant{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:          "ReservationNotPaid",
			reservationID: paidReservation.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(2).
					Return(merchant, nil)

				pendingReservation := paidReservation
				pendingReservation.Status = "pending"

				store.EXPECT().
					GetTableReservationForUpdate(gomock.Any(), gomock.Eq(paidReservation.ID)).
					Times(1).
					Return(pendingReservation, nil)

				store.EXPECT().
					GetMerchantProfile(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(db.GetMerchantProfileRow{MerchantID: merchant.ID, IsTakeoutSuspended: false}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
			},
		},
		{
			name:          "CashierAllowed",
			reservationID: paidReservation.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				staffMerchant := merchant
				staffMerchant.OwnerUserID = user.ID + 100

				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(2).
					Return(staffMerchant, nil)

				store.EXPECT().
					GetUserMerchantRole(gomock.Any(), gomock.Eq(db.GetUserMerchantRoleParams{
						MerchantID: merchant.ID,
						UserID:     user.ID,
					})).
					Times(1).
					Return("cashier", nil)

				store.EXPECT().
					GetTableReservationForUpdate(gomock.Any(), gomock.Eq(paidReservation.ID)).
					Times(1).
					Return(paidReservation, nil)

				store.EXPECT().
					GetMerchantProfile(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(db.GetMerchantProfileRow{MerchantID: merchant.ID, IsTakeoutSuspended: false}, nil)

				store.EXPECT().
					ConfirmReservationTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.ConfirmReservationTxResult{Reservation: confirmedReservation, Table: room}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/reservations/%d/confirm", tc.reservationID)
			request, err := http.NewRequest(http.MethodPost, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 取消预定测试 ====================

func TestCancelReservationAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID + 1)
	room := randomRoom(merchant.ID)
	reservation := randomReservation(room.ID, user.ID, merchant.ID)

	cancelledReservation := reservation
	cancelledReservation.Status = "cancelled"

	testCases := []struct {
		name          string
		reservationID int64
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:          "OK_ByUser",
			reservationID: reservation.ID,
			body: gin.H{
				"reason": "临时有事",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTableReservationForUpdate(gomock.Any(), gomock.Eq(reservation.ID)).
					Times(1).
					Return(reservation, nil)

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(reservation.TableID)).
					Times(1).
					Return(room, nil)

				store.EXPECT().
					CancelReservationTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CancelReservationTxResult{Reservation: cancelledReservation}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:          "NotAuthorized",
			reservationID: reservation.ID,
			body:          gin.H{},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				otherUserID := user.ID + 999
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, otherUserID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTableReservationForUpdate(gomock.Any(), gomock.Eq(reservation.ID)).
					Times(1).
					Return(reservation, nil)

				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Merchant{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:          "ReservationNotFound",
			reservationID: reservation.ID,
			body:          gin.H{},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetTableReservationForUpdate(gomock.Any(), gomock.Eq(reservation.ID)).
					Times(1).
					Return(db.TableReservation{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := fmt.Sprintf("/v1/reservations/%d/cancel", tc.reservationID)
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 商户预定列表测试 ====================

func TestListMerchantReservationsAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	room := randomRoom(merchant.ID)

	n := 5
	reservations := make([]db.ListReservationsByMerchantRow, n)
	for i := 0; i < n; i++ {
		r := randomReservation(room.ID, user.ID+int64(i)+1, merchant.ID)
		reservations[i] = db.ListReservationsByMerchantRow{
			ID:              r.ID,
			TableID:         r.TableID,
			UserID:          r.UserID,
			MerchantID:      r.MerchantID,
			ReservationDate: r.ReservationDate,
			ReservationTime: r.ReservationTime,
			GuestCount:      r.GuestCount,
			ContactName:     r.ContactName,
			ContactPhone:    r.ContactPhone,
			PaymentMode:     r.PaymentMode,
			DepositAmount:   r.DepositAmount,
			PrepaidAmount:   r.PrepaidAmount,
			RefundDeadline:  r.RefundDeadline,
			PaymentDeadline: r.PaymentDeadline,
			Status:          r.Status,
			CreatedAt:       r.CreatedAt,
			TableNo:         room.TableNo,
			TableType:       room.TableType,
		}
	}

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					ListReservationsByMerchant(gomock.Any(), gomock.Any()).
					Times(1).
					Return(reservations, nil)

				store.EXPECT().
					ListReservationItems(gomock.Any(), gomock.Any()).
					Times(len(reservations)).
					Return([]db.ListReservationItemsRow{}, nil)

				store.EXPECT().
					CountReservationsByMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "NotMerchant",
			query: "page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.Merchant{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:  "WithStatusFilter",
			query: "page_id=1&page_size=10&status=confirmed",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				// 当有status参数时，使用ListReservationsByMerchantAndStatus
				store.EXPECT().
					ListReservationsByMerchantAndStatus(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListReservationsByMerchantAndStatusRow{}, nil)

				store.EXPECT().
					CountReservationsByMerchantAndStatus(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "InvalidStatus",
			query: "page_id=1&page_size=10&status=invalid_status",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "CashierAllowed",
			query: "page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				staffMerchant := merchant
				staffMerchant.OwnerUserID = user.ID + 100

				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(staffMerchant, nil)

				store.EXPECT().
					GetUserMerchantRole(gomock.Any(), gomock.Eq(db.GetUserMerchantRoleParams{
						MerchantID: merchant.ID,
						UserID:     user.ID,
					})).
					Times(1).
					Return("cashier", nil)

				store.EXPECT().
					ListReservationsByMerchant(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListReservationsByMerchantRow{}, nil)

				store.EXPECT().
					CountReservationsByMerchant(gomock.Any(), gomock.Eq(merchant.ID)).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := "/v1/reservations/merchant?" + tc.query
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 完成预定测试 ====================

func TestCompleteReservationAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	room := randomRoom(merchant.ID)

	confirmedReservation := randomReservation(room.ID, user.ID+1, merchant.ID)
	confirmedReservation.Status = "confirmed"

	completedReservation := confirmedReservation
	completedReservation.Status = "completed"

	testCases := []struct {
		name          string
		reservationID int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:          "OK",
			reservationID: confirmedReservation.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(2).
					Return(merchant, nil)

				store.EXPECT().
					GetTableReservationForUpdate(gomock.Any(), gomock.Eq(confirmedReservation.ID)).
					Times(1).
					Return(confirmedReservation, nil)

				// 获取桌台信息以获取currentReservationID
				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(confirmedReservation.TableID)).
					Times(1).
					Return(room, nil)

				store.EXPECT().
					CompleteReservationTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CompleteReservationTxResult{Reservation: completedReservation, TableUpdated: true}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:          "NotMerchant",
			reservationID: confirmedReservation.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.Merchant{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:          "ReservationNotConfirmed",
			reservationID: confirmedReservation.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(2).
					Return(merchant, nil)

				pendingReservation := confirmedReservation
				pendingReservation.Status = "pending"

				store.EXPECT().
					GetTableReservationForUpdate(gomock.Any(), gomock.Eq(confirmedReservation.ID)).
					Times(1).
					Return(pendingReservation, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
			},
		},
		{
			name:          "WrongMerchant",
			reservationID: confirmedReservation.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				differentMerchant := merchant
				differentMerchant.ID = merchant.ID + 999

				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(2).
					Return(differentMerchant, nil)

				store.EXPECT().
					GetTableReservationForUpdate(gomock.Any(), gomock.Eq(confirmedReservation.ID)).
					Times(1).
					Return(confirmedReservation, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:          "CashierAllowed",
			reservationID: confirmedReservation.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				staffMerchant := merchant
				staffMerchant.OwnerUserID = user.ID + 100

				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(2).
					Return(staffMerchant, nil)

				store.EXPECT().
					GetUserMerchantRole(gomock.Any(), gomock.Eq(db.GetUserMerchantRoleParams{
						MerchantID: merchant.ID,
						UserID:     user.ID,
					})).
					Times(1).
					Return("cashier", nil)

				store.EXPECT().
					GetTableReservationForUpdate(gomock.Any(), gomock.Eq(confirmedReservation.ID)).
					Times(1).
					Return(confirmedReservation, nil)

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(confirmedReservation.TableID)).
					Times(1).
					Return(room, nil)

				store.EXPECT().
					CompleteReservationTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CompleteReservationTxResult{Reservation: completedReservation, TableUpdated: true}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/reservations/%d/complete", tc.reservationID)
			request, err := http.NewRequest(http.MethodPost, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 标记未到场测试 ====================

func TestMarkNoShowAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	room := randomRoom(merchant.ID)

	confirmedReservation := randomReservation(room.ID, user.ID+1, merchant.ID)
	confirmedReservation.Status = "confirmed"

	noShowReservation := confirmedReservation
	noShowReservation.Status = "no_show"

	testCases := []struct {
		name          string
		reservationID int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:          "OK",
			reservationID: confirmedReservation.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(2).
					Return(merchant, nil)

				store.EXPECT().
					GetTableReservationForUpdate(gomock.Any(), gomock.Eq(confirmedReservation.ID)).
					Times(1).
					Return(confirmedReservation, nil)

				// 获取桌台信息以获取currentReservationID
				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(confirmedReservation.TableID)).
					Times(1).
					Return(room, nil)

				store.EXPECT().
					ReleaseReservationInventoryTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)

				store.EXPECT().
					MarkNoShowTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.MarkNoShowTxResult{Reservation: noShowReservation, TableUpdated: true}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:          "NotMerchant",
			reservationID: confirmedReservation.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.Merchant{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:          "ReservationNotConfirmed",
			reservationID: confirmedReservation.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(2).
					Return(merchant, nil)

				pendingReservation := confirmedReservation
				pendingReservation.Status = "pending"

				store.EXPECT().
					GetTableReservationForUpdate(gomock.Any(), gomock.Eq(confirmedReservation.ID)).
					Times(1).
					Return(pendingReservation, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
			},
		},
		{
			name:          "CashierForbidden",
			reservationID: confirmedReservation.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				staffMerchant := merchant
				staffMerchant.OwnerUserID = user.ID + 100

				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(staffMerchant, nil)

				store.EXPECT().
					GetUserMerchantRole(gomock.Any(), gomock.Eq(db.GetUserMerchantRoleParams{
						MerchantID: merchant.ID,
						UserID:     user.ID,
					})).
					Times(1).
					Return("cashier", nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/reservations/%d/no-show", tc.reservationID)
			request, err := http.NewRequest(http.MethodPost, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

// ==================== 获取预定统计测试 ====================

func TestGetReservationStatsAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)

	stats := db.GetReservationStatsEnhancedRow{
		PendingCount:   10,
		PaidCount:      20,
		ConfirmedCount: 50,
		CheckedInCount: 15,
		CompletedCount: 30,
		CancelledCount: 10,
		ExpiredCount:   5,
		NoShowCount:    5,
	}

	testCases := []struct {
		name          string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetReservationStatsEnhanced(gomock.Any(), merchant.ID).
					Times(1).
					Return(stats, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name: "NotMerchant",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.Merchant{}, db.ErrRecordNotFound)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := "/v1/reservations/merchant/stats"
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestMerchantCreateReservationAPICashierAllowed(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID + 100)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		GetUserMerchantRole(gomock.Any(), gomock.Eq(db.GetUserMerchantRoleParams{
			MerchantID: merchant.ID,
			UserID:     user.ID,
		})).
		Times(1).
		Return("cashier", nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	req, err := http.NewRequest(http.MethodPost, "/v1/reservations/merchant/create", bytes.NewReader([]byte(`{"date":"bad"}`)))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestMerchantCreateReservationAPISuccess(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	table := randomRoom(merchant.ID)
	reservationDate := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)
	reservationTime := time.Date(0, 1, 1, 18, 30, 0, 0, time.UTC)
	createdReservation := randomReservation(table.ID, user.ID, merchant.ID)
	createdReservation.Status = "confirmed"
	createdReservation.PaymentMode = PaymentModeDeposit
	createdReservation.DepositAmount = 0
	createdReservation.PrepaidAmount = 0
	createdReservation.ReservationDate = pgtype.Date{Time: reservationDate, Valid: true}
	createdReservation.ReservationTime = pgtype.Time{Microseconds: int64((18*3600 + 30*60) * 1000000), Valid: true}
	createdReservation.ContactName = "Alice"
	createdReservation.ContactPhone = "13800138000"
	createdReservation.GuestCount = 4

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	staffMerchant := merchant
	staffMerchant.OwnerUserID = user.ID + 100

	store.EXPECT().
		GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
		Times(1).
		Return(staffMerchant, nil)

	store.EXPECT().
		GetUserMerchantRole(gomock.Any(), gomock.Eq(db.GetUserMerchantRoleParams{
			MerchantID: merchant.ID,
			UserID:     user.ID,
		})).
		Times(1).
		Return("cashier", nil)

	store.EXPECT().
		GetTable(gomock.Any(), gomock.Eq(table.ID)).
		Times(1).
		Return(table, nil)

	store.EXPECT().
		ListReservationsByTableAndDate(gomock.Any(), gomock.Eq(db.ListReservationsByTableAndDateParams{
			TableID:         table.ID,
			ReservationDate: pgtype.Date{Time: reservationDate, Valid: true},
		})).
		Times(1).
		Return([]db.TableReservation{}, nil)

	store.EXPECT().
		ListMerchantBusinessHours(gomock.Any(), gomock.Eq(merchant.ID)).
		Times(1).
		Return([]db.MerchantBusinessHour{}, nil)

	store.EXPECT().
		CreateTableReservationByMerchant(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CreateTableReservationByMerchantParams) (db.TableReservation, error) {
			require.Equal(t, table.ID, arg.TableID)
			require.Equal(t, user.ID, arg.UserID)
			require.Equal(t, merchant.ID, arg.MerchantID)
			require.Equal(t, pgtype.Date{Time: reservationDate, Valid: true}, arg.ReservationDate)
			require.Equal(t, pgtype.Time{Microseconds: int64((reservationTime.Hour()*3600 + reservationTime.Minute()*60) * 1000000), Valid: true}, arg.ReservationTime)
			require.Equal(t, int16(4), arg.GuestCount)
			require.Equal(t, "Alice", arg.ContactName)
			require.Equal(t, "13800138000", arg.ContactPhone)
			require.Equal(t, PaymentModeDeposit, arg.PaymentMode)
			require.Equal(t, int64(0), arg.DepositAmount)
			require.Equal(t, int64(0), arg.PrepaidAmount)
			require.Equal(t, pgtype.Text{}, arg.Notes)
			require.Equal(t, pgtype.Text{String: ReservationSourceMerchant, Valid: true}, arg.Source)
			require.False(t, arg.RefundDeadline.IsZero())
			require.False(t, arg.PaymentDeadline.IsZero())
			return createdReservation, nil
		})

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()
	reqBody := []byte(`{"table_id":` + fmt.Sprint(table.ID) + `,"date":"2026-04-02","time":"18:30","guest_count":4,"contact_name":"Alice","contact_phone":"13800138000"}`)
	req, err := http.NewRequest(http.MethodPost, "/v1/reservations/merchant/create", bytes.NewReader(reqBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var resp reservationResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
	require.Equal(t, createdReservation.ID, resp.ID)
	require.Equal(t, "confirmed", resp.Status)
	require.Equal(t, PaymentModeDeposit, resp.PaymentMode)
	require.Equal(t, merchant.ID, resp.MerchantID)
	require.Equal(t, table.ID, resp.TableID)
	require.Equal(t, "2026-04-02", resp.ReservationDate)
	require.Equal(t, "18:30", resp.ReservationTime)
}

func TestMerchantUpdateReservationAPICashierForbidden(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID + 100)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		GetUserMerchantRole(gomock.Any(), gomock.Eq(db.GetUserMerchantRoleParams{
			MerchantID: merchant.ID,
			UserID:     user.ID,
		})).
		Times(1).
		Return("cashier", nil)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	req, err := http.NewRequest(http.MethodPut, "/v1/reservations/1/update", bytes.NewReader([]byte(`{}`)))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusForbidden, recorder.Code)
}
