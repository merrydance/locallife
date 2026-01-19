package api

import (
	"bytes"
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

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func randomDiningSession(merchantID, tableID, userID int64) db.DiningSession {
	return db.DiningSession{
		ID:            util.RandomInt(1, 1000),
		MerchantID:    merchantID,
		TableID:       tableID,
		ReservationID: pgtype.Int8{Valid: false},
		UserID:        userID,
		Status:        "open",
		OpenedAt:      time.Now(),
		CreatedAt:     time.Now(),
	}
}

func TestTransferDiningSessionTableAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	fromTable := randomTable(merchant.ID)
	toTable := randomTable(merchant.ID)

	accessCode := "1234"
	accessHash, err := util.HashPassword(accessCode)
	require.NoError(t, err)
	toTable.AccessCodeHash = pgtype.Text{String: accessHash, Valid: true}

	session := randomDiningSession(merchant.ID, fromTable.ID, user.ID)

	testCases := []struct {
		name          string
		body          map[string]any
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "OK",
			body: map[string]any{
				"to_table_id": toTable.ID,
				"table_code":  accessCode,
				"reason":      "扫码换桌",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetDiningSession(gomock.Any(), gomock.Eq(session.ID)).
					Times(1).
					Return(session, nil)

				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.Merchant{}, db.ErrRecordNotFound)

				store.EXPECT().
					GetMerchantStaff(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.MerchantStaff{}, db.ErrRecordNotFound)

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(toTable.ID)).
					Times(1).
					Return(toTable, nil)

				store.EXPECT().
					ListReservationsByTableAndDate(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.TableReservation{}, nil)

				store.EXPECT().
					TransferDiningSessionTableTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.TransferDiningSessionTableTxResult{
						Session:   session,
						FromTable: fromTable,
						ToTable:   toTable,
					}, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var response transferDiningSessionResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Equal(t, session.ID, response.Session.ID)
				require.Equal(t, toTable.ID, response.ToTable.ID)
			},
		},
		{
			name: "ForbiddenNotOwner",
			body: map[string]any{
				"to_table_id": toTable.ID,
				"table_code":  accessCode,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID+1, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetDiningSession(gomock.Any(), gomock.Eq(session.ID)).
					Times(1).
					Return(session, nil)

				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID+1)).
					Times(1).
					Return(db.Merchant{}, db.ErrRecordNotFound)

				store.EXPECT().
					GetMerchantStaff(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.MerchantStaff{}, db.ErrRecordNotFound)

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Any()).
					Times(0)

				store.EXPECT().
					TransferDiningSessionTableTx(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "TableCodeRequired",
			body: map[string]any{
				"to_table_id": toTable.ID,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetDiningSession(gomock.Any(), gomock.Eq(session.ID)).
					Times(1).
					Return(session, nil)

				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.Merchant{}, db.ErrRecordNotFound)

				store.EXPECT().
					GetMerchantStaff(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.MerchantStaff{}, db.ErrRecordNotFound)

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(toTable.ID)).
					Times(1).
					Return(toTable, nil)

				store.EXPECT().
					ListReservationsByTableAndDate(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.TableReservation{}, nil)

				store.EXPECT().
					TransferDiningSessionTableTx(gomock.Any(), gomock.Any()).
					Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "ReservedConflict",
			body: map[string]any{
				"to_table_id": toTable.ID,
				"table_code":  accessCode,
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetDiningSession(gomock.Any(), gomock.Eq(session.ID)).
					Times(1).
					Return(session, nil)

				store.EXPECT().
					GetMerchantByOwner(gomock.Any(), gomock.Eq(user.ID)).
					Times(1).
					Return(db.Merchant{}, db.ErrRecordNotFound)

				store.EXPECT().
					GetMerchantStaff(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.MerchantStaff{}, db.ErrRecordNotFound)

				store.EXPECT().
					GetTable(gomock.Any(), gomock.Eq(toTable.ID)).
					Times(1).
					Return(toTable, nil)

				store.EXPECT().
					ListReservationsByTableAndDate(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.TableReservation{}, nil)

				store.EXPECT().
					TransferDiningSessionTableTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.TransferDiningSessionTableTxResult{}, db.ErrTargetTableReserved)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
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

			url := fmt.Sprintf("/v1/dining-sessions/%d/transfer-table", session.ID)
			body, err := json.Marshal(tc.body)
			require.NoError(t, err)
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
			require.NoError(t, err)
			request.Header.Set("Content-Type", "application/json")

			tc.setupAuth(t, request, server.tokenMaker)

			recorder := httptest.NewRecorder()
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
