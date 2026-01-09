/**
 * 统一日志工具
 * 用于替代console.log/error,支持环境判断和日志上报
 */
import { ErrorHandler } from './error-handler'

export enum LogLevel {
  DEBUG = 'DEBUG',
  INFO = 'INFO',
  WARN = 'WARN',
  ERROR = 'ERROR'
}

interface LogEntry {
  level: LogLevel
  message: string
  context?: string
  data?: unknown
  timestamp: number
}

class Logger {
  private isDev: boolean = false
  private logs: LogEntry[] = []
  private maxLogs: number = 100 // 最多保存100条日志

  constructor() {
    try {
      const accountInfo = wx.getAccountInfoSync()
      this.isDev = accountInfo.miniProgram.envVersion === 'develop'
    } catch (e) {
      this.isDev = true // 默认开发环境
    }
  }

  /**
     * 调试日志 - 仅开发环境输出
     */
  debug(message: string, data?: unknown, context?: string) {
    this.log(LogLevel.DEBUG, message, data, context)
  }

  /**
     * 信息日志
     */
  info(message: string, data?: unknown, context?: string) {
    this.log(LogLevel.INFO, message, data, context)
  }

  /**
     * 警告日志
     */
  warn(message: string, data?: unknown, context?: string) {
    this.log(LogLevel.WARN, message, data, context)
  }

  /**
     * 错误日志 - 会上报到监控平台
     */
  error(message: string, error?: unknown, context?: string) {
    // 检测是否是后端不可用错误(502/503/504)
    const isBackendError = ErrorHandler.isBackendUnavailable(error)

    if (isBackendError) {
      // 后端不可用时使用简洁的warn级别日志
      this.warn(`${message} - 后端服务未启动`, undefined, context)
    } else {
      // 正常错误使用完整日志
      this.log(LogLevel.ERROR, message, error, context)

      // 生产环境上报错误
      if (!this.isDev) {
        this.reportError(message, error, context)
      }
    }
  }

  /**
     * 核心日志方法
     */
  private log(level: LogLevel, message: string, data?: unknown, context?: string) {
    const entry: LogEntry = {
      level,
      message,
      context,
      data,
      timestamp: Date.now()
    }

    // 保存日志
    this.logs.push(entry)
    if (this.logs.length > this.maxLogs) {
      this.logs.shift() // 移除最老的日志
    }

    // 开发环境输出到控制台
    if (this.isDev) {
      const prefix = context ? `[${context}]` : ''
      const logMessage = `${prefix} ${message}`

      switch (level) {
        case LogLevel.DEBUG:
          console.log(`🔍 ${logMessage}`, data || '')
          break
        case LogLevel.INFO:
          console.log(`ℹ️ ${logMessage}`, data || '')
          break
        case LogLevel.WARN:
          console.warn(`⚠️ ${logMessage}`, data || '')
          break
        case LogLevel.ERROR:
          console.error(`❌ ${logMessage}`, data || '')
          break
      }
    }
  }

  /**
     * 上报错误到监控平台
     */
  private reportError(message: string, error?: unknown, context?: string) {
    try {
      const errorData = {
        message,
        context,
        error: error instanceof Error ? {
          name: error.name,
          message: error.message,
          stack: error.stack
        } : error,
        timestamp: Date.now(),
        userAgent: {
          ...wx.getDeviceInfo(),
          ...wx.getWindowInfo(),
          ...wx.getAppBaseInfo()
        },
        page: getCurrentPages().pop()?.route || 'unknown'
      }

      // 1. 使用微信小程序实时日志 (免费且集成在微信开发者工具)
      const realtimeLog = wx.getRealtimeLogManager ? wx.getRealtimeLogManager() : null
      if (realtimeLog) {
        realtimeLog.error('[ERROR]', {
          msg: message,
          ctx: context,
          err: String(error),
          page: errorData.page
        })
      }

      // 2. 上报到自己的后端(可选)
      if (!this.isDev) {
        wx.request({
          url: 'https://llapi.merrydance.cn/api/v1/logs/error',
          method: 'POST',
          data: errorData,
          header: {
            'Content-Type': 'application/json'
          },
          fail: () => {
            // 静默失败
          }
        })
      }

      // 3. 腾讯云日志服务CLS (如果已配置)
      // if (typeof __wxConfig !== 'undefined' && __wxConfig.logConf) {
      //     wx.reportRealtimeLog({
      //         level: 'error',
      //         msg: message,
      //         ext: errorData
      //     })
      // }

      // 开发环境输出完整错误信息
      if (this.isDev) {
        console.log('📤 Error Report:', errorData)
      }
    } catch (e) {
      // 上报失败不应影响主流程
      console.error('Failed to report error:', e)
    }
  }

  /**
     * 获取最近的日志记录
     */
  getRecentLogs(count: number = 50): LogEntry[] {
    return this.logs.slice(-count)
  }

  /**
     * 清空日志
     */
  clearLogs() {
    this.logs = []
  }

  /**
     * 导出日志 (用于调试)
     */
  exportLogs(): string {
    return JSON.stringify(this.logs, null, 2)
  }
}

// 导出单例
export const logger = new Logger()
