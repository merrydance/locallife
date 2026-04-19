package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
)

// Context keys for RBAC middleware
const (
	// 用户角色列表存储在 context 中的 key
	userRolesKey = "user_roles"
	// Operator 信息存储在 context 中的 key
	operatorKey = "operator"
	// Merchant 信息存储在 context 中的 key
	merchantKey = "merchant"
	// Rider 信息存储在 context 中的 key
	riderKey = "rider"
	// Merchant staff role 存储在 context 中的 key
	merchantStaffRoleKey = "merchant_staff_role"
)

// 系统支持的角色常量
const (
	RoleAdmin         = "admin"          // 平台管理员
	RoleOperator      = "operator"       // 区域运营商
	RoleMerchantOwner = "merchant_owner" // 商户老板
	RoleMerchantStaff = "merchant_staff" // 商户员工
	RoleRider         = "rider"          // 骑手
	RoleCustomer      = "customer"       // 普通用户
)

// RoleMiddleware 创建角色验证中间件
// 检查用户是否拥有指定角色之一
// 角色信息会缓存在 context 中供后续使用
func (server *Server) RoleMiddleware(allowedRoles ...string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// 从 context 获取 auth payload
		authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

		// 查询用户角色（优先复用已缓存的角色）
		userRoles, ok := GetUserRolesFromContext(ctx)
		if !ok {
			var err error
			userRoles, err = server.store.ListUserRoles(ctx, authPayload.UserID)
			if err != nil {
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, internalError(ctx, err))
				return
			}

			// 缓存角色到 context
			ctx.Set(userRolesKey, userRoles)
		}

		// 检查是否拥有允许的角色之一
		hasRole := false
		for _, userRole := range userRoles {
			if userRole.Status != "active" {
				continue
			}
			for _, allowed := range allowedRoles {
				if userRole.Role == allowed {
					hasRole = true
					break
				}
			}
			if hasRole {
				break
			}
		}

		if !hasRole {
			ctx.AbortWithStatusJSON(http.StatusForbidden, errorResponse(
				errors.New("you don't have permission to access this resource"),
			))
			return
		}

		ctx.Next()
	}
}

// MerchantOwnerMiddleware 创建商户老板验证中间件
// 验证用户是 merchant_owner 角色，并加载商户信息到 context
// 必须在 RoleMiddleware(RoleMerchantOwner) 之后使用
func (server *Server) MerchantOwnerMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

		if cachedMerchant, ok := GetMerchantFromContext(ctx); ok {
			bindMerchantContext(ctx, cachedMerchant)
			if cachedMerchant.Status != "active" && cachedMerchant.Status != "approved" {
				ctx.AbortWithStatusJSON(http.StatusForbidden, errorResponse(
					errors.New("merchant account is not active, please complete WeChat payment registration"),
				))
				return
			}
			ctx.Next()
			return
		}

		merchant, err := server.requireOwnedMerchantForUser(ctx, authPayload.UserID)
		if err != nil {
			if isMerchantSelectionRequiredError(err) {
				ctx.AbortWithStatusJSON(http.StatusBadRequest, errorResponse(err))
				return
			}
			if errors.Is(err, errMerchantOwnerRequired) {
				ctx.AbortWithStatusJSON(http.StatusForbidden, errorResponse(err))
				return
			}
			if isNotFoundError(err) {
				ctx.AbortWithStatusJSON(http.StatusForbidden, errorResponse(
					errors.New("merchant owner role not found"),
				))
				return
			}
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		// 检查商户状态：需要已完成微信支付开户（active）
		// 旧状态 approved 也兼容（用于尚未迁移的商户）
		if merchant.Status != "active" && merchant.Status != "approved" {
			ctx.AbortWithStatusJSON(http.StatusForbidden, errorResponse(
				errors.New("merchant account is not active, please complete WeChat payment registration"),
			))
			return
		}

		bindMerchantContext(ctx, merchant)
		ctx.Next()
	}
}

// MerchantOwnerOnlyMiddleware validates that the current user is the merchant owner.
// Unlike MerchantOwnerMiddleware and MerchantStaffMiddleware, it does not enforce
// merchant active status or region checks so it can be used on onboarding flows.
func (server *Server) MerchantOwnerOnlyMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

		merchant, err := server.resolveMerchantForUser(ctx, authPayload.UserID)
		if err != nil {
			if isMerchantSelectionRequiredError(err) {
				ctx.AbortWithStatusJSON(http.StatusBadRequest, errorResponse(err))
				return
			}
			if isNotFoundError(err) {
				ctx.AbortWithStatusJSON(http.StatusForbidden, errorResponse(
					errors.New("you are not associated with any merchant"),
				))
				return
			}
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		if merchant.OwnerUserID != authPayload.UserID {
			ctx.AbortWithStatusJSON(http.StatusForbidden, errorResponse(
				errors.New("insufficient permissions for this operation"),
			))
			return
		}

		bindMerchantContext(ctx, merchant)
		ctx.Set(merchantStaffRoleKey, "owner")
		ctx.Next()
	}
}

