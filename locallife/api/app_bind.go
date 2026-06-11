package api

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	db "github.com/merrydance/locallife/db/sqlc"
	"github.com/merrydance/locallife/token"
	"github.com/merrydance/locallife/util"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

const (
	appBindCodePrefix             = "app_bind:"
	appBindCodeTTL                = 5 * time.Minute
	appBindCodeLength             = 6
	appBindSessionUserAgentPrefix = "app-bind:"
	// App 端 refresh token 有效期 365 天
	appRefreshTokenDuration = 365 * 24 * time.Hour
)

const saveOrReuseAppBindCodeScript = `
local userKey = KEYS[1]
local candidateCodeKey = KEYS[2]
local candidateCode = ARGV[1]
local bindData = ARGV[2]
local ttlSeconds = tonumber(ARGV[3])
local codePrefix = ARGV[4]

local existingCode = redis.call("GET", userKey)
if existingCode and existingCode ~= "" then
	local existingCodeKey = codePrefix .. existingCode
	local existingBindData = redis.call("GET", existingCodeKey)
	local userTTL = redis.call("TTL", userKey)
	local codeTTL = redis.call("TTL", existingCodeKey)
	if existingBindData and userTTL > 0 and codeTTL > 0 then
		local ttl = userTTL
		if codeTTL < ttl then
			ttl = codeTTL
		end
		return {"reused", existingCode, ttl}
	end
	redis.call("DEL", userKey)
	redis.call("DEL", existingCodeKey)
end

if redis.call("EXISTS", candidateCodeKey) == 1 then
	return {"collision", "", 0}
end

redis.call("SET", candidateCodeKey, bindData, "EX", ttlSeconds)
redis.call("SET", userKey, candidateCode, "EX", ttlSeconds)
return {"generated", candidateCode, ttlSeconds}
`

const consumeAppBindCodeScript = `
local codeKey = KEYS[1]
local userKey = KEYS[2]
local expectedBindData = ARGV[1]
local code = ARGV[2]

local current = redis.call("GET", codeKey)
if not current then
	return {"missing", 0}
end
if current ~= expectedBindData then
	return {"mismatch", 0}
end

local ttl = redis.call("PTTL", codeKey)
redis.call("DEL", codeKey)
if redis.call("GET", userKey) == code then
	redis.call("DEL", userKey)
end
return {"consumed", ttl}
`

const restoreAppBindCodeScript = `
local codeKey = KEYS[1]
local userKey = KEYS[2]
local bindData = ARGV[1]
local code = ARGV[2]
local ttlMillis = tonumber(ARGV[3])

if ttlMillis <= 0 then
	return "skipped"
end
local currentUserCode = redis.call("GET", userKey)
if currentUserCode and currentUserCode ~= code then
	return "skipped"
end
if redis.call("EXISTS", codeKey) == 0 then
	redis.call("SET", codeKey, bindData, "PX", ttlMillis)
	if not currentUserCode then
		redis.call("SET", userKey, code, "PX", ttlMillis)
	end
	return "restored"
end
return "skipped"
`

type appBindCodePersistenceResult struct {
	code      string
	expiresIn int
	reused    bool
}

type appBindCodeConsumeResult struct {
	consumed  bool
	ttlMillis int
}

func makeAppBindSessionUserAgent(userAgent string) string {
	trimmed := strings.TrimSpace(userAgent)
	if trimmed == "" {
		return appBindSessionUserAgentPrefix
	}
	return appBindSessionUserAgentPrefix + trimmed
}

func isAppBindSessionUserAgent(userAgent string) bool {
	return strings.HasPrefix(userAgent, appBindSessionUserAgentPrefix)
}

