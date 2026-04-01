package api

import (
	"errors"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
)

var errMerchantOwnerRequired = errors.New("merchant owner required")

// getMerchantFromContextOrStore resolves the merchant associated with the user.
// Despite the store method name, GetMerchantByOwner already falls back to active
// merchant_staff membership. Callers that require owner-only access must still
// verify merchant.OwnerUserID after using this helper.
func (server *Server) getMerchantFromContextOrStore(ctx *gin.Context, userID int64) (db.Merchant, error) {
	if merchant, ok := GetMerchantFromContext(ctx); ok {
		return merchant, nil
	}

	return server.store.GetMerchantByOwner(ctx, userID)
}

// requireOwnedMerchantForUser resolves the user's associated merchant and makes
// the owner-only expectation explicit for sensitive actions.
func (server *Server) requireOwnedMerchantForUser(ctx *gin.Context, userID int64) (db.Merchant, error) {
	merchant, err := server.resolveMerchantForUser(ctx, userID)
	if err != nil {
		return db.Merchant{}, err
	}
	if merchant.OwnerUserID != userID {
		return db.Merchant{}, errMerchantOwnerRequired
	}
	return merchant, nil
}

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
