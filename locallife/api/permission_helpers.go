package api

import (
	"errors"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
)

// requireMerchantMatch ensures the merchant in context matches the target merchant ID.
// It returns a domain error with the provided messages for consistent responses.
func requireMerchantMatch(ctx *gin.Context, merchantID int64, contextErrMsg, permissionErrMsg string) (db.Merchant, error) {
	merchant, ok := GetMerchantFromContext(ctx)
	if !ok {
		return db.Merchant{}, errors.New(contextErrMsg)
	}
	if merchant.ID != merchantID {
		return db.Merchant{}, errors.New(permissionErrMsg)
	}
	return merchant, nil
}