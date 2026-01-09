"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.getToken = getToken;
exports.setToken = setToken;
exports.setTokenInfo = setTokenInfo;
exports.getTokenInfo = getTokenInfo;
exports.getRefreshToken = getRefreshToken;
exports.clearToken = clearToken;
exports.hasToken = hasToken;
exports.isTokenNearExpiry = isTokenNearExpiry;
const logger_1 = require("./logger");
const TOKEN_KEY = 'access_token';
const TOKEN_INFO_KEY = 'access_token_info';
function getToken() {
    try {
        // Backward compatible: if plain token stored under TOKEN_KEY
        const raw = wx.getStorageSync(TOKEN_INFO_KEY);
        if (raw && raw.token)
            return raw.token;
        return wx.getStorageSync(TOKEN_KEY) || '';
    }
    catch (error) {
        logger_1.logger.error('获取Token失败', error, 'getToken');
        return '';
    }
}
function setToken(token, expiresAt, refreshToken) {
    try {
        // Write both legacy key and info object for compatibility
        try {
            wx.setStorageSync(TOKEN_KEY, token);
        }
        catch (e) { /* ignore */ }
        const info = { token };
        if (typeof expiresAt === 'number')
            info.expires_at = expiresAt;
        if (refreshToken)
            info.refresh_token = refreshToken;
        wx.setStorageSync(TOKEN_INFO_KEY, info);
        logger_1.logger.debug('Token已设置', undefined, 'setToken');
    }
    catch (error) {
        logger_1.logger.error('设置Token失败', error, 'setToken');
    }
}
function setTokenInfo(info) {
    try {
        if (!info || !info.token)
            return;
        setToken(info.token, info.expires_at, info.refresh_token);
    }
    catch (error) {
        logger_1.logger.error('设置TokenInfo失败', error, 'setTokenInfo');
    }
}
function getTokenInfo() {
    try {
        const info = wx.getStorageSync(TOKEN_INFO_KEY);
        if (info && info.token)
            return info;
        // Fallback to legacy token key
        const legacy = wx.getStorageSync(TOKEN_KEY);
        if (legacy)
            return { token: legacy };
        return null;
    }
    catch (error) {
        logger_1.logger.error('获取TokenInfo失败', error, 'getTokenInfo');
        return null;
    }
}
function getRefreshToken() {
    try {
        const info = getTokenInfo();
        return (info === null || info === void 0 ? void 0 : info.refresh_token) || '';
    }
    catch (error) {
        logger_1.logger.error('获取RefreshToken失败', error, 'getRefreshToken');
        return '';
    }
}
function clearToken() {
    try {
        try {
            wx.removeStorageSync(TOKEN_KEY);
        }
        catch (e) { /* ignore */ }
        wx.removeStorageSync(TOKEN_INFO_KEY);
        logger_1.logger.debug('Token已清除', undefined, 'clearToken');
    }
    catch (error) {
        logger_1.logger.error('清除Token失败', error, 'clearToken');
    }
}
function hasToken() {
    const t = getToken();
    return !!t;
}
/**
 * 判断 token 是否在给定阈值内过期
 * @param thresholdMs - 如果 token 在 thresholdMs 毫秒内到期，则返回 true
 */
function isTokenNearExpiry(thresholdMs = 0) {
    try {
        const info = getTokenInfo();
        if (!info || !info.expires_at)
            return false;
        return Date.now() + thresholdMs >= info.expires_at;
    }
    catch (error) {
        logger_1.logger.error('检查 token 过期失败', error, 'isTokenNearExpiry');
        return false;
    }
}
