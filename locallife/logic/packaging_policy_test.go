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

func TestValidatePackagingPolicy(t *testing.T) {
	const merchantID int64 = 41
	packagingDishID := int64(1001)
	foodDishID := int64(1002)

	testCases := []struct {
		name       string
		orderType  string
		items      []db.CreateOrderItemParams
		buildStubs func(store *mockdb.MockStore)
		check      func(t *testing.T, err error)
	}{
		{
			name:      "IgnoreNonApplicableOrderType",
			orderType: "dine_in",
			items: []db.CreateOrderItemParams{{
				DishID:   pgtype.Int8{Int64: foodDishID, Valid: true},
				Quantity: 1,
			}},
			buildStubs: func(store *mockdb.MockStore) {},
			check: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:      "MissingPackagingSelectionRejected",
			orderType: db.OrderTypeTakeout,
			items: []db.CreateOrderItemParams{{
				DishID:   pgtype.Int8{Int64: foodDishID, Valid: true},
				Quantity: 1,
			}},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantPackagingPolicy(gomock.Any(), merchantID).
					Times(1).
					Return(db.MerchantPackagingPolicy{
						MerchantID:           merchantID,
						ApplicableOrderTypes: []string{db.OrderTypeTakeout},
						CandidateDishIds:     []int64{packagingDishID},
					}, nil)

				store.EXPECT().
					GetDishesByIDsAll(gomock.Any(), []int64{packagingDishID}).
					Times(1).
					Return([]db.GetDishesByIDsAllRow{{
						ID:          packagingDishID,
						MerchantID:  merchantID,
						Name:        "包装盒",
						IsAvailable: true,
						IsOnline:    true,
					}}, nil)
			},
			check: func(t *testing.T, err error) {
				require.EqualError(t, err, "请先选择包装方式")
			},
		},
		{
			name:      "SinglePackagingSelectionAccepted",
			orderType: "takeaway",
			items: []db.CreateOrderItemParams{{
				DishID:   pgtype.Int8{Int64: foodDishID, Valid: true},
				Quantity: 1,
			}, {
				DishID:   pgtype.Int8{Int64: packagingDishID, Valid: true},
				Quantity: 1,
			}},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantPackagingPolicy(gomock.Any(), merchantID).
					Times(1).
					Return(db.MerchantPackagingPolicy{
						MerchantID:           merchantID,
						ApplicableOrderTypes: []string{"takeaway"},
						CandidateDishIds:     []int64{packagingDishID},
					}, nil)

				store.EXPECT().
					GetDishesByIDsAll(gomock.Any(), []int64{packagingDishID}).
					Times(1).
					Return([]db.GetDishesByIDsAllRow{{
						ID:          packagingDishID,
						MerchantID:  merchantID,
						Name:        "包装盒",
						IsAvailable: true,
						IsOnline:    true,
					}}, nil)
			},
			check: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:      "MultiplePackagingSelectionsRejected",
			orderType: db.OrderTypeTakeout,
			items: []db.CreateOrderItemParams{{
				DishID:   pgtype.Int8{Int64: packagingDishID, Valid: true},
				Quantity: 2,
			}},
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					GetMerchantPackagingPolicy(gomock.Any(), merchantID).
					Times(1).
					Return(db.MerchantPackagingPolicy{
						MerchantID:           merchantID,
						ApplicableOrderTypes: []string{db.OrderTypeTakeout},
						CandidateDishIds:     []int64{packagingDishID},
					}, nil)

				store.EXPECT().
					GetDishesByIDsAll(gomock.Any(), []int64{packagingDishID}).
					Times(1).
					Return([]db.GetDishesByIDsAllRow{{
						ID:          packagingDishID,
						MerchantID:  merchantID,
						Name:        "包装盒",
						IsAvailable: true,
						IsOnline:    true,
					}}, nil)
			},
			check: func(t *testing.T, err error) {
				require.EqualError(t, err, "只能选择一种包装方式")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			service := NewOrderService(store, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			err := service.validatePackagingPolicy(context.Background(), merchantID, tc.orderType, tc.items)
			tc.check(t, err)
		})
	}
}