// generateAppBindCode godoc
// @Summary 生成 App 绑定码
// @Description 商户在小程序端调用，生成 6 位数字绑定码供 App 输入验证。需要 merchant 角色。
// @Tags 认证
// @Produce json
// @Success 200 {object} generateAppBindCodeResponse
// @Failure 403 {object} ErrorResponse "非商户角色"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Security BearerAuth
// @Router /v1/auth/app-bind/code [post]
func (server *Server) generateAppBindCode(ctx *gin.Context) {
	if server.redisClient == nil {
		err := fmt.Errorf("绑定码服务暂不可用")
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "绑定码服务暂不可用", "app bind code redis client not configured"))
		return
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	userID := authPayload.UserID

	if server.rateLimiter != nil {
		limiter := server.rateLimiter.getVisitor(
			"app-bind-code:user:"+strconv.FormatInt(userID, 10),
			rate.Limit(3.0/60.0),
			3,
		)
		if !limiter.Allow() {
			ctx.JSON(http.StatusTooManyRequests, errorResponse(fmt.Errorf("绑定码生成过于频繁，请稍后再试")))
			return
		}
	}

	// 检查用户是否有 merchant 角色
	roles, err := server.store.ListUserRoles(ctx, userID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}

	var merchantID int64
	for _, role := range roles {
		if isAppBindMerchantRole(role) {
			merchantID = role.RelatedEntityID.Int64
			break
		}
	}
	if merchantID == 0 {
		ctx.JSON(http.StatusForbidden, errorResponse(fmt.Errorf("仅商户可生成绑定码")))
		return
	}

	// 幂等索引仅在 app_bind:<code> 验证源也存在且 TTL 正常时才能复用。
	existingKey := fmt.Sprintf("%suser:%d", appBindCodePrefix, userID)
	bindData := fmt.Sprintf("%d:%d", userID, merchantID)
	result, err := server.saveOrReuseAppBindCode(ctx, existingKey, bindData)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if result.reused {
		ctx.JSON(http.StatusOK, generateAppBindCodeResponse{
			Code:      result.code,
			ExpiresIn: result.expiresIn,
		})
		return
	}

	log.Info().
		Int64("user_id", userID).
		Int64("merchant_id", merchantID).
		Msg("app bind code generated")

	ctx.JSON(http.StatusOK, generateAppBindCodeResponse{
		Code:      result.code,
		ExpiresIn: result.expiresIn,
	})
}

func (server *Server) saveOrReuseAppBindCode(ctx *gin.Context, userKey string, bindData string) (appBindCodePersistenceResult, error) {
	ttlSeconds := int(appBindCodeTTL / time.Second)
	for i := 0; i < 5; i++ {
		code, err := generateRandomDigitCode(appBindCodeLength)
		if err != nil {
			return appBindCodePersistenceResult{}, fmt.Errorf("生成绑定码失败: %w", err)
		}
		codeKey := appBindCodePrefix + code

		raw, err := server.redisClient.Eval(ctx, saveOrReuseAppBindCodeScript, []string{userKey, codeKey}, code, bindData, ttlSeconds, appBindCodePrefix).Result()
		if err != nil {
			return appBindCodePersistenceResult{}, fmt.Errorf("保存绑定码失败: %w", err)
		}

		parsed, err := parseAppBindCodePersistenceResult(raw)
		if err != nil {
			return appBindCodePersistenceResult{}, fmt.Errorf("保存绑定码失败: %w", err)
		}
		if parsed.code == "" {
			continue
		}
		return parsed, nil
	}

	return appBindCodePersistenceResult{}, fmt.Errorf("生成绑定码失败: 绑定码冲突重试次数过多")
}

func parseAppBindCodePersistenceResult(raw interface{}) (appBindCodePersistenceResult, error) {
	values, ok := raw.([]interface{})
	if !ok || len(values) != 3 {
		return appBindCodePersistenceResult{}, fmt.Errorf("unexpected redis script result")
	}

	status, ok := values[0].(string)
	if !ok {
		return appBindCodePersistenceResult{}, fmt.Errorf("unexpected redis script status")
	}
	code, ok := values[1].(string)
	if !ok {
		return appBindCodePersistenceResult{}, fmt.Errorf("unexpected redis script code")
	}
	expiresIn, err := parseRedisScriptInt(values[2])
	if err != nil {
		return appBindCodePersistenceResult{}, err
	}

	switch status {
	case "generated":
		return appBindCodePersistenceResult{code: code, expiresIn: expiresIn}, nil
	case "reused":
		return appBindCodePersistenceResult{code: code, expiresIn: expiresIn, reused: true}, nil
	case "collision":
		return appBindCodePersistenceResult{}, nil
	default:
		return appBindCodePersistenceResult{}, fmt.Errorf("unexpected redis script status %q", status)
	}
}

func parseRedisScriptInt(value interface{}) (int, error) {
	switch v := value.(type) {
	case int64:
		return int(v), nil
	case int:
		return v, nil
	case string:
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return 0, fmt.Errorf("unexpected redis script integer %q", v)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unexpected redis script integer")
	}
}

