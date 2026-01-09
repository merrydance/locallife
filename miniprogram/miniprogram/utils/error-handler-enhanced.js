"use strict";
/**
 * 增强的错误处理工具
 * 提供统一的错误处理、日志记录和用户提示
 */
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
Object.defineProperty(exports, "__esModule", { value: true });
exports.EnhancedErrorHandler = exports.ErrorLevel = exports.ErrorType = void 0;
exports.handleAsyncError = handleAsyncError;
exports.retry = retry;
exports.setupGlobalErrorHandler = setupGlobalErrorHandler;
/**
 * 错误类型
 */
var ErrorType;
(function (ErrorType) {
    ErrorType["NETWORK"] = "NETWORK";
    ErrorType["API"] = "API";
    ErrorType["VALIDATION"] = "VALIDATION";
    ErrorType["PERMISSION"] = "PERMISSION";
    ErrorType["BUSINESS"] = "BUSINESS";
    ErrorType["UNKNOWN"] = "UNKNOWN";
})(ErrorType || (exports.ErrorType = ErrorType = {}));
/**
 * 错误级别
 */
var ErrorLevel;
(function (ErrorLevel) {
    ErrorLevel["INFO"] = "INFO";
    ErrorLevel["WARNING"] = "WARNING";
    ErrorLevel["ERROR"] = "ERROR";
    ErrorLevel["FATAL"] = "FATAL";
})(ErrorLevel || (exports.ErrorLevel = ErrorLevel = {}));
/**
 * 错误日志
 */
