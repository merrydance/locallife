package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/logic"
)

const merchantSelectionHeader = "X-Merchant-ID"

var (
	errMerchantOwnerRequired     = errors.New("merchant owner required")
	errMerchantSelectionRequired = errors.New("merchant_id is required when you have access to multiple merchants")
)

func isMerchantSelectionRequiredError(err error) bool {
	return errors.Is(err, errMerchantSelectionRequired)
}

func writeMerchantSelectionError(ctx *gin.Context, err error) bool {
	if !isMerchantSelectionRequiredError(err) {
		return false
	}
	ctx.JSON(http.StatusBadRequest, errorResponse(err))
	return true
}

func bindMerchantContext(ctx *gin.Context, merchant db.Merchant) {
	ctx.Set(merchantKey, merchant)
	ctx.Request = ctx.Request.WithContext(logic.WithSelectedMerchantID(ctx.Request.Context(), merchant.ID))
}

func parsePositiveInt64(value, field string) (int64, error) {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, errors.New("invalid " + field)
	}
	return parsed, nil
}

func selectedMerchantIDFromRequest(ctx *gin.Context) (int64, bool, error) {
	if merchantParam := ctx.Param("merchant_id"); merchantParam != "" {
		merchantID, err := parsePositiveInt64(merchantParam, "merchant_id")
		return merchantID, true, err
	}

	if idParam := ctx.Param("id"); idParam != "" && strings.Contains(ctx.FullPath(), "/merchants/:id") {
		merchantID, err := parsePositiveInt64(idParam, "merchant_id")
		return merchantID, true, err
	}

	if merchantIDQuery := strings.TrimSpace(ctx.Query("merchant_id")); merchantIDQuery != "" {
		merchantID, err := parsePositiveInt64(merchantIDQuery, "merchant_id")
		return merchantID, true, err
	}

	if merchantIDHeader := strings.TrimSpace(ctx.GetHeader(merchantSelectionHeader)); merchantIDHeader != "" {
		merchantID, err := parsePositiveInt64(merchantIDHeader, "merchant_id")
		return merchantID, true, err
	}

	return 0, false, nil
}

func (server *Server) listAccessibleMerchants(ctx *gin.Context, userID int64) ([]db.Merchant, error) {
	ownedMerchants, err := server.store.ListMerchantsByOwner(ctx, userID)
	if err != nil {
		return nil, err
	}

	staffMerchants, err := server.store.ListMerchantsByStaff(ctx, userID)
	if err != nil {
		return nil, err
	}

	accessible := make([]db.Merchant, 0, len(ownedMerchants)+len(staffMerchants))
	seen := make(map[int64]struct{}, len(ownedMerchants)+len(staffMerchants))

	appendMerchant := func(merchant db.Merchant) {
		if _, ok := seen[merchant.ID]; ok {
			return
		}
		seen[merchant.ID] = struct{}{}
		accessible = append(accessible, merchant)
	}

	for _, merchant := range ownedMerchants {
		appendMerchant(merchant)
	}
	for _, merchant := range staffMerchants {
		appendMerchant(merchant)
	}

	return accessible, nil
}

func (server *Server) resolveAccessibleMerchant(ctx *gin.Context, userID int64) (db.Merchant, error) {
	selectedMerchantID, hasExplicitSelection, err := selectedMerchantIDFromRequest(ctx)
	if err != nil {
		return db.Merchant{}, err
	}

	accessibleMerchants, err := server.listAccessibleMerchants(ctx, userID)
	if err != nil {
		return db.Merchant{}, err
	}
	if len(accessibleMerchants) == 0 {
		return db.Merchant{}, db.ErrRecordNotFound
	}

	if hasExplicitSelection {
		for _, merchant := range accessibleMerchants {
			if merchant.ID == selectedMerchantID {
				return merchant, nil
			}
		}
		return db.Merchant{}, db.ErrRecordNotFound
	}

	if len(accessibleMerchants) == 1 {
		return accessibleMerchants[0], nil
	}

	return db.Merchant{}, errMerchantSelectionRequired
}

// getMerchantFromContextOrStore resolves the merchant associated with the user.
// Request-scoped merchant selection is resolved in this order:
// path merchant_id, /merchants/:id, query merchant_id, X-Merchant-ID header.
// When no explicit selection is provided and the user can access multiple
// merchants, the caller must provide an explicit merchant selection.
func (server *Server) getMerchantFromContextOrStore(ctx *gin.Context, userID int64) (db.Merchant, error) {
	if merchant, ok := GetMerchantFromContext(ctx); ok {
		bindMerchantContext(ctx, merchant)
		return merchant, nil
	}

	merchant, err := server.resolveAccessibleMerchant(ctx, userID)
	if err != nil {
		return db.Merchant{}, err
	}
	bindMerchantContext(ctx, merchant)
	return merchant, nil
}

// requireOwnedMerchantForUser resolves the user's associated merchant and makes
// the owner-only expectation explicit for sensitive actions.
func (server *Server) requireOwnedMerchantForUser(ctx *gin.Context, userID int64) (db.Merchant, error) {
	merchant, err := server.resolveAccessibleMerchant(ctx, userID)
	if err != nil {
		return db.Merchant{}, err
	}
	if merchant.OwnerUserID != userID {
		return db.Merchant{}, errMerchantOwnerRequired
	}
	bindMerchantContext(ctx, merchant)
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
