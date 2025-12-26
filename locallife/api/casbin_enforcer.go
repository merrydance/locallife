package api

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/rs/zerolog/log"
)

// CasbinEnforcer 封装 Casbin enforcer 并提供线程安全访问
type CasbinEnforcer struct {
	enforcer *casbin.Enforcer
	mu       sync.RWMutex
}

// NewCasbinEnforcer 创建新的 Casbin enforcer
// modelPath: model.conf 文件路径
// policyPath: policy.csv 文件路径
func NewCasbinEnforcer(modelPath, policyPath string) (*CasbinEnforcer, error) {
	enforcer, err := casbin.NewEnforcer(modelPath, policyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create casbin enforcer: %w", err)
	}

	// 加载策略
	if err := enforcer.LoadPolicy(); err != nil {
		return nil, fmt.Errorf("failed to load casbin policy: %w", err)
	}

	log.Info().
		Str("model", modelPath).
		Str("policy", policyPath).
		Msg("✅ Casbin enforcer initialized")

	return &CasbinEnforcer{
		enforcer: enforcer,
	}, nil
}

// NewCasbinEnforcerFromString 从字符串创建 Casbin enforcer（用于测试）
func NewCasbinEnforcerFromString(modelText, policyText string) (*CasbinEnforcer, error) {
	m, err := model.NewModelFromString(modelText)
	if err != nil {
		return nil, fmt.Errorf("failed to parse casbin model: %w", err)
	}

	enforcer, err := casbin.NewEnforcer(m)
	if err != nil {
		return nil, fmt.Errorf("failed to create casbin enforcer: %w", err)
	}

	// 解析策略文本并添加
	lines := strings.Split(policyText, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}

		// 去除空格
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}

		ptype := parts[0]
		values := parts[1:]

		// 转换为 []interface{}
		args := make([]interface{}, len(values))
		for i, v := range values {
			args[i] = v
		}

		switch ptype {
		case "p":
			if _, err := enforcer.AddPolicy(args...); err != nil {
				log.Warn().Err(err).Str("policy", line).Msg("failed to add policy")
			}
		case "g":
			if _, err := enforcer.AddGroupingPolicy(args...); err != nil {
				log.Warn().Err(err).Str("grouping", line).Msg("failed to add grouping policy")
			}
		}
	}

	return &CasbinEnforcer{
		enforcer: enforcer,
	}, nil
}

// Enforce 检查权限
// sub: 角色 (admin, operator, merchant_owner, etc.)
// obj: 资源路径 (/v1/orders/:id)
// act: 操作 (GET, POST, PUT, DELETE, PATCH)
func (ce *CasbinEnforcer) Enforce(sub, obj, act string) (bool, error) {
	ce.mu.RLock()
	defer ce.mu.RUnlock()
	return ce.enforcer.Enforce(sub, obj, act)
}

// EnforceWithRoles 检查多个角色是否有权限（任一角色有权限即可）
func (ce *CasbinEnforcer) EnforceWithRoles(roles []string, obj, act string) (bool, string, error) {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	for _, role := range roles {
		allowed, err := ce.enforcer.Enforce(role, obj, act)
		if err != nil {
			return false, "", err
		}
		if allowed {
			return true, role, nil
		}
	}
	return false, "", nil
}

// AddPolicy 动态添加策略
func (ce *CasbinEnforcer) AddPolicy(sub, obj, act string) (bool, error) {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	return ce.enforcer.AddPolicy(sub, obj, act)
}

// RemovePolicy 动态移除策略
func (ce *CasbinEnforcer) RemovePolicy(sub, obj, act string) (bool, error) {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	return ce.enforcer.RemovePolicy(sub, obj, act)
}

// ReloadPolicy 重新加载策略
func (ce *CasbinEnforcer) ReloadPolicy() error {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	return ce.enforcer.LoadPolicy()
}

// GetEnforcer 获取底层 enforcer（用于测试）
func (ce *CasbinEnforcer) GetEnforcer() *casbin.Enforcer {
	return ce.enforcer
}

// ==================== Server 集成 ====================

// casbinEnforcer 是 server 级别的 enforcer 实例
var globalCasbinEnforcer *CasbinEnforcer

// InitCasbin 初始化 Casbin enforcer
// 应在 server 启动时调用
func InitCasbin(casbinDir string) error {
	modelPath := filepath.Join(casbinDir, "model.conf")
	policyPath := filepath.Join(casbinDir, "policy.csv")

	enforcer, err := NewCasbinEnforcer(modelPath, policyPath)
	if err != nil {
		return err
	}

	globalCasbinEnforcer = enforcer
	return nil
}