func (server *Server) consumeAppBindCode(ctx *gin.Context, codeKey string, userKey string, bindData string, code string) (appBindCodeConsumeResult, error) {
	raw, err := server.redisClient.Eval(ctx, consumeAppBindCodeScript, []string{codeKey, userKey}, bindData, code).Result()
	if err != nil {
		return appBindCodeConsumeResult{}, fmt.Errorf("删除绑定码失败: %w", err)
	}
	return parseAppBindCodeConsumeResult(raw)
}

func parseAppBindCodeConsumeResult(raw interface{}) (appBindCodeConsumeResult, error) {
	values, ok := raw.([]interface{})
	if !ok || len(values) != 2 {
		return appBindCodeConsumeResult{}, fmt.Errorf("unexpected redis consume result")
	}

	status, ok := values[0].(string)
	if !ok {
		return appBindCodeConsumeResult{}, fmt.Errorf("unexpected redis consume status")
	}
	ttlMillis, err := parseRedisScriptInt(values[1])
	if err != nil {
		return appBindCodeConsumeResult{}, err
	}

	switch status {
	case "consumed":
		return appBindCodeConsumeResult{consumed: true, ttlMillis: ttlMillis}, nil
	case "missing", "mismatch":
		return appBindCodeConsumeResult{}, nil
	default:
		return appBindCodeConsumeResult{}, fmt.Errorf("unexpected redis consume status %q", status)
	}
}

func (server *Server) restoreConsumedAppBindCode(ctx *gin.Context, codeKey string, userKey string, bindData string, code string, ttlMillis int) error {
	if _, err := server.redisClient.Eval(ctx, restoreAppBindCodeScript, []string{codeKey, userKey}, bindData, code, ttlMillis).Result(); err != nil {
		return fmt.Errorf("恢复绑定码失败: %w", err)
	}
	return nil
}

type generateAppBindCodeResponse struct {
	Code      string `json:"code" example:"839471"`
	ExpiresIn int    `json:"expires_in" example:"300"`
}

