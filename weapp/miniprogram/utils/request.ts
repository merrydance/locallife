import { getToken, clearToken, setToken, isTokenNearExpiry, hasToken } from './auth'
import { ApiResponse, ErrorCode } from '../api/types'
import { logger } from './logger'
import { ErrorHandler, ErrorType, AppError } from './error-handler'
import { requestManager } from './request-manager'
import { networkMonitor } from './network-monitor'
import { CacheManager, CacheStrategy } from './cache'
import { API_CONFIG, ENV } from '../config/index'
import { performanceMonitor } from './performance-monitor'

export const API_BASE = API_CONFIG.BASE_URL

interface RequestOptions {
  url: string
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH'
  data?: unknown
  loading?: boolean
  loadingText?: string
  context?: string // 请求上下文,用于批量取消
  requestId?: string // 自定义请求ID
  retry?: boolean | number // 是否重试,或重试次数
  useCache?: boolean // 是否使用缓存
  cacheTTL?: number // 缓存时间(毫秒)
  skipAuth?: boolean // 跳过 token 验证和刷新（用于登录、刷新 token 等不需要认证的接口）
}

const cache = new CacheManager()

interface RefreshTokenPayload {
  access_token: string
  refresh_token?: string
  access_token_expires_at?: string
}

type WechatLoginPayload = RefreshTokenPayload

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

function mapSearchBadRequestMessage(url: string, backendMessage: string, fallback: string): string {
  const normalized = backendMessage.toLowerCase()
  const isNoRows = normalized.includes('no rows in result set')

  if (!isNoRows) {
    return fallback
  }

  if (url.includes('/v1/search/dishes')) {
    return '当前区域暂无可推荐菜品'
  }
  if (url.includes('/v1/search/rooms')) {
    return '当前区域暂无可预订包间'
  }
  if (url.includes('/v1/search/merchants')) {
    return '当前区域暂无可浏览商家'
  }
  if (url.includes('/v1/search/combos')) {
    return '当前区域暂无可推荐套餐'
  }

  return '当前区域暂无可用内容'
}

function isExpectedOperatorApplicationNotFound(
  method: string,
  url: string,
  statusCode: number,
  backendMessage: string,
  envelopeCode?: number
): boolean {
  if (method !== 'GET' || url !== '/v1/operator/application' || statusCode !== 404) {
    return false
  }

  if (envelopeCode === ErrorCode.NOT_FOUND) {
    return true
  }

  const normalized = backendMessage.toLowerCase()
  return normalized.includes('您还没有申请记录') || normalized.includes('no application')
}