// SetGlobalCasbinEnforcer 设置全局 enforcer（用于测试）
func SetGlobalCasbinEnforcer(enforcer *CasbinEnforcer) {
	globalCasbinEnforcer = enforcer
}

// GetGlobalCasbinEnforcer 获取全局 enforcer
func GetGlobalCasbinEnforcer() *CasbinEnforcer {
	return globalCasbinEnforcer
}

// ==================== Casbin 中间件 ====================

// CasbinMiddleware 创建基于 Casbin 的权限验证中间件
// 此中间件会：
// 1. 从 token 获取用户 ID
// 2. 查询用户的所有角色
// 3. 使用 Casbin 检查是否有任一角色有权限
//
// 注意：此中间件必须在 authMiddleware 之后使用
func (server *Server) CasbinMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if globalCasbinEnforcer == nil {
			log.Warn().Msg("Casbin enforcer not initialized, skipping permission check")
			ctx.Next()
			return
		}

		// 从 context 获取 auth payload
		authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

		// 查询用户角色
		userRoles, err := server.store.ListUserRoles(ctx, authPayload.UserID)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		// 缓存角色到 context
		ctx.Set(userRolesKey, userRoles)

		// 提取活跃角色列表
		activeRoles := make([]string, 0, len(userRoles))
		for _, role := range userRoles {
			if role.Status == "active" {
				activeRoles = append(activeRoles, role.Role)
			}
		}

		// 如果用户没有任何角色，默认为 customer
		if len(activeRoles) == 0 {
			activeRoles = append(activeRoles, RoleCustomer)
		}

		// 获取请求路径和方法
		obj := ctx.Request.URL.Path
		act := ctx.Request.Method

		// 使用 Casbin 检查权限
		allowed, matchedRole, err := globalCasbinEnforcer.EnforceWithRoles(activeRoles, obj, act)
		if err != nil {
			log.Error().Err(err).
				Str("path", obj).
				Str("method", act).
				Strs("roles", activeRoles).
				Msg("Casbin enforcement error")
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		if !allowed {
			log.Debug().
				Str("path", obj).
				Str("method", act).
				Strs("roles", activeRoles).
				Msg("Permission denied by Casbin")
			ctx.AbortWithStatusJSON(http.StatusForbidden, errorResponse(
				errors.New("you don't have permission to access this resource"),
			))
			return
		}

		// 记录匹配的角色（用于调试）
		log.Debug().
			Str("path", obj).
			Str("method", act).
			Str("matched_role", matchedRole).
			Msg("Permission granted")

		ctx.Next()
	}
}

// CasbinRoleMiddleware 创建指定角色的 Casbin 权限验证中间件
// 除了 Casbin 权限检查外，还会验证用户必须拥有指定角色
// 适用于需要特定角色的路由组
func (server *Server) CasbinRoleMiddleware(requiredRole string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if globalCasbinEnforcer == nil {
			log.Warn().Msg("Casbin enforcer not initialized, skipping permission check")
			ctx.Next()
			return
		}

		// 从 context 获取 auth payload
		authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

		// 查询用户角色
		userRoles, err := server.store.ListUserRoles(ctx, authPayload.UserID)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		// 缓存角色到 context
		ctx.Set(userRolesKey, userRoles)

		// 检查是否拥有必需角色
		hasRequiredRole := false
		for _, role := range userRoles {
			if role.Status == "active" && role.Role == requiredRole {
				hasRequiredRole = true
				break
			}
		}

		if !hasRequiredRole {
			ctx.AbortWithStatusJSON(http.StatusForbidden, errorResponse(
				fmt.Errorf("this endpoint requires %s role", requiredRole),
			))
			return
		}

		// 获取请求路径和方法
		obj := ctx.Request.URL.Path
		act := ctx.Request.Method

		// 使用 Casbin 检查权限
		allowed, err := globalCasbinEnforcer.Enforce(requiredRole, obj, act)
		if err != nil {
			log.Error().Err(err).
				Str("path", obj).
				Str("method", act).
				Str("role", requiredRole).
				Msg("Casbin enforcement error")
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		if !allowed {
			ctx.AbortWithStatusJSON(http.StatusForbidden, errorResponse(
				errors.New("you don't have permission to access this resource"),
			))
			return
		}

		ctx.Next()
	}
}

// ==================== 实体加载中间件 ====================
// 这些中间件负责加载特定角色的实体信息到 context
// 应该在 Casbin 权限检查之后使用

