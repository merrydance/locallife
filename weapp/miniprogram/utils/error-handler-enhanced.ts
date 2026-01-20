/**
 * 增强的错误处理工具
 * 提供统一的错误处理、日志记录和用户提示
 */

/**
 * 错误类型
 */
export enum ErrorType {
    NETWORK = 'NETWORK',
    API = 'API',
    VALIDATION = 'VALIDATION',
    PERMISSION = 'PERMISSION',
    BUSINESS = 'BUSINESS',
    UNKNOWN = 'UNKNOWN'
}

/**
 * 错误级别
 */
export enum ErrorLevel {
    INFO = 'INFO',
    WARNING = 'WARNING',
    ERROR = 'ERROR',
    FATAL = 'FATAL'
}

/**
 * 错误信息接口
 */
export interface ErrorInfo {
    type: ErrorType
    level: ErrorLevel
    message: string
    code?: string | number
    details?: any
    timestamp: number
    page?: string
    action?: string
}

/**
 * 错误日志
 */
class ErrorLogger {
    private static logs: ErrorInfo[] = []
    private static maxLogs = 100

    /**
     * 记录错误
     */
    static log(error: ErrorInfo): void {
        this.logs.push(error)

        // 限制日志数量
        if (this.logs.length > this.maxLogs) {
            this.logs.shift()
        }

        // 输出到控制台
        this.logToConsole(error)

        // 可以在这里添加上报到服务器的逻辑
        // this.reportToServer(error)
    }

    /**
     * 输出到控制台
     */
    private static logToConsole(error: ErrorInfo): void {
        const prefix = this.getLogPrefix(error.level)
        const message = `${prefix} [${error.type}] ${error.message}`

        switch (error.level) {
            case ErrorLevel.INFO:
                console.log(message, error.details)
                break
            case ErrorLevel.WARNING:
                console.warn(message, error.details)
                break
            case ErrorLevel.ERROR:
            case ErrorLevel.FATAL:
                console.error(message, error.details)
                break
        }
    }

    /**
     * 获取日志前缀
     */
    private static getLogPrefix(level: ErrorLevel): string {
        switch (level) {
            case ErrorLevel.INFO:
                return 'ℹ️'
            case ErrorLevel.WARNING:
                return '⚠️'
            case ErrorLevel.ERROR:
                return '❌'
            case ErrorLevel.FATAL:
                return '🔥'
        }
    }

    /**
     * 获取所有日志
     */
    static getLogs(): ErrorInfo[] {
        return [...this.logs]
    }

    /**
     * 清除日志
     */
    static clear(): void {
        this.logs = []
    }

    /**
     * 上报到服务器（示例）
     */
    private static reportToServer(error: ErrorInfo): void {
        // 这里可以实现上报逻辑
        // 例如：调用错误上报API
        if (error.level === ErrorLevel.ERROR || error.level === ErrorLevel.FATAL) {
            // wx.request({
            //     url: 'https://api.example.com/error-report',
            //     method: 'POST',
            //     data: error
            // })
        }
    }
}

/**
 * 增强的错误处理器
 */
export class EnhancedErrorHandler {
    /**
     * 处理错误
     */
    static handle(error: any, context?: { page?: string; action?: string }): void {
        const errorInfo = this.parseError(error, context)
        ErrorLogger.log(errorInfo)
        this.showUserMessage(errorInfo)
    }

    /**
     * 解析错误
     */
    private static parseError(error: any, context?: { page?: string; action?: string }): ErrorInfo {
        const errorInfo: ErrorInfo = {
            type: ErrorType.UNKNOWN,
            level: ErrorLevel.ERROR,
            message: '未知错误',
            timestamp: Date.now(),
            page: context?.page,
            action: context?.action
        }

        // 网络错误
        if (error?.errMsg?.includes('request:fail')) {
            errorInfo.type = ErrorType.NETWORK
            errorInfo.message = '网络连接失败，请检查网络设置'
            errorInfo.details = error
        }
        // API错误
        else if (error?.code || error?.statusCode) {
            errorInfo.type = ErrorType.API
            errorInfo.code = error.code || error.statusCode
            errorInfo.message = error.message || error.errMsg || 'API请求失败'
            errorInfo.details = error
        }
        // 验证错误
        else if (error?.name === 'ValidationError') {
            errorInfo.type = ErrorType.VALIDATION
            errorInfo.level = ErrorLevel.WARNING
            errorInfo.message = error.message || '数据验证失败'
            errorInfo.details = error
        }
        // 权限错误
        else if (error?.errMsg?.includes('permission')) {
            errorInfo.type = ErrorType.PERMISSION
            errorInfo.message = '权限不足，请授权后重试'
            errorInfo.details = error
        }
        // 业务错误
        else if (error?.businessCode) {
            errorInfo.type = ErrorType.BUSINESS
            errorInfo.code = error.businessCode
            errorInfo.message = error.message || '业务处理失败'
            errorInfo.details = error
        }
        // 其他错误
        else if (error instanceof Error) {
            errorInfo.message = error.message
            errorInfo.details = {
                name: error.name,
                stack: error.stack
            }
        } else if (typeof error === 'string') {
            errorInfo.message = error
        }

        return errorInfo
    }

