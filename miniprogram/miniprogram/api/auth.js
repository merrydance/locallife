"use strict";
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
const supabase_1 = require("../services/supabase");
/**
 * 微信登录 - 迁移至 Supabase Edge Function
 */
function wechatLogin(data) {
    return __awaiter(this, void 0, void 0, function* () {
        // Edge Function Invoke
        return new Promise((resolve, reject) => {
            wx.request({
                url: `${supabase_1.SUPABASE_URL}/functions/v1/wechat-login`,
                method: 'POST',
                data: data,
                header: {
                    'Content-Type': 'application/json',
                    'Authorization': `Bearer ${supabase_1.SUPABASE_KEY}`
                },
                success: (res) => {
                    var _a, _b, _c;
                    if (res.statusCode >= 200 && res.statusCode < 300) {
                        const body = res.data;
                        const { session, user } = body;
                        if (!session || !session.access_token) {
                            reject(new Error('Invalid response: missing session'));
                            return;
                        }
                        resolve({
                            access_token: session.access_token,
                            access_token_expires_at: new Date(Date.now() + session.expires_in * 1000).toISOString(),
                            refresh_token: session.refresh_token,
                            refresh_token_expires_at: new Date(Date.now() + 30 * 24 * 60 * 60 * 1000).toISOString(),
                            session_id: '0',
                            user: {
                                id: user.id,
                                full_name: ((_a = user.user_metadata) === null || _a === void 0 ? void 0 : _a.full_name) || '',
                                avatar_url: ((_b = user.user_metadata) === null || _b === void 0 ? void 0 : _b.avatar_url) || '',
                                roles: user.roles || ['CUSTOMER'],
                                wechat_openid: (_c = user.user_metadata) === null || _c === void 0 ? void 0 : _c.openid
                            }
                        });
                    }
                    else {
                        reject(new Error(`Login failed: ${JSON.stringify(res.data)}`));
                    }
                },
                fail: (err) => {
                    reject(new Error(err.errMsg || 'Network request failed'));
                }
            });
        });
    });
}
/**
 * 刷新访问令牌 - 使用 Supabase Auth API
 */
function renewAccessToken(data) {
    return new Promise((resolve, reject) => {
        wx.request({
            url: `${supabase_1.SUPABASE_URL}/auth/v1/token?grant_type=refresh_token`,
            method: 'POST',
            data: { refresh_token: data.refresh_token },
            header: {
                'Content-Type': 'application/json',
                'apikey': supabase_1.SUPABASE_KEY
            },
            success: (res) => {
                var _a;
                if (res.statusCode >= 200 && res.statusCode < 300) {
                    const session = res.data;
                    // Convert Supabase Token Response to WechatLoginResponse format (partial)
                    resolve({
                        access_token: session.access_token,
                        access_token_expires_at: new Date(Date.now() + session.expires_in * 1000).toISOString(),
                        refresh_token: session.refresh_token,
                        refresh_token_expires_at: new Date(Date.now() + 30 * 24 * 60 * 60 * 1000).toISOString(), // Dummy 30d
                        session_id: '0',
                        user: { id: (_a = session.user) === null || _a === void 0 ? void 0 : _a.id } // Minimal user info
                    });
                }
                else {
                    reject(new Error(`Token refresh failed: ${JSON.stringify(res.data)}`));
                }
            },
            fail: (err) => {
                reject(new Error(err.errMsg || 'Network request failed'));
            }
        });
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
