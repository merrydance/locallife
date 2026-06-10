package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type merchantStatusChangeCapture struct {
	merchantID  int64
	isOpen      bool
	autoCloseAt *time.Time
	source      string
}

func (p *merchantStatusChangeCapture) PublishMerchantStatusChange(_ context.Context, merchantID int64, isOpen bool, autoCloseAt *time.Time, source string) error {
	p.merchantID = merchantID
	p.isOpen = isOpen
	p.autoCloseAt = autoCloseAt
	p.source = source
	return nil
}

func TestUpdateMerchantOpenStatus_PublishesMerchantStatusChange(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{
		ID:          201,
		OwnerUserID: user.ID,
		RegionID:    1,
		Status:      "active",
		Name:        "商户X",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().GetMerchantProfile(gomock.Any(), merchant.ID).Return(db.GetMerchantProfileRow{}, db.ErrRecordNotFound)
	store.EXPECT().UpdateMerchantIsOpen(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, arg db.UpdateMerchantIsOpenParams) (db.Merchant, error) {
			return db.Merchant{
				ID:          merchant.ID,
				IsOpen:      arg.IsOpen,
				Status:      merchant.Status,
				Name:        merchant.Name,
				Phone:       merchant.Phone,
				Address:     merchant.Address,
				RegionID:    merchant.RegionID,
				Version:     merchant.Version,
				AutoCloseAt: pgtype.Timestamptz{Time: arg.AutoCloseAt.Time, Valid: arg.AutoCloseAt.Valid},
				CreatedAt:   merchant.CreatedAt,
				UpdatedAt:   time.Now(),
			}, nil
		},
	)

	server := newTestServer(t, store)
	capture := &merchantStatusChangeCapture{}
	server.merchantStatusChangePublisher = capture

	body, err := json.Marshal(map[string]any{"is_open": false})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPatch, "/v1/merchants/me/status", bytes.NewReader(body))
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, merchant.ID, capture.merchantID)
	require.False(t, capture.isOpen)
	require.Equal(t, "manual", capture.source)
	require.Nil(t, capture.autoCloseAt)
	require.Contains(t, recorder.Body.String(), "店铺已打烊")
}

func TestUpdateMerchantOpenStatus_PublishesAutoCloseAt(t *testing.T) {
	user, _ := randomUser(t)
	merchant := db.Merchant{
		ID:          202,
		OwnerUserID: user.ID,
		RegionID:    1,
		Status:      "active",
		Name:        "商户Y",
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().GetMerchantProfile(gomock.Any(), merchant.ID).Return(db.GetMerchantProfileRow{}, db.ErrRecordNotFound)
	store.EXPECT().GetBaofuAccountBindingByOwner(gomock.Any(), db.GetBaofuAccountBindingByOwnerParams{
		OwnerType: db.BaofuAccountOwnerTypeMerchant,
		OwnerID:   merchant.ID,
	}).Return(db.BaofuAccountBinding{
		OwnerType:    db.BaofuAccountOwnerTypeMerchant,
		OwnerID:      merchant.ID,
		AccountType:  db.BaofuAccountTypeBusiness,
		OpenState:    db.BaofuAccountOpenStateActive,
		ContractNo:   pgtype.Text{String: "contract-202", Valid: true},
		SharingMerID: pgtype.Text{String: "sharing-202", Valid: true},
	}, nil)
	store.EXPECT().GetBaofuMerchantReportByOwner(gomock.Any(), db.GetBaofuMerchantReportByOwnerParams{
		OwnerType:  db.BaofuAccountOwnerTypeMerchant,
		OwnerID:    merchant.ID,
		ReportType: db.BaofuMerchantReportTypeWechat,
	}).Return(db.BaofuMerchantReport{
		OwnerType:       db.BaofuAccountOwnerTypeMerchant,
		OwnerID:         merchant.ID,
		ReportType:      db.BaofuMerchantReportTypeWechat,
		ReportState:     db.BaofuMerchantReportStateSucceeded,
		SubMchID:        pgtype.Text{String: "sub-202", Valid: true},
		AppletAuthState: db.BaofuMerchantReportAppletAuthStateSucceeded,
	}, nil)
	store.EXPECT().GetMerchantPaymentConfig(gomock.Any(), merchant.ID).Return(db.MerchantPaymentConfig{
		MerchantID: merchant.ID,
		SubMchID:   "sub-202",
		Status:     db.MerchantPaymentConfigStatusActive,
	}, nil)
	store.EXPECT().UpdateMerchantIsOpen(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, arg db.UpdateMerchantIsOpenParams) (db.Merchant, error) {
			return db.Merchant{
				ID:       merchant.ID,
				IsOpen:   arg.IsOpen,
				Status:   merchant.Status,
				Name:     merchant.Name,
				Phone:    merchant.Phone,
				Address:  merchant.Address,
				RegionID: merchant.RegionID,
				Version:  merchant.Version,
				AutoCloseAt: pgtype.Timestamptz{
					Time:  arg.AutoCloseAt.Time,
					Valid: arg.AutoCloseAt.Valid,
				},
				CreatedAt: merchant.CreatedAt,
				UpdatedAt: time.Now(),
			}, nil
		},
	)

	server := newTestServer(t, store)
	capture := &merchantStatusChangeCapture{}
	server.merchantStatusChangePublisher = capture

	autoCloseAt := time.Now().Add(30 * time.Minute).UTC().Format(time.RFC3339)
	body, err := json.Marshal(map[string]any{
		"is_open":       true,
		"auto_close_at": autoCloseAt,
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPatch, "/v1/merchants/me/status", bytes.NewReader(body))
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, merchant.ID, capture.merchantID)
	require.True(t, capture.isOpen)
	require.Equal(t, "manual", capture.source)
	require.NotNil(t, capture.autoCloseAt)
	require.Contains(t, recorder.Body.String(), "店铺已开始营业")
}

