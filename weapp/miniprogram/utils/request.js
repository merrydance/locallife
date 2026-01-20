"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.API_BASE = void 0;
exports.uploadFile = uploadFile;
exports.request = request;
const auth_1 = require("./auth");
const types_1 = require("../api/types");
const logger_1 = require("./logger");
const error_handler_1 = require("./error-handler");
const request_manager_1 = require("./request-manager");
const network_monitor_1 = require("./network-monitor");
const cache_1 = require("./cache");
const index_1 = require("../config/index");
const performance_monitor_1 = require("./performance-monitor");
exports.API_BASE = index_1.API_CONFIG.BASE_URL;
const cache = new cache_1.CacheManager();
function uploadFile(filePath, url = '/upload/image', name = 'file', formData = {}) {
    return new Promise((resolve, reject) => {
        const doUpload = () => {
            wx.uploadFile({
                url: `${exports.API_BASE}${url}`,
                filePath,
                name,
                header: {
                    'Authorization': `Bearer ${(0, auth_1.getToken)()}`
                },
                formData: formData,
                success: (res) => {
                    // wx.uploadFile returns data as string
                    let data;
                    try {
                        data = JSON.parse(res.data);
                    }
                    catch (e) {
                        // If not JSON, probably error page or simple string
                        data = res.data;
                    }
                    if (res.statusCode === 200 || res.statusCode === 201) {
                        // Verify code if it exists in envelope
                        if (data && typeof data === 'object' && data.code !== undefined) {
                            if (data.code === 0) {
                                logger_1.logger.debug('文件上传成功', { url: url }, 'uploadFile');
                                // Return the data part as T, or the whole thing?
                                // request() returns response.data. 
                                // Existing uploadFile returned data.data.url string.
                                // To be generic, let's return data.data usually.
                                resolve(data.data);
                            }
                            else {
                                reject(new error_handler_1.AppError({
                                    type: error_handler_1.ErrorType.BUSINESS,
                                    message: `上传失败: ${data.message}`,
                                    userMessage: data.message
                                }));
                            }
                        }
                        else {
                            // Legacy behavior or different format
                            resolve(data);
                        }
                    }
                    else if (res.statusCode === 401) {
                        // Token expired
                        logger_1.logger.warn('Token已过期(upload),尝试自动刷新', undefined, 'uploadFile');
                        performTokenRefresh(true).then(() => {
                            // Retry upload
                            doUpload();
                        }).catch((err) => {
                            (0, auth_1.clearToken)();
                            // User requested silent login, no redirect.
                            reject(new error_handler_1.AppError({
                                type: error_handler_1.ErrorType.AUTH,
                                message: '登录已过期且刷新失败',
                                userMessage: '登录状态失效，请重试'
                            }));
                        });
                    }
                    else {
                        // 解析后端返回的错误信息
                        const errMsg = (data && data.message) || (data && data.error) || '文件上传失败';
                        const userMsg = (data && data.userMessage) || '文件上传失败';
                        logger_1.logger.warn(`上传失败 HTTP ${res.statusCode}`, { url: url, response: data }, 'uploadFile');
                        reject(new error_handler_1.AppError({
                            type: error_handler_1.ErrorType.NETWORK,
                            message: `HTTP ${res.statusCode}: ${errMsg}`,
                            userMessage: userMsg
                        }, data));
                    }
                },
                fail: (err) => {
                    const error = error_handler_1.ErrorHandler.handleNetworkError(err, 'uploadFile');
                    reject(error);
                }
            });
        };
        // Check token expiry before starting (optional optimization)
        if ((0, auth_1.isTokenNearExpiry)(60000)) {
            refreshTokenOnce().then(doUpload).catch(() => doUpload());
        }
        else {
            doUpload();
        }
    });
}
async function request(options) {
    var _a, _b;
    const { url, method = 'GET', data, loading = true, loadingText = '加载中...', context, requestId = `${method}_${url}_${Date.now()}`, retry = false, useCache = false, cacheTTL = 5 * 60 * 1000, // 默认5分钟
    skipAuth = false // 是否跳过认证
     } = options;
    // 智能缓存策略(GET请求)
    if (useCache && method === 'GET') {
        const cacheKey = `api_${url}_${JSON.stringify(data || {})}`;
        const cached = cache.get(cacheKey);
        if (cached) {
            logger_1.logger.debug(`✅ 命中缓存: ${url}`, { cacheTTL }, 'request');
            // 记录性能监控 - 缓存命中
            performance_monitor_1.performanceMonitor.recordRequest(true);
            // 后台静默刷新缓存（如果缓存即将过期）
            const cacheAge = cache.getAge(cacheKey);
            if (cacheAge && cacheAge > cacheTTL * 0.8) {
                logger_1.logger.debug(`🔄 后台刷新缓存: ${url}`, undefined, 'request');
                // 异步刷新，不阻塞当前请求
                setTimeout(() => {
                    request({ ...options, useCache: false }).then((freshData) => {
                        cache.set(cacheKey, freshData, cacheTTL);
                    }).catch(() => {
                        // 刷新失败，保留旧缓存
                    });
                }, 100);
            }
            return cached;
        }
    }
    // 检查网络状态
    if (!network_monitor_1.networkMonitor.isOnline()) {
        const error = new error_handler_1.AppError({
            type: error_handler_1.ErrorType.NETWORK,
            message: '网络不可用',
            userMessage: '网络连接失败,请检查网络设置'
        });
        if (loading)
            wx.hideLoading();
        // 显示重试按钮
        showRetryDialog(error, () => request(options));
        throw error;
    }
    if (loading) {
        wx.showLoading({ title: loadingText, mask: true });
    }
    try {
        // 在每次请求前，若 token 在阈值内即将过期，则先尝试刷新一次（单并发）
        // 跳过认证的请求（如登录、刷新 token）不需要检查 token
        if (!skipAuth) {
            await ensureValidToken();
        }
        const app = getApp();
        const latitude = ((_a = app === null || app === void 0 ? void 0 : app.globalData) === null || _a === void 0 ? void 0 : _a.latitude) || 0;
        const longitude = ((_b = app === null || app === void 0 ? void 0 : app.globalData) === null || _b === void 0 ? void 0 : _b.longitude) || 0;
        logger_1.logger.debug(`API请求: ${method} ${url}`, { data, latitude, longitude, requestId }, 'request');
        const result = await new Promise((resolve, reject) => {
            const task = wx.request({
                url: `${exports.API_BASE}${url}`,
                method: method,
                data,
                header: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${(0, auth_1.getToken)()}`,
                    'X-User-Latitude': String(latitude),
                    'X-User-Longitude': String(longitude),
                    'X-Response-Envelope': '1' // 启用统一响应信封：{ code, message, data }
                },
                success: (res) => {
                    request_manager_1.requestManager.unregister(requestId);
                    resolve(res);
                },
                fail: (err) => {
                    request_manager_1.requestManager.unregister(requestId);
                    reject(err);
                }
            });
            // 注册请求任务以便取消
            request_manager_1.requestManager.register(requestId, task, context);
        });
        // 204 No Content 视为成功（如 DELETE 返回空）
        if (result.statusCode === 204) {
            performance_monitor_1.performanceMonitor.recordRequest(false);
            logger_1.logger.debug(`API响应成功(204): ${method} ${url}`, undefined, 'request');
            return undefined;
        }
        // 检查HTTP状态码
        if (result.statusCode !== 200 && result.statusCode !== 201) {
            // 特殊处理 401 Unauthorized
            if (result.statusCode === 401) {
                logger_1.logger.warn('Token无效(HTTP 401),尝试自动刷新', undefined, 'request');
                try {
                    await performTokenRefresh(true);
                    logger_1.logger.info('Token自动刷新成功,重试请求', undefined, 'request');
                    if (loading)
                        wx.hideLoading();
                    return request(options);
                }
                catch (refreshError) {
                    logger_1.logger.error('Token刷新失败(HTTP 401)', refreshError, 'request');
                    (0, auth_1.clearToken)();
                    if (loading)
                        wx.hideLoading();
                    // User requested silent login, so no redirect. Just throw.
                    throw new error_handler_1.AppError({
                        type: error_handler_1.ErrorType.AUTH,
                        message: '登录已过期且自动刷新失败',
                        userMessage: '登录状态失效，请重试'
                    });
                }
            }
            logger_1.logger.error(`HTTP错误: ${method} ${url}`, {
                statusCode: result.statusCode,
                data: result.data
            }, 'request');
            // 尝试从后端响应中提取错误信息
            const responseData = result.data;
            const backendMessage = (responseData === null || responseData === void 0 ? void 0 : responseData.message) || (responseData === null || responseData === void 0 ? void 0 : responseData.error) || '';
            // 常见HTTP错误处理
            let userMessage = backendMessage || '服务器响应异常,请稍后重试';
            let errorDetail = `HTTP ${result.statusCode}`;
            let errorType = error_handler_1.ErrorType.NETWORK;
            if (result.statusCode === 400) {
                // 400 Bad Request - 请求参数错误，显示后端返回的具体错误信息
                userMessage = backendMessage || '请求参数错误';
                errorDetail = `参数错误(400): ${backendMessage}`;
                errorType = error_handler_1.ErrorType.BUSINESS;
            }
            else if (result.statusCode === 409) {
                // 409 Conflict - 冲突错误（如时间段已被预订），显示后端返回的具体错误信息
                userMessage = backendMessage || '操作冲突，请稍后重试';
                errorDetail = `冲突(409): ${backendMessage}`;
                errorType = error_handler_1.ErrorType.BUSINESS;
            }
            else if (result.statusCode === 404) {
                userMessage = '服务暂时不可用,请稍后重试';
                errorDetail = '服务未找到(404) - 可能是后端服务未启动';
            }
            else if (result.statusCode === 502 || result.statusCode === 503 || result.statusCode === 504) {
                userMessage = '服务暂时不可用,请稍后重试';
                errorDetail = `网关错误(${result.statusCode}) - 后端服务可能未启动`;
            }
            else if (result.statusCode >= 500) {
                userMessage = '服务器内部错误,请稍后重试';
                errorDetail = `服务器错误(${result.statusCode})`;
            }
            else if (result.statusCode >= 400) {
                // 其他 4xx 客户端错误，优先使用后端返回的消息
                userMessage = backendMessage || '请求失败，请稍后重试';
                errorDetail = `客户端错误(${result.statusCode}): ${backendMessage}`;
                errorType = error_handler_1.ErrorType.BUSINESS;
            }
            throw new error_handler_1.AppError({
                type: errorType,
                message: errorDetail,
                userMessage
            });
        }
        const response = result.data;
        // 检查响应是否为HTML(Nginx错误页面)
        if (typeof result.data === 'string') {
            const dataStr = result.data.trim();
            const isHtml = dataStr.startsWith('<') || dataStr.includes('<!DOCTYPE') || dataStr.includes('<html');
            if (isHtml) {
                // 检测Nginx特征
                const isNginxPage = dataStr.includes('nginx') || dataStr.includes('502 Bad Gateway') ||
                    dataStr.includes('503 Service') || dataStr.includes('504 Gateway');
                const errorMsg = isNginxPage
                    ? 'Nginx错误页面 - 后端服务未响应'
                    : 'HTML响应 - 期望JSON格式';
                logger_1.logger.error(`收到HTML响应而非JSON: ${method} ${url}`, {
                    isNginxPage,
                    preview: dataStr.substring(0, 300),
                    statusCode: result.statusCode
                }, 'request');
                // 开发环境显示更多信息
                const userMsg = index_1.ENV.isDev && isNginxPage
                    ? '后端服务未启动\n请检查: \n1. 后端服务是否运行\n2. Nginx配置是否正确\n3. 端口是否被占用'
                    : '服务暂时不可用,请稍后重试';
                throw new error_handler_1.AppError({
                    type: error_handler_1.ErrorType.NETWORK,
                    message: errorMsg,
                    userMessage: userMsg
                });
            }
        }
        // 验证响应格式
        if (!response || typeof response !== 'object') {
            logger_1.logger.error(`API响应格式错误: ${method} ${url}`, {
                dataType: typeof result.data,
                data: result.data
            }, 'request');
            throw new error_handler_1.AppError({
                type: error_handler_1.ErrorType.BUSINESS,
                message: 'API响应格式错误 - 非对象类型',
                userMessage: '服务器响应异常,请稍后重试'
            });
        }
        // 检查 code 字段是否存在（统一响应信封格式要求所有接口都有 code 字段）
        // 部分旧接口仍直接返回数组/对象，此时视为成功并直接返回原始数据，避免前端崩溃
        if (response.code === undefined || response.code === null) {
            logger_1.logger.warn(`API响应缺少code字段，按兼容模式处理: ${method} ${url}`, response, 'request');
            // 记录性能监控 - 网络请求成功
            performance_monitor_1.performanceMonitor.recordRequest(false);
            // 缓存兼容：直接缓存原始数据
            if (useCache && method === 'GET') {
                const cacheKey = `api_${url}_${JSON.stringify(data || {})}`;
                cache.set(cacheKey, response, cacheTTL, cache_1.CacheStrategy.MEMORY_FIRST);
            }
            return response;
        }
        if (response.code === types_1.ErrorCode.SUCCESS) {
            logger_1.logger.debug(`API响应成功: ${method} ${url}`, response.data, 'request');
            // 记录性能监控 - 网络请求成功
            performance_monitor_1.performanceMonitor.recordRequest(false);
            // 保存缓存(GET请求)
            if (useCache && method === 'GET') {
                const cacheKey = `api_${url}_${JSON.stringify(data || {})}`;
                cache.set(cacheKey, response.data, cacheTTL, cache_1.CacheStrategy.MEMORY_FIRST);
            }
            return response.data;
        }
        else if (response.code === types_1.ErrorCode.TOKEN_EXPIRED) {
            // Token过期,自动静默刷新
            logger_1.logger.warn('Token已过期,尝试自动刷新', undefined, 'request');
            try {
                // 重新静默登录并刷新 token（通过独立请求避免循环依赖）
                await performTokenRefresh(true);
                logger_1.logger.info('Token自动刷新成功,重试请求', undefined, 'request');
                // 关闭loading后再重试
                if (loading)
                    wx.hideLoading();
                return request(options);
            }
            catch (refreshError) {
                // 刷新失败
                logger_1.logger.error('Token刷新失败', refreshError, 'request');
                (0, auth_1.clearToken)();
                if (loading)
                    wx.hideLoading();
                // Silent failure
                throw new error_handler_1.AppError({
                    type: error_handler_1.ErrorType.AUTH,
                    message: '登录已过期且刷新失败',
                    userMessage: '登录状态失效，请重试'
                });
            }
        }
        else {
            // 业务错误
            const errorCode = response.code || 'UNKNOWN';
            const errorMessage = response.message || '未知错误';
            logger_1.logger.warn(`API业务错误: ${method} ${url}`, { code: errorCode, message: errorMessage, fullResponse: response }, 'request');
            throw new error_handler_1.AppError({
                type: error_handler_1.ErrorType.BUSINESS,
                message: `API错误 [${errorCode}]: ${errorMessage}`,
                userMessage: errorMessage
            });
        }
    }
    catch (error) {
        // 网络错误或其他错误
        if (error instanceof error_handler_1.AppError) {
            // 如果启用了重试
            if (retry) {
                const retryCount = typeof retry === 'number' ? retry : 1;
                logger_1.logger.warn(`请求失败,将重试 ${retryCount} 次`, { url }, 'request');
                // 关闭loading后再重试
                if (loading)
                    wx.hideLoading();
                return retryRequest(options, retryCount);
            }
            throw error;
        }
        // 静默处理abort错误（并发请求被取消的正常情况）
        const errMsg = (error === null || error === void 0 ? void 0 : error.errMsg) || '';
        if (errMsg.includes('abort')) {
            // abort是正常的并发控制，静默处理
            if (retry) {
                const retryCount = typeof retry === 'number' ? retry : 1;
                if (loading)
                    wx.hideLoading();
                return retryRequest(options, retryCount);
            }
            throw error;
        }
        logger_1.logger.error(`API请求失败: ${method} ${url}`, error, 'request');
        const networkError = error_handler_1.ErrorHandler.handleNetworkError(error, `request:${method}:${url}`);
        // 如果启用了重试
        if (retry) {
            const retryCount = typeof retry === 'number' ? retry : 1;
            // 关闭loading后再重试
            if (loading)
                wx.hideLoading();
            return retryRequest(options, retryCount);
        }
        throw networkError;
    }
    finally {
        // 确保hideLoading在所有情况下都被调用
        if (loading) {
            try {
                wx.hideLoading();
            }
            catch (e) {
                // 忽略hideLoading的错误
            }
        }
    }
}
/**
 * 重试请求
 */
async function retryRequest(options, retryCount, currentAttempt = 0) {
    if (currentAttempt >= retryCount) {
        throw new error_handler_1.AppError({
            type: error_handler_1.ErrorType.NETWORK,
            message: `请求失败,已重试${retryCount}次`,
            userMessage: '网络请求失败,请稍后重试'
        });
    }
    // 等待一段时间再重试(指数退避)
    const delay = Math.min(1000 * Math.pow(2, currentAttempt), 5000);
    await new Promise((resolve) => setTimeout(resolve, delay));
    try {
        logger_1.logger.info(`第${currentAttempt + 1}次重试: ${options.url}`, undefined, 'retryRequest');
        return await request({ ...options, retry: false });
    }
    catch (error) {
        return retryRequest(options, retryCount, currentAttempt + 1);
    }
}
/**
 * 显示重试对话框
 */
function showRetryDialog(error, retryFn) {
    wx.showModal({
        title: '网络异常',
        content: error.userMessage || '网络连接失败',
        confirmText: '重试',
        cancelText: '取消',
        success: (res) => {
            if (res.confirm) {
                retryFn().catch(() => {
                    // 重试失败
                });
            }
        }
    });
}
// ==================== Token刷新机制 ====================
// 单次并发刷新锁
let _refreshingPromise = null;
const REFRESH_THRESHOLD = 5 * 60 * 1000; // 5分钟
const REFRESH_TIMEOUT = 10000; // 10秒
/**
 * 统一的Token刷新入口 (带锁)
 * @param force 是否强制刷新(忽略有效期检查)
 */
async function performTokenRefresh(force = false) {
    // 如果已有刷新任务在执行，直接复用
    if (_refreshingPromise) {
        logger_1.logger.debug('检测到正在刷新Token,复用Promise', undefined, 'performTokenRefresh');
        return _refreshingPromise;
    }
    // 非强制模式下，检查是否真的需要刷新
    if (!force && !(0, auth_1.isTokenNearExpiry)(REFRESH_THRESHOLD)) {
        return;
    }
    logger_1.logger.info('开始刷新Token', { force }, 'performTokenRefresh');
    _refreshingPromise = new Promise(async (resolve, reject) => {
        try {
            await refreshTokenWithTimeout();
            resolve();
        }
        catch (e) {
            reject(e);
        }
        finally {
            // 延迟清除锁，防止瞬间并发穿透
            setTimeout(() => {
                _refreshingPromise = null;
            }, 500);
        }
    });
    return _refreshingPromise;
}
/**
 * 确保Token有效性(请求前检查)
 */
async function ensureValidToken() {
    if (!(0, auth_1.hasToken)()) {
        return performTokenRefresh(true);
    }
    return performTokenRefresh(false);
}
/**
 * 带超时的Token刷新实现
 */
async function refreshTokenWithTimeout() {
    return Promise.race([
        refreshTokenOnce(),
        new Promise((_, reject) => {
            setTimeout(() => {
                reject(new error_handler_1.AppError({
                    type: error_handler_1.ErrorType.NETWORK,
                    message: 'Token刷新超时',
                    userMessage: '网络超时,请重试'
                }));
            }, REFRESH_TIMEOUT);
        })
    ]);
}
/**
 * 刷新Token核心逻辑 - 优先使用refresh_token,降级到重新登录
 */
async function refreshTokenOnce() {
    var _a, _b;
    try {
        const { getRefreshToken } = require('./auth');
        const { getDeviceId } = require('./location');
        const refreshToken = getRefreshToken();
        // 策略1: 使用refresh_token刷新
        if (refreshToken) {
            logger_1.logger.info('尝试使用refresh_token刷新', undefined, 'refreshTokenOnce');
            try {
                const res = await new Promise((resolve, reject) => {
                    wx.request({
                        url: `${exports.API_BASE}/v1/auth/refresh`,
                        method: 'POST',
                        data: { refresh_token: refreshToken },
                        header: { 'Content-Type': 'application/json', 'X-Response-Envelope': '1' },
                        success: resolve,
                        fail: reject,
                        timeout: 5000
                    });
                });
                const response = res.data;
                if (res.statusCode === 200 && response.code === types_1.ErrorCode.SUCCESS && ((_a = response.data) === null || _a === void 0 ? void 0 : _a.access_token)) {
                    const d = response.data;
                    const expiresAt = d.access_token_expires_at ? new Date(d.access_token_expires_at).getTime() : undefined;
                    (0, auth_1.setToken)(d.access_token, expiresAt, d.refresh_token);
                    logger_1.logger.info('refresh_token刷新成功', undefined, 'refreshTokenOnce');
                    return;
                }
                logger_1.logger.warn('refresh_token刷新失效，尝试重新登录', response, 'refreshTokenOnce');
            }
            catch (e) {
                logger_1.logger.warn('refresh_token请求失败，尝试重新登录', e, 'refreshTokenOnce');
            }
        }
        // 策略2: 降级到wx.login重新登录
        logger_1.logger.info('开始wx.login重新登录', undefined, 'refreshTokenOnce');
        const code = await new Promise((resolve, reject) => {
            wx.login({
                success: (res) => res.code ? resolve(res.code) : reject(new Error('获取code失败')),
                fail: reject,
                timeout: 5000
            });
        });
        const deviceId = getDeviceId();
        const res2 = await new Promise((resolve, reject) => {
            wx.request({
                url: `${exports.API_BASE}/v1/auth/wechat-login`,
                method: 'POST',
                data: { code, device_id: deviceId, device_type: 'miniprogram' },
                header: { 'Content-Type': 'application/json', 'X-Response-Envelope': '1' },
                success: resolve,
                fail: reject,
                timeout: 5000
            });
        });
        const response2 = res2.data;
        if (res2.statusCode === 200 && response2.code === types_1.ErrorCode.SUCCESS && ((_b = response2.data) === null || _b === void 0 ? void 0 : _b.access_token)) {
            const d = response2.data;
            const expiresAt = d.access_token_expires_at ? new Date(d.access_token_expires_at).getTime() : undefined;
            (0, auth_1.setToken)(d.access_token, expiresAt, d.refresh_token);
            logger_1.logger.info('wx.login重登录成功', undefined, 'refreshTokenOnce');
            return;
        }
        throw new error_handler_1.AppError({
            type: error_handler_1.ErrorType.AUTH,
            message: '自动登录失败',
            userMessage: '登录已过期，请手动重新登录'
        });
    }
    catch (err) {
        logger_1.logger.error('Token刷新流程彻底失败', err, 'refreshTokenOnce');
        (0, auth_1.clearToken)();
        throw err;
    }
}
