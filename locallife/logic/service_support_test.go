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

func TestDefaultIDGeneratorPickupCodeUsesFourDigits(t *testing.T) {
	code, err := DefaultIDGenerator{}.PickupCode(time.Now())

	require.NoError(t, err)
	require.Regexp(t, `^\d{4}$`, code)
}

func TestResolveMerchantForUserRejectsPendingStaffFallback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	const userID int64 = 10
	store := mockdb.NewMockStore(ctrl)
	store.EXPECT().
		GetMerchantByOwner(gomock.Any(), userID).
		Times(1).
		Return(db.Merchant{}, db.ErrRecordNotFound)

	_, err := resolveMerchantForUser(context.Background(), store, userID)
	require.ErrorIs(t, err, db.ErrRecordNotFound)
}

func TestDefaultOrderPolicyRejectsTakeawayAddress(t *testing.T) {
	addressID := int64(10)
	dishID := int64(20)

	err := DefaultOrderPolicy{}.ValidateCreateInput(CreateOrderCommandInput{
		MerchantID: 1,
		OrderType:  "takeaway",
		AddressID:  &addressID,
		Items: []OrderItemInput{{
			DishID:   &dishID,
			Quantity: 1,
		}},
	})

	require.EqualError(t, err, "address_id is not allowed for takeaway orders")
}

func TestDefaultOrderPolicyAllowsTakeawayWithoutAddress(t *testing.T) {
	dishID := int64(20)

	err := DefaultOrderPolicy{}.ValidateCreateInput(CreateOrderCommandInput{
		MerchantID: 1,
		OrderType:  "takeaway",
		Items: []OrderItemInput{{
			DishID:   &dishID,
			Quantity: 1,
		}},
	})

	require.NoError(t, err)
}
