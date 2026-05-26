package api

import (
	"context"

	"github.com/merrydance/locallife/logic"
)

type merchantFoodPermitOfficialVerifier interface {
	VerifyMerchantFoodPermit(ctx context.Context, rawResult []byte) (logic.MerchantFoodPermitOfficialVerification, error)
}