// MerchantStaffMiddleware 创建商户员工验证中间件
// 验证用户是商户老板或员工，检查细分角色权限，加载商户信息到 context
// allowedRoles: 允许的细分角色列表（owner, manager, chef, cashier）
func (server *Server) MerchantStaffMiddleware(allowedRoles ...string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

		merchant, staffRole, err := server.resolveMerchantStaffIdentity(ctx, authPayload.UserID)
		if err != nil {
			if isMerchantSelectionRequiredError(err) {
				ctx.AbortWithStatusJSON(http.StatusBadRequest, errorResponse(err))
				return
			}
			if isNotFoundError(err) {
				ctx.AbortWithStatusJSON(http.StatusForbidden, errorResponse(
					errors.New("you are not associated with any merchant"),
				))
				return
			}
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		// 检查商户状态
		if merchant.Status != "active" && merchant.Status != "approved" {
			ctx.AbortWithStatusJSON(http.StatusForbidden, errorResponse(
				errors.New("merchant account is not active"),
			))
			return
		}

		if merchant.RegionID == 0 {
			server.writeAuditLog(ctx, AuditLogInput{
				ActorUserID: authPayload.UserID,
				ActorRole:   "merchant",
				Action:      "region_access_denied",
				TargetType:  "region",
				RegionID:    nil,
				Metadata: map[string]any{
					"reason": "merchant_region_unset",
					"path":   ctx.Request.URL.Path,
					"method": ctx.Request.Method,
				},
			})
			ctx.AbortWithStatusJSON(http.StatusForbidden, errorResponse(
				errors.New("merchant region is not set"),
			))
			return
		}

		// 3. 检查角色权限
		hasPermission := false
		for _, allowed := range allowedRoles {
			if staffRole == allowed {
				hasPermission = true
				break
			}
		}

		if !hasPermission {
			ctx.AbortWithStatusJSON(http.StatusForbidden, errorResponse(
				errors.New("insufficient permissions for this operation"),
			))
			return
		}

		// 4. 缓存到 context
		bindMerchantContext(ctx, merchant)
		ctx.Set(merchantStaffRoleKey, staffRole)
		ctx.Next()
	}
}

func (server *Server) resolveMerchantStaffIdentity(ctx *gin.Context, userID int64) (db.Merchant, string, error) {
	merchant, err := server.resolveMerchantForUser(ctx, userID)
	if err != nil {
		return db.Merchant{}, "", err
	}

	if merchant.OwnerUserID == userID {
		return merchant, "owner", nil
	}

	role, err := server.store.GetUserMerchantRole(ctx, db.GetUserMerchantRoleParams{
		MerchantID: merchant.ID,
		UserID:     userID,
	})
	if err != nil {
		if isNotFoundError(err) {
			return db.Merchant{}, "", errors.New("you are not a staff of this merchant")
		}
		return db.Merchant{}, "", err
	}

	return merchant, role, nil
}

// RiderMiddleware 创建骑手验证中间件
// 验证用户是 rider 角色，并加载骑手信息到 context
// 必须在 RoleMiddleware(RoleRider) 之后使用
func (server *Server) RiderMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

		if cachedRider, ok := GetRiderFromContext(ctx); ok {
			if cachedRider.Status != "approved" {
				ctx.AbortWithStatusJSON(http.StatusForbidden, errorResponse(
					errors.New("rider account is not approved"),
				))
				return
			}
			ctx.Next()
			return
		}

		// 加载骑手信息
		rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
		if err != nil {
			if isNotFoundError(err) {
				ctx.AbortWithStatusJSON(http.StatusForbidden, errorResponse(
					errors.New("rider profile not found"),
				))
				return
			}
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		// 检查骑手状态
		if rider.Status != "approved" {
			ctx.AbortWithStatusJSON(http.StatusForbidden, errorResponse(
				errors.New("rider account is not approved"),
			))
			return
		}

		// 缓存到 context
		ctx.Set(riderKey, rider)
		ctx.Next()
	}
}

// AdminMiddleware 创建管理员验证中间件（简化版）
// 直接检查 admin 角色
func (server *Server) AdminMiddleware() gin.HandlerFunc {
	return server.RoleMiddleware(RoleAdmin)
}

// ==================== 辅助函数 ====================

// GetUserRolesFromContext 从 context 获取用户角色列表
func GetUserRolesFromContext(ctx *gin.Context) ([]db.UserRole, bool) {
	val, exists := ctx.Get(userRolesKey)
	if !exists {
		return nil, false
	}
	roles, ok := val.([]db.UserRole)
	return roles, ok
}

// GetOperatorFromContext 从 context 获取 operator 信息
func GetOperatorFromContext(ctx *gin.Context) (db.Operator, bool) {
	val, exists := ctx.Get(operatorKey)
	if !exists {
		return db.Operator{}, false
	}
	operator, ok := val.(db.Operator)
	return operator, ok
}

// GetMerchantFromContext 从 context 获取商户信息
func GetMerchantFromContext(ctx *gin.Context) (db.Merchant, bool) {
	val, exists := ctx.Get(merchantKey)
	if !exists {
		return db.Merchant{}, false
	}
	merchant, ok := val.(db.Merchant)
	return merchant, ok
}

// GetRiderFromContext 从 context 获取骑手信息
func GetRiderFromContext(ctx *gin.Context) (db.Rider, bool) {
	val, exists := ctx.Get(riderKey)
	if !exists {
		return db.Rider{}, false
	}
	rider, ok := val.(db.Rider)
	return rider, ok
}

// GetMerchantStaffRoleFromContext 从 context 获取商户员工角色
func GetMerchantStaffRoleFromContext(ctx *gin.Context) (string, bool) {
	val, exists := ctx.Get(merchantStaffRoleKey)
	if !exists {
		return "", false
	}
	role, ok := val.(string)
	return role, ok
}
