package logic

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/maps"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type createOrderNormalizerStub struct{}

func (createOrderNormalizerStub) Normalize(context.Context, int64, map[string]interface{}) ([]byte, int64, error) {
	return nil, 0, nil
}

type createOrderClockStub struct {
	now time.Time
}

func (s createOrderClockStub) Now() time.Time {
	return s.now
}

type createOrderIDGeneratorStub struct {
	orderNo string
}

func (s createOrderIDGeneratorStub) OrderNo(time.Time) (string, error) {
	return s.orderNo, nil
}

func (s createOrderIDGeneratorStub) PickupCode(time.Time) (string, error) {
	return "", nil
}

func (s createOrderIDGeneratorStub) OutTradeNo(string, time.Time) (string, error) {
	return "", nil
}

func (s createOrderIDGeneratorStub) OutRefundNo(time.Time) (string, error) {
	return "", nil
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

func TestOrderServiceCreateOrderMapsVoucherTemplateUnavailable(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	service := NewOrderService(
		store,
		nil,
		nil,
		nil,
		nil,
		createOrderNormalizerStub{},
		nil,
		nil,
		createOrderClockStub{now: time.Date(2026, 6, 10, 10, 0, 0, 0, time.UTC)},
		createOrderIDGeneratorStub{orderNo: "ORDER-VOUCHER-TEMPLATE-BLOCK"},
		nil,
	)

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
		Return(db.Dish{ID: dishID, MerchantID: merchantID, Name: "菜品", Price: 2000, IsOnline: true, IsAvailable: true}, nil)
	store.EXPECT().
		CountActivePackagingDishesByMerchant(gomock.Any(), merchantID).
		Times(1).
		Return(int64(0), nil)
	store.EXPECT().
		ListActiveDiscountRules(gomock.Any(), merchantID).
		Times(1).
		Return([]db.DiscountRule{}, nil)
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
	store.EXPECT().
		CreateOrderTx(gomock.Any(), gomock.Any()).
		Times(1).
		Return(db.CreateOrderTxResult{}, db.ErrVoucherTemplateUnavailable)

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
	require.Equal(t, "优惠券已停用或已失效", reqErr.Err.Error())
	require.ErrorIs(t, err, db.ErrVoucherTemplateUnavailable)
}

func TestOrderServiceCreateOrder_MarketingTotalsMatchCartAndOrderPreview(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)
	service := NewOrderService(
		store,
		nil,
		nil,
		nil,
		nil,
		createOrderNormalizerStub{},
		nil,
		nil,
		createOrderClockStub{now: time.Date(2026, 6, 9, 10, 0, 0, 0, time.UTC)},
		createOrderIDGeneratorStub{orderNo: "ORDER-MARKETING-PARITY"},
		nil,
	)

	ctx := context.Background()
	userID := int64(20)
	merchantID := int64(10)
	dishID := int64(30)
	addressID := int64(40)
	voucherID := int64(50)
	expectedDistance := int32(2400)
	now := time.Now()
	merchant := db.Merchant{
		ID:        merchantID,
		Status:    "active",
		IsOpen:    true,
		RegionID:  7,
		Latitude:  numericFromFloat(30.0),
		Longitude: numericFromFloat(120.0),
	}
	address := db.UserAddress{
		ID:        addressID,
		UserID:    userID,
		RegionID:  7,
		Latitude:  numericFromFloat(30.01),
		Longitude: numericFromFloat(120.01),
	}
	discountRule := db.DiscountRule{
		ID:                  1,
		Name:                "满减",
		MinOrderAmount:      1000,
		DiscountAmount:      300,
		ValidFrom:           now.Add(-time.Hour),
		ValidUntil:          now.Add(time.Hour),
		CanStackWithVoucher: true,
	}
	voucher := db.GetUserVoucherRow{
		ID:                voucherID,
		UserID:            userID,
		Status:            "unused",
		ExpiresAt:         now.Add(time.Hour),
		MerchantID:        merchantID,
		Amount:            200,
		MinOrderAmount:    1000,
		AllowedOrderTypes: []string{db.OrderTypeTakeout},
		Name:              "代金券",
	}
	mapClient := &fakeMapClient{
		route: &maps.RouteResult{Distance: int(expectedDistance), Duration: 600},
	}
	feeCalculator := func(_ context.Context, regionID, feeMerchantID int64, distance int32, orderAmount int64) (DeliveryFeeComputation, error) {
		require.Equal(t, int64(7), regionID)
		require.Equal(t, merchantID, feeMerchantID)
		require.Equal(t, expectedDistance, distance)
		require.Equal(t, int64(2000), orderAmount)
		return DeliveryFeeComputation{
			Fee:      int64(distance / 4),
			Discount: int64(distance / 24),
		}, nil
	}
	cartItem := db.ListCartItemsRow{
		DishID:    pgtype.Int8{Int64: dishID, Valid: true},
		DishName:  pgtype.Text{String: "菜品", Valid: true},
		DishPrice: pgtype.Int8{Int64: 2000, Valid: true},
		Quantity:  1,
	}

	store.EXPECT().
		GetCartByUserAndMerchant(gomock.Any(), db.GetCartByUserAndMerchantParams{
			UserID:     userID,
			MerchantID: merchantID,
			OrderType:  db.OrderTypeTakeout,
		}).
		Times(1).
		Return(db.Cart{ID: 69}, nil)
	store.EXPECT().
		ListCartItems(gomock.Any(), int64(69)).
		Times(1).
		Return([]db.ListCartItemsRow{cartItem}, nil)
	store.EXPECT().
		GetUserAddress(gomock.Any(), addressID).
		Times(1).
		Return(address, nil)
	store.EXPECT().
		ListActiveDiscountRules(gomock.Any(), merchantID).
		Times(1).
		Return([]db.DiscountRule{discountRule}, nil)
	store.EXPECT().
		ListUserAvailableVouchersForMerchant(gomock.Any(), db.ListUserAvailableVouchersForMerchantParams{
			UserID:         userID,
			MerchantID:     merchantID,
			MinOrderAmount: 2000,
		}).
		Times(1).
		Return([]db.ListUserAvailableVouchersForMerchantRow{}, nil)
	store.EXPECT().
		GetUserVoucher(gomock.Any(), voucherID).
		Times(1).
		Return(voucher, nil)
	store.EXPECT().
		GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{MerchantID: merchantID, UserID: userID}).
		Times(1).
		Return(db.MerchantMembership{}, db.ErrRecordNotFound)
	store.EXPECT().
		GetMerchantAvgPrepareTime(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.GetMerchantAvgPrepareTimeParams) (int64, error) {
			require.Equal(t, merchantID, arg.MerchantID)
			return 0, nil
		})

	cartPreview, err := CalculateCartPreview(
		ctx,
		store,
		mapClient,
		merchant,
		feeCalculator,
		CartPreviewInput{
			UserID:     userID,
			MerchantID: merchantID,
			OrderType:  db.OrderTypeTakeout,
			AddressID:  &addressID,
			VoucherID:  &voucherID,
		},
	)
	require.NoError(t, err)

	store.EXPECT().
		GetCartByUserAndMerchant(gomock.Any(), db.GetCartByUserAndMerchantParams{
			UserID:     userID,
			MerchantID: merchantID,
			OrderType:  db.OrderTypeTakeout,
		}).
		Times(1).
		Return(db.Cart{ID: 70}, nil)
	store.EXPECT().
		ListCartItems(gomock.Any(), int64(70)).
		Times(1).
		Return([]db.ListCartItemsRow{cartItem}, nil)
	store.EXPECT().
		GetMerchant(gomock.Any(), merchantID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		GetUserAddress(gomock.Any(), addressID).
		Times(1).
		Return(address, nil)
	store.EXPECT().
		GetUserVoucher(gomock.Any(), voucherID).
		Times(2).
		Return(voucher, nil)
	store.EXPECT().
		ListActiveDiscountRules(gomock.Any(), merchantID).
		Times(2).
		Return([]db.DiscountRule{discountRule}, nil)
	store.EXPECT().
		ListUserAvailableVouchersForMerchant(gomock.Any(), db.ListUserAvailableVouchersForMerchantParams{
			UserID:         userID,
			MerchantID:     merchantID,
			MinOrderAmount: 2000,
		}).
		Times(1).
		Return([]db.ListUserAvailableVouchersForMerchantRow{}, nil)
	store.EXPECT().
		GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{MerchantID: merchantID, UserID: userID}).
		Times(1).
		Return(db.MerchantMembership{}, db.ErrRecordNotFound)

	preview, err := CalculateOrderPreview(
		ctx,
		store,
		mapClient,
		OrderCalculationInput{
			UserID:        userID,
			MerchantID:    merchantID,
			OrderType:     db.OrderTypeTakeout,
			AddressID:     &addressID,
			UserVoucherID: &voucherID,
		},
		func(context.Context, int64, map[string]interface{}) ([]byte, int64, error) {
			return nil, 0, nil
		},
		feeCalculator,
	)
	require.NoError(t, err)
	require.Equal(t, cartPreview.Subtotal, preview.Subtotal)
	require.Equal(t, cartPreview.DeliveryFee, preview.DeliveryFee)
	require.Equal(t, cartPreview.DeliveryFeeDiscount, preview.DeliveryFeeDiscount)
	require.NotNil(t, cartPreview.Promotion)
	require.Equal(t, int64(300), cartPreview.Promotion.MerchantDiscount)
	require.Equal(t, int64(200), cartPreview.Promotion.VoucherDiscount)
	require.Equal(t, cartPreview.Promotion.MerchantDiscount+cartPreview.Promotion.VoucherDiscount, preview.DiscountAmount)
	require.Equal(t, preview.TotalAmount, cartPreview.Promotion.TotalAmount)
	requireOrderPromotion(t, preview.Promotions, "merchant", "满减", 300)
	requireOrderPromotion(t, preview.Promotions, "voucher", "代金券", 200)
	requireOrderPromotion(t, preview.Promotions, "delivery", "代取费减免", 100)

	store.EXPECT().
		GetMerchant(gomock.Any(), merchantID).
		Times(1).
		Return(merchant, nil)
	store.EXPECT().
		GetMerchantProfile(gomock.Any(), merchantID).
		Times(1).
		Return(db.GetMerchantProfileRow{MerchantID: merchantID, IsTakeoutSuspended: false}, nil)
	store.EXPECT().
		GetDish(gomock.Any(), dishID).
		Times(1).
		Return(db.Dish{ID: dishID, MerchantID: merchantID, Name: "菜品", Price: 2000, IsOnline: true, IsAvailable: true}, nil)
	store.EXPECT().
		CountActivePackagingDishesByMerchant(gomock.Any(), merchantID).
		Times(1).
		Return(int64(0), nil)
	store.EXPECT().
		GetUserAddress(gomock.Any(), addressID).
		Times(1).
		Return(address, nil)
	store.EXPECT().
		ListActiveDiscountRules(gomock.Any(), merchantID).
		Times(1).
		Return([]db.DiscountRule{discountRule}, nil)
	store.EXPECT().
		GetUserVoucher(gomock.Any(), voucherID).
		Times(1).
		Return(voucher, nil)
	store.EXPECT().
		GetActiveBehaviorBlocklist(gomock.Any(), db.GetActiveBehaviorBlocklistParams{
			EntityType: "user",
			EntityID:   userID,
		}).
		Times(1).
		Return(db.BehaviorBlocklist{}, db.ErrRecordNotFound)
	store.EXPECT().
		CreateOrderTx(gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, arg db.CreateOrderTxParams) (db.CreateOrderTxResult, error) {
			require.Equal(t, preview.Subtotal, arg.CreateOrderParams.Subtotal)
			require.Equal(t, preview.DeliveryFee, arg.CreateOrderParams.DeliveryFee)
			require.Equal(t, preview.DeliveryFeeDiscount, arg.CreateOrderParams.DeliveryFeeDiscount)
			require.Equal(t, cartPreview.Promotion.MerchantDiscount, arg.CreateOrderParams.DiscountAmount)
			require.Equal(t, cartPreview.Promotion.VoucherDiscount, arg.CreateOrderParams.VoucherAmount)
			require.Equal(t, preview.TotalAmount, arg.CreateOrderParams.TotalAmount)
			return db.CreateOrderTxResult{Order: db.Order{
				ID:                  90,
				OrderNo:             arg.CreateOrderParams.OrderNo,
				UserID:              userID,
				MerchantID:          merchantID,
				OrderType:           db.OrderTypeTakeout,
				Subtotal:            arg.CreateOrderParams.Subtotal,
				DeliveryFee:         arg.CreateOrderParams.DeliveryFee,
				DeliveryFeeDiscount: arg.CreateOrderParams.DeliveryFeeDiscount,
				DiscountAmount:      arg.CreateOrderParams.DiscountAmount,
				VoucherAmount:       arg.CreateOrderParams.VoucherAmount,
				TotalAmount:         arg.CreateOrderParams.TotalAmount,
				Status:              arg.CreateOrderParams.Status,
			}}, nil
		})

	createResult, err := service.CreateOrder(ctx, CreateOrderCommandInput{
		UserID:                userID,
		MerchantID:            merchantID,
		OrderType:             db.OrderTypeTakeout,
		AddressID:             &addressID,
		UserVoucherID:         &voucherID,
		MapClient:             mapClient,
		DeliveryFeeCalculator: feeCalculator,
		Items: []OrderItemInput{{
			DishID:   &dishID,
			Quantity: 1,
		}},
	})
	require.NoError(t, err)
	require.Equal(t, preview.TotalAmount, createResult.Order.TotalAmount)
}

func requireOrderPromotion(t *testing.T, promotions []OrderPromotion, promotionType, title string, amount int64) {
	t.Helper()

	for _, promotion := range promotions {
		if promotion.Type == promotionType && promotion.Title == title && promotion.Amount == amount {
			return
		}
	}
	require.Failf(t, "missing promotion", "type=%s title=%s amount=%d promotions=%+v", promotionType, title, amount, promotions)
}
