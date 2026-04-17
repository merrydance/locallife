package logic

import (
	"context"
	"testing"
	"time"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type createOrderNormalizerStub struct{}

func (createOrderNormalizerStub) Normalize(context.Context, int64, map[string]interface{}) ([]byte, int64, error) {
	return nil, 0, nil
}

func TestOrderServiceCreateOrder_RejectsVoucherWhenMerchantDiscountCannotStack(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	service := NewOrderService(store, nil, nil, nil, nil, createOrderNormalizerStub{}, nil, nil, nil, nil, nil)

	merchantID := int64(10)
	userID := int64(20)
	dishID := int64(30)
	voucherID := int64(40)
	now := time.Now()

	store.EXPECT().
		GetMerchant(gomock.Any(), merchantID).
		Times(1).
		Return(db.Merchant{ID: merchantID, Status: "active", IsOpen: true}, nil)
	store.EXPECT().
		GetDish(gomock.Any(), dishID).
		Times(1).
		Return(db.Dish{ID: dishID, MerchantID: merchantID, Name: "套餐", Price: 2000, IsOnline: true, IsAvailable: true}, nil)
	store.EXPECT().
		CountActivePackagingDishesByMerchant(gomock.Any(), merchantID).
		Times(1).
		Return(int64(0), nil)
	store.EXPECT().
		ListActiveDiscountRules(gomock.Any(), merchantID).
		Times(1).
		Return([]db.DiscountRule{{
			ID:                  1,
			Name:                "会员日满减",
			MinOrderAmount:      1000,
			DiscountAmount:      300,
			ValidFrom:           now.Add(-time.Hour),
			ValidUntil:          now.Add(time.Hour),
			CanStackWithVoucher: false,
		}}, nil)
	store.EXPECT().
		GetUserVoucher(gomock.Any(), voucherID).
		Times(1).
		Return(db.GetUserVoucherRow{
			ID:                voucherID,
			UserID:            userID,
			Status:            "unused",
			ExpiresAt:         now.Add(time.Hour),
			MerchantID:        merchantID,
			Amount:            200,
			MinOrderAmount:    1000,
			AllowedOrderTypes: []string{"takeaway"},
		}, nil)

	_, err := service.CreateOrder(context.Background(), CreateOrderCommandInput{
		UserID:        userID,
		MerchantID:    merchantID,
		OrderType:     "takeaway",
		UserVoucherID: &voucherID,
		Items: []OrderItemInput{{
			DishID:   &dishID,
			Quantity: 1,
		}},
	})

	reqErr := assertRequestError(t, err)
	require.Equal(t, 400, reqErr.Status)
	require.Equal(t, "当前活动不可与所选优惠券叠加", reqErr.Err.Error())
}
