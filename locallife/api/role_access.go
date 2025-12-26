package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// RoleAccessEntry describes which roles may call a path prefix.
type RoleAccessEntry struct {
	PathPrefix   string   `json:"path_prefix" example:"/v1/dishes"`
	Roles        []string `json:"roles" example:"merchant"`
	AuthRequired bool     `json:"auth_required" example:"true"`
	Notes        string   `json:"notes,omitempty" example:"merchant ownership validated inside handler"`
}

// RoleAccessResponse wraps the role access metadata.
type RoleAccessResponse struct {
	GeneratedAt time.Time         `json:"generated_at" example:"2024-12-31T23:59:59Z"`
	Entries     []RoleAccessEntry `json:"entries"`
}

// getRoleAccessMetadata provides a machine-readable map of path prefixes to allowed roles.
// @Summary API role access metadata
// @Description Machine-readable role matrix to help clients avoid calling merchant/operator/admin endpoints with wrong identities.
// @Tags Metadata
// @Produce json
// @Success 200 {object} RoleAccessResponse
// @Router /v1/role-access [get]
func (server *Server) getRoleAccessMetadata(ctx *gin.Context) {
	entries := []RoleAccessEntry{
		// Public (no auth)
		{PathPrefix: "/health", Roles: []string{"public"}, AuthRequired: false, Notes: "liveness"},
		{PathPrefix: "/ready", Roles: []string{"public"}, AuthRequired: false, Notes: "readiness"},
		{PathPrefix: "/metrics", Roles: []string{"public"}, AuthRequired: false, Notes: "prometheus"},
		{PathPrefix: "/v1/auth/wechat-login", Roles: []string{"public"}, AuthRequired: false},
		{PathPrefix: "/v1/auth/refresh", Roles: []string{"public"}, AuthRequired: false},
		{PathPrefix: "/v1/webhooks", Roles: []string{"public"}, AuthRequired: false, Notes: "wechat callbacks"},
		{PathPrefix: "/v1/regions", Roles: []string{"public"}, AuthRequired: false, Notes: "region lookup"},
		{PathPrefix: "/v1/search", Roles: []string{"public"}, AuthRequired: false},
		{PathPrefix: "/v1/merchants/:id/promotions", Roles: []string{"public"}, AuthRequired: false},
		{PathPrefix: "/v1/scan/table", Roles: []string{"public"}, AuthRequired: false},

		// Authenticated (any user)
		{PathPrefix: "/v1/users/me", Roles: []string{"user"}, AuthRequired: true},
		{PathPrefix: "/v1/location/reverse-geocode", Roles: []string{"user"}, AuthRequired: true, Notes: "reverse geocode"},
		{PathPrefix: "/v1/location/direction/bicycling", Roles: []string{"user"}, AuthRequired: true, Notes: "proxy tencent bicycling route"},
		{PathPrefix: "/v1/addresses", Roles: []string{"user"}, AuthRequired: true},
		{PathPrefix: "/v1/auth/bind-phone", Roles: []string{"user"}, AuthRequired: true},
		{PathPrefix: "/v1/cart", Roles: []string{"user"}, AuthRequired: true},
		{PathPrefix: "/v1/favorites", Roles: []string{"user"}, AuthRequired: true},
		{PathPrefix: "/v1/history/browse", Roles: []string{"user"}, AuthRequired: true},
		{PathPrefix: "/v1/memberships", Roles: []string{"user"}, AuthRequired: true},
		{PathPrefix: "/v1/reviews", Roles: []string{"user"}, AuthRequired: true, Notes: "read/create/reply; delete is operator"},
		{PathPrefix: "/v1/behaviors/track", Roles: []string{"user"}, AuthRequired: true},
		{PathPrefix: "/v1/recommendations", Roles: []string{"user"}, AuthRequired: true},
		{PathPrefix: "/v1/vouchers", Roles: []string{"user"}, AuthRequired: true, Notes: "user voucher flows"},
		{PathPrefix: "/v1/orders", Roles: []string{"user"}, AuthRequired: true, Notes: "customer namespace"},
		{PathPrefix: "/v1/payments", Roles: []string{"user"}, AuthRequired: true},
		{PathPrefix: "/v1/refunds", Roles: []string{"user"}, AuthRequired: true},
		{PathPrefix: "/v1/claims", Roles: []string{"user"}, AuthRequired: true, Notes: "user claim list/detail"},
		{PathPrefix: "/v1/trust-score/claims", Roles: []string{"user"}, AuthRequired: true, Notes: "submit claim"},
		{PathPrefix: "/v1/trust-score/appeals", Roles: []string{"user"}, AuthRequired: true},

		// Merchant
		{PathPrefix: "/v1/merchants/me", Roles: []string{"merchant"}, AuthRequired: true},
		{PathPrefix: "/v1/merchant/application", Roles: []string{"merchant"}, AuthRequired: true, Notes: "merchant onboarding"},
		{PathPrefix: "/v1/merchant/applyment", Roles: []string{"merchant"}, AuthRequired: true, Notes: "wechat pay onboarding"},
		{PathPrefix: "/v1/dishes", Roles: []string{"merchant"}, AuthRequired: true},
		{PathPrefix: "/v1/combos", Roles: []string{"merchant"}, AuthRequired: true},
		{PathPrefix: "/v1/inventory", Roles: []string{"merchant"}, AuthRequired: true},
		{PathPrefix: "/v1/tables", Roles: []string{"merchant"}, AuthRequired: true},
		{PathPrefix: "/v1/rooms", Roles: []string{"merchant"}, AuthRequired: true, Notes: "merchant-facing reservation ops"},
		{PathPrefix: "/v1/reservations/merchant", Roles: []string{"merchant"}, AuthRequired: true},
		{PathPrefix: "/v1/merchant/orders", Roles: []string{"merchant"}, AuthRequired: true},
		{PathPrefix: "/v1/kitchen", Roles: []string{"merchant"}, AuthRequired: true},
		{PathPrefix: "/v1/merchant/claims", Roles: []string{"merchant"}, AuthRequired: true},
		{PathPrefix: "/v1/merchant/appeals", Roles: []string{"merchant"}, AuthRequired: true},
		{PathPrefix: "/v1/merchant/finance", Roles: []string{"merchant"}, AuthRequired: true},
		{PathPrefix: "/v1/merchant/stats", Roles: []string{"merchant"}, AuthRequired: true},
		{PathPrefix: "/v1/merchant/display-config", Roles: []string{"merchant"}, AuthRequired: true},
		{PathPrefix: "/v1/merchant/devices", Roles: []string{"merchant"}, AuthRequired: true},
		{PathPrefix: "/v1/merchants/:id/recharge-rules", Roles: []string{"merchant"}, AuthRequired: true},
		{PathPrefix: "/v1/merchants/:id/vouchers", Roles: []string{"merchant"}, AuthRequired: true},
		{PathPrefix: "/v1/merchants/:id/discounts", Roles: []string{"merchant"}, AuthRequired: true},
		{PathPrefix: "/v1/delivery-fee/merchants", Roles: []string{"merchant"}, AuthRequired: true, Notes: "delivery promotions"},

		// Rider
		{PathPrefix: "/v1/rider", Roles: []string{"rider"}, AuthRequired: true},
		{PathPrefix: "/v1/delivery", Roles: []string{"rider"}, AuthRequired: true},
		{PathPrefix: "/v1/rider/claims", Roles: []string{"rider"}, AuthRequired: true},
		{PathPrefix: "/v1/rider/appeals", Roles: []string{"rider"}, AuthRequired: true},

		// Operator
		{PathPrefix: "/v1/delivery-fee/regions", Roles: []string{"operator"}, AuthRequired: true},
		{PathPrefix: "/v1/operator", Roles: []string{"operator"}, AuthRequired: true},
		{PathPrefix: "/v1/operators/me/finance", Roles: []string{"operator"}, AuthRequired: true},
		{PathPrefix: "/v1/regions/:id/recommendation-config", Roles: []string{"operator"}, AuthRequired: true},
		{PathPrefix: "DELETE /v1/reviews/:id", Roles: []string{"operator"}, AuthRequired: true, Notes: "delete only"},

		// Admin
		{PathPrefix: "/v1/admin/merchants/applications", Roles: []string{"admin"}, AuthRequired: true},
		{PathPrefix: "/v1/admin/riders", Roles: []string{"admin"}, AuthRequired: true},
		{PathPrefix: "/v1/platform/stats", Roles: []string{"admin"}, AuthRequired: true},
		{PathPrefix: "/v1/trust-score/claims/:id/review", Roles: []string{"admin"}, AuthRequired: true},
		{PathPrefix: "/v1/trust-score/merchants/:id/suspend", Roles: []string{"admin"}, AuthRequired: true},
		{PathPrefix: "/v1/trust-score/fraud/detect", Roles: []string{"admin"}, AuthRequired: true},
	}

	resp := RoleAccessResponse{
		GeneratedAt: time.Now().UTC(),
		Entries:     entries,
	}

	ctx.JSON(http.StatusOK, resp)
}
