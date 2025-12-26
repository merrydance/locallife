/**
 * 统一错误处理器
 * 提供一致的错误处理策略和用户提示
 */

import { logger } from './logger'

declare const __wxConfig: { envVersion?: string } | undefined

export enum ErrorType {
  NETWORK = 'NETWORK',           // 网络错误
  AUTH = 'AUTH',                 // 认证错误
  PERMISSION = 'PERMISSION',     // 权限错误
  VALIDATION = 'VALIDATION',     // 数据验证错误
  BUSINESS = 'BUSINESS',         // 业务逻辑错误
  UNKNOWN = 'UNKNOWN'            // 未知错误
}

interface ErrorConfig {
  type: ErrorType
  message: string
  userMessage?: string  // 用户友好的提示信息
  showToast?: boolean   // 是否显示Toast
  duration?: number     // Toast显示时长
  reportError?: boolean // 是否上报错误
}

export class AppError extends Error {
  type: ErrorType
  userMessage: string
  originalError?: unknown

  constructor(config: ErrorConfig, originalError?: unknown) {
    super(config.message)
    this.name = 'AppError'
    this.type = config.type
    this.userMessage = config.userMessage || config.message
    this.originalError = originalError
  }
}

export class ErrorHandler {
  /**
     * 统一错误处理入口
     */
  static handle(error: unknown, context?: string) {
    let appError: AppError

    // 转换为AppError
    if (error instanceof AppError) {
      appError = error
    } else if (error instanceof Error) {
      appError = new AppError({
        type: ErrorType.UNKNOWN,
        message: error.message,
        userMessage: '操作失败,请稍后重试'
      }, error)
    } else if (typeof error === 'string') {
      appError = new AppError({
        type: ErrorType.UNKNOWN,
        message: error,
        userMessage: error
      })
    } else {
      appError = new AppError({
        type: ErrorType.UNKNOWN,
        message: '未知错误',
        userMessage: '操作失败,请稍后重试'
      }, error)
    }

    // 记录日志(后端服务不可用时使用简洁日志)
    if (ErrorHandler.isBackendUnavailable(appError)) {
      logger.warn(`[后端服务不可用] ${appError.message}`, undefined, context)
    } else {
      logger.error(appError.message, appError.originalError, context)
    }

    // 显示用户提示
    this.showUserMessage(appError)

    return appError
  }

  /**
     * 处理网络错误
     */
  static handleNetworkError(error: unknown, context?: string): AppError {
    const detail = this.resolveErrorText(error) || '未知错误'
    const appError = new AppError({
      type: ErrorType.NETWORK,
      message: `网络请求失败: ${detail}`,
      userMessage: '网络连接失败,请检查网络设置'
    }, error)

    // 后端不可用时使用简洁日志
    if (this.isBackendUnavailable(appError)) {
      logger.warn(`[后端服务不可用] ${appError.message}`, undefined, context)
    } else {
      logger.error(appError.message, error, context)
    }
    this.showUserMessage(appError)

    return appError
  }

  /**
     * 处理认证错误
     */
  static handleAuthError(error: unknown, context?: string): AppError {
    const detail = this.resolveErrorText(error) || '未知错误'
    const appError = new AppError({
      type: ErrorType.AUTH,
      message: `认证失败: ${detail}`,
      userMessage: '登录已过期,请重新登录'
    }, error)

    logger.error(appError.message, error, context)

    // 认证错误跳转到登录页
    wx.reLaunch({
      url: '/pages/user_center/index'
    })

    return appError
  }

  /**
     * 处理权限错误
     */
  static handlePermissionError(permission: string, context?: string): AppError {
    const permissionMessages: Record<string, string> = {
      'scope.userLocation': '需要获取您的位置信息',
      'scope.userInfo': '需要获取您的用户信息',
      'scope.writePhotosAlbum': '需要保存图片到相册',
      'scope.camera': '需要使用相机'
    }

    const appError = new AppError({
      type: ErrorType.PERMISSION,
      message: `权限被拒绝: ${permission}`,
      userMessage: permissionMessages[permission] || '需要相关权限才能继续'
    })

    logger.warn(appError.message, { permission }, context)

    // 显示权限引导
    wx.showModal({
      title: '需要授权',
      content: `${appError.userMessage},请在设置中开启`,
      confirmText: '去设置',
      success: (res) => {
        if (res.confirm) {
          wx.openSetting()
        }
      }
    })

    return appError
  }

