package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
)

var errMerchantSelectableTagNotAllowed = errors.New("tag is not selectable for this merchant")

func respondMerchantSelectableTagTxError(ctx *gin.Context, err error) bool {
	if errors.Is(err, db.ErrMerchantSelectableTagUnavailable) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errMerchantSelectableTagNotAllowed))
		return true
	}
	if errors.Is(err, db.ErrTagTypeNotSelectable) {
		ctx.JSON(http.StatusBadRequest, errorResponse(errMerchantSelectableTagNotAllowed))
		return true
	}
	return false
}
