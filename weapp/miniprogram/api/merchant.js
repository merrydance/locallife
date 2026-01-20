"use strict";
/**
 * 商户基础管理接口
 * 基于swagger.json完全重构，仅保留后端支持的接口
 */
Object.defineProperty(exports, "__esModule", { value: true });
exports.getMerchants = exports.MerchantManagementAdapter = exports.MerchantManagementService = void 0;
exports.getMerchantDashboard = getMerchantDashboard;
exports.searchMerchants = searchMerchants;
exports.getRecommendedMerchants = getRecommendedMerchants;
exports.getPublicMerchantDetail = getPublicMerchantDetail;
exports.getPublicMerchantDishes = getPublicMerchantDishes;
exports.getPublicMerchantCombos = getPublicMerchantCombos;
exports.getMerchantOrders = getMerchantOrders;
exports.getMerchantDishes = getMerchantDishes;
exports.acceptOrder = acceptOrder;
exports.rejectOrder = rejectOrder;
exports.readyOrder = readyOrder;
exports.upsertDish = upsertDish;
const request_1 = require("../utils/request");
const auth_1 = require("../utils/auth");
const logger_1 = require("../utils/logger");
// ==================== 商户基础管理服务 ====================
/**
 * 商户基础管理服务
 * 基于swagger.json完全重构，仅包含后端支持的接口
 */
class MerchantManagementService {
    /**
     * 获取当前商户信息
     * GET /v1/merchants/me
     */
    static async getMerchantInfo() {
        return await (0, request_1.request)({
            url: '/v1/merchants/me',
            method: 'GET',
            useCache: true,
            cacheTTL: 5 * 60 * 1000 // 5分钟缓存
        });
    }
    /**
     * 获取当前用户拥有的所有商户列表
     * GET /v1/merchants/my
     * 用于多店铺切换功能
     */
    static async getMyMerchants() {
        return await (0, request_1.request)({
            url: '/v1/merchants/my',
            method: 'GET',
            useCache: true,
            cacheTTL: 5 * 60 * 1000 // 5分钟缓存
        });
    }
    /**
     * 更新商户信息
     * PATCH /v1/merchants/me
     * 使用乐观锁防止并发冲突
     */
    static async updateMerchantInfo(data) {
        return await (0, request_1.request)({
            url: '/v1/merchants/me',
            method: 'PATCH',
            data
        });
    }
    /**
     * 获取商户营业状态
     * GET /v1/merchants/me/status
     */
    static async getMerchantStatus() {
        return await (0, request_1.request)({
            url: '/v1/merchants/me/status',
            method: 'GET'
        });
    }
    /**
     * 更新商户营业状态
     * PATCH /v1/merchants/me/status
     */
    static async updateMerchantStatus(data) {
        return await (0, request_1.request)({
            url: '/v1/merchants/me/status',
            method: 'PATCH',
            data
        });
    }
    /**
     * 获取商户营业时间
     * GET /v1/merchants/me/business-hours
     */
    static async getBusinessHours() {
        return await (0, request_1.request)({
            url: '/v1/merchants/me/business-hours',
            method: 'GET'
        });
    }
    /**
     * 设置商户营业时间
     * PUT /v1/merchants/me/business-hours
     */
    static async setBusinessHours(data) {
        return await (0, request_1.request)({
            url: '/v1/merchants/me/business-hours',
            method: 'PUT',
            data
        });
    }
    /**
     * 上传商户图片
     * POST /v1/merchants/images/upload
     * 支持营业执照、身份证、Logo等图片上传
     */
    static async uploadImage(filePath, category) {
        const token = (0, auth_1.getToken)();
        return new Promise((resolve, reject) => {
            wx.uploadFile({
                url: `${request_1.API_BASE}/v1/merchants/images/upload`,
                filePath: filePath,
                name: 'image',
                formData: { category },
                header: {
                    'Authorization': `Bearer ${token}`
                },
                success: (res) => {
                    if (res.statusCode === 200) {
                        try {
                            const data = JSON.parse(res.data);
                            logger_1.logger.debug('Upload Response Raw', data, 'Merchant'); // DEBUG
                            // Helper to normalize
                            const normalize = (url) => {
                                if (url && !url.startsWith('http')) {
                                    if (url.startsWith('/'))
                                        url = url.substring(1);
                                    return `${request_1.API_BASE}/${url}`;
                                }
                                return url;
                            };
                            if (data.code === 0 && data.data) {
                                // Envelope format
                                if (data.data.image_url) {
                                    data.data.image_url = normalize(data.data.image_url);
                                }
                                resolve(data.data);
                            }
                            else if (data.image_url) {
                                // Direct format (Unwrapped)
                                data.image_url = normalize(data.image_url);
                                resolve(data);
                            }
                            else {
                                // Fallback
                                resolve(data);
                            }
                        }
                        catch (e) {
                            reject(new Error('Parse upload response failed'));
                        }
                    }
                    else {
                        logger_1.logger.error('Upload failed', res, 'Merchant');
                        reject(new Error(`HTTP ${res.statusCode}`));
                    }
                },
                fail: (err) => {
                    logger_1.logger.error('Upload network error', err, 'Merchant');
                    reject(err);
                }
            });
        });
    }
}
exports.MerchantManagementService = MerchantManagementService;
/**
 * 获取商户经营概览
 */
