"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.getDeviceId = exports.login = void 0;
exports.wechatLogin = wechatLogin;
exports.renewAccessToken = renewAccessToken;
exports.getUserInfo = getUserInfo;
exports.updateUserInfo = updateUserInfo;
const request_1 = require("../utils/request");
const location_1 = require("../utils/location");
Object.defineProperty(exports, "getDeviceId", { enumerable: true, get: function () { return location_1.getDeviceId; } });
// Helper to normalize avatar URL
function normalizeUser(user) {
    if (user && user.avatar_url && !user.avatar_url.startsWith('http')) {
        let url = user.avatar_url;
        if (url.startsWith('/'))
            url = url.substring(1);
        user.avatar_url = `${request_1.API_BASE}/${url}`;
    }
    return user;
}
/**
 * 微信登录 - 使用正确的swagger路径
 * 后端已启用统一响应信封（X-Response-Envelope: 1），返回 { code, message, data } 格式
 */
function wechatLogin(data) {
    return (0, request_1.request)({
        url: '/v1/auth/wechat-login',
        method: 'POST',
        data,
        skipAuth: true // 登录接口不需要认证，跳过 token 验证和刷新
    }).then(res => {
        if (res.user) {
            normalizeUser(res.user);
        }
        return res;
    });
}
/**
 * 刷新访问令牌 - 基于 /v1/auth/renew-access
 * 后端已启用统一响应信封（X-Response-Envelope: 1），返回 { code, message, data } 格式
 */
function renewAccessToken(data) {
    return (0, request_1.request)({
        url: '/v1/auth/refresh',
        method: 'POST',
        data,
        skipAuth: true // 刷新接口不需要认证，跳过 token 验证和刷新（避免循环调用）
    });
}
/**
 * 获取用户信息 - 基于 /v1/users/me
 */
function getUserInfo() {
    return (0, request_1.request)({
        url: '/v1/users/me', // 使用swagger中定义的正确路径
        method: 'GET'
    }).then(normalizeUser);
}
/**
 * 更新用户信息 - 基于 PATCH /v1/users/me
 */
function updateUserInfo(data) {
    return (0, request_1.request)({
        url: '/v1/users/me',
        method: 'PATCH',
        data
    }).then(normalizeUser);
}
// 兼容性：保留旧接口名称，但使用新的实现
exports.login = wechatLogin;
