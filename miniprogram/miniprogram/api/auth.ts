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
  id: string  // 用户ID (Supabase UUID)
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
  session_id: string  // 会话ID
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
import { SUPABASE_URL, SUPABASE_KEY } from '../services/supabase'

/**
 * 微信登录 - 迁移至 Supabase Edge Function
 */
export async function wechatLogin(data: WechatLoginRequest): Promise<WechatLoginResponse> {
  // Edge Function Invoke
  return new Promise((resolve, reject) => {
    wx.request({
      url: `${SUPABASE_URL}/functions/v1/wechat-login`,
      method: 'POST',
      data: data,
      header: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${SUPABASE_KEY}`
      },
      success: (res) => {
        if (res.statusCode >= 200 && res.statusCode < 300) {
          const body = res.data as any
          const { session, user } = body

          if (!session || !session.access_token) {
            reject(new Error('Invalid response: missing session'))
            return
          }

          resolve({
            access_token: session.access_token,
            access_token_expires_at: new Date(Date.now() + session.expires_in * 1000).toISOString(),
            refresh_token: session.refresh_token,
            refresh_token_expires_at: new Date(Date.now() + 30 * 24 * 60 * 60 * 1000).toISOString(),
            session_id: '0',
            user: {
              id: user.id,
              full_name: user.user_metadata?.full_name || '',
              avatar_url: user.user_metadata?.avatar_url || '',
              roles: user.roles || ['CUSTOMER'],
              wechat_openid: user.user_metadata?.openid
            } as any
          })
        } else {
          reject(new Error(`Login failed: ${JSON.stringify(res.data)}`))
        }
      },
      fail: (err) => {
        reject(new Error(err.errMsg || 'Network request failed'))
      }
    })
  })
}

/**
 * 刷新访问令牌 - 使用 Supabase Auth API
 */
export function renewAccessToken(data: RefreshTokenRequest) {
  return new Promise<WechatLoginResponse>((resolve, reject) => {
    wx.request({
      url: `${SUPABASE_URL}/auth/v1/token?grant_type=refresh_token`,
      method: 'POST',
      data: { refresh_token: data.refresh_token },
      header: {
        'Content-Type': 'application/json',
        'apikey': SUPABASE_KEY
      },
      success: (res) => {
        if (res.statusCode >= 200 && res.statusCode < 300) {
          const session = res.data as any
          // Convert Supabase Token Response to WechatLoginResponse format (partial)
          resolve({
            access_token: session.access_token,
            access_token_expires_at: new Date(Date.now() + session.expires_in * 1000).toISOString(),
            refresh_token: session.refresh_token,
            refresh_token_expires_at: new Date(Date.now() + 30 * 24 * 60 * 60 * 1000).toISOString(), // Dummy 30d
            session_id: '0',
            user: { id: session.user?.id } as any // Minimal user info
          })
        } else {
          reject(new Error(`Token refresh failed: ${JSON.stringify(res.data)}`))
        }
      },
      fail: (err) => {
        reject(new Error(err.errMsg || 'Network request failed'))
      }
    })
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