    /**
     * 显示用户提示
     */
    private static showUserMessage(error: ErrorInfo): void {
        // 根据错误级别决定是否显示
        if (error.level === ErrorLevel.INFO) {
            return
        }

        // 获取用户友好的错误消息
        const userMessage = this.getUserFriendlyMessage(error)

        // 显示提示
        if (error.level === ErrorLevel.FATAL) {
            wx.showModal({
                title: '严重错误',
                content: userMessage,
                showCancel: false,
                confirmText: '我知道了'
            })
        } else {
            wx.showToast({
                title: userMessage,
                icon: 'none',
                duration: 3000
            })
        }
    }

    /**
     * 获取用户友好的错误消息
     */
    private static getUserFriendlyMessage(error: ErrorInfo): string {
        // 根据错误类型返回友好的消息
        switch (error.type) {
            case ErrorType.NETWORK:
                return '网络连接失败，请检查网络后重试'
            case ErrorType.API:
                if (error.code === 401) {
                    return '登录已过期，请重新登录'
                } else if (error.code === 403) {
                    return '没有权限访问此功能'
                } else if (error.code === 404) {
                    return '请求的资源不存在'
                } else if (error.code === 500) {
                    return '服务器错误，请稍后重试'
                }
                return error.message || 'API请求失败'
            case ErrorType.VALIDATION:
                return error.message || '输入的数据格式不正确'
            case ErrorType.PERMISSION:
                return '需要您的授权才能使用此功能'
            case ErrorType.BUSINESS:
                return error.message || '操作失败，请重试'
            default:
                return error.message || '操作失败，请重试'
        }
    }

    /**
     * 创建错误
     */
    static createError(
        type: ErrorType,
        message: string,
        details?: any
    ): ErrorInfo {
        return {
            type,
            level: ErrorLevel.ERROR,
            message,
            details,
            timestamp: Date.now()
        }
    }

    /**
     * 获取错误日志
     */
    static getLogs(): ErrorInfo[] {
        return ErrorLogger.getLogs()
    }

    /**
     * 清除错误日志
     */
    static clearLogs(): void {
        ErrorLogger.clear()
    }
}

/**
 * 异步错误处理装饰器
 */
export function handleAsyncError(
    target: any,
    propertyKey: string,
    descriptor: PropertyDescriptor
) {
    const originalMethod = descriptor.value

    descriptor.value = async function (...args: any[]) {
        try {
            return await originalMethod.apply(this, args)
        } catch (error) {
            const pageRoute = (this as { route?: string }).route
            EnhancedErrorHandler.handle(error, {
                page: pageRoute || 'unknown',
                action: propertyKey
            })
            throw error
        }
    }

    return descriptor
}

/**
 * 重试装饰器
 */
export function retry(maxRetries: number = 3, delay: number = 1000) {
    return function (
        target: any,
        propertyKey: string,
        descriptor: PropertyDescriptor
    ) {
        const originalMethod = descriptor.value

        descriptor.value = async function (...args: any[]) {
            let lastError: any

            for (let i = 0; i < maxRetries; i++) {
                try {
                    return await originalMethod.apply(this, args)
                } catch (error) {
                    lastError = error
                    console.log(`Retry ${i + 1}/${maxRetries} for ${propertyKey}`)

                    if (i < maxRetries - 1) {
                        await new Promise(resolve => setTimeout(resolve, delay))
                    }
                }
            }

            const pageRoute = (this as { route?: string }).route
            EnhancedErrorHandler.handle(lastError, {
                page: pageRoute || 'unknown',
                action: propertyKey
            })
            throw lastError
        }

        return descriptor
    }
}

/**
 * 全局错误处理器
 */
export function setupGlobalErrorHandler(): void {
    // 捕获未处理的Promise错误
    if (typeof wx !== 'undefined') {
        wx.onError((error) => {
            EnhancedErrorHandler.handle(error, {
                page: 'global',
                action: 'unhandledError'
            })
        })
    }
}
