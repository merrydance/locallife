package logic

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetDeliveryForViewerByOrder(t *testing.T) {
	userID := int64(10)
	orderID := int64(20)
	riderID := int64(30)

	testCases := []struct {
		name       string
		input      DeliveryOrderViewerInput
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, delivery db.Delivery, err error)
	}{
		{
			name:  "OwnerSuccess",
			input: DeliveryOrderViewerInput{UserID: userID, OrderID: orderID, ForbiddenMessage: "无权查看此订单配送信息"},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), orderID).
					Times(1).
					Return(db.Order{ID: orderID, UserID: userID}, nil)
				store.EXPECT().
					GetDeliveryByOrderID(gomock.Any(), orderID).
					Times(1).
					Return(db.Delivery{ID: 77, OrderID: orderID}, nil)
			},
			check: func(t *testing.T, delivery db.Delivery, err error) {
				require.NoError(t, err)
				require.Equal(t, int64(77), delivery.ID)
			},
		},
		{
			name:  "AssignedRiderSuccess",
			input: DeliveryOrderViewerInput{UserID: userID, OrderID: orderID, ForbiddenMessage: "无权查看此订单配送信息"},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), orderID).
					Times(1).
					Return(db.Order{ID: orderID, UserID: userID + 1}, nil)
				store.EXPECT().
					GetDeliveryByOrderID(gomock.Any(), orderID).
					Times(1).
					Return(db.Delivery{ID: 88, OrderID: orderID, RiderID: pgtype.Int8{Int64: riderID, Valid: true}}, nil)
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), userID).
					Times(1).
					Return(db.Rider{ID: riderID, UserID: userID}, nil)
			},
			check: func(t *testing.T, delivery db.Delivery, err error) {
				require.NoError(t, err)
				require.Equal(t, int64(88), delivery.ID)
			},
		},
		{
			name:  "ForbiddenForOtherUser",
			input: DeliveryOrderViewerInput{UserID: userID, OrderID: orderID, ForbiddenMessage: "无权查看此订单配送信息"},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), orderID).
					Times(1).
					Return(db.Order{ID: orderID, UserID: userID + 1}, nil)
				store.EXPECT().
					GetDeliveryByOrderID(gomock.Any(), orderID).
					Times(1).
					Return(db.Delivery{ID: 99, OrderID: orderID, RiderID: pgtype.Int8{Int64: riderID, Valid: true}}, nil)
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), userID).
					Times(1).
					Return(db.Rider{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, _ db.Delivery, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 403, reqErr.Status)
				require.Equal(t, "无权查看此订单配送信息", reqErr.Err.Error())
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

			delivery, err := GetDeliveryForViewerByOrder(context.Background(), store, tc.input)
			tc.check(t, delivery, err)
		})
	}
}

func TestValidateDeliveryViewer(t *testing.T) {
	userID := int64(10)
	deliveryID := int64(20)
	orderID := int64(30)

	testCases := []struct {
		name       string
		input      DeliveryViewerInput
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, result DeliveryViewerResult, err error)
	}{
		{
			name:  "DeliveryNotFound",
			input: DeliveryViewerInput{UserID: userID, DeliveryID: deliveryID, ForbiddenMessage: "无权查看此配送单轨迹"},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetDelivery(gomock.Any(), deliveryID).
					Times(1).
					Return(db.Delivery{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, _ DeliveryViewerResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 404, reqErr.Status)
				require.Equal(t, "配送单不存在", reqErr.Err.Error())
			},
		},
		{
			name:  "OrderNotFound",
			input: DeliveryViewerInput{UserID: userID, DeliveryID: deliveryID, ForbiddenMessage: "无权查看此配送单轨迹"},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetDelivery(gomock.Any(), deliveryID).
					Times(1).
					Return(db.Delivery{ID: deliveryID, OrderID: orderID}, nil)
				store.EXPECT().
					GetOrder(gomock.Any(), orderID).
					Times(1).
					Return(db.Order{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, _ DeliveryViewerResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 404, reqErr.Status)
				require.Equal(t, "订单不存在", reqErr.Err.Error())
			},
		},
		{
			name:  "Forbidden",
			input: DeliveryViewerInput{UserID: userID, DeliveryID: deliveryID, ForbiddenMessage: "无权查看此配送单轨迹"},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetDelivery(gomock.Any(), deliveryID).
					Times(1).
					Return(db.Delivery{ID: deliveryID, OrderID: orderID, RiderID: pgtype.Int8{Int64: 88, Valid: true}}, nil)
				store.EXPECT().
					GetOrder(gomock.Any(), orderID).
					Times(1).
					Return(db.Order{ID: orderID, UserID: userID + 1}, nil)
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), userID).
					Times(1).
					Return(db.Rider{}, db.ErrRecordNotFound)
			},
			check: func(t *testing.T, _ DeliveryViewerResult, err error) {
				reqErr := assertRequestError(t, err)
				require.Equal(t, 403, reqErr.Status)
				require.Equal(t, "无权查看此配送单轨迹", reqErr.Err.Error())
			},
		},
		{
			name:  "Owner",
			input: DeliveryViewerInput{UserID: userID, DeliveryID: deliveryID, ForbiddenMessage: "无权查看此配送单轨迹"},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetDelivery(gomock.Any(), deliveryID).
					Times(1).
					Return(db.Delivery{ID: deliveryID, OrderID: orderID}, nil)
				store.EXPECT().
					GetOrder(gomock.Any(), orderID).
					Times(1).
					Return(db.Order{ID: orderID, UserID: userID}, nil)
			},
			check: func(t *testing.T, result DeliveryViewerResult, err error) {
				require.NoError(t, err)
				require.True(t, result.IsOrderOwner)
				require.False(t, result.IsRider)
			},
		},
		{
			name:  "Rider",
			input: DeliveryViewerInput{UserID: userID, DeliveryID: deliveryID, ForbiddenMessage: "无权查看此配送单轨迹"},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetDelivery(gomock.Any(), deliveryID).
					Times(1).
					Return(db.Delivery{ID: deliveryID, OrderID: orderID, RiderID: pgtype.Int8{Int64: 88, Valid: true}}, nil)
				store.EXPECT().
					GetOrder(gomock.Any(), orderID).
					Times(1).
					Return(db.Order{ID: orderID, UserID: userID + 1}, nil)
				store.EXPECT().
					GetRiderByUserID(gomock.Any(), userID).
					Times(1).
					Return(db.Rider{ID: 88, UserID: userID}, nil)
			},
			check: func(t *testing.T, result DeliveryViewerResult, err error) {
				require.NoError(t, err)
				require.False(t, result.IsOrderOwner)
				require.True(t, result.IsRider)
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

			result, err := ValidateDeliveryViewer(context.Background(), store, tc.input)
			tc.check(t, result, err)
		})
	}
}
