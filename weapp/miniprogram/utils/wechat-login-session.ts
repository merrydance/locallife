import { API_CONFIG } from '../config/index'
import { setToken, clearToken } from './auth'
import { logger } from './logger'
import { ErrorCode, type ApiResponse } from '../api/types'
import { AppError, ErrorType } from './error-handler'
import { getDeviceId } from './device-id'
import { markNativeOperationStart } from './native-diagnostics'

export interface WechatLoginSessionData {
  access_token: string
  access_token_expires_at?: string
  refresh_token: string
  refresh_token_expires_at?: string
  session_id: number
  user: {
    id: number
    full_name?: string
    roles: string[]
  }
}

const WECHAT_LOGIN_REQUEST_TIMEOUT = 10000
const WECHAT_LOGIN_NATIVE_TIMEOUT_GUARD = 15000

let _wechatLoginSessionPromise: Promise<WechatLoginSessionData> | null = null

async function requestWechatLoginSession(code: string): Promise<WechatLoginSessionData> {
  const deviceId = getDeviceId()

  const response = await new Promise<WechatMiniprogram.RequestSuccessCallbackResult>((resolve, reject) => {
    const finishNativeOperation = markNativeOperationStart('wx.request', {
      source: 'requestWechatLoginSession',
      method: 'POST',
      path: '/v1/auth/wechat-login',
      timeout: WECHAT_LOGIN_REQUEST_TIMEOUT
    })
    wx.request({
      url: `${API_CONFIG.BASE_URL}/v1/auth/wechat-login`,
      method: 'POST',
      data: {
        code,
        device_id: deviceId,
        device_type: 'miniprogram'
      },
      header: {
        'Content-Type': 'application/json',
        'X-Response-Envelope': '1'
      },
      timeout: WECHAT_LOGIN_REQUEST_TIMEOUT,
      success: (res) => {
        finishNativeOperation('success', { statusCode: res.statusCode })
        resolve(res)
      },
      fail: (err) => {
        finishNativeOperation('fail', err)
        reject(err)
      }
    })
  })

  const envelope = response.data as ApiResponse<WechatLoginSessionData>
  if (response.statusCode !== 200 || envelope.code !== ErrorCode.SUCCESS || !envelope.data?.access_token) {
    throw new AppError({
      type: ErrorType.AUTH,
      message: `微信登录失败: ${response.statusCode}`,
      userMessage: '登录失败，请稍后重试',
      statusCode: response.statusCode
    }, envelope)
  }

  return envelope.data
}

async function doWechatLoginSession(): Promise<WechatLoginSessionData> {
  const code = await new Promise<string>((resolve, reject) => {
    let settled = false
    const startedAt = Date.now()
    const finishNativeOperation = markNativeOperationStart('wx.login', {
      timeoutGuardMs: WECHAT_LOGIN_NATIVE_TIMEOUT_GUARD
    })
    const timeoutHandle = setTimeout(() => {
      if (settled) return
      settled = true
      finishNativeOperation('timeout', { durationMs: Date.now() - startedAt })
      reject(new AppError({
        type: ErrorType.NETWORK,
        message: 'wx.login JS guard timeout',
        userMessage: '登录超时，请稍后重试'
      }))
    }, WECHAT_LOGIN_NATIVE_TIMEOUT_GUARD)

    const settle = (callback: () => void) => {
      if (settled) return
      settled = true
      clearTimeout(timeoutHandle)
      callback()
    }

    logger.debug('开始调用 wx.login', undefined, 'doWechatLoginSession')
    wx.login({
      success: (res) => {
        if (res.code) {
          settle(() => {
            finishNativeOperation('success', { durationMs: Date.now() - startedAt })
            logger.info('wx.login 成功', { durationMs: Date.now() - startedAt }, 'doWechatLoginSession')
            resolve(res.code)
          })
          return
        }
        settle(() => {
          finishNativeOperation('fail', { durationMs: Date.now() - startedAt, errMsg: res.errMsg })
          reject(new Error('wx.login success without code'))
        })
      },
      fail: (err) => {
        settle(() => {
          finishNativeOperation('fail', { durationMs: Date.now() - startedAt, err })
          logger.warn('wx.login 失败', {
            durationMs: Date.now() - startedAt,
            err
          }, 'doWechatLoginSession')
          reject(err || new Error('wx.login failed'))
        })
      }
    })
  })

  const loginData = await requestWechatLoginSession(code)
  const expiresAt = loginData.access_token_expires_at
    ? new Date(loginData.access_token_expires_at).getTime()
    : undefined
  setToken(loginData.access_token, expiresAt, loginData.refresh_token)
  return loginData
}

export async function ensureWechatLoginSession(): Promise<WechatLoginSessionData> {
  if (_wechatLoginSessionPromise) {
    logger.debug('复用微信登录Promise', undefined, 'ensureWechatLoginSession')
    return _wechatLoginSessionPromise
  }

  _wechatLoginSessionPromise = doWechatLoginSession().catch((err) => {
    clearToken()
    throw err
  }).finally(() => {
    setTimeout(() => {
      _wechatLoginSessionPromise = null
    }, 300)
  })

  return _wechatLoginSessionPromise
}
