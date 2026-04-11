package logic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPaymentLedgerServiceListPaymentLedger(t *testing.T) {
	timestamp := time.Date(2026, 3, 27, 10, 30, 0, 0, time.UTC)

	testCases := []struct {
		name       string
		input      ListPaymentLedgerInput
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, result ListPaymentLedgerResult, err error)
	}{
		{
			name:  "Success",
			input: ListPaymentLedgerInput{UserID: 1001, PageID: 2, PageSize: 5},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListPaymentLedgerEntriesByUser(gomock.Any(), db.ListPaymentLedgerEntriesByUserParams{
						UserID: 1001,
						Limit:  5,
						Offset: 5,
					}).
					Return([]db.ListPaymentLedgerEntriesByUserRow{
						{
							ID:             1,
							EntryType:      "payment",
							PaymentOrderID: 1,
							BusinessType:   "order",
							Amount:         9800,
							Status:         "paid",
							OccurredAt:     timestamp,
							CreatedAt:      timestamp,
						},
						{
							ID:             2,
							EntryType:      "refund",
							PaymentOrderID: 1,
							RefundOrderID:  pgtype.Int8{Int64: 2, Valid: true},
							OrderID:        pgtype.Int8{Int64: 99, Valid: true},
							BusinessType:   "order",
							Amount:         3000,
							Status:         "success",
							OccurredAt:     timestamp.Add(2 * time.Minute),
							CreatedAt:      timestamp.Add(time.Minute),
						},
					}, nil)
				store.EXPECT().
					CountPaymentLedgerEntriesByUser(gomock.Any(), int64(1001)).
					Return(int64(8), nil)
			},
			check: func(t *testing.T, result ListPaymentLedgerResult, err error) {
				require.NoError(t, err)
				require.Len(t, result.Entries, 2)
				require.Equal(t, int64(8), result.TotalCount)
				require.Nil(t, result.Entries[0].RefundOrderID)
				require.NotNil(t, result.Entries[1].RefundOrderID)
				require.Equal(t, int64(2), *result.Entries[1].RefundOrderID)
				require.NotNil(t, result.Entries[1].OrderID)
				require.Equal(t, int64(99), *result.Entries[1].OrderID)
			},
		},
		{
			name:  "ListError",
			input: ListPaymentLedgerInput{UserID: 1002, PageID: 1, PageSize: 10},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListPaymentLedgerEntriesByUser(gomock.Any(), db.ListPaymentLedgerEntriesByUserParams{
						UserID: 1002,
						Limit:  10,
						Offset: 0,
					}).
					Return(nil, errors.New("db unavailable"))
			},
			check: func(t *testing.T, result ListPaymentLedgerResult, err error) {
				require.Error(t, err)
				require.Empty(t, result.Entries)
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

			svc := NewPaymentLedgerService(store)
			result, err := svc.ListPaymentLedger(context.Background(), tc.input)
			tc.check(t, result, err)
		})
	}
}
