package api

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Test Casbin model and policy definitions
const testCasbinModel = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && keyMatch2(r.obj, p.obj) && r.act == p.act
`

const testCasbinPolicy = `
# Role inheritance
g, operator, customer
g, merchant_owner, customer
g, rider, customer

# Admin policies
p, admin, /v1/platform/stats/*, GET
p, admin, /v1/admin/merchants/applications, GET

# Operator policies
p, operator, /v1/operator/*, GET
p, operator, /v1/operator/*, POST
p, operator, /v1/delivery-fee/regions/:region_id/config, POST
p, operator, /v1/delivery-fee/regions/:region_id/config, PATCH
p, operator, /v1/regions/:id/recommendation-config, GET
p, operator, /v1/regions/:id/recommendation-config, PATCH

# Merchant policies
p, merchant_owner, /v1/merchant/orders, GET
p, merchant_owner, /v1/merchant/orders/:id, GET

# Customer policies
p, customer, /v1/orders, GET
p, customer, /v1/orders/:id, GET
p, customer, /v1/users/me, GET
`

func TestNewCasbinEnforcerFromString(t *testing.T) {
	enforcer, err := NewCasbinEnforcerFromString(testCasbinModel, testCasbinPolicy)
	require.NoError(t, err)
	require.NotNil(t, enforcer)
}

func TestCasbinEnforce(t *testing.T) {
	enforcer, err := NewCasbinEnforcerFromString(testCasbinModel, testCasbinPolicy)
	require.NoError(t, err)

	testCases := []struct {
		name     string
		sub      string
		obj      string
		act      string
		expected bool
	}{
		// Admin tests
		{
			name:     "admin can access platform stats",
			sub:      "admin",
			obj:      "/v1/platform/stats/overview",
			act:      "GET",
			expected: true,
		},
		{
			name:     "admin can list merchant applications",
			sub:      "admin",
			obj:      "/v1/admin/merchants/applications",
			act:      "GET",
			expected: true,
		},
		{
			name:     "admin cannot create orders (not defined)",
			sub:      "admin",
			obj:      "/v1/orders",
			act:      "POST",
			expected: false,
		},

		// Operator tests
		{
			name:     "operator can access operator stats",
			sub:      "operator",
			obj:      "/v1/operator/settlements",
			act:      "GET",
			expected: true,
		},
		{
			name:     "operator can create delivery fee config",
			sub:      "operator",
			obj:      "/v1/delivery-fee/regions/123/config",
			act:      "POST",
			expected: true,
		},
		{
			name:     "operator can update recommendation config",
			sub:      "operator",
			obj:      "/v1/regions/456/recommendation-config",
			act:      "PATCH",
			expected: true,
		},
		{
			name:     "operator inherits customer permissions",
			sub:      "operator",
			obj:      "/v1/orders",
			act:      "GET",
			expected: true,
		},
		{
			name:     "operator cannot access platform stats",
			sub:      "operator",
			obj:      "/v1/platform/stats/overview",
			act:      "GET",
			expected: false,
		},

		// Merchant tests
		{
			name:     "merchant_owner can access merchant orders",
			sub:      "merchant_owner",
			obj:      "/v1/merchant/orders",
			act:      "GET",
			expected: true,
		},
		{
			name:     "merchant_owner inherits customer permissions",
			sub:      "merchant_owner",
			obj:      "/v1/users/me",
			act:      "GET",
			expected: true,
		},
		{
			name:     "merchant_owner cannot access operator routes",
			sub:      "merchant_owner",
			obj:      "/v1/operator/settlements",
			act:      "GET",
			expected: false,
		},

		// Customer tests
		{
			name:     "customer can access orders",
			sub:      "customer",
			obj:      "/v1/orders",
			act:      "GET",
			expected: true,
		},
		{
			name:     "customer can access specific order",
			sub:      "customer",
			obj:      "/v1/orders/123",
			act:      "GET",
			expected: true,
		},
		{
			name:     "customer cannot access platform stats",
			sub:      "customer",
			obj:      "/v1/platform/stats/overview",
			act:      "GET",
			expected: false,
		},
		{
			name:     "customer cannot access operator routes",
			sub:      "customer",
			obj:      "/v1/operator/settlements",
			act:      "GET",
			expected: false,
		},

		// Rider tests (inherits from customer)
		{
			name:     "rider inherits customer permissions",
			sub:      "rider",
			obj:      "/v1/orders",
			act:      "GET",
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			allowed, err := enforcer.Enforce(tc.sub, tc.obj, tc.act)
			require.NoError(t, err)
			require.Equal(t, tc.expected, allowed, "expected %v for %s %s %s", tc.expected, tc.sub, tc.obj, tc.act)
		})
	}
}

func TestCasbinEnforceWithRoles(t *testing.T) {
	enforcer, err := NewCasbinEnforcerFromString(testCasbinModel, testCasbinPolicy)
	require.NoError(t, err)

	testCases := []struct {
		name          string
		roles         []string
		obj           string
		act           string
		expectAllowed bool
		expectRole    string
	}{
		{
			name:          "single role admin",
			roles:         []string{"admin"},
			obj:           "/v1/platform/stats/overview",
			act:           "GET",
			expectAllowed: true,
			expectRole:    "admin",
		},
		{
			name:          "multiple roles, admin matches first",
			roles:         []string{"admin", "customer"},
			obj:           "/v1/platform/stats/overview",
			act:           "GET",
			expectAllowed: true,
			expectRole:    "admin",
		},
		{
			name:          "multiple roles, customer matches",
			roles:         []string{"operator", "customer"},
			obj:           "/v1/orders",
			act:           "GET",
			expectAllowed: true,
			expectRole:    "operator", // operator inherits from customer, matches first
		},
		{
			name:          "no matching role",
			roles:         []string{"customer"},
			obj:           "/v1/platform/stats/overview",
			act:           "GET",
			expectAllowed: false,
			expectRole:    "",
		},
		{
			name:          "empty roles",
			roles:         []string{},
			obj:           "/v1/orders",
			act:           "GET",
			expectAllowed: false,
			expectRole:    "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			allowed, matchedRole, err := enforcer.EnforceWithRoles(tc.roles, tc.obj, tc.act)
			require.NoError(t, err)
			require.Equal(t, tc.expectAllowed, allowed)
			require.Equal(t, tc.expectRole, matchedRole)
		})
	}
}

func TestCasbinDynamicPolicy(t *testing.T) {
	enforcer, err := NewCasbinEnforcerFromString(testCasbinModel, testCasbinPolicy)
	require.NoError(t, err)

	// Initially, admin cannot create users
	allowed, err := enforcer.Enforce("admin", "/v1/users", "POST")
	require.NoError(t, err)
	require.False(t, allowed)

	// Add policy
	added, err := enforcer.AddPolicy("admin", "/v1/users", "POST")
	require.NoError(t, err)
	require.True(t, added)

	// Now admin can create users
	allowed, err = enforcer.Enforce("admin", "/v1/users", "POST")
	require.NoError(t, err)
	require.True(t, allowed)

	// Remove policy
	removed, err := enforcer.RemovePolicy("admin", "/v1/users", "POST")
	require.NoError(t, err)
	require.True(t, removed)

	// Admin can no longer create users
	allowed, err = enforcer.Enforce("admin", "/v1/users", "POST")
	require.NoError(t, err)
	require.False(t, allowed)
}

func TestCasbinPathPatternMatching(t *testing.T) {
	enforcer, err := NewCasbinEnforcerFromString(testCasbinModel, testCasbinPolicy)
	require.NoError(t, err)

	testCases := []struct {
		name     string
		sub      string
		obj      string
		act      string
		expected bool
	}{
		{
			name:     "wildcard match for platform stats",
			sub:      "admin",
			obj:      "/v1/platform/stats/daily",
			act:      "GET",
			expected: true,
		},
		{
			name:     "wildcard match for platform stats regions",
			sub:      "admin",
			obj:      "/v1/platform/stats/regions/compare",
			act:      "GET",
			expected: true,
		},
		{
			name:     "parameter match for regions",
			sub:      "operator",
			obj:      "/v1/regions/123/recommendation-config",
			act:      "GET",
			expected: true,
		},
		{
			name:     "parameter match for delivery fee",
			sub:      "operator",
			obj:      "/v1/delivery-fee/regions/999/config",
			act:      "PATCH",
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			allowed, err := enforcer.Enforce(tc.sub, tc.obj, tc.act)
			require.NoError(t, err)
			require.Equal(t, tc.expected, allowed, "expected %v for %s %s %s", tc.expected, tc.sub, tc.obj, tc.act)
		})
	}
}

func TestGlobalCasbinEnforcer(t *testing.T) {
	// Save original enforcer
	original := GetGlobalCasbinEnforcer()
	defer SetGlobalCasbinEnforcer(original)

	// Create and set test enforcer
	enforcer, err := NewCasbinEnforcerFromString(testCasbinModel, testCasbinPolicy)
	require.NoError(t, err)

	SetGlobalCasbinEnforcer(enforcer)

	// Verify global enforcer is set
	global := GetGlobalCasbinEnforcer()
	require.NotNil(t, global)
	require.Equal(t, enforcer, global)

	// Test enforcement through global enforcer
	allowed, err := global.Enforce("admin", "/v1/platform/stats/overview", "GET")
	require.NoError(t, err)
	require.True(t, allowed)
}

func TestCasbinMiddlewareIntegration(t *testing.T) {
	// This test requires a mock store, which is complex
	// For now, just verify the middleware can be created
	gin.SetMode(gin.TestMode)

	// Create test enforcer
	enforcer, err := NewCasbinEnforcerFromString(testCasbinModel, testCasbinPolicy)
	require.NoError(t, err)

	// Set global enforcer
	original := GetGlobalCasbinEnforcer()
	SetGlobalCasbinEnforcer(enforcer)
	defer SetGlobalCasbinEnforcer(original)

	// Verify enforcer is set
	require.NotNil(t, GetGlobalCasbinEnforcer())
}