func TestUpdateMerchantOpenStatus_SetsManualOverrideUntilNextBusinessHoursSwitch(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.ID = 203
	merchant.Status = "active"
	merchant.AutoOpenByBusinessHours = true

	now := time.Date(2026, time.June, 10, 10, 30, 0, 0, time.Local)
	dayOfWeek := int32(now.Weekday())

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().GetMerchantProfile(gomock.Any(), merchant.ID).Return(db.GetMerchantProfileRow{}, db.ErrRecordNotFound)
	store.EXPECT().GetDatabaseLocalClock(gomock.Any()).Return(databaseClockRowForStatusTest(now), nil)
	store.EXPECT().ListMerchantBusinessHoursAll(gomock.Any(), merchant.ID).Return([]db.MerchantBusinessHour{
		{
			MerchantID: merchant.ID,
			DayOfWeek:  dayOfWeek,
			OpenTime:   pgtype.Time{Microseconds: 0, Valid: true},
			CloseTime:  pgtype.Time{Microseconds: 24 * 3600 * 1000000, Valid: true},
			IsClosed:   false,
		},
	}, nil)
	store.EXPECT().UpdateMerchantIsOpen(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, arg db.UpdateMerchantIsOpenParams) (db.Merchant, error) {
			require.Equal(t, merchant.ID, arg.ID)
			require.False(t, arg.IsOpen)
			require.True(t, arg.ManualOpenStatusUntil.Valid)
			require.WithinDuration(t, startOfLocalDay(now).AddDate(0, 0, 1), arg.ManualOpenStatusUntil.Time, 2*time.Second)
			return db.Merchant{
				ID:                    merchant.ID,
				IsOpen:                arg.IsOpen,
				Status:                merchant.Status,
				Name:                  merchant.Name,
				Phone:                 merchant.Phone,
				Address:               merchant.Address,
				RegionID:              merchant.RegionID,
				Version:               merchant.Version,
				ManualOpenStatusUntil: arg.ManualOpenStatusUntil,
				CreatedAt:             merchant.CreatedAt,
				UpdatedAt:             time.Now(),
			}, nil
		},
	)

	server := newTestServer(t, store)

	body, err := json.Marshal(map[string]any{"is_open": false})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPatch, "/v1/merchants/me/status", bytes.NewReader(body))
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestUpdateMerchantOpenStatus_SetsInfiniteManualOverrideWhenAutoModeHasNoFutureSwitch(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.ID = 204
	merchant.Status = "active"
	merchant.AutoOpenByBusinessHours = true

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	expectResolveSingleOwnedMerchant(store, user.ID, merchant)
	store.EXPECT().GetMerchantProfile(gomock.Any(), merchant.ID).Return(db.GetMerchantProfileRow{}, db.ErrRecordNotFound)
	store.EXPECT().GetDatabaseLocalClock(gomock.Any()).Return(databaseClockRowForStatusTest(time.Date(2026, time.June, 10, 10, 30, 0, 0, time.Local)), nil)
	store.EXPECT().ListMerchantBusinessHoursAll(gomock.Any(), merchant.ID).Return([]db.MerchantBusinessHour{}, nil)
	store.EXPECT().UpdateMerchantIsOpen(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, arg db.UpdateMerchantIsOpenParams) (db.Merchant, error) {
			require.Equal(t, merchant.ID, arg.ID)
			require.False(t, arg.IsOpen)
			require.True(t, arg.ManualOpenStatusUntil.Valid)
			require.Equal(t, pgtype.Infinity, arg.ManualOpenStatusUntil.InfinityModifier)
			return db.Merchant{
				ID:                    merchant.ID,
				IsOpen:                arg.IsOpen,
				Status:                merchant.Status,
				Name:                  merchant.Name,
				Phone:                 merchant.Phone,
				Address:               merchant.Address,
				RegionID:              merchant.RegionID,
				Version:               merchant.Version,
				ManualOpenStatusUntil: arg.ManualOpenStatusUntil,
				CreatedAt:             merchant.CreatedAt,
				UpdatedAt:             time.Now(),
			}, nil
		},
	)

	server := newTestServer(t, store)

	body, err := json.Marshal(map[string]any{"is_open": false})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodPatch, "/v1/merchants/me/status", bytes.NewReader(body))
	require.NoError(t, err)
	addAuthorization(t, req, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)

	server.router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func databaseClockRowForStatusTest(t time.Time) db.GetDatabaseLocalClockRow {
	return db.GetDatabaseLocalClockRow{
		CurrentYear:     int32(t.Year()),
		CurrentMonth:    int32(t.Month()),
		CurrentDay:      int32(t.Day()),
		LocalTimeMicros: localTimeMicros(t),
	}
}
