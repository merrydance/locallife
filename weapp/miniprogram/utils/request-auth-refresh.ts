import { clearToken, setToken, isTokenNearExpiry, hasToken } from './auth'
import { ApiResponse, ErrorCode } from '../api/types'
import { logger } from './logger'
import { ErrorType, AppError } from './error-handler'
import { API_CONFIG } from '../config/index'
import { ensureWechatLoginSession } from './wechat-login-session'
import { markNativeOperationStart } from './native-diagnostics'

interface RefreshTokenPayload {
  access_token: string
  refresh_token?: string
  access_token_expires_at?: string
}

let refreshingPromise: Promise<void> | null = null

const REFRESH_THRESHOLD = 5 * 60 * 1000
const REFRESH_TIMEOUT = 25000
const TOKEN_REFRESH_REQUEST_TIMEOUT = 10000

export async function performTokenRefresh(force: boolean = false): Promise<void> {
  if (refreshingPromise) {
    logger.debug('检测到正在刷新Token,复用Promise', undefined, 'performTokenRefresh')
    return refreshingPromise
  }

  if (!force && !isTokenNearExpiry(REFRESH_THRESHOLD)) {
    return
  }

  logger.info('开始刷新Token', { force }, 'performTokenRefresh')

  refreshingPromise = new Promise<void>((resolve, reject) => {
    refreshTokenWithTimeout()
      .then(resolve)
      .catch(reject)
      .finally(() => {
        setTimeout(() => {
          refreshingPromise = null
        }, 500)
      })
  })

  return refreshingPromise
}

export async function ensureValidToken(): Promise<void> {
  if (!hasToken()) {
    return performTokenRefresh(true)
  }
  return performTokenRefresh(false)
}

export async function refreshAuthToken(force: boolean = false): Promise<void> {
  return performTokenRefresh(force)
}

async function refreshTokenWithTimeout(): Promise<void> {
  return new Promise<void>((resolve, reject) => {
    const timeoutHandle = setTimeout(() => {
      reject(new AppError({
        type: ErrorType.NETWORK,
        message: 'Token刷新超时',
        userMessage: '网络超时,请重试'
      }))
    }, REFRESH_TIMEOUT)

    refreshTokenOnce()
      .then(resolve)
      .catch(reject)
      .finally(() => {
        clearTimeout(timeoutHandle)
      })
  })
}

async function refreshTokenOnce(): Promise<void> {
  try {
    const { getRefreshToken } = require('./auth')
    const refreshToken = getRefreshToken()

    if (refreshToken) {
      logger.info('尝试使用refresh_token刷新', undefined, 'refreshTokenOnce')
      try {
        const res = await new Promise<WechatMiniprogram.RequestSuccessCallbackResult>((resolve, reject) => {
          const finishNativeOperation = markNativeOperationStart('wx.request', {
            source: 'refreshTokenOnce',
            method: 'POST',
            path: '/v1/auth/refresh',
            timeout: TOKEN_REFRESH_REQUEST_TIMEOUT
          })
          wx.request({
            url: `${API_CONFIG.BASE_URL}/v1/auth/refresh`,
            method: 'POST',
            data: { refresh_token: refreshToken },
            header: { 'Content-Type': 'application/json', 'X-Response-Envelope': '1' },
            success: (response) => {
              finishNativeOperation('success', { statusCode: response.statusCode })
              resolve(response)
            },
            fail: (err) => {
              finishNativeOperation('fail', err)
              reject(err)
            },
            timeout: TOKEN_REFRESH_REQUEST_TIMEOUT
          })
        })

        const response = res.data as ApiResponse<RefreshTokenPayload>
        if (res.statusCode === 200 && response.code === ErrorCode.SUCCESS && response.data?.access_token) {
          const d = response.data
          const expiresAt = d.access_token_expires_at ? new Date(d.access_token_expires_at).getTime() : undefined
          setToken(d.access_token, expiresAt, d.refresh_token)
          logger.info('refresh_token刷新成功', undefined, 'refreshTokenOnce')
          return
        }
        logger.warn('refresh_token刷新失效，尝试重新登录', response, 'refreshTokenOnce')
      } catch (e) {
        logger.warn('refresh_token请求失败，尝试重新登录', e, 'refreshTokenOnce')
      }
    }

    logger.info('开始wx.login重新登录', undefined, 'refreshTokenOnce')
    const loginData = await ensureWechatLoginSession()
    if (loginData?.access_token) {
      logger.info('wx.login重登录成功', undefined, 'refreshTokenOnce')
      return
    }

    throw new AppError({
      type: ErrorType.AUTH,
      message: '自动登录失败',
      userMessage: '登录已过期，请手动重新登录'
    })
  } catch (err) {
    logger.error('Token刷新流程彻底失败', err, 'refreshTokenOnce')
    clearToken()
    throw err
  }
}
