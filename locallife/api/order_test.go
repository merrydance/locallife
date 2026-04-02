package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgtype"

	mockdb "github.com/merrydance/locallife/db/mock"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/merrydance/locallife/worker"
	mockworker "github.com/merrydance/locallife/worker/mock"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func dishWithCustomizationsFromDish(dish db.Dish) db.GetDishWithCustomizationsRow {
	return db.GetDishWithCustomizationsRow{
		ID:                  dish.ID,
		MerchantID:          dish.MerchantID,
		CategoryID:          dish.CategoryID,
		Name:                dish.Name,
		Description:         dish.Description,
		ImageMediaAssetID:   dish.ImageMediaAssetID,
		Price:               dish.Price,
		MemberPrice:         dish.MemberPrice,
		IsAvailable:         dish.IsAvailable,
		IsOnline:            dish.IsOnline,
		SortOrder:           dish.SortOrder,
		CreatedAt:           dish.CreatedAt,
		UpdatedAt:           dish.UpdatedAt,
		PrepareTime:         dish.PrepareTime,
		DeletedAt:           dish.DeletedAt,
		MonthlySales:        dish.MonthlySales,
		RepurchaseRate:      dish.RepurchaseRate,
		CustomizationGroups: []interface{}{},
	}
}

func expectNoPackagingPolicy(store *mockdb.MockStore) {
	store.EXPECT().
		GetMerchantPackagingPolicy(gomock.Any(), gomock.Any()).
		AnyTimes().
		Return(db.MerchantPackagingPolicy{}, db.ErrRecordNotFound)
}

func orderWithDetailsFromOrder(order db.Order) db.GetOrderWithDetailsRow {
	return db.GetOrderWithDetailsRow{
		ID:                   order.ID,
		OrderNo:              order.OrderNo,
		UserID:               order.UserID,
		MerchantID:           order.MerchantID,
		OrderType:            order.OrderType,
		AddressID:            order.AddressID,
		DeliveryFee:          order.DeliveryFee,
		DeliveryDistance:     order.DeliveryDistance,
		TableID:              order.TableID,
		ReservationID:        order.ReservationID,
		Subtotal:             order.Subtotal,
		DiscountAmount:       order.DiscountAmount,
		DeliveryFeeDiscount:  order.DeliveryFeeDiscount,
		TotalAmount:          order.TotalAmount,
		Status:               order.Status,
		PaymentMethod:        order.PaymentMethod,
		PaidAt:               order.PaidAt,
		Notes:                order.Notes,
		CreatedAt:            order.CreatedAt,
		UpdatedAt:            order.UpdatedAt,
		CompletedAt:          order.CompletedAt,
		CancelledAt:          order.CancelledAt,
		CancelReason:         order.CancelReason,
		FinalAmount:          order.FinalAmount,
		PlatformCommission:   order.PlatformCommission,
		UserVoucherID:        order.UserVoucherID,
		VoucherAmount:        order.VoucherAmount,
		BalancePaid:          order.BalancePaid,
		MembershipID:         order.MembershipID,
		FulfillmentStatus:    order.FulfillmentStatus,
		ReplacedByOrderID:    order.ReplacedByOrderID,
		PickupCode:           order.PickupCode,
		DispatchOrderID:      order.DispatchOrderID,
		FlowID:               order.FlowID,
		StatusHint:           order.StatusHint,
		Badges:               order.Badges,
		ExceptionState:       order.ExceptionState,
		ClaimChannel:         order.ClaimChannel,
		Overtime:             order.Overtime,
		PrepStartAt:          order.PrepStartAt,
		ReadyAt:              order.ReadyAt,
		CourierAcceptAt:      order.CourierAcceptAt,
		PickedAt:             order.PickedAt,
		RiderDeliveredAt:     order.RiderDeliveredAt,
		UserDeliveredAt:      order.UserDeliveredAt,
		AutoUserDeliveredAt:  order.AutoUserDeliveredAt,
		MerchantName:         "",
		MerchantPhone:        "",
		MerchantAddress:      "",
		DeliveryContactName:  "",
		DeliveryContactPhone: "",
		DeliveryAddress:      "",
	}
}

func TestCreateOrderAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	// 设置商户状态为 active（order.go 验证需要）
	merchant.Status = "active"
	merchant.IsOpen = true
	dish := randomDish(merchant.ID, nil)
	region := randomRegion()
	address := randomUserAddress(user.ID, region.ID)
	table := randomTable(merchant.ID)
	openSession := db.DiningSession{
		ID:         util.RandomInt(1, 1000),
		MerchantID: merchant.ID,
		TableID:    table.ID,
		UserID:     user.ID + 1,
		Status:     "open",
		OpenedAt:   time.Now(),
		CreatedAt:  time.Now(),
	}
	billingGroup := db.BillingGroup{
		ID:              util.RandomInt(1, 1000),
		DiningSessionID: openSession.ID,
		Status:          "open",
		IsDefault:       false,
		TotalAmount:     0,
		PaidAmount:      0,
		CreatedAt:       time.Now(),
	}
	member := db.BillingGroupMember{
		ID:             util.RandomInt(1, 1000),
		BillingGroupID: billingGroup.ID,
		UserID:         user.ID,
		Role:           "member",
		JoinedAt:       time.Now(),
	}

	testCases := []struct {
		name          string
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name: "DineInBillingGroupNotMember",
			body: gin.H{
				"merchant_id":      merchant.ID,
				"order_type":       "dine_in",
				"table_id":         table.ID,
				"billing_group_id": billingGroup.ID,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)
				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)
				store.EXPECT().
					GetActiveDiningSessionByTable(gomock.Any(), table.ID).
					Times(1).
					Return(openSession, nil)
				store.EXPECT().
					GetBillingGroup(gomock.Any(), billingGroup.ID).
					Times(1).
					Return(billingGroup, nil)
				store.EXPECT().
					GetActiveBillingGroupMember(gomock.Any(), db.GetActiveBillingGroupMemberParams{
						BillingGroupID: billingGroup.ID,
						UserID:         user.ID,
					}).
					Times(1).
					Return(db.BillingGroupMember{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "DineInBillingGroupMismatch",
			body: gin.H{
				"merchant_id":      merchant.ID,
				"order_type":       "dine_in",
				"table_id":         table.ID,
				"billing_group_id": billingGroup.ID,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)
				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)
				store.EXPECT().
					GetActiveDiningSessionByTable(gomock.Any(), table.ID).
					Times(1).
					Return(openSession, nil)
				mismatch := billingGroup
				mismatch.DiningSessionID = openSession.ID + 1
				store.EXPECT().
					GetBillingGroup(gomock.Any(), billingGroup.ID).
					Times(1).
					Return(mismatch, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusConflict, recorder.Code)
			},
		},
		{
			name: "DineInBillingGroupMemberOK",
			body: gin.H{
				"merchant_id":      merchant.ID,
				"order_type":       "dine_in",
				"table_id":         table.ID,
				"billing_group_id": billingGroup.ID,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)
				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)
				store.EXPECT().
					GetActiveDiningSessionByTable(gomock.Any(), table.ID).
					Times(1).
					Return(openSession, nil)
				store.EXPECT().
					GetBillingGroup(gomock.Any(), billingGroup.ID).
					Times(1).
					Return(billingGroup, nil)
				store.EXPECT().
					GetActiveBillingGroupMember(gomock.Any(), db.GetActiveBillingGroupMemberParams{
						BillingGroupID: billingGroup.ID,
						UserID:         user.ID,
					}).
					Times(1).
					Return(member, nil)
				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)
				store.EXPECT().
					GetDishWithCustomizations(gomock.Any(), dish.ID).
					Times(1).
					Return(dishWithCustomizationsFromDish(dish), nil)
				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)
				store.EXPECT().
					CreateOrderTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateOrderTxResult{
						Order: db.Order{
							ID:          1,
							OrderNo:     "20240101120000123456",
							UserID:      user.ID,
							MerchantID:  merchant.ID,
							OrderType:   "dine_in",
							Subtotal:    dish.Price,
							TotalAmount: dish.Price,
							Status:      "pending",
							CreatedAt:   time.Now(),
						},
					}, nil)
				store.EXPECT().
					UpdateDiningSessionActiveOrder(gomock.Any(), gomock.Any()).
					Times(1)
				store.EXPECT().
					GetCartByUserAndMerchant(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Cart{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
			},
		},
		{
			name: "OK",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "takeaway",
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 2,
					},
				},
				"notes": "少辣",
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)
				store.EXPECT().
					GetDishWithCustomizations(gomock.Any(), dish.ID).
					Times(1).
					Return(dishWithCustomizationsFromDish(dish), nil)

				// 满减规则查询
				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				store.EXPECT().
					CreateOrderTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateOrderTxResult{
						Order: db.Order{
							ID:          1,
							OrderNo:     "20240101120000123456",
							UserID:      user.ID,
							MerchantID:  merchant.ID,
							OrderType:   "takeaway",
							Subtotal:    dish.Price * 2,
							TotalAmount: dish.Price * 2,
							Status:      "pending",
							CreatedAt:   time.Now(),
						},
						Items: []db.OrderItem{
							{
								ID:        1,
								OrderID:   1,
								DishID:    pgtype.Int8{Int64: dish.ID, Valid: true},
								Name:      dish.Name,
								UnitPrice: dish.Price,
								Quantity:  2,
								Subtotal:  dish.Price * 2,
							},
						},
					}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
			},
		},
		{
			name: "MerchantNotFound",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "takeaway",
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(db.Merchant{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "InvalidOrderType",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "invalid",
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "TakeoutRequiresAddress",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "takeout",
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "DineInRequiresTable",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "dine_in",
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "TakeoutAddressNotFound",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "takeout",
				"address_id":  int64(99999),
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)
				// calculateOrderItems 会先调用 GetDish
				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)
				store.EXPECT().
					GetDishWithCustomizations(gomock.Any(), dish.ID).
					Times(1).
					Return(dishWithCustomizationsFromDish(dish), nil)
				store.EXPECT().
					GetUserAddress(gomock.Any(), int64(99999)).
					Times(1).
					Return(db.UserAddress{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "TakeoutAddressBelongsToOtherUser",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "takeout",
				"address_id":  address.ID,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)
				// calculateOrderItems 会先调用 GetDish
				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)
				store.EXPECT().
					GetDishWithCustomizations(gomock.Any(), dish.ID).
					Times(1).
					Return(dishWithCustomizationsFromDish(dish), nil)
				// 返回属于其他用户的地址
				otherUserAddress := address
				otherUserAddress.UserID = user.ID + 1
				store.EXPECT().
					GetUserAddress(gomock.Any(), address.ID).
					Times(1).
					Return(otherUserAddress, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "DineInTableNotFound",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "dine_in",
				"table_id":    int64(99999),
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)
				store.EXPECT().
					GetTable(gomock.Any(), int64(99999)).
					Times(1).
					Return(db.Table{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "DineInTableBelongsToOtherMerchant",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "dine_in",
				"table_id":    table.ID,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)
				// 返回属于其他商户的桌台
				otherMerchantTable := table
				otherMerchantTable.MerchantID = merchant.ID + 1
				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(otherMerchantTable, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "NoAuth",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "takeaway",
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
					},
				},
			},
			setupAuth:  func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
		{
			name: "MerchantNotActive",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "takeaway",
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				inactiveMerchant := merchant
				inactiveMerchant.Status = "inactive"
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(inactiveMerchant, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "DishOffline",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "takeaway",
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				offlineDish := dish
				offlineDish.IsOnline = false
				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(offlineDish, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "ItemsExceedsMax",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "takeaway",
				"items":       generateManyItems(dish.ID, 51), // max=50
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "ItemsAtMax_OK",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "takeaway",
				"items":       generateManyItems(dish.ID, 50), // max=50, should pass validation
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				// 50个商品的GetDish调用
				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(50).
					Return(dish, nil)
				store.EXPECT().
					GetDishWithCustomizations(gomock.Any(), dish.ID).
					Times(50).
					Return(dishWithCustomizationsFromDish(dish), nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				store.EXPECT().
					CreateOrderTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CreateOrderTxResult{
						Order: db.Order{
							ID:          1,
							OrderNo:     "20240101120000123456",
							UserID:      user.ID,
							MerchantID:  merchant.ID,
							OrderType:   "takeaway",
							Subtotal:    dish.Price * 50,
							TotalAmount: dish.Price * 50,
							Status:      "pending",
							CreatedAt:   time.Now(),
						},
						Items: []db.OrderItem{},
					}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
			},
		},
		{
			name: "CustomizationsExceedsMax",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "takeaway",
				"items": []gin.H{
					{
						"dish_id":        dish.ID,
						"quantity":       1,
						"customizations": generateManyCustomizations(11), // max=10
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "CustomizationNameTooLong",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "takeaway",
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
						"customizations": []gin.H{
							{
								"name":        generateLongString(51), // max=50
								"value":       "medium",
								"extra_price": 0,
							},
						},
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "CustomizationValueTooLong",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "takeaway",
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
						"customizations": []gin.H{
							{
								"name":        "spicy",
								"value":       generateLongString(51), // max=50
								"extra_price": 0,
							},
						},
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "CustomizationExtraPriceNegative",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "takeaway",
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
						"customizations": []gin.H{
							{
								"name":        "spicy",
								"value":       "medium",
								"extra_price": -100, // min=0
							},
						},
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "CustomizationExtraPriceExceedsMax",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "takeaway",
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1,
						"customizations": []gin.H{
							{
								"name":        "spicy",
								"value":       "medium",
								"extra_price": 10001, // max=10000
							},
						},
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
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
			expectNoPackagingPolicy(store)
			store.EXPECT().
				GetMerchantProfile(gomock.Any(), gomock.Any()).
				AnyTimes().
				Return(db.GetMerchantProfileRow{}, db.ErrRecordNotFound)
			store.EXPECT().
				GetActiveBehaviorBlocklist(gomock.Any(), gomock.Any()).
				AnyTimes().
				Return(db.BehaviorBlocklist{}, db.ErrRecordNotFound)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/orders"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestGetOrderAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	order := randomOrder(user.ID, merchant.ID)

	testCases := []struct {
		name          string
		orderID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrderWithDetails(gomock.Any(), order.ID).
					Times(1).
					Return(orderWithDetailsFromOrder(order), nil)

				store.EXPECT().
					ListOrderItemsWithDishByOrder(gomock.Any(), order.ID).
					Times(1).
					Return([]db.ListOrderItemsWithDishByOrderRow{}, nil)

				store.EXPECT().
					GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
					AnyTimes().
					Return(db.PaymentOrder{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "NotFound",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrderWithDetails(gomock.Any(), order.ID).
					Times(1).
					Return(db.GetOrderWithDetailsRow{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:    "Forbidden",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID+1, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrderWithDetails(gomock.Any(), order.ID).
					Times(1).
					Return(orderWithDetailsFromOrder(order), nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:      "NoAuth",
			orderID:   order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			store.EXPECT().
				GetMerchantProfile(gomock.Any(), gomock.Any()).
				AnyTimes().
				Return(db.GetMerchantProfileRow{}, db.ErrRecordNotFound)
			store.EXPECT().
				GetActiveBehaviorBlocklist(gomock.Any(), gomock.Any()).
				AnyTimes().
				Return(db.BehaviorBlocklist{}, db.ErrRecordNotFound)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/orders/%d", tc.orderID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestCancelOrderAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	order := randomOrder(user.ID, merchant.ID)

	testCases := []struct {
		name          string
		orderID       int64
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			orderID: order.ID,
			body:    gin.H{"reason": "不想要了"},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), order.ID).
					Times(1).
					Return(order, nil)

				store.EXPECT().
					CancelOrderTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CancelOrderTxResult{
						Order: db.Order{
							ID:           order.ID,
							OrderNo:      order.OrderNo,
							UserID:       order.UserID,
							MerchantID:   order.MerchantID,
							Status:       "cancelled",
							CancelledAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
							CancelReason: pgtype.Text{String: "不想要了", Valid: true},
						},
					}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "AlreadyPaid_WithRefund",
			orderID: order.ID,
			body:    gin.H{},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				paidOrder := order
				paidOrder.Status = "paid"
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), order.ID).
					Times(1).
					Return(paidOrder, nil)
				store.EXPECT().
					CancelOrderTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CancelOrderTxResult{Order: paidOrder}, nil)
				store.EXPECT().
					GetPaymentOrdersByOrder(gomock.Any(), gomock.Eq(pgtype.Int8{Int64: order.ID, Valid: true})).
					Times(1).
					Return([]db.PaymentOrder{{ID: 1, OrderID: pgtype.Int8{Int64: order.ID, Valid: true}, Status: "paid", Amount: 10 * fenPerYuan}}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "AlreadyPreparing",
			orderID: order.ID,
			body:    gin.H{},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				preparingOrder := order
				preparingOrder.Status = "preparing"
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), order.ID).
					Times(1).
					Return(preparingOrder, nil)

				store.EXPECT().
					UpdateOrderExceptionState(gomock.Any(), gomock.Any()).
					Times(1).
					Return(preparingOrder, nil)

				store.EXPECT().
					CreateOrderStatusLog(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.OrderStatusLog{}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
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
			store.EXPECT().
				GetMerchantProfile(gomock.Any(), gomock.Any()).
				AnyTimes().
				Return(db.GetMerchantProfileRow{}, db.ErrRecordNotFound)
			store.EXPECT().
				GetActiveBehaviorBlocklist(gomock.Any(), gomock.Any()).
				AnyTimes().
				Return(db.BehaviorBlocklist{}, db.ErrRecordNotFound)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := fmt.Sprintf("/v1/orders/%d/cancel", tc.orderID)
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

// Helper function for order tests
func randomOrder(userID, merchantID int64) db.Order {
	return db.Order{
		ID:          util.RandomInt(1, 1000),
		OrderNo:     fmt.Sprintf("%d%d", time.Now().Unix(), util.RandomInt(100000, 999999)),
		UserID:      userID,
		MerchantID:  merchantID,
		OrderType:   "takeaway",
		Subtotal:    util.RandomInt(1000, 10000),
		TotalAmount: util.RandomInt(1000, 10000),
		Status:      "pending",
		CreatedAt:   time.Now(),
	}
}

// ==================== 商户端订单管理测试 ====================

func TestListMerchantOrdersAPI(t *testing.T) {
	merchantOwner, _ := randomUser(t)
	merchant := randomMerchant(merchantOwner.ID)
	otherUser, _ := randomUser(t)

	orders := []db.Order{
		randomOrder(otherUser.ID, merchant.ID),
		randomOrder(otherUser.ID, merchant.ID),
	}

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)

				store.EXPECT().
					ListOrdersByMerchantWithFilters(gomock.Any(), gomock.Eq(db.ListOrdersByMerchantWithFiltersParams{
						MerchantID: merchant.ID,
						Status:     pgtype.Text{},
						OrderType:  pgtype.Text{},
						Limit:      10,
						Offset:     0,
					})).
					Times(1).
					Return(orders, nil)

				store.EXPECT().
					CountOrdersByMerchantWithFilters(gomock.Any(), gomock.Eq(db.CountOrdersByMerchantWithFiltersParams{
						MerchantID: merchant.ID,
						Status:     pgtype.Text{},
						OrderType:  pgtype.Text{},
					})).
					Times(1).
					Return(int64(len(orders)), nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "NotAMerchant",
			query: "page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, otherUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveNoAccessibleMerchants(store, otherUser.ID)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:  "InvalidPageID",
			query: "page_id=0&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "FilterByStatus",
			query: "page_id=1&page_size=10&status=paid",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)

				store.EXPECT().
					ListOrdersByMerchantWithFilters(gomock.Any(), gomock.Eq(db.ListOrdersByMerchantWithFiltersParams{
						MerchantID: merchant.ID,
						Status:     pgtype.Text{String: "paid", Valid: true},
						OrderType:  pgtype.Text{},
						Limit:      10,
						Offset:     0,
					})).
					Times(1).
					Return(orders, nil)

				store.EXPECT().
					CountOrdersByMerchantWithFilters(gomock.Any(), db.CountOrdersByMerchantWithFiltersParams{
						MerchantID: merchant.ID,
						Status:     pgtype.Text{String: "paid", Valid: true},
						OrderType:  pgtype.Text{},
					}).
					Times(1).
					Return(int64(len(orders)), nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "InvalidStatus",
			query: "page_id=1&page_size=10&status=invalid",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
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
			store.EXPECT().
				GetMerchantProfile(gomock.Any(), gomock.Any()).
				AnyTimes().
				Return(db.GetMerchantProfileRow{}, db.ErrRecordNotFound)
			store.EXPECT().
				GetActiveBehaviorBlocklist(gomock.Any(), gomock.Any()).
				AnyTimes().
				Return(db.BehaviorBlocklist{}, db.ErrRecordNotFound)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/merchant/orders?%s", tc.query)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestAcceptOrderAPI(t *testing.T) {
	merchantOwner, _ := randomUser(t)
	merchant := randomMerchant(merchantOwner.ID)
	otherMerchantOwner, _ := randomUser(t)
	otherMerchant := randomMerchant(otherMerchantOwner.ID)
	otherMerchant.ID = merchant.ID + 1
	customer, _ := randomUser(t)

	paidOrder := randomOrder(customer.ID, merchant.ID)
	paidOrder.Status = "paid"

	pendingOrder := randomOrder(customer.ID, merchant.ID)
	pendingOrder.Status = "pending"

	testCases := []struct {
		name          string
		orderID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			orderID: paidOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)

				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), paidOrder.ID).
					Times(1).
					Return(paidOrder, nil)

				acceptedOrder := paidOrder
				acceptedOrder.Status = "preparing"
				store.EXPECT().
					UpdateOrderStatusTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.UpdateOrderStatusTxResult{Order: acceptedOrder}, nil)

				store.EXPECT().
					GetOrderDisplayConfigByMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(db.OrderDisplayConfig{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "OrderNotBelongToMerchant",
			orderID: paidOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, otherMerchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, otherMerchantOwner.ID, otherMerchant)

				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), paidOrder.ID).
					Times(1).
					Return(paidOrder, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:    "OrderNotPaid",
			orderID: pendingOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)

				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), pendingOrder.ID).
					Times(1).
					Return(pendingOrder, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:    "OrderNotFound",
			orderID: 99999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)

				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), int64(99999)).
					Times(1).
					Return(db.Order{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
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
			store.EXPECT().
				GetMerchantProfile(gomock.Any(), gomock.Any()).
				AnyTimes().
				Return(db.GetMerchantProfileRow{}, db.ErrRecordNotFound)
			store.EXPECT().
				GetActiveBehaviorBlocklist(gomock.Any(), gomock.Any()).
				AnyTimes().
				Return(db.BehaviorBlocklist{}, db.ErrRecordNotFound)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/merchant/orders/%d/accept", tc.orderID)
			request, err := http.NewRequest(http.MethodPost, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestRejectOrderAPI(t *testing.T) {
	merchantOwner, _ := randomUser(t)
	merchant := randomMerchant(merchantOwner.ID)
	customer, _ := randomUser(t)

	paidOrder := randomOrder(customer.ID, merchant.ID)
	paidOrder.Status = "paid"

	testCases := []struct {
		name          string
		orderID       int64
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			orderID: paidOrder.ID,
			body:    gin.H{"reason": "材料售罄"},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)

				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), paidOrder.ID).
					Times(1).
					Return(paidOrder, nil)

				rejectedOrder := paidOrder
				rejectedOrder.Status = "cancelled"
				store.EXPECT().
					CancelOrderTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CancelOrderTxResult{Order: rejectedOrder}, nil)
				// 退款相关调用
				store.EXPECT().
					GetLatestPaymentOrderByOrder(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.PaymentOrder{}, db.ErrRecordNotFound) // 模拟无支付订单
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "MissingReason",
			orderID: paidOrder.ID,
			body:    gin.H{},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:    "ReasonTooShort",
			orderID: paidOrder.ID,
			body:    gin.H{"reason": "x"},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
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
			expectNoPackagingPolicy(store)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := fmt.Sprintf("/v1/merchant/orders/%d/reject", tc.orderID)
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestGetOrderStatsAPI(t *testing.T) {
	merchantOwner, _ := randomUser(t)
	merchant := randomMerchant(merchantOwner.ID)
	otherUser, _ := randomUser(t)

	stats := db.GetOrderStatsRow{
		PendingCount:    2,
		PaidCount:       5,
		PreparingCount:  3,
		ReadyCount:      1,
		DeliveringCount: 2,
		CompletedCount:  10,
		CancelledCount:  1,
	}

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "start_date=2025-01-01&end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)

				store.EXPECT().
					GetOrderStats(gomock.Any(), gomock.Any()).
					Times(1).
					Return(stats, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "NotAMerchant",
			query: "start_date=2025-01-01&end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, otherUser.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveNoAccessibleMerchants(store, otherUser.ID)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:  "InvalidDateFormat",
			query: "start_date=2025/01/01&end_date=2025-01-31",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "DateRangeExceeds90Days",
			query: "start_date=2025-01-01&end_date=2025-06-01",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "StartDateAfterEndDate",
			query: "start_date=2025-02-01&end_date=2025-01-01",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
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
			expectNoPackagingPolicy(store)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/merchant/orders/stats?%s", tc.query)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

// generateManyItems 生成指定数量的订单项（使用同一个dish_id）
func generateManyItems(dishID int64, count int) []gin.H {
	items := make([]gin.H, count)
	for i := 0; i < count; i++ {
		items[i] = gin.H{
			"dish_id":  dishID,
			"quantity": 1,
		}
	}
	return items
}

// generateCustomizations 生成指定数量的定制项
func generateCustomizations(count int) []gin.H {
	customizations := make([]gin.H, count)
	for i := 0; i < count; i++ {
		customizations[i] = gin.H{
			"name":  fmt.Sprintf("Option%d", i+1),
			"value": fmt.Sprintf("Value%d", i+1),
		}
	}
	return customizations
}

// generateManyCustomizations 生成指定数量的定制项
func generateManyCustomizations(count int) []gin.H {
	return generateCustomizations(count)
}

// generateLongString 生成指定长度的字符串
func generateLongString(length int) string {
	return strings.Repeat("a", length)
}

// ==================== 用户端订单操作测试 ====================

func TestUrgeOrderAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	order := randomOrder(user.ID, merchant.ID)

	testCases := []struct {
		name          string
		orderID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK_Paid",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				paidOrder := order
				paidOrder.Status = "paid"
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(paidOrder, nil)
				store.EXPECT().
					CountRecentOrderStatusLogs(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), nil)
				store.EXPECT().
					CreateOrderStatusLog(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.OrderStatusLog{}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "OK_Preparing",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				preparingOrder := order
				preparingOrder.Status = "preparing"
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(preparingOrder, nil)
				store.EXPECT().
					CountRecentOrderStatusLogs(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), nil)
				store.EXPECT().
					CreateOrderStatusLog(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.OrderStatusLog{}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "OK_Delivering",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				deliveringOrder := order
				deliveringOrder.Status = "delivering"
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(deliveringOrder, nil)
				store.EXPECT().
					CountRecentOrderStatusLogs(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), nil)
				store.EXPECT().
					GetDeliveryByOrderID(gomock.Any(), order.ID).
					Times(1).
					Return(db.Delivery{}, db.ErrRecordNotFound) // 无骑手信息
				store.EXPECT().
					CreateOrderStatusLog(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.OrderStatusLog{}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "OrderNotFound",
			orderID: 99999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), int64(99999)).
					Times(1).
					Return(db.Order{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:    "OrderNotBelongToUser",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID+1, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(order, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:    "InvalidStatus_Pending",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				pendingOrder := order
				pendingOrder.Status = "pending"
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(pendingOrder, nil)
				store.EXPECT().
					CountRecentOrderStatusLogs(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:    "InvalidStatus_Completed",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				completedOrder := order
				completedOrder.Status = "completed"
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(completedOrder, nil)
				store.EXPECT().
					CountRecentOrderStatusLogs(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:      "NoAuth",
			orderID:   order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			expectNoPackagingPolicy(store)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/orders/%d/urge", tc.orderID)
			request, err := http.NewRequest(http.MethodPost, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestConfirmOrderAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	order := randomOrder(user.ID, merchant.ID)
	order.OrderType = "takeout"

	testCases := []struct {
		name          string
		orderID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				deliveringOrder := order
				deliveringOrder.Status = "rider_delivered"
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(deliveringOrder, nil)

				completedOrder := order
				completedOrder.Status = "completed"
				store.EXPECT().
					CompleteTakeoutOrderByUser(gomock.Any(), order.ID).
					Times(1).
					Return(completedOrder, nil)
				store.EXPECT().
					CreateOrderStatusLog(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.OrderStatusLog{}, nil)
				store.EXPECT().
					GetDeliveryByOrderID(gomock.Any(), order.ID).
					Times(1).
					Return(db.Delivery{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "OrderNotFound",
			orderID: 99999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetOrder(gomock.Any(), int64(99999)).
					Times(1).
					Return(db.Order{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:    "OrderNotBelongToUser",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID+1, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				deliveringOrder := order
				deliveringOrder.Status = "rider_delivered"
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(deliveringOrder, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:    "NotTakeoutOrder",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				dineInOrder := order
				dineInOrder.OrderType = "dine_in"
				dineInOrder.Status = "rider_delivered"
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(dineInOrder, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:    "NotDeliveringStatus",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				paidOrder := order
				paidOrder.Status = "paid"
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(paidOrder, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:      "NoAuth",
			orderID:   order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
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

			url := fmt.Sprintf("/v1/orders/%d/confirm", tc.orderID)
			request, err := http.NewRequest(http.MethodPost, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

// ==================== 商户端订单操作测试 ====================

func TestGetMerchantOrderAPI(t *testing.T) {
	merchantOwner, _ := randomUser(t)
	merchant := randomMerchant(merchantOwner.ID)
	otherMerchantOwner, _ := randomUser(t)
	otherMerchant := randomMerchant(otherMerchantOwner.ID)
	otherMerchant.ID = merchant.ID + 1
	customer, _ := randomUser(t)

	order := randomOrder(customer.ID, merchant.ID)

	testCases := []struct {
		name          string
		orderID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(order, nil)
				store.EXPECT().
					ListOrderItemsWithDishByOrder(gomock.Any(), order.ID).
					Times(1).
					Return([]db.ListOrderItemsWithDishByOrderRow{}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "OrderNotFound",
			orderID: 99999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
				store.EXPECT().
					GetOrder(gomock.Any(), int64(99999)).
					Times(1).
					Return(db.Order{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:    "OrderNotBelongToMerchant",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, otherMerchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, otherMerchantOwner.ID, otherMerchant)
				store.EXPECT().
					GetOrder(gomock.Any(), order.ID).
					Times(1).
					Return(order, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:    "NotAMerchant",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, customer.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveNoAccessibleMerchants(store, customer.ID)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:      "NoAuth",
			orderID:   order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
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

			url := fmt.Sprintf("/v1/merchant/orders/%d", tc.orderID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestMarkOrderReadyAPI(t *testing.T) {
	merchantOwner, _ := randomUser(t)
	merchant := randomMerchant(merchantOwner.ID)
	otherMerchantOwner, _ := randomUser(t)
	otherMerchant := randomMerchant(otherMerchantOwner.ID)
	otherMerchant.ID = merchant.ID + 1
	customer, _ := randomUser(t)

	preparingOrder := randomOrder(customer.ID, merchant.ID)
	preparingOrder.Status = "preparing"

	testCases := []struct {
		name          string
		orderID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			orderID: preparingOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), preparingOrder.ID).
					Times(1).
					Return(preparingOrder, nil)

				readyOrder := preparingOrder
				readyOrder.Status = "ready"
				store.EXPECT().
					UpdateOrderStatusTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.UpdateOrderStatusTxResult{Order: readyOrder}, nil)

				store.EXPECT().
					GetOrderDisplayConfigByMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(db.OrderDisplayConfig{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "OrderNotPreparing",
			orderID: preparingOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
				paidOrder := preparingOrder
				paidOrder.Status = "paid"
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), preparingOrder.ID).
					Times(1).
					Return(paidOrder, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:    "OrderNotBelongToMerchant",
			orderID: preparingOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, otherMerchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, otherMerchantOwner.ID, otherMerchant)
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), preparingOrder.ID).
					Times(1).
					Return(preparingOrder, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:    "OrderNotFound",
			orderID: 99999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), int64(99999)).
					Times(1).
					Return(db.Order{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:      "NoAuth",
			orderID:   preparingOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
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

			url := fmt.Sprintf("/v1/merchant/orders/%d/ready", tc.orderID)
			request, err := http.NewRequest(http.MethodPost, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestCompleteOrderAPI(t *testing.T) {
	merchantOwner, _ := randomUser(t)
	merchant := randomMerchant(merchantOwner.ID)
	otherMerchantOwner, _ := randomUser(t)
	otherMerchant := randomMerchant(otherMerchantOwner.ID)
	otherMerchant.ID = merchant.ID + 1
	customer, _ := randomUser(t)

	readyOrder := randomOrder(customer.ID, merchant.ID)
	readyOrder.Status = "ready"
	readyOrder.OrderType = "dine_in" // 堂食订单

	testCases := []struct {
		name          string
		orderID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK_DineIn",
			orderID: readyOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), readyOrder.ID).
					Times(1).
					Return(readyOrder, nil)

				completedOrder := readyOrder
				completedOrder.Status = "completed"
				store.EXPECT().
					CompleteOrderTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CompleteOrderTxResult{Order: completedOrder}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "OK_Takeaway",
			orderID: readyOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
				takeawayOrder := readyOrder
				takeawayOrder.OrderType = "takeaway"
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), readyOrder.ID).
					Times(1).
					Return(takeawayOrder, nil)

				completedOrder := takeawayOrder
				completedOrder.Status = "completed"
				store.EXPECT().
					CompleteOrderTx(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.CompleteOrderTxResult{Order: completedOrder}, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "TakeoutOrderCannotComplete",
			orderID: readyOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
				takeoutOrder := readyOrder
				takeoutOrder.OrderType = "takeout"
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), readyOrder.ID).
					Times(1).
					Return(takeoutOrder, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:    "OrderNotReady",
			orderID: readyOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
				preparingOrder := readyOrder
				preparingOrder.Status = "preparing"
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), readyOrder.ID).
					Times(1).
					Return(preparingOrder, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:    "OrderNotBelongToMerchant",
			orderID: readyOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, otherMerchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, otherMerchantOwner.ID, otherMerchant)
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), readyOrder.ID).
					Times(1).
					Return(readyOrder, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name:    "OrderNotFound",
			orderID: 99999,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
				store.EXPECT().
					GetOrderForUpdate(gomock.Any(), int64(99999)).
					Times(1).
					Return(db.Order{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:      "NoAuth",
			orderID:   readyOrder.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
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

			url := fmt.Sprintf("/v1/merchant/orders/%d/complete", tc.orderID)
			request, err := http.NewRequest(http.MethodPost, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestPrintMerchantOrderAPI(t *testing.T) {
	merchantOwner, _ := randomUser(t)
	merchant := randomMerchant(merchantOwner.ID)
	otherMerchantOwner, _ := randomUser(t)
	otherMerchant := randomMerchant(otherMerchantOwner.ID)
	otherMerchant.ID = merchant.ID + 1
	customer, _ := randomUser(t)

	order := randomOrder(customer.ID, merchant.ID)
	order.Status = db.OrderStatusPreparing
	order.OrderType = db.OrderTypeTakeout

	testCases := []struct {
		name          string
		orderID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore, distributor *mockworker.MockTaskDistributor)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, distributor *mockworker.MockTaskDistributor) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
				store.EXPECT().GetOrder(gomock.Any(), order.ID).Times(1).Return(order, nil)
				store.EXPECT().GetOrderDisplayConfigByMerchant(gomock.Any(), merchant.ID).Times(1).Return(db.OrderDisplayConfig{
					MerchantID:        merchant.ID,
					EnablePrint:       true,
					PrintTakeout:      true,
					PrintDineIn:       true,
					PrintReservation:  true,
					PrintDispatchMode: "single_full",
					PrintTriggerMode:  "manual",
				}, nil)
				distributor.EXPECT().DistributeTaskPrintOrder(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(_ any, payload *worker.PrintOrderPayload, _ ...asynq.Option) error {
					require.Equal(t, order.ID, payload.OrderID)
					require.Equal(t, "manual", payload.Trigger)
					require.NotEmpty(t, payload.TaskKey)
					return nil
				})
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:    "ManualModeDisabled",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, distributor *mockworker.MockTaskDistributor) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
				store.EXPECT().GetOrder(gomock.Any(), order.ID).Times(1).Return(order, nil)
				store.EXPECT().GetOrderDisplayConfigByMerchant(gomock.Any(), merchant.ID).Times(1).Return(db.OrderDisplayConfig{
					MerchantID:        merchant.ID,
					EnablePrint:       true,
					PrintTakeout:      true,
					PrintDineIn:       true,
					PrintReservation:  true,
					PrintDispatchMode: "single_full",
					PrintTriggerMode:  "accepted",
				}, nil)
				distributor.EXPECT().DistributeTaskPrintOrder(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:    "OrderNotBelongToMerchant",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, otherMerchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore, distributor *mockworker.MockTaskDistributor) {
				expectResolveSingleOwnedMerchant(store, otherMerchantOwner.ID, otherMerchant)
				store.EXPECT().GetOrder(gomock.Any(), order.ID).Times(1).Return(order, nil)
				distributor.EXPECT().DistributeTaskPrintOrder(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
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
			distributor := mockworker.NewMockTaskDistributor(ctrl)
			tc.buildStubs(store, distributor)

			server := newTestServer(t, store)
			server.SetTaskDistributorForTest(distributor)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/merchant/orders/%d/print-jobs", tc.orderID)
			request, err := http.NewRequest(http.MethodPost, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestListMerchantOrderPrintJobsAPI(t *testing.T) {
	merchantOwner, _ := randomUser(t)
	merchant := randomMerchant(merchantOwner.ID)
	otherMerchantOwner, _ := randomUser(t)
	otherMerchant := randomMerchant(otherMerchantOwner.ID)
	otherMerchant.ID = merchant.ID + 1
	customer, _ := randomUser(t)

	order := randomOrder(customer.ID, merchant.ID)
	now := time.Now()
	printedAt := pgtype.Timestamptz{Time: now.Add(-time.Minute), Valid: true}
	failedMessage := pgtype.Text{String: "printer offline", Valid: true}
	printLogs := []db.ListPrintLogsByOrderRow{
		{
			ID:            1,
			OrderID:       order.ID,
			PrinterID:     101,
			PrinterName:   "前台打印机",
			Status:        "success",
			VendorOrderID: pgtype.Text{String: "vendor-1", Valid: true},
			PrintedAt:     printedAt,
			CreatedAt:     now.Add(-2 * time.Minute),
		},
		{
			ID:           2,
			OrderID:      order.ID,
			PrinterID:    102,
			PrinterName:  "后厨打印机",
			Status:       "failed",
			ErrorMessage: failedMessage,
			CreatedAt:    now.Add(-time.Minute),
		},
	}

	testCases := []struct {
		name          string
		orderID       int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:    "OK",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
				store.EXPECT().GetOrder(gomock.Any(), order.ID).Times(1).Return(order, nil)
				store.EXPECT().ListOrderItemsWithDishByOrder(gomock.Any(), order.ID).Times(1).Return([]db.ListOrderItemsWithDishByOrderRow{}, nil)
				store.EXPECT().ListPrintLogsByOrder(gomock.Any(), order.ID).Times(1).Return(printLogs, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp listMerchantOrderPrintJobsResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, order.ID, resp.OrderID)
				require.Len(t, resp.Items, 2)
				require.Equal(t, "前台打印机", resp.Items[0].PrinterName)
				require.NotNil(t, resp.Items[0].VendorOrderID)
				require.Equal(t, "vendor-1", *resp.Items[0].VendorOrderID)
				require.Nil(t, resp.Items[0].ErrorMessage)
				require.NotNil(t, resp.Items[0].PrintedAt)
				require.Equal(t, "printer offline", *resp.Items[1].ErrorMessage)
			},
		},
		{
			name:    "OrderNotBelongToMerchant",
			orderID: order.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, otherMerchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, otherMerchantOwner.ID, otherMerchant)
				store.EXPECT().GetOrder(gomock.Any(), order.ID).Times(1).Return(order, nil)
				store.EXPECT().ListPrintLogsByOrder(gomock.Any(), gomock.Any()).Times(0)
				store.EXPECT().ListOrderItemsWithDishByOrder(gomock.Any(), gomock.Any()).Times(0)
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

			url := fmt.Sprintf("/v1/merchant/orders/%d/print-jobs", tc.orderID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestGetMerchantOrderPrintJobStatusAPI(t *testing.T) {
	merchantOwner, _ := randomUser(t)
	merchant := randomMerchant(merchantOwner.ID)
	customer, _ := randomUser(t)
	order := randomOrder(customer.ID, merchant.ID)
	printer := randomCloudPrinter(merchant.ID)
	printLog := db.PrintLog{
		ID:            9001,
		OrderID:       order.ID,
		PrinterID:     printer.ID,
		Status:        "success",
		VendorOrderID: pgtype.Text{String: "vendor-job-1", Valid: true},
	}

	testCases := []struct {
		name          string
		orderID       int64
		printLogID    int64
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		buildClient   func() *printerClientStub
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder, client *printerClientStub)
	}{
		{
			name:       "OK",
			orderID:    order.ID,
			printLogID: printLog.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
				store.EXPECT().GetOrder(gomock.Any(), order.ID).Times(1).Return(order, nil)
				store.EXPECT().ListOrderItemsWithDishByOrder(gomock.Any(), order.ID).Times(1).Return([]db.ListOrderItemsWithDishByOrderRow{}, nil)
				store.EXPECT().GetPrintLog(gomock.Any(), printLog.ID).Times(1).Return(printLog, nil)
				store.EXPECT().GetCloudPrinter(gomock.Any(), printer.ID).Times(1).Return(printer, nil)
			},
			buildClient: func() *printerClientStub {
				return &printerClientStub{queryPrinted: true}
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder, client *printerClientStub) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp merchantOrderPrintJobStatusResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, printLog.ID, resp.PrintLogID)
				require.True(t, resp.CloudQueryAvailable)
				require.NotNil(t, resp.CloudPrinted)
				require.True(t, *resp.CloudPrinted)
				require.Equal(t, "vendor-job-1", client.queryOrderID)
			},
		},
		{
			name:       "NoVendorOrderID",
			orderID:    order.ID,
			printLogID: printLog.ID,
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
				store.EXPECT().GetOrder(gomock.Any(), order.ID).Times(1).Return(order, nil)
				store.EXPECT().ListOrderItemsWithDishByOrder(gomock.Any(), order.ID).Times(1).Return([]db.ListOrderItemsWithDishByOrderRow{}, nil)
				noVendorLog := printLog
				noVendorLog.VendorOrderID = pgtype.Text{}
				store.EXPECT().GetPrintLog(gomock.Any(), printLog.ID).Times(1).Return(noVendorLog, nil)
				store.EXPECT().GetCloudPrinter(gomock.Any(), printer.ID).Times(1).Return(printer, nil)
			},
			buildClient: func() *printerClientStub { return &printerClientStub{} },
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder, client *printerClientStub) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp merchantOrderPrintJobStatusResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.False(t, resp.CloudQueryAvailable)
				require.Nil(t, resp.CloudPrinted)
				require.Empty(t, client.queryOrderID)
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
			client := tc.buildClient()
			server.SetPrinterClientForTest(client)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/merchant/orders/%d/print-jobs/%d/status", tc.orderID, tc.printLogID)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder, client)
		})
	}
}

func TestListMerchantPrintAnomaliesAPI(t *testing.T) {
	merchantOwner, _ := randomUser(t)
	merchant := randomMerchant(merchantOwner.ID)
	now := time.Now()

	testCases := []struct {
		name          string
		query         string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "?page_id=1&page_size=20",
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
				store.EXPECT().ListMerchantPrintAnomalies(gomock.Any(), gomock.Eq(db.ListMerchantPrintAnomaliesParams{
					MerchantID: merchant.ID,
					Limit:      20,
					Offset:     0,
					Status:     pgtype.Text{},
				})).Times(1).Return([]db.ListMerchantPrintAnomaliesRow{{
					ID:           11,
					OrderID:      101,
					OrderNo:      "ORD-101",
					OrderType:    db.OrderTypeTakeout,
					PrinterID:    201,
					PrinterName:  "前台打印机",
					PrinterType:  printerTypeFeieyun,
					IsActive:     true,
					Status:       "failed",
					ErrorMessage: pgtype.Text{String: "printer offline", Valid: true},
					CreatedAt:    now,
				}}, nil)
				store.EXPECT().CountMerchantPrintAnomalies(gomock.Any(), gomock.Eq(db.CountMerchantPrintAnomaliesParams{
					MerchantID: merchant.ID,
					Status:     pgtype.Text{},
				})).Times(1).Return(int64(1), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp listMerchantPrintAnomaliesResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Len(t, resp.Items, 1)
				require.Equal(t, int64(1), resp.Total)
				require.True(t, resp.Items[0].CanRetry)
				require.NotNil(t, resp.Items[0].ErrorMessage)
				require.Equal(t, "printer offline", *resp.Items[0].ErrorMessage)
			},
		},
		{
			name:  "StatusFilter",
			query: "?status=pending&page_size=10",
			buildStubs: func(store *mockdb.MockStore) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
				store.EXPECT().ListMerchantPrintAnomalies(gomock.Any(), gomock.Eq(db.ListMerchantPrintAnomaliesParams{
					MerchantID: merchant.ID,
					Limit:      10,
					Offset:     0,
					Status:     pgtype.Text{String: "pending", Valid: true},
				})).Times(1).Return([]db.ListMerchantPrintAnomaliesRow{{
					ID:          12,
					OrderID:     102,
					OrderNo:     "ORD-102",
					OrderType:   db.OrderTypeTakeout,
					PrinterID:   202,
					PrinterName: "后厨打印机",
					PrinterType: printerTypeFeieyun,
					IsActive:    false,
					Status:      "pending",
					CreatedAt:   now,
				}}, nil)
				store.EXPECT().CountMerchantPrintAnomalies(gomock.Any(), gomock.Eq(db.CountMerchantPrintAnomaliesParams{
					MerchantID: merchant.ID,
					Status:     pgtype.Text{String: "pending", Valid: true},
				})).Times(1).Return(int64(1), nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp listMerchantPrintAnomaliesResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Len(t, resp.Items, 1)
				require.False(t, resp.Items[0].CanRetry)
				require.Equal(t, "printer is inactive", resp.Items[0].RetryHint)
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

			request, err := http.NewRequest(http.MethodGet, "/v1/merchant/orders/print-anomalies"+tc.query, nil)
			require.NoError(t, err)
			addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestRetryMerchantOrderPrintJobAPI(t *testing.T) {
	merchantOwner, _ := randomUser(t)
	merchant := randomMerchant(merchantOwner.ID)
	customer, _ := randomUser(t)
	order := randomOrder(customer.ID, merchant.ID)
	printer := randomCloudPrinter(merchant.ID)
	printLog := db.PrintLog{
		ID:        9002,
		OrderID:   order.ID,
		PrinterID: printer.ID,
		Status:    "failed",
	}

	testCases := []struct {
		name          string
		printLog      db.PrintLog
		buildStubs    func(store *mockdb.MockStore, distributor *mockworker.MockTaskDistributor, currentPrintLog db.PrintLog)
		checkResponse func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:     "OK",
			printLog: printLog,
			buildStubs: func(store *mockdb.MockStore, distributor *mockworker.MockTaskDistributor, currentPrintLog db.PrintLog) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
				store.EXPECT().GetOrder(gomock.Any(), order.ID).Times(1).Return(order, nil)
				store.EXPECT().ListOrderItemsWithDishByOrder(gomock.Any(), order.ID).Times(1).Return([]db.ListOrderItemsWithDishByOrderRow{}, nil)
				store.EXPECT().GetPrintLog(gomock.Any(), currentPrintLog.ID).Times(1).Return(currentPrintLog, nil)
				store.EXPECT().GetLatestPrintLogByOrderAndPrinter(gomock.Any(), db.GetLatestPrintLogByOrderAndPrinterParams{OrderID: order.ID, PrinterID: printer.ID}).Times(1).Return(currentPrintLog, nil)
				store.EXPECT().GetCloudPrinter(gomock.Any(), printer.ID).Times(1).Return(printer, nil)
				distributor.EXPECT().DistributeTaskPrintOrder(gomock.Any(), gomock.Any()).Times(1).DoAndReturn(func(_ any, payload *worker.PrintOrderPayload, _ ...asynq.Option) error {
					require.Equal(t, order.ID, payload.OrderID)
					require.Equal(t, "retry", payload.Trigger)
					require.Equal(t, currentPrintLog.ID, payload.RetryPrintLogID)
					require.NotEmpty(t, payload.TaskKey)
					return nil
				})
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
				var resp retryMerchantOrderPrintJobResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &resp)
				require.Equal(t, order.ID, resp.OrderID)
				require.Equal(t, printLog.ID, resp.PrintLogID)
				require.Equal(t, "retry", resp.Trigger)
			},
		},
		{
			name:     "PrinterInactive",
			printLog: printLog,
			buildStubs: func(store *mockdb.MockStore, distributor *mockworker.MockTaskDistributor, currentPrintLog db.PrintLog) {
				inactivePrinter := printer
				inactivePrinter.IsActive = false
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
				store.EXPECT().GetOrder(gomock.Any(), order.ID).Times(1).Return(order, nil)
				store.EXPECT().ListOrderItemsWithDishByOrder(gomock.Any(), order.ID).Times(1).Return([]db.ListOrderItemsWithDishByOrderRow{}, nil)
				store.EXPECT().GetPrintLog(gomock.Any(), currentPrintLog.ID).Times(1).Return(currentPrintLog, nil)
				store.EXPECT().GetLatestPrintLogByOrderAndPrinter(gomock.Any(), db.GetLatestPrintLogByOrderAndPrinterParams{OrderID: order.ID, PrinterID: printer.ID}).Times(1).Return(currentPrintLog, nil)
				store.EXPECT().GetCloudPrinter(gomock.Any(), printer.ID).Times(1).Return(inactivePrinter, nil)
				distributor.EXPECT().DistributeTaskPrintOrder(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "SuccessfulPrintLogRejected",
			printLog: func() db.PrintLog {
				successLog := printLog
				successLog.Status = "success"
				return successLog
			}(),
			buildStubs: func(store *mockdb.MockStore, distributor *mockworker.MockTaskDistributor, currentPrintLog db.PrintLog) {
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
				store.EXPECT().GetOrder(gomock.Any(), order.ID).Times(1).Return(order, nil)
				store.EXPECT().ListOrderItemsWithDishByOrder(gomock.Any(), order.ID).Times(1).Return([]db.ListOrderItemsWithDishByOrderRow{}, nil)
				store.EXPECT().GetPrintLog(gomock.Any(), currentPrintLog.ID).Times(1).Return(currentPrintLog, nil)
				distributor.EXPECT().DistributeTaskPrintOrder(gomock.Any(), gomock.Any()).Times(0)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:     "OldFailedPrintLogRejected",
			printLog: printLog,
			buildStubs: func(store *mockdb.MockStore, distributor *mockworker.MockTaskDistributor, currentPrintLog db.PrintLog) {
				latestPrintLog := currentPrintLog
				latestPrintLog.ID = currentPrintLog.ID + 1
				latestPrintLog.Status = "failed"
				expectResolveSingleOwnedMerchant(store, merchantOwner.ID, merchant)
				store.EXPECT().GetOrder(gomock.Any(), order.ID).Times(1).Return(order, nil)
				store.EXPECT().ListOrderItemsWithDishByOrder(gomock.Any(), order.ID).Times(1).Return([]db.ListOrderItemsWithDishByOrderRow{}, nil)
				store.EXPECT().GetPrintLog(gomock.Any(), currentPrintLog.ID).Times(1).Return(currentPrintLog, nil)
				store.EXPECT().GetLatestPrintLogByOrderAndPrinter(gomock.Any(), db.GetLatestPrintLogByOrderAndPrinterParams{OrderID: order.ID, PrinterID: printer.ID}).Times(1).Return(latestPrintLog, nil)
				distributor.EXPECT().DistributeTaskPrintOrder(gomock.Any(), gomock.Any()).Times(0)
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
			distributor := mockworker.NewMockTaskDistributor(ctrl)
			tc.buildStubs(store, distributor, tc.printLog)

			server := newTestServer(t, store)
			server.SetTaskDistributorForTest(distributor)
			recorder := httptest.NewRecorder()

			url := fmt.Sprintf("/v1/merchant/orders/%d/print-jobs/%d/retry", order.ID, tc.printLog.ID)
			request, err := http.NewRequest(http.MethodPost, url, nil)
			require.NoError(t, err)
			addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, merchantOwner.ID, time.Minute)

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestCancelOrderAPI_ReasonTooLong(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	order := randomOrder(user.ID, merchant.ID)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockdb.NewMockStore(ctrl)

	server := newTestServer(t, store)
	recorder := httptest.NewRecorder()

	// 超过500字符的取消原因
	longReason := generateLongString(501)
	data, err := json.Marshal(gin.H{"reason": longReason})
	require.NoError(t, err)

	url := fmt.Sprintf("/v1/orders/%d/cancel", order.ID)
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
	require.NoError(t, err)

	addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
	server.router.ServeHTTP(recorder, request)

	// 应该返回400，因为reason超过了最大长度
	require.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestListOrdersAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)

	orders := []db.ListOrdersByUserWithFiltersRow{
		{
			ID:           1,
			OrderNo:      "20251210000001",
			UserID:       user.ID,
			MerchantID:   merchant.ID,
			MerchantName: "测试商户",
			OrderType:    "takeaway",
			Subtotal:     1000,
			TotalAmount:  1000,
			Status:       "pending",
			CreatedAt:    time.Now(),
		},
	}

	testCases := []struct {
		name          string
		query         string
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name:  "OK",
			query: "page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListOrdersByUserWithFilters(gomock.Any(), gomock.Any()).
					Times(1).
					Return(orders, nil)
				store.EXPECT().
					CountOrdersByUserWithFilters(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(21), nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)

				var response listOrdersResponse
				requireUnmarshalAPIResponseData(t, recorder.Body.Bytes(), &response)
				require.Len(t, response.Orders, 1)
				require.Equal(t, int64(21), response.Total)
			},
		},
		{
			name:  "OK_WithStatusFilter",
			query: "page_id=1&page_size=10&status=paid",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					ListOrdersByUserWithFilters(gomock.Any(), gomock.Any()).
					Times(1).
					Return([]db.ListOrdersByUserWithFiltersRow{}, nil)
				store.EXPECT().
					CountOrdersByUserWithFilters(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusOK, recorder.Code)
			},
		},
		{
			name:  "InvalidPageID",
			query: "page_id=0&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "InvalidPageSize_TooSmall",
			query: "page_id=1&page_size=4",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "InvalidPageSize_TooLarge",
			query: "page_id=1&page_size=21",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:  "InvalidStatus",
			query: "page_id=1&page_size=10&status=invalid",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name:      "NoAuth",
			query:     "page_id=1&page_size=10",
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {},
			buildStubs: func(store *mockdb.MockStore) {
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
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

			url := fmt.Sprintf("/v1/orders?%s", tc.query)
			request, err := http.NewRequest(http.MethodGet, url, nil)
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

// ==================== 优惠券和余额支付测试 ====================

func TestCreateOrderWithVoucherAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.Status = "active"
	merchant.IsOpen = true
	dish := randomDish(merchant.ID, nil)
	dish.Price = 50 * fenPerYuan // 设置为50元，5个就是250元，满足100元最低消费
	table := randomTable(merchant.ID)
	table.Status = "available"
	session := db.DiningSession{
		ID:         1,
		MerchantID: merchant.ID,
		TableID:    table.ID,
		UserID:     user.ID,
		Status:     "open",
		OpenedAt:   time.Now(),
		CreatedAt:  time.Now(),
	}
	billingGroup := db.BillingGroup{
		ID:              util.RandomInt(1, 1000),
		DiningSessionID: session.ID,
		Status:          "open",
		IsDefault:       true,
		TotalAmount:     0,
		PaidAmount:      0,
		CreatedAt:       time.Now(),
	}
	member := db.BillingGroupMember{
		ID:             util.RandomInt(1, 1000),
		BillingGroupID: billingGroup.ID,
		UserID:         user.ID,
		Role:           "member",
		JoinedAt:       time.Now(),
	}

	// 创建用户优惠券（默认允许所有订单类型）
	userVoucher := db.GetUserVoucherRow{
		ID:                1,
		VoucherID:         1,
		UserID:            user.ID,
		Status:            "unused",
		ExpiresAt:         time.Now().Add(24 * time.Hour),
		MerchantID:        merchant.ID,
		Code:              "VOUCHER001",
		Name:              "满100减10",
		Amount:            10 * fenPerYuan,                                           // 10元
		MinOrderAmount:    100 * fenPerYuan,                                          // 最低100元
		AllowedOrderTypes: []string{"takeout", "dine_in", "takeaway", "reservation"}, // 默认允许所有
	}

	testCases := []struct {
		name          string
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name: "WithVoucher_OK",
			body: gin.H{
				"merchant_id":     merchant.ID,
				"order_type":      "takeaway",
				"user_voucher_id": userVoucher.ID,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 5, // 确保金额>=100
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)
				store.EXPECT().
					GetDishWithCustomizations(gomock.Any(), dish.ID).
					Times(1).
					Return(dishWithCustomizationsFromDish(dish), nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				// 获取用户优惠券
				store.EXPECT().
					GetUserVoucher(gomock.Any(), userVoucher.ID).
					Times(1).
					Return(userVoucher, nil)

				store.EXPECT().
					CreateOrderTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx interface{}, arg db.CreateOrderTxParams) (db.CreateOrderTxResult, error) {
						// 验证优惠券参数传递正确
						require.NotNil(t, arg.UserVoucherID)
						require.Equal(t, userVoucher.ID, *arg.UserVoucherID)
						require.Equal(t, userVoucher.Amount, arg.VoucherAmount)
						return db.CreateOrderTxResult{
							Order: db.Order{
								ID:            1,
								OrderNo:       "20240101120000123456",
								UserID:        user.ID,
								MerchantID:    merchant.ID,
								OrderType:     "takeaway",
								Subtotal:      dish.Price * 5,
								VoucherAmount: userVoucher.Amount,
								TotalAmount:   dish.Price*5 - userVoucher.Amount,
								Status:        "pending",
								CreatedAt:     time.Now(),
							},
						}, nil
					})
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
			},
		},
		{
			name: "VoucherNotFound",
			body: gin.H{
				"merchant_id":     merchant.ID,
				"order_type":      "takeaway",
				"user_voucher_id": int64(9999),
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 5,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)
				store.EXPECT().
					GetDishWithCustomizations(gomock.Any(), dish.ID).
					Times(1).
					Return(dishWithCustomizationsFromDish(dish), nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				store.EXPECT().
					GetUserVoucher(gomock.Any(), int64(9999)).
					Times(1).
					Return(db.GetUserVoucherRow{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name: "VoucherNotOwnedByUser",
			body: gin.H{
				"merchant_id":     merchant.ID,
				"order_type":      "takeaway",
				"user_voucher_id": userVoucher.ID,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 5,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)
				store.EXPECT().
					GetDishWithCustomizations(gomock.Any(), dish.ID).
					Times(1).
					Return(dishWithCustomizationsFromDish(dish), nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				// 返回其他用户的优惠券
				otherUserVoucher := userVoucher
				otherUserVoucher.UserID = user.ID + 999
				store.EXPECT().
					GetUserVoucher(gomock.Any(), userVoucher.ID).
					Times(1).
					Return(otherUserVoucher, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "VoucherAlreadyUsed",
			body: gin.H{
				"merchant_id":     merchant.ID,
				"order_type":      "takeaway",
				"user_voucher_id": userVoucher.ID,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 5,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)
				store.EXPECT().
					GetDishWithCustomizations(gomock.Any(), dish.ID).
					Times(1).
					Return(dishWithCustomizationsFromDish(dish), nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				usedVoucher := userVoucher
				usedVoucher.Status = "used"
				store.EXPECT().
					GetUserVoucher(gomock.Any(), userVoucher.ID).
					Times(1).
					Return(usedVoucher, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "VoucherExpired",
			body: gin.H{
				"merchant_id":     merchant.ID,
				"order_type":      "takeaway",
				"user_voucher_id": userVoucher.ID,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 5,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)
				store.EXPECT().
					GetDishWithCustomizations(gomock.Any(), dish.ID).
					Times(1).
					Return(dishWithCustomizationsFromDish(dish), nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				expiredVoucher := userVoucher
				expiredVoucher.ExpiresAt = time.Now().Add(-1 * time.Hour) // 已过期
				store.EXPECT().
					GetUserVoucher(gomock.Any(), userVoucher.ID).
					Times(1).
					Return(expiredVoucher, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "VoucherWrongMerchant",
			body: gin.H{
				"merchant_id":     merchant.ID,
				"order_type":      "takeaway",
				"user_voucher_id": userVoucher.ID,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 5,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)
				store.EXPECT().
					GetDishWithCustomizations(gomock.Any(), dish.ID).
					Times(1).
					Return(dishWithCustomizationsFromDish(dish), nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				wrongMerchantVoucher := userVoucher
				wrongMerchantVoucher.MerchantID = merchant.ID + 999 // 其他商户
				store.EXPECT().
					GetUserVoucher(gomock.Any(), userVoucher.ID).
					Times(1).
					Return(wrongMerchantVoucher, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "VoucherMinOrderNotMet",
			body: gin.H{
				"merchant_id":     merchant.ID,
				"order_type":      "takeaway",
				"user_voucher_id": userVoucher.ID,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 1, // 金额不足100元
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				// 设置菜品价格为50元
				lowPriceDish := dish
				lowPriceDish.Price = 50 * fenPerYuan
				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(lowPriceDish, nil)
				store.EXPECT().
					GetDishWithCustomizations(gomock.Any(), dish.ID).
					Times(1).
					Return(dishWithCustomizationsFromDish(lowPriceDish), nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				store.EXPECT().
					GetUserVoucher(gomock.Any(), userVoucher.ID).
					Times(1).
					Return(userVoucher, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "VoucherAmountExceedsOrder_TotalBecomesZero",
			body: gin.H{
				"merchant_id":     merchant.ID,
				"order_type":      "takeaway",
				"user_voucher_id": userVoucher.ID,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 5,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				// 菜品价格很低，5个只有100元
				lowPriceDish := dish
				lowPriceDish.Price = 20 * fenPerYuan
				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(lowPriceDish, nil)
				store.EXPECT().
					GetDishWithCustomizations(gomock.Any(), dish.ID).
					Times(1).
					Return(dishWithCustomizationsFromDish(lowPriceDish), nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				// 优惠券金额大于订单金额（满100减200）
				largeVoucher := userVoucher
				largeVoucher.Amount = 200 * fenPerYuan
				largeVoucher.MinOrderAmount = 100 * fenPerYuan
				store.EXPECT().
					GetUserVoucher(gomock.Any(), userVoucher.ID).
					Times(1).
					Return(largeVoucher, nil)

				store.EXPECT().
					CreateOrderTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx interface{}, arg db.CreateOrderTxParams) (db.CreateOrderTxResult, error) {
						// 验证订单总额为0（不能为负）
						require.Equal(t, int64(0), arg.CreateOrderParams.TotalAmount)
						return db.CreateOrderTxResult{
							Order: db.Order{
								ID:            1,
								OrderNo:       "20240101120000123456",
								UserID:        user.ID,
								MerchantID:    merchant.ID,
								OrderType:     "takeaway",
								Subtotal:      2000 * 5,
								VoucherAmount: 20000,
								TotalAmount:   0, // 优惠后为0
								Status:        "pending",
								CreatedAt:     time.Now(),
							},
						}, nil
					})
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
			},
		},
		{
			name: "VoucherOrderTypeNotAllowed_TakeoutVoucherOnDineIn",
			body: gin.H{
				"merchant_id":     merchant.ID,
				"order_type":      "dine_in", // 堂食订单
				"table_id":        table.ID,
				"user_voucher_id": userVoucher.ID,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 5,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				// 堂食订单需要验证桌台
				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					GetActiveDiningSessionByTable(gomock.Any(), table.ID).
					Times(1).
					Return(session, nil)
				store.EXPECT().
					GetDefaultBillingGroupBySession(gomock.Any(), session.ID).
					Times(1).
					Return(billingGroup, nil)
				store.EXPECT().
					GetActiveBillingGroupMember(gomock.Any(), db.GetActiveBillingGroupMemberParams{
						BillingGroupID: billingGroup.ID,
						UserID:         user.ID,
					}).
					Times(1).
					Return(member, nil)

				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)
				store.EXPECT().
					GetDishWithCustomizations(gomock.Any(), dish.ID).
					Times(1).
					Return(dishWithCustomizationsFromDish(dish), nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				// 返回仅限外卖的优惠券
				takeoutOnlyVoucher := userVoucher
				takeoutOnlyVoucher.AllowedOrderTypes = []string{"takeout"} // 仅限外卖
				store.EXPECT().
					GetUserVoucher(gomock.Any(), userVoucher.ID).
					Times(1).
					Return(takeoutOnlyVoucher, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				// 验证错误信息
				var response APIResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Contains(t, response.Message, "不适用于此订单类型")
			},
		},
		{
			name: "VoucherOrderTypeAllowed_DineInVoucherOnDineIn",
			body: gin.H{
				"merchant_id":     merchant.ID,
				"order_type":      "dine_in", // 堂食订单
				"table_id":        table.ID,
				"user_voucher_id": userVoucher.ID,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 5,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)
				store.EXPECT().
					GetDishWithCustomizations(gomock.Any(), dish.ID).
					Times(1).
					Return(dishWithCustomizationsFromDish(dish), nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				// 返回允许堂食和外带的优惠券（店内券）
				dineInVoucher := userVoucher
				dineInVoucher.AllowedOrderTypes = []string{"dine_in", "takeaway"} // 店内券
				store.EXPECT().
					GetUserVoucher(gomock.Any(), userVoucher.ID).
					Times(1).
					Return(dineInVoucher, nil)

				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					GetActiveDiningSessionByTable(gomock.Any(), table.ID).
					Times(1).
					Return(session, nil)
				store.EXPECT().
					GetDefaultBillingGroupBySession(gomock.Any(), session.ID).
					Times(1).
					Return(billingGroup, nil)
				store.EXPECT().
					GetActiveBillingGroupMember(gomock.Any(), db.GetActiveBillingGroupMemberParams{
						BillingGroupID: billingGroup.ID,
						UserID:         user.ID,
					}).
					Times(1).
					Return(member, nil)

				store.EXPECT().
					CreateOrderTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx interface{}, arg db.CreateOrderTxParams) (db.CreateOrderTxResult, error) {
						require.NotNil(t, arg.UserVoucherID)
						return db.CreateOrderTxResult{
							Order: db.Order{
								ID:            1,
								OrderNo:       "20240101120000123456",
								UserID:        user.ID,
								MerchantID:    merchant.ID,
								OrderType:     "dine_in",
								Subtotal:      dish.Price * 5,
								VoucherAmount: dineInVoucher.Amount,
								TotalAmount:   dish.Price*5 - dineInVoucher.Amount,
								Status:        "pending",
								CreatedAt:     time.Now(),
							},
						}, nil
					})

				store.EXPECT().
					UpdateDiningSessionActiveOrder(gomock.Any(), gomock.Any()).
					Times(1).
					Return(session, nil)

				store.EXPECT().
					GetCartByUserAndMerchant(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Cart{ID: util.RandomInt(1, 1000)}, nil)

				store.EXPECT().
					ClearCart(gomock.Any(), gomock.Any()).
					Times(1).
					Return(nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			expectNoPackagingPolicy(store)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/orders"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestCreateOrderWithBalanceAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.Status = "active"
	merchant.IsOpen = true
	dish := randomDish(merchant.ID, nil)
	table := randomTable(merchant.ID)
	session := db.DiningSession{
		ID:         1,
		MerchantID: merchant.ID,
		TableID:    table.ID,
		UserID:     user.ID,
		Status:     "open",
		OpenedAt:   time.Now(),
		CreatedAt:  time.Now(),
	}
	billingGroup := db.BillingGroup{
		ID:              util.RandomInt(1, 1000),
		DiningSessionID: session.ID,
		Status:          "open",
		IsDefault:       true,
		TotalAmount:     0,
		PaidAmount:      0,
		CreatedAt:       time.Now(),
	}
	member := db.BillingGroupMember{
		ID:             util.RandomInt(1, 1000),
		BillingGroupID: billingGroup.ID,
		UserID:         user.ID,
		Role:           "member",
		JoinedAt:       time.Now(),
	}

	// 创建会员卡
	membership := db.MerchantMembership{
		ID:             1,
		MerchantID:     merchant.ID,
		UserID:         user.ID,
		Balance:        500 * fenPerYuan,
		TotalRecharged: 500 * fenPerYuan,
		TotalConsumed:  0,
	}

	// 会员设置
	membershipSettings := db.MerchantMembershipSetting{
		MerchantID:          merchant.ID,
		BalanceUsableScenes: []string{"dine_in", "takeaway"},
		BonusUsableScenes:   []string{"dine_in"},
		AllowWithVoucher:    true,
		AllowWithDiscount:   true,
		MaxDeductionPercent: 100,
	}

	testCases := []struct {
		name          string
		body          gin.H
		setupAuth     func(t *testing.T, request *http.Request, tokenMaker token.Maker)
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(recorder *httptest.ResponseRecorder)
	}{
		{
			name: "WithBalance_DineIn_OK",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "dine_in",
				"table_id":    table.ID,
				"use_balance": true,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 2,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					GetActiveDiningSessionByTable(gomock.Any(), table.ID).
					Times(1).
					Return(session, nil)
				store.EXPECT().
					GetDefaultBillingGroupBySession(gomock.Any(), session.ID).
					Times(1).
					Return(billingGroup, nil)
				store.EXPECT().
					GetActiveBillingGroupMember(gomock.Any(), db.GetActiveBillingGroupMemberParams{
						BillingGroupID: billingGroup.ID,
						UserID:         user.ID,
					}).
					Times(1).
					Return(member, nil)

				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)

				store.EXPECT().
					GetDishWithCustomizations(gomock.Any(), dish.ID).
					Times(1).
					Return(dishWithCustomizationsFromDish(dish), nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				// 获取会员卡
				store.EXPECT().
					GetMembershipByMerchantAndUser(gomock.Any(), db.GetMembershipByMerchantAndUserParams{
						MerchantID: merchant.ID,
						UserID:     user.ID,
					}).
					Times(1).
					Return(membership, nil)

				// 获取会员设置
				store.EXPECT().
					GetMerchantMembershipSettings(gomock.Any(), merchant.ID).
					Times(1).
					Return(membershipSettings, nil)

				store.EXPECT().
					CreateOrderTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx interface{}, arg db.CreateOrderTxParams) (db.CreateOrderTxResult, error) {
						// 验证余额支付参数
						require.NotNil(t, arg.MembershipID)
						require.Equal(t, membership.ID, *arg.MembershipID)
						require.True(t, arg.BalancePaid > 0)
						return db.CreateOrderTxResult{
							Order: db.Order{
								ID:          1,
								OrderNo:     "20240101120000123456",
								UserID:      user.ID,
								MerchantID:  merchant.ID,
								OrderType:   "dine_in",
								Subtotal:    dish.Price * 2,
								BalancePaid: dish.Price * 2,
								TotalAmount: dish.Price * 2,
								Status:      "pending",
								CreatedAt:   time.Now(),
							},
						}, nil
					})
				store.EXPECT().
					UpdateDiningSessionActiveOrder(gomock.Any(), gomock.Any()).
					Times(1).
					Return(session, nil)
				store.EXPECT().
					GetCartByUserAndMerchant(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Cart{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
			},
		},
		{
			name: "WithBalance_Takeout_Rejected",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "takeout",
				"address_id":  int64(1),
				"use_balance": true,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 2,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				merchantWithLocation := merchant
				merchantWithLocation.Latitude = pgtype.Numeric{Int: big.NewInt(312304), Exp: -4, Valid: true}
				merchantWithLocation.Longitude = pgtype.Numeric{Int: big.NewInt(1214737), Exp: -4, Valid: true}

				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchantWithLocation, nil)

				store.EXPECT().
					GetMerchantProfile(gomock.Any(), merchant.ID).
					Times(1).
					Return(db.GetMerchantProfileRow{}, nil)

				region := randomRegion()
				address := randomUserAddress(user.ID, region.ID)
				address.Latitude = pgtype.Numeric{Int: big.NewInt(312304), Exp: -4, Valid: true}
				address.Longitude = pgtype.Numeric{Int: big.NewInt(1214737), Exp: -4, Valid: true}
				store.EXPECT().
					GetUserAddress(gomock.Any(), int64(1)).
					Times(1).
					Return(address, nil)

				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)

				store.EXPECT().
					GetDishWithCustomizations(gomock.Any(), dish.ID).
					Times(1).
					Return(dishWithCustomizationsFromDish(dish), nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				// 配送费计算相关mock
				store.EXPECT().
					GetDeliveryFeeConfigByRegion(gomock.Any(), region.ID).
					Times(1).
					Return(db.DeliveryFeeConfig{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				// 验证错误信息
				var response APIResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Contains(t, response.Message, "外卖和预定订单暂不支持余额支付")
			},
		},
		{
			name: "WithBalance_NotMember",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "dine_in",
				"table_id":    table.ID,
				"use_balance": true,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 2,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					GetActiveDiningSessionByTable(gomock.Any(), table.ID).
					Times(1).
					Return(session, nil)
				store.EXPECT().
					GetDefaultBillingGroupBySession(gomock.Any(), session.ID).
					Times(1).
					Return(billingGroup, nil)

				store.EXPECT().
					GetActiveBillingGroupMember(gomock.Any(), db.GetActiveBillingGroupMemberParams{
						BillingGroupID: billingGroup.ID,
						UserID:         user.ID,
					}).
					Times(1).
					Return(db.BillingGroupMember{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusForbidden, recorder.Code)
			},
		},
		{
			name: "WithBalance_InsufficientBalance",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "dine_in",
				"table_id":    table.ID,
				"use_balance": true,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 2,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					GetActiveDiningSessionByTable(gomock.Any(), table.ID).
					Times(1).
					Return(session, nil)

				store.EXPECT().
					GetDefaultBillingGroupBySession(gomock.Any(), session.ID).
					Times(1).
					Return(billingGroup, nil)

				store.EXPECT().
					GetActiveBillingGroupMember(gomock.Any(), db.GetActiveBillingGroupMemberParams{
						BillingGroupID: billingGroup.ID,
						UserID:         user.ID,
					}).
					Times(1).
					Return(member, nil)

				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)

				store.EXPECT().
					GetDishWithCustomizations(gomock.Any(), dish.ID).
					Times(1).
					Return(dishWithCustomizationsFromDish(dish), nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				// 余额为0
				zeroBalanceMembership := membership
				zeroBalanceMembership.Balance = 0
				store.EXPECT().
					GetMembershipByMerchantAndUser(gomock.Any(), gomock.Any()).
					Times(1).
					Return(zeroBalanceMembership, nil)

				store.EXPECT().
					GetMerchantMembershipSettings(gomock.Any(), merchant.ID).
					Times(1).
					Return(membershipSettings, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "WithBalance_SceneNotAllowed",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "dine_in",
				"table_id":    table.ID,
				"use_balance": true,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 2,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					GetActiveDiningSessionByTable(gomock.Any(), table.ID).
					Times(1).
					Return(session, nil)

				store.EXPECT().
					GetDefaultBillingGroupBySession(gomock.Any(), session.ID).
					Times(1).
					Return(billingGroup, nil)

				store.EXPECT().
					GetActiveBillingGroupMember(gomock.Any(), db.GetActiveBillingGroupMemberParams{
						BillingGroupID: billingGroup.ID,
						UserID:         user.ID,
					}).
					Times(1).
					Return(member, nil)

				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(dish, nil)

				store.EXPECT().
					GetDishWithCustomizations(gomock.Any(), dish.ID).
					Times(1).
					Return(dishWithCustomizationsFromDish(dish), nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				store.EXPECT().
					GetMembershipByMerchantAndUser(gomock.Any(), gomock.Any()).
					Times(1).
					Return(membership, nil)

				// 设置不允许堂食使用余额
				restrictedSettings := membershipSettings
				restrictedSettings.BalanceUsableScenes = []string{"takeaway"} // 只允许自提
				store.EXPECT().
					GetMerchantMembershipSettings(gomock.Any(), merchant.ID).
					Times(1).
					Return(restrictedSettings, nil)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
		{
			name: "WithBalance_PartialPayment_OK",
			body: gin.H{
				"merchant_id": merchant.ID,
				"order_type":  "dine_in",
				"table_id":    table.ID,
				"use_balance": true,
				"items": []gin.H{
					{
						"dish_id":  dish.ID,
						"quantity": 2,
					},
				},
			},
			setupAuth: func(t *testing.T, request *http.Request, tokenMaker token.Maker) {
				addAuthorization(t, request, tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
			},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchant(gomock.Any(), merchant.ID).
					Times(1).
					Return(merchant, nil)

				store.EXPECT().
					GetTable(gomock.Any(), table.ID).
					Times(1).
					Return(table, nil)

				store.EXPECT().
					GetActiveDiningSessionByTable(gomock.Any(), table.ID).
					Times(1).
					Return(session, nil)

				store.EXPECT().
					GetDefaultBillingGroupBySession(gomock.Any(), session.ID).
					Times(1).
					Return(billingGroup, nil)

				store.EXPECT().
					GetActiveBillingGroupMember(gomock.Any(), db.GetActiveBillingGroupMemberParams{
						BillingGroupID: billingGroup.ID,
						UserID:         user.ID,
					}).
					Times(1).
					Return(member, nil)

				// 设置固定价格的菜品：200元/个，2个=400元
				fixedPriceDish := dish
				fixedPriceDish.Price = 200 * fenPerYuan
				store.EXPECT().
					GetDish(gomock.Any(), dish.ID).
					Times(1).
					Return(fixedPriceDish, nil)
				store.EXPECT().
					GetDishWithCustomizations(gomock.Any(), dish.ID).
					Times(1).
					Return(dishWithCustomizationsFromDish(fixedPriceDish), nil)

				store.EXPECT().
					ListActiveDiscountRules(gomock.Any(), merchant.ID).
					Times(1).
					Return([]db.DiscountRule{}, nil)

				// 余额只有100元，不够支付400元的订单
				partialBalanceMembership := membership
				partialBalanceMembership.Balance = 100 * fenPerYuan
				store.EXPECT().
					GetMembershipByMerchantAndUser(gomock.Any(), gomock.Any()).
					Times(1).
					Return(partialBalanceMembership, nil)

				store.EXPECT().
					GetMerchantMembershipSettings(gomock.Any(), merchant.ID).
					Times(1).
					Return(membershipSettings, nil)

				store.EXPECT().
					CreateOrderTx(gomock.Any(), gomock.Any()).
					Times(1).
					DoAndReturn(func(ctx interface{}, arg db.CreateOrderTxParams) (db.CreateOrderTxResult, error) {
						// 验证部分余额支付：订单400元，余额100元，只用100元
						require.NotNil(t, arg.MembershipID)
						require.Equal(t, int64(100*fenPerYuan), arg.BalancePaid) // 只使用100元余额
						return db.CreateOrderTxResult{
							Order: db.Order{
								ID:          1,
								OrderNo:     "20240101120000123456",
								UserID:      user.ID,
								MerchantID:  merchant.ID,
								OrderType:   "dine_in",
								Subtotal:    400 * fenPerYuan, // 200*2
								BalancePaid: 100 * fenPerYuan,
								TotalAmount: 400 * fenPerYuan,
								Status:      "pending",
								CreatedAt:   time.Now(),
							},
						}, nil
					})
				store.EXPECT().
					UpdateDiningSessionActiveOrder(gomock.Any(), gomock.Any()).
					Times(1).
					Return(session, nil)
				store.EXPECT().
					GetCartByUserAndMerchant(gomock.Any(), gomock.Any()).
					Times(1).
					Return(db.Cart{}, db.ErrRecordNotFound)
			},
			checkResponse: func(recorder *httptest.ResponseRecorder) {
				require.Equal(t, http.StatusCreated, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			expectNoPackagingPolicy(store)
			tc.buildStubs(store)

			server := newTestServer(t, store)
			recorder := httptest.NewRecorder()

			data, err := json.Marshal(tc.body)
			require.NoError(t, err)

			url := "/v1/orders"
			request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
			require.NoError(t, err)

			tc.setupAuth(t, request, server.tokenMaker)
			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(recorder)
		})
	}
}

func TestCreateOrderWithVoucherAndBalanceAPI(t *testing.T) {
	user, _ := randomUser(t)
	merchant := randomMerchant(user.ID)
	merchant.Status = "active"
	merchant.IsOpen = true
	dish := randomDish(merchant.ID, nil)
	dish.Price = 200 * fenPerYuan
	table := randomTable(merchant.ID)
	session := db.DiningSession{
		ID:         1,
		MerchantID: merchant.ID,
		TableID:    table.ID,
		UserID:     user.ID,
		Status:     "open",
		OpenedAt:   time.Now(),
		CreatedAt:  time.Now(),
	}
	billingGroup := db.BillingGroup{
		ID:              util.RandomInt(1, 1000),
		DiningSessionID: session.ID,
		Status:          "open",
		IsDefault:       true,
		TotalAmount:     0,
		PaidAmount:      0,
		CreatedAt:       time.Now(),
	}
	member := db.BillingGroupMember{
		ID:             util.RandomInt(1, 1000),
		BillingGroupID: billingGroup.ID,
		UserID:         user.ID,
		Role:           "member",
		JoinedAt:       time.Now(),
	}

	// 创建用户优惠券（允许堂食）
	userVoucher := db.GetUserVoucherRow{
		ID:                1,
		VoucherID:         1,
		UserID:            user.ID,
		Status:            "unused",
		ExpiresAt:         time.Now().Add(24 * time.Hour),
		MerchantID:        merchant.ID,
		Code:              "VOUCHER001",
		Name:              "满100减10",
		Amount:            10 * fenPerYuan,
		MinOrderAmount:    100 * fenPerYuan,
		AllowedOrderTypes: []string{"takeout", "dine_in", "takeaway", "reservation"}, // 允许所有类型
	}

	// 创建会员卡
	membership := db.MerchantMembership{
		ID:             1,
		MerchantID:     merchant.ID,
		UserID:         user.ID,
		Balance:        500 * fenPerYuan,
		TotalRecharged: 500 * fenPerYuan,
		TotalConsumed:  0,
	}

	membershipSettings := db.MerchantMembershipSetting{
		MerchantID:          merchant.ID,
		BalanceUsableScenes: []string{"dine_in", "takeaway"},
		AllowWithVoucher:    true,
		AllowWithDiscount:   true,
		MaxDeductionPercent: 100,
	}

	t.Run("VoucherAndBalance_Combined", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		store := mockdb.NewMockStore(ctrl)
		expectNoPackagingPolicy(store)

		store.EXPECT().
			GetMerchant(gomock.Any(), merchant.ID).
			Times(1).
			Return(merchant, nil)

		store.EXPECT().
			GetTable(gomock.Any(), table.ID).
			Times(1).
			Return(table, nil)

		store.EXPECT().
			GetActiveDiningSessionByTable(gomock.Any(), table.ID).
			Times(1).
			Return(session, nil)
		store.EXPECT().
			GetDefaultBillingGroupBySession(gomock.Any(), session.ID).
			Times(1).
			Return(billingGroup, nil)

		store.EXPECT().
			GetActiveBillingGroupMember(gomock.Any(), db.GetActiveBillingGroupMemberParams{
				BillingGroupID: billingGroup.ID,
				UserID:         user.ID,
			}).
			Times(1).
			Return(member, nil)

		store.EXPECT().
			GetDish(gomock.Any(), dish.ID).
			Times(1).
			Return(dish, nil)

		store.EXPECT().
			GetDishWithCustomizations(gomock.Any(), dish.ID).
			Times(1).
			Return(dishWithCustomizationsFromDish(dish), nil)

		store.EXPECT().
			ListActiveDiscountRules(gomock.Any(), merchant.ID).
			Times(1).
			Return([]db.DiscountRule{}, nil)

		store.EXPECT().
			GetUserVoucher(gomock.Any(), userVoucher.ID).
			Times(1).
			Return(userVoucher, nil)

		store.EXPECT().
			GetMembershipByMerchantAndUser(gomock.Any(), gomock.Any()).
			Times(1).
			Return(membership, nil)

		store.EXPECT().
			GetMerchantMembershipSettings(gomock.Any(), merchant.ID).
			Times(1).
			Return(membershipSettings, nil)

		store.EXPECT().
			CreateOrderTx(gomock.Any(), gomock.Any()).
			Times(1).
			DoAndReturn(func(ctx interface{}, arg db.CreateOrderTxParams) (db.CreateOrderTxResult, error) {
				// 验证同时使用优惠券和余额
				require.NotNil(t, arg.UserVoucherID)
				require.Equal(t, userVoucher.ID, *arg.UserVoucherID)
				require.Equal(t, userVoucher.Amount, arg.VoucherAmount)
				require.NotNil(t, arg.MembershipID)
				require.Equal(t, membership.ID, *arg.MembershipID)
				// 订单金额200元 - 优惠券10元 = 190元，余额500元足够
				require.Equal(t, int64(19000), arg.BalancePaid)
				return db.CreateOrderTxResult{
					Order: db.Order{
						ID:            1,
						OrderNo:       "20240101120000123456",
						UserID:        user.ID,
						MerchantID:    merchant.ID,
						OrderType:     "dine_in",
						Subtotal:      dish.Price,
						VoucherAmount: userVoucher.Amount,
						BalancePaid:   19000,
						TotalAmount:   dish.Price - userVoucher.Amount,
						Status:        "pending",
						CreatedAt:     time.Now(),
					},
				}, nil
			})
		store.EXPECT().
			UpdateDiningSessionActiveOrder(gomock.Any(), gomock.Any()).
			Times(1).
			Return(session, nil)
		store.EXPECT().
			GetCartByUserAndMerchant(gomock.Any(), gomock.Any()).
			Times(1).
			Return(db.Cart{}, db.ErrRecordNotFound)

		server := newTestServer(t, store)
		recorder := httptest.NewRecorder()

		body := gin.H{
			"merchant_id":     merchant.ID,
			"order_type":      "dine_in",
			"table_id":        table.ID,
			"user_voucher_id": userVoucher.ID,
			"use_balance":     true,
			"items": []gin.H{
				{
					"dish_id":  dish.ID,
					"quantity": 1,
				},
			},
		}
		data, err := json.Marshal(body)
		require.NoError(t, err)

		url := "/v1/orders"
		request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(data))
		require.NoError(t, err)

		addAuthorization(t, request, server.tokenMaker, authorizationTypeBearer, user.ID, time.Minute)
		server.router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusCreated, recorder.Code)
	})
}