export function uploadFile<T = unknown>(filePath: string, url: string = '/upload/image', name: string = 'file', formData: Record<string, unknown> = {}): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const doUpload = () => {
      wx.uploadFile({
        url: `${API_BASE}${url}`,
        filePath,
        name,
        header: {
          'Authorization': `Bearer ${getToken()}`,
          'X-Response-Envelope': '1'
        },
        formData,
        success: (res) => {
          // wx.uploadFile returns data as string
          let data: unknown
          try {
            data = JSON.parse(res.data)
          } catch (e) {
            // If not JSON, probably error page or simple string
            data = res.data
          }

          if (res.statusCode === 200 || res.statusCode === 201) {
            // Verify code if it exists in envelope
            if (isRecord(data) && typeof data.code === 'number') {
              if (data.code === 0) {
                logger.debug('文件上传成功', { url }, 'uploadFile')
                // Return the data part as T, or the whole thing?
                // request() returns response.data. 
                // Existing uploadFile returned data.data.url string.
                // To be generic, let's return data.data usually.
                resolve(data.data as T)
              } else {
                const message = typeof data.message === 'string' ? data.message : '上传失败'
                reject(new AppError({
                  type: ErrorType.BUSINESS,
                  message: `上传失败: ${message}`,
                  userMessage: message
                }))
              }
            } else {
              // Legacy behavior or different format
              resolve(data as T)
            }
          } else if (res.statusCode === 401) {
            // Token expired
            logger.warn('Token已过期(upload),尝试自动刷新', undefined, 'uploadFile')
            performTokenRefresh(true).then(() => {
              // Retry upload
              doUpload()
            }).catch((_err) => {
              clearToken()
              // User requested silent login, no redirect.
              reject(new AppError({
                type: ErrorType.AUTH,
                message: '登录已过期且刷新失败',
                userMessage: '登录状态失效，请重试'
              }))
            })
          } else {
            // 解析后端返回的错误信息
            const errMsg = isRecord(data) && typeof data.message === 'string'
              ? data.message
              : isRecord(data) && typeof data.error === 'string'
                ? data.error
                : '文件上传失败'
            const userMsg = isRecord(data) && typeof data.userMessage === 'string'
              ? data.userMessage
              : '文件上传失败'

            logger.warn(`上传失败 HTTP ${res.statusCode}`, { url, response: data }, 'uploadFile')

            reject(new AppError({
              type: ErrorType.NETWORK,
              message: `HTTP ${res.statusCode}: ${errMsg}`,
              userMessage: userMsg
            }, data))
          }
        },
        fail: (err) => {
          const error = ErrorHandler.handleNetworkError(err, 'uploadFile')
          reject(error)
        }
      })
    }

    // Check token expiry before starting (optional optimization)
    if (isTokenNearExpiry(60000)) {
      refreshTokenOnce().then(doUpload).catch(() => doUpload())
    } else {
      doUpload()
    }
  })
}