  /**
     * 判断是否为后端服务不可用错误
     */
  static isBackendUnavailable(error: unknown): boolean {
    if (!error) return false

    let msg = ''
    if (error instanceof AppError) {
      msg = error.message
    } else if (error instanceof Error) {
      msg = error.message
    } else if (typeof error === 'string') {
      msg = error
    } else if (typeof error === 'object' && 'message' in error) {
      msg = String(error.message)
    }

    const lowerMsg = msg.toLowerCase()
    return (
      lowerMsg.includes('502') ||
      lowerMsg.includes('503') ||
      lowerMsg.includes('504') ||
      lowerMsg.includes('后端服务') ||
      lowerMsg.includes('nginx') ||
      lowerMsg.includes('gateway')
    )
  }

  /**
     * 处理验证错误
     */
  static handleValidationError(field: string, message: string, context?: string): AppError {
    const appError = new AppError({
      type: ErrorType.VALIDATION,
      message: `验证失败: ${field} - ${message}`,
      userMessage: message
    })

    logger.warn(appError.message, { field }, context)
    this.showUserMessage(appError)

    return appError
  }

  /**
     * 处理业务错误
     */
  static handleBusinessError(message: string, context?: string): AppError {
    const appError = new AppError({
      type: ErrorType.BUSINESS,
      message: `业务错误: ${message}`,
      userMessage: message
    })

    logger.warn(appError.message, undefined, context)
    this.showUserMessage(appError)

    return appError
  }

  /**
     * 显示用户提示
     */
  private static showUserMessage(error: AppError) {
    // 认证错误不显示Toast,因为会跳转
    if (error.type === ErrorType.AUTH) {
      return
    }

    // 权限错误已经显示Modal
    if (error.type === ErrorType.PERMISSION) {
      return
    }

    // 后端服务不可用时不弹Toast
    if (this.isBackendUnavailable(error)) {
      return
    }

    // 开发环境显示详细错误信息
    const isDev = typeof __wxConfig !== 'undefined' && __wxConfig.envVersion === 'develop'

    if (isDev && error.type === ErrorType.NETWORK) {
      // 开发环境显示详细的网络错误
      wx.showModal({
        title: '开发环境 - 网络错误',
        content: `${error.userMessage}\n\n技术详情: ${error.message}`,
        showCancel: false,
        confirmText: '知道了'
      })
    } else {
      // 生产环境或其他错误使用Toast
      wx.showToast({
        title: error.userMessage,
        icon: 'none',
        duration: 2500
      })
    }
  }

  /**
     * 安全执行异步函数
     */
  static async safeExecute<T>(
    fn: () => Promise<T>,
    context?: string,
    fallback?: T
  ): Promise<T | undefined> {
    try {
      return await fn()
    } catch (error) {
      this.handle(error, context)
      return fallback
    }
  }

  /**
     * 创建带重试的执行器
     */
  static async executeWithRetry<T>(
    fn: () => Promise<T>,
    options: {
      maxRetries?: number
      retryDelay?: number
      context?: string
    } = {}
  ): Promise<T> {
    const { maxRetries = 3, retryDelay = 1000, context } = options
    let lastError: any

    for (let i = 0; i < maxRetries; i++) {
      try {
        return await fn()
      } catch (error) {
        lastError = error
        logger.warn(`执行失败,准备重试 (${i + 1}/${maxRetries})`, error, context)

        if (i < maxRetries - 1) {
          await new Promise((resolve) => setTimeout(resolve, retryDelay))
        }
      }
    }

    throw this.handle(lastError, context)
  }

  private static resolveErrorText(error: unknown): string {
    if (!error) {
      return ''
    }

    if (typeof error === 'string') {
      return error
    }

    if (error instanceof Error) {
      return error.message
    }

    if (typeof error === 'object' && error !== null) {
      const withErrMsg = error as { errMsg?: unknown }
      if (typeof withErrMsg.errMsg === 'string') {
        return withErrMsg.errMsg
      }

      const withMessage = error as { message?: unknown }
      if (typeof withMessage.message === 'string') {
        return withMessage.message
      }
    }

    return ''
  }
}