async function getMerchantDashboard(merchantId) {
    return (0, request_1.request)({
        url: '/v1/merchants/me/dashboard',
        method: 'GET'
    });
}
// ==================== 顾客端商户接口 ====================
/**
 * 搜索商户 - 基于 /v1/search/merchants
 * 注意：后端要求 keyword, page_id, page_size 为必填参数
 */
async function searchMerchants(params) {
    // 后端要求必填参数，提供默认值
    const requestParams = {
        keyword: params.keyword || '', // 空字符串表示搜索全部
        page_id: params.page_id || 1,
        page_size: params.page_size || 20
    };
    // 仅添加有效的经纬度
    if (params.user_latitude !== undefined && params.user_latitude !== null) {
        requestParams.user_latitude = params.user_latitude;
    }
    if (params.user_longitude !== undefined && params.user_longitude !== null) {
        requestParams.user_longitude = params.user_longitude;
    }
    if (params.region_id !== undefined && params.region_id !== null) {
        requestParams.region_id = params.region_id;
    }
    const response = await (0, request_1.request)({
        url: '/v1/search/merchants',
        method: 'GET',
        data: requestParams,
        useCache: true,
        cacheTTL: 2 * 60 * 1000 // 2分钟缓存
    });
    // 后端返回 { merchants: [...], total, page_id, page_size }，转换到 MerchantSummary
    return (response.merchants || []).map((item) => ({
        id: item.id,
        name: item.name,
        address: item.address || '',
        description: item.description || '',
        logo_url: item.logo_url || '',
        distance: item.distance,
        estimated_delivery_fee: item.estimated_delivery_fee,
        total_orders: item.total_orders,
        region_id: item.region_id,
        status: item.status,
    }));
}
/**
 * 获取推荐商户 - 基于 /v1/search/merchants
 * 支持分页，返回包含 has_more 的完整响应
 */
async function getRecommendedMerchants(params) {
    var _a, _b, _c, _d, _e, _f;
    const page = (_a = params === null || params === void 0 ? void 0 : params.page) !== null && _a !== void 0 ? _a : 1;
    const pageSize = (_b = params === null || params === void 0 ? void 0 : params.limit) !== null && _b !== void 0 ? _b : 20;
    const response = await (0, request_1.request)({
        url: '/v1/search/merchants',
        method: 'GET',
        data: {
            keyword: '',
            region_id: params === null || params === void 0 ? void 0 : params.region_id,
            user_latitude: params === null || params === void 0 ? void 0 : params.user_latitude,
            user_longitude: params === null || params === void 0 ? void 0 : params.user_longitude,
            page_id: page,
            page_size: pageSize
        },
        useCache: page === 1,
        cacheTTL: 3 * 60 * 1000 // 3分钟缓存
    });
    const total = (_f = (_d = (_c = response.total_count) !== null && _c !== void 0 ? _c : response.total) !== null && _d !== void 0 ? _d : (_e = response.merchants) === null || _e === void 0 ? void 0 : _e.length) !== null && _f !== void 0 ? _f : 0;
    return {
        merchants: response.merchants || [],
        has_more: page * pageSize < total,
        page,
        total_count: total
    };
}
/**
 * 获取商户详情（消费者端）
 * GET /v1/public/merchants/:id
 * 返回包含标签、营业时间、证照等完整信息
 */
async function getPublicMerchantDetail(merchantId) {
    return await (0, request_1.request)({
        url: `/v1/public/merchants/${merchantId}`,
        method: 'GET',
        useCache: true,
        cacheTTL: 5 * 60 * 1000 // 5分钟缓存
    });
}
/**
 * 获取商户菜品列表（消费者端）
 * GET /v1/public/merchants/:id/dishes
 */
async function getPublicMerchantDishes(merchantId) {
    return await (0, request_1.request)({
        url: `/v1/public/merchants/${merchantId}/dishes`,
        method: 'GET',
        useCache: true,
        cacheTTL: 5 * 60 * 1000
    });
}
/**
 * 获取商户套餐列表（消费者端）
 * GET /v1/public/merchants/:id/combos
 */
async function getPublicMerchantCombos(merchantId) {
    return await (0, request_1.request)({
        url: `/v1/public/merchants/${merchantId}/combos`,
        method: 'GET',
        useCache: true,
        cacheTTL: 5 * 60 * 1000
    });
}
// ==================== 商户基础管理适配器 ====================
/**
 * 商户基础管理数据适配器
 * 处理前端展示数据和后端API数据之间的转换
 */
