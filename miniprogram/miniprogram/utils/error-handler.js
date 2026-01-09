"use strict";
/**
 * 统一错误处理器
 * 提供一致的错误处理策略和用户提示
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
exports.ErrorHandler = exports.AppError = exports.ErrorType = void 0;
const logger_1 = require("./logger");
var ErrorType;
(function (ErrorType) {
    ErrorType["NETWORK"] = "NETWORK";
    ErrorType["AUTH"] = "AUTH";
    ErrorType["PERMISSION"] = "PERMISSION";
    ErrorType["VALIDATION"] = "VALIDATION";
    ErrorType["BUSINESS"] = "BUSINESS";
    ErrorType["UNKNOWN"] = "UNKNOWN"; // 未知错误
})(ErrorType || (exports.ErrorType = ErrorType = {}));
class AppError extends Error {
    constructor(config, originalError) {
        super(config.message);
        this.name = 'AppError';
        this.type = config.type;
        this.userMessage = config.userMessage || config.message;
        this.originalError = originalError;
    }
}
exports.AppError = AppError;
class ErrorHandler {
    /**
       * 统一错误处理入口
       */
    static handle(error, context) {
        let appError;
        // 转换为AppError
        if (error instanceof AppError) {
            appError = error;
        }
        else if (error instanceof Error) {
            appError = new AppError({
                type: ErrorType.UNKNOWN,
                message: error.message,
                userMessage: '操作失败,请稍后重试'
            }, error);
        }
        else if (typeof error === 'string') {
            appError = new AppError({
                type: ErrorType.UNKNOWN,
                message: error,
                userMessage: error
            });
        }
        else {
            appError = new AppError({
                type: ErrorType.UNKNOWN,
                message: '未知错误',
                userMessage: '操作失败,请稍后重试'
            }, error);
        }
        // 记录日志(后端服务不可用时使用简洁日志)
        if (ErrorHandler.isBackendUnavailable(appError)) {
            logger_1.logger.warn(`[后端服务不可用] ${appError.message}`, undefined, context);
        }
        else {
            logger_1.logger.error(appError.message, appError.originalError, context);
        }
        // 显示用户提示
        this.showUserMessage(appError);
        return appError;
    }
    /**
       * 处理网络错误
       */
    static handleNetworkError(error, context) {
        const detail = this.resolveErrorText(error) || '未知错误';
        const appError = new AppError({
            type: ErrorType.NETWORK,
            message: `网络请求失败: ${detail}`,
            userMessage: '网络连接失败,请检查网络设置'
        }, error);
        // 后端不可用时使用简洁日志
        if (this.isBackendUnavailable(appError)) {
            logger_1.logger.warn(`[后端服务不可用] ${appError.message}`, undefined, context);
        }
        else {
            logger_1.logger.error(appError.message, error, context);
        }
        this.showUserMessage(appError);
        return appError;
    }
    /**
       * 处理认证错误
       */
    static handleAuthError(error, context) {
        const detail = this.resolveErrorText(error) || '未知错误';
        const appError = new AppError({
            type: ErrorType.AUTH,
            message: `认证失败: ${detail}`,
            userMessage: '登录已过期,请重新登录'
        }, error);
        logger_1.logger.error(appError.message, error, context);
        // 认证错误跳转到登录页
        wx.reLaunch({
            url: '/pages/user_center/index'
        });
        return appError;
    }
    /**
       * 处理权限错误
       */
    static handlePermissionError(permission, context) {
        const permissionMessages = {
            'scope.userLocation': '需要获取您的位置信息',
            'scope.userInfo': '需要获取您的用户信息',
            'scope.writePhotosAlbum': '需要保存图片到相册',
            'scope.camera': '需要使用相机'
        };
        const appError = new AppError({
            type: ErrorType.PERMISSION,
            message: `权限被拒绝: ${permission}`,
            userMessage: permissionMessages[permission] || '需要相关权限才能继续'
        });
        logger_1.logger.warn(appError.message, { permission }, context);
        // 显示权限引导
        wx.showModal({
            title: '需要授权',
            content: `${appError.userMessage},请在设置中开启`,
            confirmText: '去设置',
            success: (res) => {
                if (res.confirm) {
                    wx.openSetting();
                }
            }
        });
        return appError;
    }
    /**
       * 判断是否为后端服务不可用错误
       */
    static isBackendUnavailable(error) {
        if (!error)
            return false;
        let msg = '';
        if (error instanceof AppError) {
            msg = error.message;
        }
        else if (error instanceof Error) {
            msg = error.message;
        }
        else if (typeof error === 'string') {
            msg = error;
        }
        else if (typeof error === 'object' && 'message' in error) {
            msg = String(error.message);
        }
        const lowerMsg = msg.toLowerCase();
        return (lowerMsg.includes('502') ||
            lowerMsg.includes('503') ||
            lowerMsg.includes('504') ||
            lowerMsg.includes('后端服务') ||
            lowerMsg.includes('nginx') ||
            lowerMsg.includes('gateway'));
    }
    /**
       * 处理验证错误
       */
    static handleValidationError(field, message, context) {
        const appError = new AppError({
            type: ErrorType.VALIDATION,
            message: `验证失败: ${field} - ${message}`,
            userMessage: message
        });
        logger_1.logger.warn(appError.message, { field }, context);
        this.showUserMessage(appError);
        return appError;
    }
    /**
       * 处理业务错误
       */
    static handleBusinessError(message, context) {
        const appError = new AppError({
            type: ErrorType.BUSINESS,
            message: `业务错误: ${message}`,
            userMessage: message
        });
        logger_1.logger.warn(appError.message, undefined, context);
        this.showUserMessage(appError);
        return appError;
    }
    /**
       * 显示用户提示
       */
    static showUserMessage(error) {
        // 认证错误不显示Toast,因为会跳转
        if (error.type === ErrorType.AUTH) {
            return;
        }
        // 权限错误已经显示Modal
        if (error.type === ErrorType.PERMISSION) {
            return;
        }
        // 后端服务不可用时不弹Toast
        if (this.isBackendUnavailable(error)) {
            return;
        }
        // 开发环境显示详细错误信息
        const isDev = typeof __wxConfig !== 'undefined' && __wxConfig.envVersion === 'develop';
        if (isDev && error.type === ErrorType.NETWORK) {
            // 开发环境显示详细的网络错误
            wx.showModal({
                title: '开发环境 - 网络错误',
                content: `${error.userMessage}\n\n技术详情: ${error.message}`,
                showCancel: false,
                confirmText: '知道了'
            });
        }
        else {
            // 生产环境或其他错误使用Toast
            wx.showToast({
                title: error.userMessage,
                icon: 'none',
                duration: 2500
            });
        }
    }
    /**
       * 安全执行异步函数
       */
    static safeExecute(fn, context, fallback) {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                return yield fn();
            }
            catch (error) {
                this.handle(error, context);
                return fallback;
            }
        });
    }
    /**
       * 创建带重试的执行器
       */
    static executeWithRetry(fn_1) {
        return __awaiter(this, arguments, void 0, function* (fn, options = {}) {
            const { maxRetries = 3, retryDelay = 1000, context } = options;
            let lastError;
            for (let i = 0; i < maxRetries; i++) {
                try {
                    return yield fn();
                }
                catch (error) {
                    lastError = error;
                    logger_1.logger.warn(`执行失败,准备重试 (${i + 1}/${maxRetries})`, error, context);
                    if (i < maxRetries - 1) {
                        yield new Promise((resolve) => setTimeout(resolve, retryDelay));
                    }
                }
            }
            throw this.handle(lastError, context);
        });
    }
    static resolveErrorText(error) {
        if (!error) {
            return '';
        }
        if (typeof error === 'string') {
            return error;
        }
        if (error instanceof Error) {
            return error.message;
        }
        if (typeof error === 'object' && error !== null) {
            const withErrMsg = error;
            if (typeof withErrMsg.errMsg === 'string') {
                return withErrMsg.errMsg;
            }
            const withMessage = error;
            if (typeof withMessage.message === 'string') {
                return withMessage.message;
            }
        }
        return '';
    }
}
exports.ErrorHandler = ErrorHandler;
