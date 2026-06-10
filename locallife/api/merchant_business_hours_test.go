package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetMerchantBusinessHours_IncludesAutoOpenByBusinessHours(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.AutoOpenByBusinessHours = true

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().ListMerchantBusinessHoursAll(gomock.Any(), merchant.ID).Return([]db.MerchantBusinessHour{}, nil)

	server := newTestServer(t, store)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/v1/merchants/me/business-hours", nil)
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response businessHoursListResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.True(t, response.AutoOpenByBusinessHours)
	require.Len(t, response.Hours, 0)
}

func TestSetMerchantBusinessHours_PersistsAutoOpenByBusinessHours(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().SetBusinessHoursTx(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ any, arg db.SetBusinessHoursTxParams) (db.SetBusinessHoursTxResult, error) {
			require.True(t, arg.AutoOpenByBusinessHours)
			require.Len(t, arg.Hours, 1)
			return db.SetBusinessHoursTxResult{
				AutoOpenByBusinessHours: true,
				Hours: []db.MerchantBusinessHour{{
					ID:         1,
					MerchantID: merchant.ID,
					DayOfWeek:  1,
					OpenTime:   pgtype.Time{Microseconds: 9 * 3600 * 1000000, Valid: true},
					CloseTime:  pgtype.Time{Microseconds: 21 * 3600 * 1000000, Valid: true},
					IsClosed:   false,
				}},
			}, nil
		},
	)

	server := newTestServer(t, store)
	body, err := json.Marshal(map[string]any{
		"auto_open_by_business_hours": true,
		"hours": []map[string]any{{
			"day_of_week": 1,
			"open_time":   "09:00",
			"close_time":  "21:00",
			"is_closed":   false,
		}},
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPut, "/v1/merchants/me/business-hours", bytes.NewReader(body))
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response businessHoursListResponse
	requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
	require.True(t, response.AutoOpenByBusinessHours)
	require.Len(t, response.Hours, 1)
	require.Equal(t, "09:00", response.Hours[0].OpenTime)
}

func TestSetMerchantBusinessHours_RejectsReverseOpenCloseWindow(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().SetBusinessHoursTx(gomock.Any(), gomock.Any()).Times(0)

	server := newTestServer(t, store)
	body, err := json.Marshal(map[string]any{
		"auto_open_by_business_hours": true,
		"hours": []map[string]any{{
			"day_of_week": 1,
			"open_time":   "21:00",
			"close_time":  "09:00",
			"is_closed":   false,
		}},
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPut, "/v1/merchants/me/business-hours", bytes.NewReader(body))
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "open_time must be earlier than close_time")
}