class MerchantManagementAdapter {
    /**
     * 格式化商户状态显示文本
     */
    static formatMerchantStatus(status) {
        const statusMap = {
            'active': '正常营业',
            'inactive': '暂停营业',
            'suspended': '已暂停',
            'pending': '待审核'
        };
        return statusMap[status] || status;
    }
    /**
     * 格式化营业状态显示文本
     */
    static formatBusinessStatus(isOpen) {
        return isOpen ? '营业中' : '已打烊';
    }
    /**
     * 格式化星期显示文本
     */
    static formatDayOfWeek(dayOfWeek) {
        const dayNames = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'];
        return dayNames[dayOfWeek] || `星期${dayOfWeek}`;
    }
    /**
     * 生成默认营业时间（周一到周日 9:00-21:00）
     */
    static generateDefaultBusinessHours() {
        const defaultHours = [];
        for (let i = 0; i < 7; i++) {
            defaultHours.push({
                day_of_week: i,
                open_time: '09:00',
                close_time: '21:00',
                is_closed: false
            });
        }
        return defaultHours;
    }
    /**
     * 验证营业时间数据
     */
    static validateBusinessHours(hours) {
        const errors = [];
        if (!hours || hours.length === 0) {
            errors.push('营业时间不能为空');
            return { isValid: false, errors };
        }
        if (hours.length > 7) {
            errors.push('营业时间最多7天');
        }
        hours.forEach((hour, index) => {
            if (!hour.open_time || !hour.close_time) {
                errors.push(`第${index + 1}项营业时间缺少开始或结束时间`);
            }
            if (hour.open_time && hour.close_time) {
                const openTime = new Date(`2000-01-01 ${hour.open_time}:00`);
                const closeTime = new Date(`2000-01-01 ${hour.close_time}:00`);
                if (openTime >= closeTime) {
                    errors.push(`第${index + 1}项营业时间：开始时间不能晚于或等于结束时间`);
                }
            }
            if (hour.day_of_week !== undefined && (hour.day_of_week < 0 || hour.day_of_week > 6)) {
                errors.push(`第${index + 1}项营业时间：星期数值无效`);
            }
        });
        return {
            isValid: errors.length === 0,
            errors
        };
    }
    /**
     * 检查当前是否在营业时间内
     */
    static isCurrentlyOpen(businessHours) {
        const now = new Date();
        const currentDay = now.getDay(); // 0=周日, 1=周一, ..., 6=周六
        const currentTime = now.toTimeString().slice(0, 5); // HH:MM格式
        const todayHours = businessHours.find(hour => hour.day_of_week === currentDay);
        if (!todayHours || todayHours.is_closed) {
            return false;
        }
        return currentTime >= todayHours.open_time && currentTime <= todayHours.close_time;
    }
    /**
     * 获取下次营业时间
     */
    static getNextOpenTime(businessHours) {
        const now = new Date();
        const currentDay = now.getDay();
        const currentTime = now.toTimeString().slice(0, 5);
        // 检查今天剩余时间
        const todayHours = businessHours.find(hour => hour.day_of_week === currentDay);
        if (todayHours && !todayHours.is_closed && currentTime < todayHours.open_time) {
            return `今天 ${todayHours.open_time}`;
        }
        // 检查未来7天
        for (let i = 1; i <= 7; i++) {
            const checkDay = (currentDay + i) % 7;
            const dayHours = businessHours.find(hour => hour.day_of_week === checkDay);
            if (dayHours && !dayHours.is_closed) {
                const dayName = this.formatDayOfWeek(checkDay);
                return `${dayName} ${dayHours.open_time}`;
            }
        }
        return null;
    }
}
exports.MerchantManagementAdapter = MerchantManagementAdapter;
// ==================== 导出默认服务 ====================
exports.default = MerchantManagementService;
exports.getMerchants = searchMerchants;
/**
 * 获取商户订单列表
 */
function getMerchantOrders(merchantId, status) {
    return (0, request_1.request)({
        url: `/merchant/${merchantId}/orders`,
        method: 'GET',
        data: { status }
    });
}
/**
 * 获取商户菜品列表
 */
function getMerchantDishes(merchantId) {
    return (0, request_1.request)({
        url: `/v1/public/merchants/${merchantId}/dishes`,
        method: 'GET'
    });
}
/**
 * 接单
 */
function acceptOrder(merchantId, orderId) {
    return (0, request_1.request)({
        url: `/merchant/orders/${orderId}/accept`,
        method: 'POST'
    });
}
/**
 * 拒单
 */
function rejectOrder(orderId, reason) {
    return (0, request_1.request)({
        url: `/merchant/orders/${orderId}/reject`,
        method: 'POST',
        data: { reason }
    });
}
/**
 * 出餐
 */
function readyOrder(orderId) {
    return (0, request_1.request)({
        url: `/merchant/orders/${orderId}/ready`,
        method: 'POST'
    });
}
/**
 * 更新/新增菜品
 */
function upsertDish(merchantId, dish) {
    return (0, request_1.request)({
        url: `/merchant/${merchantId}/dishes`,
        method: 'POST',
        data: dish
    });
}
