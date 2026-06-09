package logic

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDefaultIDGeneratorPickupCodeUsesFourDigits(t *testing.T) {
	code, err := DefaultIDGenerator{}.PickupCode(time.Now())

	require.NoError(t, err)
	require.Regexp(t, `^\d{4}$`, code)
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
