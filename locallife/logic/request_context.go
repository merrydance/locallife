package logic

import "context"

type requestContextKey string

const selectedMerchantIDContextKey requestContextKey = "selected_merchant_id"

func WithSelectedMerchantID(ctx context.Context, merchantID int64) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	return context.WithValue(ctx, selectedMerchantIDContextKey, merchantID)
}

func SelectedMerchantIDFromContext(ctx context.Context) (int64, bool) {
	if ctx == nil {
		return 0, false
	}

	merchantID, ok := ctx.Value(selectedMerchantIDContextKey).(int64)
	if !ok || merchantID <= 0 {
		return 0, false
	}

	return merchantID, true
}