// verifyAppBindCode godoc
// @Summary 验证 App 绑定码
// @Description App 端调用，使用绑定码换取 JWT token。公开端点，无需认证。
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body verifyAppBindCodeRequest true "绑定码验证请求"
// @Success 200 {object} verifyAppBindCodeResponse
// @Failure 400 {object} ErrorResponse "请求参数错误或绑定码无效"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /v1/auth/app-bind/verify [post]
func (server *Server) verifyAppBindCode(ctx *gin.Context) {
	if server.redisClient == nil {
		err := fmt.Errorf("绑定码服务暂不可用")
		ctx.JSON(http.StatusServiceUnavailable, loggedServerError(ctx, err, "绑定码服务暂不可用", "app bind verify redis client not configured"))
		return
	}

	var req verifyAppBindCodeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// 从 Redis 读取并验证码
	codeKey := appBindCodePrefix + req.Code
	bindData, err := server.redisClient.Get(ctx, codeKey).Result()
	if err == redis.Nil {
		log.Warn().
			Str("device_id", req.DeviceID).
			Str("client_ip", ctx.ClientIP()).
			Str("user_agent", ctx.Request.UserAgent()).
			Str("reason", "redis_nil").
			Msg("app bind code verify rejected")
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("绑定码无效或已过期")))
		return
	}
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("验证绑定码失败: %w", err)))
		return
	}

	// 解析 userID:merchantID
	var userID, merchantID int64
	if _, err := fmt.Sscanf(bindData, "%d:%d", &userID, &merchantID); err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("绑定码数据损坏: %w", err)))
		return
	}

	userKey := fmt.Sprintf("%suser:%d", appBindCodePrefix, userID)

	// 二次校验：确认用户仍有同一商户的 merchant 角色
	roles, err := server.store.ListUserRoles(ctx, userID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("获取用户角色失败: %w", err)))
		return
	}

	hasMatchingMerchantRole := false
	for _, role := range roles {
		if isAppBindMerchantRole(role) && role.RelatedEntityID.Int64 == merchantID {
			hasMatchingMerchantRole = true
			break
		}
	}
	if !hasMatchingMerchantRole {
		if _, err := server.consumeAppBindCode(ctx, codeKey, userKey, bindData, req.Code); err != nil {
			ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
			return
		}
		ctx.JSON(http.StatusForbidden, errorResponse(fmt.Errorf("用户不再具有该商户权限")))
		return
	}

	// 获取用户信息
	user, err := server.store.GetUser(ctx, userID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("获取用户信息失败: %w", err)))
		return
	}

	// 生成 access token（使用标准有效期）
	accessToken, accessPayload, err := server.tokenMaker.CreateToken(
		user.ID,
		server.config.AccessTokenDuration,
		token.TokenTypeAccessToken,
	)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("生成访问令牌失败: %w", err)))
		return
	}

	// 生成 refresh token（App 专用 365 天有效期）
	refreshToken, refreshPayload, err := server.tokenMaker.CreateToken(
		user.ID,
		appRefreshTokenDuration,
		token.TokenTypeRefreshToken,
	)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("生成刷新令牌失败: %w", err)))
		return
	}

	accessTokenHash, err := util.HashToken(accessToken, server.config.TokenSymmetricKey)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("hash access token: %w", err)))
		return
	}

	refreshTokenHash, err := util.HashToken(refreshToken, server.config.TokenSymmetricKey)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("hash refresh token: %w", err)))
		return
	}

	userRolesList, workbenches, err := server.buildUserAccessProfile(ctx, user.ID, roles)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("构建用户信息失败: %w", err)))
		return
	}

	consumeResult, err := server.consumeAppBindCode(ctx, codeKey, userKey, bindData, req.Code)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, err))
		return
	}
	if !consumeResult.consumed {
		ctx.JSON(http.StatusBadRequest, errorResponse(fmt.Errorf("绑定码无效或已过期")))
		return
	}

	// 创建 session（复用现有机制）
	session, err := server.store.CreateSession(ctx, db.CreateSessionParams{
		UserID:                user.ID,
		AccessToken:           accessTokenHash,
		RefreshToken:          refreshTokenHash,
		AccessTokenExpiresAt:  accessPayload.ExpiredAt,
		RefreshTokenExpiresAt: refreshPayload.ExpiredAt,
		UserAgent:             makeAppBindSessionUserAgent(ctx.Request.UserAgent()),
		ClientIp:              ctx.ClientIP(),
	})
	if err != nil {
		if restoreErr := server.restoreConsumedAppBindCode(ctx, codeKey, userKey, bindData, req.Code, consumeResult.ttlMillis); restoreErr != nil {
			log.Error().
				Err(restoreErr).
				Int64("user_id", userID).
				Int64("merchant_id", merchantID).
				Msg("restore app bind code after session creation failure failed")
		}
		ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("创建会话失败: %w", err)))
		return
	}

	log.Info().
		Int64("user_id", userID).
		Int64("merchant_id", merchantID).
		Str("device_id", req.DeviceID).
		Str("device_model", req.DeviceModel).
		Str("os_version", req.OSVersion).
		Str("app_version", req.AppVersion).
		Msg("app bind code verified, session created")

	ctx.JSON(http.StatusOK, verifyAppBindCodeResponse{
		SessionID:             session.ID,
		AccessToken:           accessToken,
		AccessTokenExpiresAt:  accessPayload.ExpiredAt,
		RefreshToken:          refreshToken,
		RefreshTokenExpiresAt: refreshPayload.ExpiredAt,
		User:                  newUserResponse(user, userRolesList, workbenches),
	})
}

func isAppBindMerchantRole(role db.UserRole) bool {
	if role.Status != "" && role.Status != "active" {
		return false
	}
	if !role.RelatedEntityID.Valid || role.RelatedEntityID.Int64 == 0 {
		return false
	}

	switch role.Role {
	case "merchant", "merchant_owner", "merchant_manager":
		return true
	default:
		return false
	}
}

type verifyAppBindCodeRequest struct {
	Code        string `json:"code" binding:"required,len=6,numeric"`
	DeviceID    string `json:"device_id" binding:"required,min=1,max=255"`
	DeviceModel string `json:"device_model" binding:"omitempty,max=100"`
	OSVersion   string `json:"os_version" binding:"omitempty,max=50"`
	AppVersion  string `json:"app_version" binding:"omitempty,max=20"`
}

type verifyAppBindCodeResponse struct {
	SessionID             int64        `json:"session_id"`
	AccessToken           string       `json:"access_token"`
	AccessTokenExpiresAt  time.Time    `json:"access_token_expires_at"`
	RefreshToken          string       `json:"refresh_token"`
	RefreshTokenExpiresAt time.Time    `json:"refresh_token_expires_at"`
	User                  userResponse `json:"user"`
}

// generateRandomDigitCode generates a cryptographically secure random digit code.
func generateRandomDigitCode(length int) (string, error) {
	code := make([]byte, length)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		code[i] = byte('0' + n.Int64())
	}
	return string(code), nil
}
