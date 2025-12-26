import { request, API_BASE } from '../utils/request'
import { getDeviceId } from '../utils/location'

// Helper to normalize avatar URL
function normalizeUser(user: UserResponse): UserResponse {
  if (user && user.avatar_url && !user.avatar_url.startsWith('http')) {
    let url = user.avatar_url
    if (url.startsWith('/')) url = url.substring(1)
    user.avatar_url = `${API_BASE}/${url}`
  }
  return user
}

// 基于swagger.json重构的认证接口数据模型

/**
 * 微信登录请求 - 对齐 api.wechatLoginRequest
 */
export interface WechatLoginRequest extends Record<string, unknown> {
  code: string  // 微信授权码，1-256字符
  device_id: string  // 设备ID，1-128字符
  device_type: 'ios' | 'android' | 'miniprogram' | 'h5'  // 设备类型枚举
}

/**
 * 用户信息响应 - 对齐 api.userResponse
 */
export interface UserResponse {
  id: number  // 用户ID，改为number类型
  avatar_url?: string  // 头像URL
  full_name?: string  // 全名
  phone?: string  // 手机号
  roles: string[]  // 用户角色数组
  wechat_openid?: string  // 微信OpenID
  created_at?: string  // 创建时间
}

/**
 * 微信登录响应 - 对齐 api.wechatLoginResponse
 */
export interface WechatLoginResponse {
  access_token: string  // 访问令牌
  access_token_expires_at: string  // 访问令牌过期时间
  refresh_token: string  // 刷新令牌
  refresh_token_expires_at: string  // 刷新令牌过期时间
  session_id: number  // 会话ID
  user: UserResponse  // 用户信息
}

/**
 * 刷新令牌请求 - 对齐 api.renewAccessTokenRequest
 */
export interface RenewAccessTokenRequest extends Record<string, unknown> {
  refresh_token: string  // 刷新令牌（1-1024字符，必填）
}

/**
 * 刷新令牌响应 - 对齐 api.renewAccessTokenResponse
 */
export interface RenewAccessTokenResponse {
  access_token: string  // 新的访问令牌
  access_token_expires_at: string  // 访问令牌过期时间
}

/**
 * 更新用户信息请求 - 对齐 api.updateUserRequest
 */
export interface UpdateUserRequest extends Record<string, unknown> {
  full_name?: string  // 用户全名（1-50字符）
}

/**
 * 错误响应 - 对齐 api.ErrorResponse
 */
export interface ErrorResponse {
  error: string  // 错误消息
}

/**
 * 错误消息 - 对齐 api.errorMessage
 */
export interface ErrorMessage {
  error: string  // 错误消息
}

/**
 * 错误响应 - 对齐 api.errorRes
 */
export interface ErrorRes {
  error: string  // 错误消息
}

/**
 * 角色访问条目 - 对齐 api.RoleAccessEntry
 */
export interface RoleAccessEntry {
  path_prefix: string  // 路径前缀
  roles: string[]  // 允许的角色
  auth_required: boolean  // 是否需要认证
  notes?: string  // 备注
}

/**
 * 角色访问响应 - 对齐 api.RoleAccessResponse
 */
export interface RoleAccessResponse {
  entries: RoleAccessEntry[]  // 访问条目列表
  generated_at: string  // 生成时间
}

// 兼容性别名
export type RefreshTokenRequest = RenewAccessTokenRequest

/**
 * 微信登录 - 使用正确的swagger路径
 * 后端已启用统一响应信封（X-Response-Envelope: 1），返回 { code, message, data } 格式
 */
export function wechatLogin(data: WechatLoginRequest) {
  return request<WechatLoginResponse>({
    url: '/v1/auth/wechat-login',
    method: 'POST',
    data,
    skipAuth: true // 登录接口不需要认证，跳过 token 验证和刷新
  }).then(res => {
    if (res.user) {
      normalizeUser(res.user)
    }
    return res
  })
}

/**
 * 刷新访问令牌 - 基于 /v1/auth/renew-access
 * 后端已启用统一响应信封（X-Response-Envelope: 1），返回 { code, message, data } 格式
 */
export function renewAccessToken(data: RefreshTokenRequest) {
  return request<WechatLoginResponse>({
    url: '/v1/auth/refresh',
    method: 'POST',
    data,
    skipAuth: true // 刷新接口不需要认证，跳过 token 验证和刷新（避免循环调用）
  })
}

/**
 * 获取用户信息 - 基于 /v1/users/me
 */
export function getUserInfo() {
  return request<UserResponse>({
    url: '/v1/users/me',  // 使用swagger中定义的正确路径
    method: 'GET'
  }).then(normalizeUser)
}

/**
 * 更新用户信息 - 基于 PATCH /v1/users/me
 */
export function updateUserInfo(data: { avatar_url?: string; full_name?: string }) {
  return request<UserResponse>({
    url: '/v1/users/me',
    method: 'PATCH',
    data
  }).then(normalizeUser)
}

// 兼容性：保留旧接口名称，但使用新的实现
export const login = wechatLogin
export type LoginRequest = WechatLoginRequest
export type LoginData = WechatLoginResponse
export type UserInfoData = UserResponse

// 导出设备ID生成函数
export { getDeviceId }