export async function request<T = unknown>(options: RequestOptions): Promise<T> {
  const {
    url,
    method = 'GET',
    data,
    loading = false, // 默认为 false，由页面根据业务逻辑显示骨架屏或局部加载
    loadingText = '加载中...',
    context,
    requestId = `${method}_${url}_${Date.now()}`,
    retry = false,
    useCache = false,
    cacheTTL = 5 * 60 * 1000, // 默认5分钟
    skipAuth = false // 是否跳过认证
  } = options

  // 智能缓存策略(GET请求)
  if (useCache && method === 'GET') {
    const cacheKey = `api_${url}_${JSON.stringify(data || {})}`
    const cached = cache.get<T>(cacheKey)
    if (cached) {
      logger.debug(`✅ 命中缓存: ${url}`, { cacheTTL }, 'request')

      // 记录性能监控 - 缓存命中
      performanceMonitor.recordRequest(true)
      const cacheAge = cache.getAge(cacheKey)
      if (cacheAge && cacheAge > cacheTTL * 0.8) {
        logger.debug(`🔄 后台刷新缓存: ${url}`, undefined, 'request')
        // 异步刷新，不阻塞当前请求
        setTimeout(() => {
          request({ ...options, useCache: false }).then((freshData) => {
            cache.set(cacheKey, freshData, cacheTTL)
          }).catch(() => {
            // 刷新失败，保留旧缓存
          })
        }, 100)
      }

      return cached
    }
  }

  // 检查网络状态
  if (!networkMonitor.isOnline()) {
    const error = new AppError({
      type: ErrorType.NETWORK,
      message: '网络不可用',
      userMessage: '网络连接失败,请检查网络设置'
    })

    if (loading) wx.hideLoading()

    // 显示重试按钮
    showRetryDialog(error, () => request(options))
    throw error
  }

  if (loading) {
    wx.showLoading({ title: loadingText, mask: true })
  }

  try {
    // 在每次请求前，若 token 在阈值内即将过期，则先尝试刷新一次（单并发）
    // 跳过认证的请求（如登录、刷新 token）不需要检查 token
    if (!skipAuth) {
      await ensureValidToken()
    }

    const app = getApp<IAppOption>()
    const latitude = app?.globalData?.latitude || 0
    const longitude = app?.globalData?.longitude || 0

    logger.debug(`API请求: ${method} ${url}`, { data, latitude, longitude, requestId }, 'request')

    const requestData = data as WechatMiniprogram.IAnyObject | string | ArrayBuffer | undefined
    const result = await new Promise<WechatMiniprogram.RequestSuccessCallbackResult>((resolve, reject) => {
      const task = wx.request({
        url: `${API_BASE}${url}`,
        method: method as WechatMiniprogram.RequestOption['method'],
        data: requestData,
        header: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${getToken()}`,
          'X-User-Latitude': String(latitude),
          'X-User-Longitude': String(longitude),
          'X-Response-Envelope': '1' // 启用统一响应信封：{ code, message, data }
        },
        success: (res) => {
          requestManager.unregister(requestId)
          resolve(res)
        },
        fail: (err) => {
          requestManager.unregister(requestId)
          // Ensure err is an object, not null or undefined
          const errorInfo = err || { errMsg: 'wx.request failed with no error info' }
          reject(errorInfo)
        }
      })

      // 注册请求任务以便取消
      requestManager.register(requestId, task, context)
    })

    // 204 No Content 视为成功（如 DELETE 返回空）
    if (result.statusCode === 204) {
      performanceMonitor.recordRequest(false)
      logger.debug(`API响应成功(204): ${method} ${url}`, undefined, 'request')
      return undefined as T
    }

    // 检查HTTP状态码
    if (result.statusCode !== 200 && result.statusCode !== 201) {
      // 特殊处理 401 Unauthorized
      if (result.statusCode === 401) {
        logger.warn('Token无效(HTTP 401),尝试自动刷新', undefined, 'request')
        try {
          await performTokenRefresh(true)
          logger.info('Token自动刷新成功,重试请求', undefined, 'request')
          if (loading) wx.hideLoading()
          return request<T>(options)
        } catch (refreshError) {
          logger.error('Token刷新失败(HTTP 401)', refreshError, 'request')
          clearToken()
          if (loading) wx.hideLoading()
          // User requested silent login, so no redirect. Just throw.
          throw new AppError({
            type: ErrorType.AUTH,
            message: '登录已过期且自动刷新失败',
            userMessage: '登录状态失效，请重试'
          })
        }
      }

      // 尝试从后端响应中提取错误信息
      const responseData = result.data as unknown
      const envelopeCode = isRecord(responseData) && typeof responseData.code === 'number'
        ? responseData.code
        : undefined
      const envelopeMessage = isRecord(responseData) && typeof responseData.message === 'string'
        ? responseData.message
        : undefined

      const backendMessage = (() => {
        if (envelopeMessage) return envelopeMessage
        if (!isRecord(responseData)) return ''
        if (typeof responseData.error === 'string') return responseData.error
        const dataField = responseData.data
        if (isRecord(dataField)) {
          if (typeof dataField.error === 'string') return dataField.error
          if (typeof dataField.message === 'string') return dataField.message
        }
        return ''
      })()

      if (isExpectedOperatorApplicationNotFound(method, url, result.statusCode, backendMessage, envelopeCode)) {
        logger.debug(`预期业务状态: ${method} ${url} 无申请记录`, {
          statusCode: result.statusCode,
          code: envelopeCode,
          message: backendMessage
        }, 'request')
      } else {
        logger.error(`HTTP错误: ${method} ${url}`, {
          statusCode: result.statusCode,
          data: result.data
        }, 'request')
      }

      if (envelopeCode !== undefined) {
        let userMessage = backendMessage || '服务器响应异常,请稍后重试'
        let errorDetail = `API ${envelopeCode}`
        let errorType = ErrorType.BUSINESS

        if (envelopeCode === ErrorCode.BAD_REQUEST) {
          userMessage = backendMessage || '请求参数错误'
          userMessage = mapSearchBadRequestMessage(url, backendMessage, userMessage)
          if (backendMessage === 'merchant is not accepting takeout orders') {
            userMessage = '商户休息中～'
          }
          errorDetail = `参数错误(${envelopeCode}): ${backendMessage}`
        } else if (envelopeCode === ErrorCode.CONFLICT) {
          userMessage = backendMessage || '操作冲突，请稍后重试'
          errorDetail = `冲突(${envelopeCode}): ${backendMessage}`
        } else if (envelopeCode === ErrorCode.NOT_FOUND) {
          userMessage = backendMessage || '服务暂时不可用,请稍后重试'
          errorDetail = backendMessage ? `服务未找到(${envelopeCode}): ${backendMessage}` : `服务未找到(${envelopeCode})`
        } else if (
          envelopeCode === ErrorCode.BAD_GATEWAY ||
          envelopeCode === ErrorCode.SERVICE_UNAVAILABLE ||
          envelopeCode === ErrorCode.GATEWAY_TIMEOUT
        ) {
          userMessage = '服务暂时不可用,请稍后重试'
          errorDetail = `网关错误(${envelopeCode})`
          errorType = ErrorType.NETWORK
        } else if (envelopeCode === ErrorCode.INTERNAL_ERROR) {
          userMessage = '服务器内部错误,请稍后重试'
          errorDetail = `服务器错误(${envelopeCode})`
          errorType = ErrorType.NETWORK
        } else if (envelopeCode === ErrorCode.UNAUTHORIZED) {
          userMessage = '登录已过期,请重新登录'
          errorDetail = `认证失败(${envelopeCode})`
          errorType = ErrorType.AUTH
        } else if (envelopeCode === ErrorCode.FORBIDDEN) {
          userMessage = backendMessage || '无权限操作'
          errorDetail = `权限不足(${envelopeCode}): ${backendMessage}`
          errorType = ErrorType.PERMISSION
        } else if (envelopeCode === ErrorCode.UNPROCESSABLE) {
          userMessage = backendMessage || '请求语义错误'
          errorDetail = `语义错误(${envelopeCode}): ${backendMessage}`
        } else if (envelopeCode === ErrorCode.TOO_MANY_REQUESTS) {
          userMessage = '请求过于频繁，请稍后重试'
          errorDetail = `限流(${envelopeCode})`
        }

        throw new AppError({
          type: errorType,
          message: errorDetail,
          userMessage
        }, responseData)
        }

      // 常见HTTP错误处理
      let userMessage = backendMessage || '服务器响应异常,请稍后重试'
      let errorDetail = `HTTP ${result.statusCode}`
      let errorType = ErrorType.NETWORK

      if (result.statusCode === 400) {
        // 400 Bad Request - 请求参数错误，显示后端返回的具体错误信息
        userMessage = backendMessage || '请求参数错误'
        userMessage = mapSearchBadRequestMessage(url, backendMessage, userMessage)
        
        // 关键逻辑：如果是商户休息中，返回更友好的提示
        if (backendMessage === 'merchant is not accepting takeout orders') {
            userMessage = '商户休息中～'
        }
        
        errorDetail = `参数错误(400): ${backendMessage}`
        errorType = ErrorType.BUSINESS
      } else if (result.statusCode === 409) {
        // 409 Conflict - 冲突错误（如时间段已被预订），显示后端返回的具体错误信息
        userMessage = backendMessage || '操作冲突，请稍后重试'
        errorDetail = `冲突(409): ${backendMessage}`
        errorType = ErrorType.BUSINESS
      } else if (result.statusCode === 404) {
        userMessage = backendMessage || '服务暂时不可用,请稍后重试'
        errorDetail = backendMessage ? `服务未找到(404): ${backendMessage}` : '服务未找到(404) - 可能是后端服务未启动'
      } else if (result.statusCode === 502 || result.statusCode === 503 || result.statusCode === 504) {
        userMessage = '服务暂时不可用,请稍后重试'
        errorDetail = `网关错误(${result.statusCode}) - 后端服务可能未启动`
      } else if (result.statusCode >= 500) {
        userMessage = '服务器内部错误,请稍后重试'
        errorDetail = `服务器错误(${result.statusCode})`
      } else if (result.statusCode >= 400) {
        // 其他 4xx 客户端错误，优先使用后端返回的消息
        userMessage = backendMessage || '请求失败，请稍后重试'
        errorDetail = `客户端错误(${result.statusCode}): ${backendMessage}`
        errorType = ErrorType.BUSINESS
      }

      throw new AppError({
        type: errorType,
        message: errorDetail,
        userMessage
      })
    }

    const response = result.data as ApiResponse<T>

    // 检查响应是否为HTML(Nginx错误页面)
    if (typeof result.data === 'string') {
      const dataStr = result.data.trim()
      const isHtml = dataStr.startsWith('<') || dataStr.includes('<!DOCTYPE') || dataStr.includes('<html')

      if (isHtml) {
        // 检测Nginx特征
        const isNginxPage = dataStr.includes('nginx') || dataStr.includes('502 Bad Gateway') ||
          dataStr.includes('503 Service') || dataStr.includes('504 Gateway')

        const errorMsg = isNginxPage
          ? 'Nginx错误页面 - 后端服务未响应'
          : 'HTML响应 - 期望JSON格式'

        logger.error(`收到HTML响应而非JSON: ${method} ${url}`, {
          isNginxPage,
          preview: dataStr.substring(0, 300),
          statusCode: result.statusCode
        }, 'request')

        // 开发环境显示更多信息
        const userMsg = ENV.isDev && isNginxPage
          ? '后端服务未启动\n请检查: \n1. 后端服务是否运行\n2. Nginx配置是否正确\n3. 端口是否被占用'
          : '服务暂时不可用,请稍后重试'

        throw new AppError({
          type: ErrorType.NETWORK,
          message: errorMsg,
          userMessage: userMsg
        })
      }
    }

    // 验证响应格式
    if (!response || typeof response !== 'object') {
      logger.error(`API响应格式错误: ${method} ${url}`, {
        dataType: typeof result.data,
        data: result.data
      }, 'request')
      throw new AppError({
        type: ErrorType.BUSINESS,
        message: 'API响应格式错误 - 非对象类型',
        userMessage: '服务器响应异常,请稍后重试'
      })
    }

    // 检查 code 字段是否存在（统一响应信封格式要求所有接口都有 code 字段）
    // 部分旧接口仍直接返回数组/对象，此时视为成功并直接返回原始数据，避免前端崩溃
    if (response.code === undefined || response.code === null) {
      logger.warn(`API响应缺少code字段，按兼容模式处理: ${method} ${url}`, response, 'request')

      // 记录性能监控 - 网络请求成功
      performanceMonitor.recordRequest(false)

      // 缓存兼容：直接缓存原始数据
      if (useCache && method === 'GET') {
        const cacheKey = `api_${url}_${JSON.stringify(data || {})}`
        cache.set(cacheKey, response as unknown as T, cacheTTL, CacheStrategy.MEMORY_FIRST)
      }

      return response as unknown as T
    }

    if (response.code === ErrorCode.SUCCESS) {
      logger.debug(`API响应成功: ${method} ${url}`, response.data, 'request')

      // 记录性能监控 - 网络请求成功
      performanceMonitor.recordRequest(false)

      // 保存缓存(GET请求)
      if (useCache && method === 'GET') {
        const cacheKey = `api_${url}_${JSON.stringify(data || {})}`
        cache.set(cacheKey, response.data, cacheTTL, CacheStrategy.MEMORY_FIRST)
      }

      return response.data
    } else if (response.code === ErrorCode.TOKEN_EXPIRED || response.code === ErrorCode.UNAUTHORIZED) {
      // Token过期,自动静默刷新
      logger.warn('Token已过期,尝试自动刷新', undefined, 'request')

      try {
        // 重新静默登录并刷新 token（通过独立请求避免循环依赖）
        await performTokenRefresh(true)
        logger.info('Token自动刷新成功,重试请求', undefined, 'request')
        // 关闭loading后再重试
        if (loading) wx.hideLoading()
        return request<T>(options)
      } catch (refreshError) {
        // 刷新失败
        logger.error('Token刷新失败', refreshError, 'request')
        clearToken()
        if (loading) wx.hideLoading()
        // Silent failure
        throw new AppError({
          type: ErrorType.AUTH,
          message: '登录已过期且刷新失败',
          userMessage: '登录状态失效，请重试'
        })
      }
    } else {
      // 业务错误
      const errorCode = response.code || 'UNKNOWN'
      const errorMessage = response.message || '未知错误'
      logger.warn(`API业务错误: ${method} ${url}`, { code: errorCode, message: errorMessage, fullResponse: response }, 'request')
      throw new AppError({
        type: ErrorType.BUSINESS,
        message: `API错误 [${errorCode}]: ${errorMessage}`,
        userMessage: errorMessage
      })
    }
  } catch (error) {
    // 网络错误或其他错误
    if (error instanceof AppError) {
      // 如果启用了重试
      if (retry) {
        const retryCount = typeof retry === 'number' ? retry : 1
        logger.warn(`请求失败,将重试 ${retryCount} 次`, { url }, 'request')
        // 关闭loading后再重试
        if (loading) wx.hideLoading()
        return retryRequest(options, retryCount)
      }
      throw error
    }

    // 静默处理abort错误（并发请求被取消的正常情况）
    const errMsg = (error as { errMsg?: string })?.errMsg || ''
    if (errMsg.includes('abort')) {
      // abort是正常的并发控制，静默处理
      if (retry) {
        const retryCount = typeof retry === 'number' ? retry : 1
        if (loading) wx.hideLoading()
        return retryRequest(options, retryCount)
      }
      throw error
    }

    logger.error(`API请求失败: ${method} ${url}`, error || 'Unknown error', 'request')
    const networkError = ErrorHandler.handleNetworkError(error || new Error('Network request failed'), `request:${method}:${url}`)

    // 如果启用了重试
    if (retry) {
      const retryCount = typeof retry === 'number' ? retry : 1
      // 关闭loading后再重试
      if (loading) wx.hideLoading()
      return retryRequest(options, retryCount)
    }

    throw networkError
  } finally {
    // 确保hideLoading在所有情况下都被调用
    if (loading) {
      try {
        wx.hideLoading()
      } catch (e) {
        // 忽略hideLoading的错误
      }
    }
  }
}

/**
 * 重试请求
 */
async function retryRequest<T>(options: RequestOptions, retryCount: number, currentAttempt: number = 0): Promise<T> {
  if (currentAttempt >= retryCount) {
    throw new AppError({
      type: ErrorType.NETWORK,
      message: `请求失败,已重试${retryCount}次`,
      userMessage: '网络请求失败,请稍后重试'
    })
  }

  // 等待一段时间再重试(指数退避)
  const delay = Math.min(1000 * Math.pow(2, currentAttempt), 5000)
  await new Promise((resolve) => setTimeout(resolve, delay))

  try {
    logger.info(`第${currentAttempt + 1}次重试: ${options.url}`, undefined, 'retryRequest')
    return await request({ ...options, retry: false })
  } catch (error) {
    return retryRequest(options, retryCount, currentAttempt + 1)
  }
}

/**
 * 显示重试对话框
 */
function showRetryDialog(error: AppError, retryFn: () => Promise<unknown>) {
  wx.showModal({
    title: '网络异常',
    content: error.userMessage || '网络连接失败',
    confirmText: '重试',
    cancelText: '取消',
    success: (res) => {
      if (res.confirm) {
        retryFn().catch(() => {
          // 重试失败
        })
      }
    }
  })
}

// ==================== Token刷新机制 ====================

// 单次并发刷新锁
let _refreshingPromise: Promise<void> | null = null
const REFRESH_THRESHOLD = 5 * 60 * 1000 // 5分钟
const REFRESH_TIMEOUT = 10000 // 10秒

/**
 * 统一的Token刷新入口 (带锁)
 * @param force 是否强制刷新(忽略有效期检查)
 */
async function performTokenRefresh(force: boolean = false): Promise<void> {
  // 如果已有刷新任务在执行，直接复用
  if (_refreshingPromise) {
    logger.debug('检测到正在刷新Token,复用Promise', undefined, 'performTokenRefresh')
    return _refreshingPromise
  }

  // 非强制模式下，检查是否真的需要刷新
  if (!force && !isTokenNearExpiry(REFRESH_THRESHOLD)) {
    return
  }

  logger.info('开始刷新Token', { force }, 'performTokenRefresh')

  _refreshingPromise = new Promise<void>((resolve, reject) => {
    refreshTokenWithTimeout()
      .then(resolve)
      .catch(reject)
      .finally(() => {
        // 延迟清除锁，防止瞬间并发穿透
        setTimeout(() => {
          _refreshingPromise = null
        }, 500)
      })
  })

  return _refreshingPromise
}

/**
 * 确保Token有效性(请求前检查)
 */
async function ensureValidToken(): Promise<void> {
  if (!hasToken()) {
    return performTokenRefresh(true)
  }
  return performTokenRefresh(false)
}

/**
 * 带超时的Token刷新实现
 */
async function refreshTokenWithTimeout(): Promise<void> {
  return Promise.race([
    refreshTokenOnce(),
    new Promise<void>((_, reject) => {
      setTimeout(() => {
        reject(new AppError({
          type: ErrorType.NETWORK,
          message: 'Token刷新超时',
          userMessage: '网络超时,请重试'
        }))
      }, REFRESH_TIMEOUT)
    })
  ])
}

/**
 * 刷新Token核心逻辑 - 优先使用refresh_token,降级到重新登录
 */
async function refreshTokenOnce(): Promise<void> {
  try {
    const { getRefreshToken } = require('./auth')
    const { getDeviceId } = require('./location')
    const refreshToken = getRefreshToken()

    // 策略1: 使用refresh_token刷新
    if (refreshToken) {
      logger.info('尝试使用refresh_token刷新', undefined, 'refreshTokenOnce')
      try {
        const res = await new Promise<WechatMiniprogram.RequestSuccessCallbackResult>((resolve, reject) => {
          wx.request({
            url: `${API_BASE}/v1/auth/refresh`,
            method: 'POST',
            data: { refresh_token: refreshToken },
            header: { 'Content-Type': 'application/json', 'X-Response-Envelope': '1' },
            success: resolve,
            fail: reject,
            timeout: 5000
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

    // 策略2: 降级到wx.login重新登录
    logger.info('开始wx.login重新登录', undefined, 'refreshTokenOnce')
    const code = await new Promise<string>((resolve, reject) => {
      wx.login({
        success: (res) => res.code ? resolve(res.code) : reject(new Error('获取code失败: res.code missing')),
        fail: (err) => reject(err || new Error('wx.login failed')),
        timeout: 5000
      })
    })

    const deviceId = getDeviceId()
    const res2 = await new Promise<WechatMiniprogram.RequestSuccessCallbackResult>((resolve, reject) => {
      wx.request({
        url: `${API_BASE}/v1/auth/wechat-login`,
        method: 'POST',
        data: { code, device_id: deviceId, device_type: 'miniprogram' },
        header: { 'Content-Type': 'application/json', 'X-Response-Envelope': '1' },
        success: resolve,
        fail: reject,
        timeout: 5000
      })
    })

    const response2 = res2.data as ApiResponse<WechatLoginPayload>
    if (res2.statusCode === 200 && response2.code === ErrorCode.SUCCESS && response2.data?.access_token) {
      const d = response2.data
      const expiresAt = d.access_token_expires_at ? new Date(d.access_token_expires_at).getTime() : undefined
      setToken(d.access_token, expiresAt, d.refresh_token)
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
