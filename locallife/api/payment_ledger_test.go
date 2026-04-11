package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestListPaymentLedgerAPI(t *testing.T) {
	user, _ := randomUser(t)
	timestamp := time.Date(2026, 3, 27, 12, 0, 0, 0, time.UTC)

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "page_id=2&page_size=5",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListPaymentLedgerEntriesByUser(gomock.Any(), db.ListPaymentLedgerEntriesByUserParams{
						UserID: user.ID,
						Limit:  5,
						Offset: 5,
					}).
					Return([]db.ListPaymentLedgerEntriesByUserRow{
						{
							ID:             101,
							EntryType:      "payment",
							PaymentOrderID: 101,
							BusinessType:   "order",
							Amount:         1200,
							Status:         "paid",
							OccurredAt:     timestamp,
							CreatedAt:      timestamp,
						},
						{
							ID:             202,
							EntryType:      "refund",
							PaymentOrderID: 101,
							RefundOrderID:  pgtype.Int8{Int64: 202, Valid: true},
							OrderID:        pgtype.Int8{Int64: 501, Valid: true},
							BusinessType:   "order",
							Amount:         300,
							Status:         "success",
							OccurredAt:     timestamp.Add(time.Minute),
							CreatedAt:      timestamp.Add(time.Minute),
						},
					}, nil)
				store.EXPECT().
					CountPaymentLedgerEntriesByUser(gomock.Any(), user.ID).
					Return(int64(9), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response listPaymentLedgerResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Len(t, response.Entries, 2)
				require.Equal(t, int64(9), response.Total)
				require.Equal(t, int32(2), response.PageID)
				require.Equal(t, "payment", response.Entries[0].EntryType)
				require.Equal(t, "refund", response.Entries[1].EntryType)
				require.NotNil(t, response.Entries[1].RefundOrderID)
			},
		},
		{
			name:  "InvalidPageSize",
			query: "page_id=1&page_size=21",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:       "NoAuthorization",
			query:      "page_id=1&page_size=10",
			setupAuth:  func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/payments/ledger?%s", tc.query)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}