class ErrorLogger {
    /**
     * 记录错误
     */
    static log(error) {
        this.logs.push(error);
        // 限制日志数量
        if (this.logs.length > this.maxLogs) {
            this.logs.shift();
        }
        // 输出到控制台
        this.logToConsole(error);
        // 可以在这里添加上报到服务器的逻辑
        // this.reportToServer(error)
    }
    /**
     * 输出到控制台
     */
    static logToConsole(error) {
        const prefix = this.getLogPrefix(error.level);
        const message = `${prefix} [${error.type}] ${error.message}`;
        switch (error.level) {
            case ErrorLevel.INFO:
                console.log(message, error.details);
                break;
            case ErrorLevel.WARNING:
                console.warn(message, error.details);
                break;
            case ErrorLevel.ERROR:
            case ErrorLevel.FATAL:
                console.error(message, error.details);
                break;
        }
    }
    /**
     * 获取日志前缀
     */
    static getLogPrefix(level) {
        switch (level) {
            case ErrorLevel.INFO:
                return 'ℹ️';
            case ErrorLevel.WARNING:
                return '⚠️';
            case ErrorLevel.ERROR:
                return '❌';
            case ErrorLevel.FATAL:
                return '🔥';
        }
    }
    /**
     * 获取所有日志
     */
    static getLogs() {
        return [...this.logs];
    }
    /**
     * 清除日志
     */
    static clear() {
        this.logs = [];
    }
    /**
     * 上报到服务器（示例）
     */
    static reportToServer(error) {
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
ErrorLogger.logs = [];
ErrorLogger.maxLogs = 100;
/**
 * 增强的错误处理器
 */
class EnhancedErrorHandler {
    /**
     * 处理错误
     */
    static handle(error, context) {
        const errorInfo = this.parseError(error, context);
        ErrorLogger.log(errorInfo);
        this.showUserMessage(errorInfo);
    }
    /**
     * 解析错误
     */
    static parseError(error, context) {
        var _a, _b;
        const errorInfo = {
            type: ErrorType.UNKNOWN,
            level: ErrorLevel.ERROR,
            message: '未知错误',
            timestamp: Date.now(),
            page: context === null || context === void 0 ? void 0 : context.page,
            action: context === null || context === void 0 ? void 0 : context.action
        };
        // 网络错误
        if ((_a = error === null || error === void 0 ? void 0 : error.errMsg) === null || _a === void 0 ? void 0 : _a.includes('request:fail')) {
            errorInfo.type = ErrorType.NETWORK;
            errorInfo.message = '网络连接失败，请检查网络设置';
            errorInfo.details = error;
        }
        // API错误
        else if ((error === null || error === void 0 ? void 0 : error.code) || (error === null || error === void 0 ? void 0 : error.statusCode)) {
            errorInfo.type = ErrorType.API;
            errorInfo.code = error.code || error.statusCode;
            errorInfo.message = error.message || error.errMsg || 'API请求失败';
            errorInfo.details = error;
        }
        // 验证错误
        else if ((error === null || error === void 0 ? void 0 : error.name) === 'ValidationError') {
            errorInfo.type = ErrorType.VALIDATION;
            errorInfo.level = ErrorLevel.WARNING;
            errorInfo.message = error.message || '数据验证失败';
            errorInfo.details = error;
        }
        // 权限错误
        else if ((_b = error === null || error === void 0 ? void 0 : error.errMsg) === null || _b === void 0 ? void 0 : _b.includes('permission')) {
            errorInfo.type = ErrorType.PERMISSION;
            errorInfo.message = '权限不足，请授权后重试';
            errorInfo.details = error;
        }
        // 业务错误
        else if (error === null || error === void 0 ? void 0 : error.businessCode) {
            errorInfo.type = ErrorType.BUSINESS;
            errorInfo.code = error.businessCode;
            errorInfo.message = error.message || '业务处理失败';
            errorInfo.details = error;
        }
        // 其他错误
        else if (error instanceof Error) {
            errorInfo.message = error.message;
            errorInfo.details = {
                name: error.name,
                stack: error.stack
            };
        }
        else if (typeof error === 'string') {
            errorInfo.message = error;
        }
        return errorInfo;
    }
    /**
     * 显示用户提示
     */
    static showUserMessage(error) {
        // 根据错误级别决定是否显示
        if (error.level === ErrorLevel.INFO) {
            return;
        }
        // 获取用户友好的错误消息
        const userMessage = this.getUserFriendlyMessage(error);
        // 显示提示
        if (error.level === ErrorLevel.FATAL) {
            wx.showModal({
                title: '严重错误',
                content: userMessage,
                showCancel: false,
                confirmText: '我知道了'
            });
        }
        else {
            wx.showToast({
                title: userMessage,
                icon: 'none',
                duration: 3000
            });
        }
    }
    /**
     * 获取用户友好的错误消息
     */
    static getUserFriendlyMessage(error) {
        // 根据错误类型返回友好的消息
        switch (error.type) {
            case ErrorType.NETWORK:
                return '网络连接失败，请检查网络后重试';
            case ErrorType.API:
                if (error.code === 401) {
                    return '登录已过期，请重新登录';
                }
                else if (error.code === 403) {
                    return '没有权限访问此功能';
                }
                else if (error.code === 404) {
                    return '请求的资源不存在';
                }
                else if (error.code === 500) {
                    return '服务器错误，请稍后重试';
                }
                return error.message || 'API请求失败';
            case ErrorType.VALIDATION:
                return error.message || '输入的数据格式不正确';
            case ErrorType.PERMISSION:
                return '需要您的授权才能使用此功能';
            case ErrorType.BUSINESS:
                return error.message || '操作失败，请重试';
            default:
                return error.message || '操作失败，请重试';
        }
    }
    /**
     * 创建错误
     */
    static createError(type, message, details) {
        return {
            type,
            level: ErrorLevel.ERROR,
            message,
            details,
            timestamp: Date.now()
        };
    }
    /**
     * 获取错误日志
     */
    static getLogs() {
        return ErrorLogger.getLogs();
    }
    /**
     * 清除错误日志
     */
    static clearLogs() {
        ErrorLogger.clear();
    }
}
exports.EnhancedErrorHandler = EnhancedErrorHandler;
/**
 * 异步错误处理装饰器
 */
function handleAsyncError(target, propertyKey, descriptor) {
    const originalMethod = descriptor.value;
    descriptor.value = function (...args) {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                return yield originalMethod.apply(this, args);
            }
            catch (error) {
                EnhancedErrorHandler.handle(error, {
                    page: this.route || 'unknown',
                    action: propertyKey
                });
                throw error;
            }
        });
    };
    return descriptor;
}
/**
 * 重试装饰器
 */
function retry(maxRetries = 3, delay = 1000) {
    return function (target, propertyKey, descriptor) {
        const originalMethod = descriptor.value;
        descriptor.value = function (...args) {
            return __awaiter(this, void 0, void 0, function* () {
                let lastError;
                for (let i = 0; i < maxRetries; i++) {
                    try {
                        return yield originalMethod.apply(this, args);
                    }
                    catch (error) {
                        lastError = error;
                        console.log(`Retry ${i + 1}/${maxRetries} for ${propertyKey}`);
                        if (i < maxRetries - 1) {
                            yield new Promise(resolve => setTimeout(resolve, delay));
                        }
                    }
                }
                EnhancedErrorHandler.handle(lastError, {
                    page: this.route || 'unknown',
                    action: propertyKey
                });
                throw lastError;
            });
        };
        return descriptor;
    };
}
/**
 * 全局错误处理器
 */
function setupGlobalErrorHandler() {
    // 捕获未处理的Promise错误
    if (typeof wx !== 'undefined') {
        const originalOnError = wx.onError;
        wx.onError = function (error) {
            EnhancedErrorHandler.handle(error, {
                page: 'global',
                action: 'unhandledError'
            });
            if (originalOnError) {
                originalOnError(error);
            }
        };
    }
}