// LoadOperatorMiddleware 加载 operator 信息到 context
// 必须在 CasbinRoleMiddleware(RoleOperator) 之后使用
func (server *Server) LoadOperatorMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

		// 加载 operator 信息
		operator, err := server.store.GetOperatorByUser(ctx, authPayload.UserID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				ctx.AbortWithStatusJSON(http.StatusForbidden, errorResponse(
					errors.New("operator profile not found"),
				))
				return
			}
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		// 检查 operator 状态
		if operator.Status != "active" {
			ctx.AbortWithStatusJSON(http.StatusForbidden, errorResponse(
				errors.New("operator account is not active"),
			))
			return
		}

		// 缓存到 context
		ctx.Set(operatorKey, operator)
		ctx.Next()
	}
}

// LoadMerchantMiddleware 加载商户信息到 context
// 必须在 CasbinRoleMiddleware(RoleMerchantOwner) 之后使用
func (server *Server) LoadMerchantMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

		// 通过 user_role 的 related_entity_id 获取商户 ID
		userRole, err := server.store.GetUserRoleByType(ctx, db.GetUserRoleByTypeParams{
			UserID: authPayload.UserID,
			Role:   RoleMerchantOwner,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				ctx.AbortWithStatusJSON(http.StatusForbidden, errorResponse(
					errors.New("merchant owner role not found"),
				))
				return
			}
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		if !userRole.RelatedEntityID.Valid {
			ctx.AbortWithStatusJSON(http.StatusForbidden, errorResponse(
				errors.New("merchant not associated with this user"),
			))
			return
		}

		// 加载商户信息
		merchant, err := server.store.GetMerchant(ctx, userRole.RelatedEntityID.Int64)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				ctx.AbortWithStatusJSON(http.StatusForbidden, errorResponse(
					errors.New("merchant not found"),
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

		// 缓存到 context
		ctx.Set(merchantKey, merchant)
		ctx.Next()
	}
}

// LoadRiderMiddleware 加载骑手信息到 context
// 必须在 CasbinRoleMiddleware(RoleRider) 之后使用
func (server *Server) LoadRiderMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

		// 加载骑手信息
		rider, err := server.store.GetRiderByUserID(ctx, authPayload.UserID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				ctx.AbortWithStatusJSON(http.StatusForbidden, errorResponse(
					errors.New("rider profile not found"),
				))
				return
			}
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		// 检查骑手状态：需要已完成微信支付开户（active）
		// 旧状态 approved 也兼容（用于尚未迁移的骑手）
		if rider.Status != "active" && rider.Status != "approved" {
			ctx.AbortWithStatusJSON(http.StatusForbidden, errorResponse(
				errors.New("rider account is not active, please complete WeChat payment registration"),
			))
			return
		}

		// 缓存到 context
		ctx.Set(riderKey, rider)
		ctx.Next()
	}
}

// ==================== 区域验证中间件 ====================

// ValidateOperatorRegionMiddleware 验证 operator 是否管理指定区域
// 必须在 LoadOperatorMiddleware 之后使用
// regionParamName 是 URL 参数名，如 "region_id" 或 "id"
func (server *Server) ValidateOperatorRegionMiddleware(regionParamName string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// 从 context 获取 operator
		operator, exists := GetOperatorFromContext(ctx)
		if !exists {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, internalError(ctx,
				errors.New("operator not loaded, ensure LoadOperatorMiddleware is applied"),
			))
			return
		}

		// 从 URL 获取区域 ID
		var uri struct {
			RegionID int64 `uri:"region_id"`
			ID       int64 `uri:"id"`
		}
		if err := ctx.ShouldBindUri(&uri); err != nil {
			ctx.AbortWithStatusJSON(http.StatusBadRequest, errorResponse(err))
			return
		}

		regionID := uri.RegionID
		if regionParamName == "id" {
			regionID = uri.ID
		}

		if regionID == 0 {
			ctx.AbortWithStatusJSON(http.StatusBadRequest, errorResponse(
				errors.New("region_id is required"),
			))
			return
		}

		// 检查 operator 是否管理该区域
		manages, err := server.store.CheckOperatorManagesRegion(ctx, db.CheckOperatorManagesRegionParams{
			OperatorID: operator.ID,
			RegionID:   regionID,
		})
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}

		if !manages {
			ctx.AbortWithStatusJSON(http.StatusForbidden, errorResponse(
				errors.New("you don't have permission to manage this region"),
			))
			return
		}

		ctx.Next()
	}
}
