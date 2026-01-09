import { logger } from './logger'

const TOKEN_KEY = 'access_token'
const TOKEN_INFO_KEY = 'access_token_info'

export interface TokenInfo {
    token: string
    expires_at?: number // ms since epoch
    refresh_token?: string
}

export function getToken(): string {
  try {
    // Backward compatible: if plain token stored under TOKEN_KEY
    const raw = wx.getStorageSync(TOKEN_INFO_KEY)
    if (raw && raw.token) return raw.token

    return wx.getStorageSync(TOKEN_KEY) || ''
  } catch (error) {
    logger.error('获取Token失败', error, 'getToken')
    return ''
  }
}

export function setToken(token: string, expiresAt?: number, refreshToken?: string): void {
  try {
    // Write both legacy key and info object for compatibility
    try { wx.setStorageSync(TOKEN_KEY, token) } catch (e) { /* ignore */ }

    const info: TokenInfo = { token }
    if (typeof expiresAt === 'number') info.expires_at = expiresAt
    if (refreshToken) info.refresh_token = refreshToken

    wx.setStorageSync(TOKEN_INFO_KEY, info)
    logger.debug('Token已设置', undefined, 'setToken')
  } catch (error) {
    logger.error('设置Token失败', error, 'setToken')
  }
}

export function setTokenInfo(info: TokenInfo): void {
  try {
    if (!info || !info.token) return
    setToken(info.token, info.expires_at, info.refresh_token)
  } catch (error) {
    logger.error('设置TokenInfo失败', error, 'setTokenInfo')
  }
}

export function getTokenInfo(): TokenInfo | null {
  try {
    const info = wx.getStorageSync(TOKEN_INFO_KEY) as TokenInfo | null
    if (info && info.token) return info
    // Fallback to legacy token key
    const legacy = wx.getStorageSync(TOKEN_KEY)
    if (legacy) return { token: legacy }
    return null
  } catch (error) {
    logger.error('获取TokenInfo失败', error, 'getTokenInfo')
    return null
  }
}

export function getRefreshToken(): string {
  try {
    const info = getTokenInfo()
    return info?.refresh_token || ''
  } catch (error) {
    logger.error('获取RefreshToken失败', error, 'getRefreshToken')
    return ''
  }
}

export function clearToken(): void {
  try {
    try { wx.removeStorageSync(TOKEN_KEY) } catch (e) { /* ignore */ }
    wx.removeStorageSync(TOKEN_INFO_KEY)
    logger.debug('Token已清除', undefined, 'clearToken')
  } catch (error) {
    logger.error('清除Token失败', error, 'clearToken')
  }
}

export function hasToken(): boolean {
  const t = getToken()
  return !!t
}

/**
 * 判断 token 是否在给定阈值内过期
 * @param thresholdMs - 如果 token 在 thresholdMs 毫秒内到期，则返回 true
 */
export function isTokenNearExpiry(thresholdMs: number = 0): boolean {
  try {
    const info = getTokenInfo()
    if (!info || !info.expires_at) return false
    return Date.now() + thresholdMs >= info.expires_at
  } catch (error) {
    logger.error('检查 token 过期失败', error, 'isTokenNearExpiry')
    return false
  }
}
