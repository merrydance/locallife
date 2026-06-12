package api

import (
	"errors"
	"net/http"
	"sort"
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
	entries, err := buildRoleAccessEntries()
	if err != nil {
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "role access metadata unavailable", "role access metadata build failed"))
		return
	}

	resp := RoleAccessResponse{
		GeneratedAt: time.Now().UTC(),
		Entries:     entries,
	}

	ctx.JSON(http.StatusOK, resp)
}

func buildRoleAccessEntries() ([]RoleAccessEntry, error) {
	entries := []RoleAccessEntry{
		{PathPrefix: "/health", Roles: []string{"public"}, AuthRequired: false, Notes: "liveness"},
		{PathPrefix: "/ready", Roles: []string{"public"}, AuthRequired: false, Notes: "readiness"},
		{PathPrefix: "/metrics", Roles: []string{"public"}, AuthRequired: false, Notes: "prometheus"},
		{PathPrefix: "/uploads", Roles: []string{"public"}, AuthRequired: false, Notes: "signed download"},
		{PathPrefix: "/v1/role-access", Roles: []string{"public"}, AuthRequired: false},
		{PathPrefix: "/v1/auth/wechat-login", Roles: []string{"public"}, AuthRequired: false},
		{PathPrefix: "/v1/auth/refresh", Roles: []string{"public"}, AuthRequired: false},
		{PathPrefix: "/v1/webhooks", Roles: []string{"public"}, AuthRequired: false, Notes: "wechat callbacks"},
		{PathPrefix: "/v1/platform/stats/traffic/summary", Roles: []string{"admin"}, AuthRequired: true, Notes: "traffic snapshot"},
	}

	enforcer := GetGlobalCasbinEnforcer()
	if enforcer == nil {
		return nil, errors.New("casbin enforcer not initialized")
	}

	policy, err := enforcer.GetEnforcer().GetPolicy()
	if err != nil {
		return nil, err
	}
	roleIndex := map[string]map[string]struct{}{}
	for _, rule := range policy {
		if len(rule) < 3 {
			continue
		}
		sub := rule[0]
		obj := rule[1]
		act := rule[2]

		key := obj
		if act != "" && act != "*" {
			key = act + " " + obj
		}
		if _, ok := roleIndex[key]; !ok {
			roleIndex[key] = map[string]struct{}{}
		}
		roleIndex[key][sub] = struct{}{}
	}

	keys := make([]string, 0, len(roleIndex))
	for key := range roleIndex {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		roles := make([]string, 0, len(roleIndex[key]))
		for role := range roleIndex[key] {
			roles = append(roles, role)
		}
		sort.Strings(roles)

		entries = append(entries, RoleAccessEntry{
			PathPrefix:   key,
			Roles:        roles,
			AuthRequired: true,
		})
	}

	return entries, nil
}
