package logic

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	db "github.com/merrydance/locallife/db/sqlc"
)

// GetBestDiscountAmount selects the best discount amount based on current rules.
func GetBestDiscountAmount(ctx context.Context, store db.Store, merchantID int64, subtotal int64) (int64, error) {
	rules, err := store.ListActiveDiscountRules(ctx, merchantID)
	if err != nil {
		return 0, err
	}

	var bestAmount int64
	for _, rule := range rules {
		if subtotal < rule.MinOrderAmount {
			continue
		}
		if rule.DiscountAmount > bestAmount {
			bestAmount = rule.DiscountAmount
		}
	}
	return bestAmount, nil
}

// VoucherValidationInput defines the voucher validation input.
type VoucherValidationInput struct {
	UserID        int64
	MerchantID    int64
	OrderType     string
	Subtotal      int64
	UserVoucherID *int64
}

// VoucherValidationResult describes the validated voucher result.
type VoucherValidationResult struct {
	UserVoucherID *int64
	VoucherAmount int64
}

// ValidateVoucher validates user voucher for an order context.
func ValidateVoucher(ctx context.Context, store db.Store, input VoucherValidationInput) (VoucherValidationResult, error) {
	var result VoucherValidationResult
	if input.UserVoucherID == nil {
		return result, nil
	}

	voucher, err := store.GetUserVoucher(ctx, *input.UserVoucherID)
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return result, NewRequestError(http.StatusNotFound, errors.New("优惠券不存在"))
		}
		return result, err
	}

	if voucher.UserID != input.UserID {
		return result, NewRequestError(http.StatusForbidden, errors.New("优惠券不属于您"))
	}
	if voucher.Status != "unused" {
		return result, NewRequestError(http.StatusBadRequest, errors.New("优惠券已使用或已过期"))
	}
	if time.Now().After(voucher.ExpiresAt) {
		return result, NewRequestError(http.StatusBadRequest, errors.New("优惠券已过期"))
	}
	if voucher.MerchantID != input.MerchantID {
		return result, NewRequestError(http.StatusBadRequest, errors.New("该优惠券不能在此商户使用"))
	}
	if input.Subtotal < voucher.MinOrderAmount {
		return result, NewRequestError(http.StatusBadRequest, fmt.Errorf("未达到最低消费 %d 元", voucher.MinOrderAmount/100))
	}

	orderTypeAllowed := false
	if len(voucher.AllowedOrderTypes) == 0 {
		orderTypeAllowed = true
	}
	for _, allowed := range voucher.AllowedOrderTypes {
		if allowed == input.OrderType {
			orderTypeAllowed = true
			break
		}
	}
	if !orderTypeAllowed {
		return result, NewRequestError(http.StatusBadRequest, errors.New("该代金券不适用于此订单类型"))
	}

	result.UserVoucherID = input.UserVoucherID
	result.VoucherAmount = voucher.Amount
	return result, nil
}
