package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestResolveReservationDepositDeduction(t *testing.T) {
	reservationID := int64(88)
	reservation := &db.TableReservation{ID: reservationID, PaymentMode: paymentModeDeposit}

	testCases := []struct {
		name        string
		reservation *db.TableReservation
		buildStubs  func(store *mockdb.MockStore)
		check       func(t *testing.T, amount int64, err error)
	}{
		{
			name:        "NilReservation",
			reservation: nil,
			buildStubs:  func(store *mockdb.MockStore) {},
			check: func(t *testing.T, amount int64, err error) {
				require.NoError(t, err)
				require.Zero(t, amount)
			},
		},
		{
			name:        "NonDepositReservation",
			reservation: &db.TableReservation{ID: reservationID, PaymentMode: paymentModeFull},
			buildStubs:  func(store *mockdb.MockStore) {},
			check: func(t *testing.T, amount int64, err error) {
				require.NoError(t, err)
				require.Zero(t, amount)
			},
		},
		{
			name:        "UsePaidPaymentOrderAmount",
			reservation: reservation,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetLatestPaymentOrderByReservation(gomock.Any(), db.GetLatestPaymentOrderByReservationParams{
						ReservationID: pgtype.Int8{Int64: reservationID, Valid: true},
						BusinessType:  businessTypeReservation,
					}).
					Times(1).
					Return(db.PaymentOrder{Amount: 660, Status: paymentStatusPaid}, nil)
			},
			check: func(t *testing.T, amount int64, err error) {
				require.NoError(t, err)
				require.Equal(t, int64(660), amount)
			},
		},
		{
			name:        "PaymentOrderMissing",
			reservation: reservation,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetLatestPaymentOrderByReservation(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.PaymentOrder{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, amount int64, err error) {
				require.Zero(t, amount)
				reqErr := assertRequestError(t, err)
				require.Equal(t, 409, reqErr.Status)
				require.Equal(t, "reservation deposit payment record not found", reqErr.Err.Error())
			},
		},
		{
			name:        "PaymentOrderNotSettled",
			reservation: reservation,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetLatestPaymentOrderByReservation(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.PaymentOrder{Amount: 800, Status: paymentStatusPending}, nil)
			},
			check: func(t *testing.T, amount int64, err error) {
				require.Zero(t, amount)
				reqErr := assertRequestError(t, err)
				require.Equal(t, 409, reqErr.Status)
				require.Equal(t, "reservation deposit payment is not settled", reqErr.Err.Error())
			},
		},
		{
			name:        "StoreError",
			reservation: reservation,
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetLatestPaymentOrderByReservation(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.PaymentOrder{}, errors.New("boom"))
			},
			check: func(t *testing.T, amount int64, err error) {
				require.Zero(t, amount)
				require.EqualError(t, err, "boom")
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			if tc.buildStubs != nil {
				tc.buildStubs(store)
			}

			amount, err := ResolveReservationDepositDeduction(context.Background(), store, tc.reservation)
			tc.check(t, amount, err)
		})
	}
}

func TestComputeOrderTotals(t *testing.T) {
	testCases := []struct {
		name  string
		input OrderTotalsInput
		check func(t *testing.T, result OrderTotalsResult, err error)
	}{
		{
			name: "BasicTotal",
			input: OrderTotalsInput{
				Subtotal:            1000,
				DiscountAmount:      100,
				VoucherAmount:       200,
				DeliveryFee:         50,
				DeliveryFeeDiscount: 10,
			},
			check: func(t *testing.T, result OrderTotalsResult, err error) {
				require.NoError(t, err)
				require.Equal(t, int64(740), result.TotalAmount)
				require.Equal(t, int64(0), result.BalancePaid)
			},
		},
		{
			name: "PackagingFeeIncludedSeparately",
			input: OrderTotalsInput{
				Subtotal:            1000,
				DiscountAmount:      100,
				VoucherAmount:       200,
				PackagingFee:        150,
				DeliveryFee:         50,
				DeliveryFeeDiscount: 10,
			},
			check: func(t *testing.T, result OrderTotalsResult, err error) {
				require.NoError(t, err)
				require.Equal(t, int64(890), result.TotalAmount)
				require.Equal(t, int64(0), result.BalancePaid)
			},
		},
		{
			name: "TotalNegative",
			input: OrderTotalsInput{
				Subtotal:       100,
				DiscountAmount: 200,
			},
			check: func(t *testing.T, result OrderTotalsResult, err error) {
				require.NoError(t, err)
				require.Equal(t, int64(0), result.TotalAmount)
			},
		},
		{
			name: "DepositDeductionPartial",
			input: OrderTotalsInput{
				Subtotal:         1000,
				DepositDeduction: 300,
			},
			check: func(t *testing.T, result OrderTotalsResult, err error) {
				require.NoError(t, err)
				require.Equal(t, int64(700), result.TotalAmount)
			},
		},
		{
			name: "DepositDeductionCap",
			input: OrderTotalsInput{
				Subtotal:         500,
				DepositDeduction: 800,
			},
			check: func(t *testing.T, result OrderTotalsResult, err error) {
				require.NoError(t, err)
				require.Equal(t, int64(0), result.TotalAmount)
			},
		},
		{
			name: "BalanceRequired",
			input: OrderTotalsInput{
				Subtotal:          300,
				UseBalance:        true,
				MembershipBalance: 0,
			},
			check: func(t *testing.T, _ OrderTotalsResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 400, reqErr.Status)
				require.Equal(t, "insufficient membership balance", reqErr.Err.Error())
			},
		},
		{
			name: "BalancePartial",
			input: OrderTotalsInput{
				Subtotal:          1000,
				UseBalance:        true,
				MembershipBalance: 300,
			},
			check: func(t *testing.T, result OrderTotalsResult, err error) {
				require.NoError(t, err)
				require.Equal(t, int64(1000), result.TotalAmount)
				require.Equal(t, int64(300), result.BalancePaid)
			},
		},
		{
			name: "BalanceFull",
			input: OrderTotalsInput{
				Subtotal:          500,
				UseBalance:        true,
				MembershipBalance: 700,
			},
			check: func(t *testing.T, result OrderTotalsResult, err error) {
				require.NoError(t, err)
				require.Equal(t, int64(500), result.TotalAmount)
				require.Equal(t, int64(500), result.BalancePaid)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			result, err := ComputeOrderTotals(tc.input)
			tc.check(t, result, err)
		})
	}
}
