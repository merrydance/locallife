"use strict";
/**
 * 菜品和套餐管理接口
 * 基于swagger.json完全重构，仅保留后端支持的接口
 */
Object.defineProperty(exports, "__esModule", { value: true });
exports.InventoryManagementService = exports.ComboManagementService = exports.DishManagementService = exports.TagService = void 0;
exports.searchDishes = searchDishes;
exports.getRecommendedCombos = getRecommendedCombos;
exports.getTags = getTags;
exports.searchCombos = searchCombos;
const request_1 = require("../utils/request");
const auth_1 = require("../utils/auth");
// ==================== 标签管理服务 ====================
/**
 * 标签服务
 * 提供标签查询功能
 */
class TagService {
    /**
     * 获取指定类型的标签列表
     * GET /v1/tags?type=xxx
     * @param type 标签类型: dish, merchant, combo, table, customization
     */
    static async listTags(type) {
        const response = await (0, request_1.request)({
            url: '/v1/tags',
            method: 'GET',
            data: { type }
        });
        return response.tags || [];
    }
    /**
     * 获取菜品属性标签列表
     * 便捷方法，等同于 listTags('dish')
     */
    static async listDishTags() {
        return this.listTags('dish');
    }
    /**
     * 获取定制选项标签列表
     * 便捷方法，等同于 listTags('customization')
     */
    static async listCustomizationTags() {
        return this.listTags('customization');
    }
    /**
     * 创建标签
     * POST /v1/tags
     */
    static async createTag(data) {
        return await (0, request_1.request)({
            url: '/v1/tags',
            method: 'POST',
            data
        });
    }
    /**
     * 删除标签
     * DELETE /v1/tags/:id
     */
    static async deleteTag(id) {
        await (0, request_1.request)({
            url: `/v1/tags/${id}`,
            method: 'DELETE'
        });
    }
}
exports.TagService = TagService;
// ==================== 菜品管理服务 ====================
/**
 * 菜品管理服务
 * 基于swagger.json完全重构，仅包含后端支持的接口
 */
class DishManagementService {
    /**
     * 获取商户菜品列表
     * GET /v1/dishes
     */
    static async listDishes(params) {
        return await (0, request_1.request)({
            url: '/v1/dishes',
            method: 'GET',
            data: params
        });
    }
    /**
     * 创建菜品
     * POST /v1/dishes
     */
    static async createDish(data) {
        return await (0, request_1.request)({
            url: '/v1/dishes',
            method: 'POST',
            data
        });
    }
    /**
     * 获取菜品详情（消费者端）
     * GET /v1/public/dishes/{id}
     * 注意：使用公开接口，无需商户权限
     */
    static async getDishDetail(dishId) {
        return await (0, request_1.request)({
            url: `/v1/public/dishes/${dishId}`,
            method: 'GET'
        });
    }
    /**
     * 更新菜品信息
     * PUT /v1/dishes/{id}
     */
    static async updateDish(dishId, data) {
        return await (0, request_1.request)({
            url: `/v1/dishes/${dishId}`,
            method: 'PUT',
            data
        });
    }
    /**
     * 删除菜品
     * DELETE /v1/dishes/{id}
     */
    static async deleteDish(dishId) {
        return await (0, request_1.request)({
            url: `/v1/dishes/${dishId}`,
            method: 'DELETE'
        });
    }
    /**
     * 更新菜品状态
     * PATCH /v1/dishes/{id}/status (使用PUT方法)
     */
    static async updateDishStatus(dishId, data) {
        return await (0, request_1.request)({
            url: `/v1/dishes/${dishId}/status`,
            method: 'PUT',
            data
        });
    }
    /**
     * 批量更新菜品状态
     * PATCH /v1/dishes/batch/status
     */
    static async batchUpdateDishStatus(data) {
        return await (0, request_1.request)({
            url: '/v1/dishes/batch/status',
            method: 'PATCH',
            data
        });
    }
    /**
     * 获取菜品定制化选项
     * GET /v1/dishes/{id}/customizations
     */
    static async getDishCustomizations(dishId) {
        return await (0, request_1.request)({
            url: `/v1/dishes/${dishId}/customizations`,
            method: 'GET'
        });
    }
    /**
     * 设置菜品定制化选项
     * PUT /v1/dishes/{id}/customizations
     */
    static async setDishCustomizations(dishId, groups) {
        return await (0, request_1.request)({
            url: `/v1/dishes/${dishId}/customizations`,
            method: 'PUT',
            data: groups
        });
    }
    /**
     * 获取菜品分类列表
     * GET /v1/dishes/categories
     */
    static async getDishCategories() {
        const response = await (0, request_1.request)({
            url: '/v1/dishes/categories',
            method: 'GET'
        });
        // 后端返回 { categories: [...] }，需要提取数组
        return response.categories || [];
    }
    /**
     * 创建菜品分类
     * POST /v1/dishes/categories
     */
    static async createDishCategory(data) {
        return await (0, request_1.request)({
            url: '/v1/dishes/categories',
            method: 'POST',
            data
        });
    }
    /**
     * 更新菜品分类
     * PUT /v1/dishes/categories/{id}
     */
    static async updateDishCategory(id, data) {
        return await (0, request_1.request)({
            url: `/v1/dishes/categories/${id}`,
            method: 'PUT',
            data
        });
    }
    /**
     * 删除菜品分类
     * DELETE /v1/dishes/categories/{id}
     */
    static async deleteDishCategory(id) {
        await (0, request_1.request)({
            url: `/v1/dishes/categories/${id}`,
            method: 'DELETE'
        });
    }
    /**
     * 上传菜品图片
     * POST /v1/dishes/images/upload
     */
    static async uploadDishImage(filePath) {
        return new Promise((resolve, reject) => {
            const token = (0, auth_1.getToken)();
            wx.uploadFile({
                url: `${request_1.API_BASE}/v1/dishes/images/upload`,
                filePath,
                name: 'image',
                header: {
                    'Authorization': `Bearer ${token}`
                },
                success: (res) => {
                    var _a;
                    if (res.statusCode === 200) {
                        try {
                            const data = JSON.parse(res.data);
                            if (data.code === 0 && data.data && data.data.image_url) {
                                resolve(data.data.image_url);
                            }
                            else if (data.image_url) {
                                resolve(data.image_url);
                            }
                            else {
                                resolve(((_a = data.data) === null || _a === void 0 ? void 0 : _a.image_url) || data.image_url);
                            }
                        }
                        catch (e) {
                            reject(new Error('Parse response failed'));
                        }
                    }
                    else {
                        reject(new Error(`HTTP ${res.statusCode}`));
                    }
                },
                fail: reject
            });
        });
    }
}
exports.DishManagementService = DishManagementService;
// ==================== 套餐管理服务 ====================
/**
 * 套餐管理服务
 * 基于swagger.json完全重构，仅包含后端支持的接口
 */
class ComboManagementService {
    /**
     * 获取商户套餐列表
     * GET /v1/combos
     */
    static async listCombos(params) {
        return await (0, request_1.request)({
            url: '/v1/combos',
            method: 'GET',
            data: params
        });
    }
    /**
     * 获取套餐详情
     * GET /v1/combos/{id}
     */
    static async getComboDetail(comboId) {
        return await (0, request_1.request)({
            url: `/v1/combos/${comboId}`,
            method: 'GET'
        });
    }
    /**
     * 更新套餐信息
     * PUT /v1/combos/{id}
     */
    static async updateCombo(comboId, data) {
        return await (0, request_1.request)({
            url: `/v1/combos/${comboId}`,
            method: 'PUT',
            data
        });
    }
    /**
     * 创建套餐
     * POST /v1/combos
     */
    static async createCombo(data) {
        return await (0, request_1.request)({
            url: '/v1/combos',
            method: 'POST',
            data
        });
    }
    /**
     * 删除套餐
     * DELETE /v1/combos/{id}
     */
    static async deleteCombo(comboId) {
        return await (0, request_1.request)({
            url: `/v1/combos/${comboId}`,
            method: 'DELETE'
        });
    }
    /**
     * 添加菜品到套餐
     * POST /v1/combos/{id}/dishes
     */
    static async addDishToCombo(comboId, data) {
        return await (0, request_1.request)({
            url: `/v1/combos/${comboId}/dishes`,
            method: 'POST',
            data
        });
    }
    /**
     * 从套餐中移除菜品
     * DELETE /v1/combos/{id}/dishes/{dish_id}
     */
    static async removeDishFromCombo(comboId, dishId) {
        return await (0, request_1.request)({
            url: `/v1/combos/${comboId}/dishes/${dishId}`,
            method: 'DELETE'
        });
    }
    /**
     * 更新套餐上架状态
     * PUT /v1/combos/{id}/online
     */
    static async updateComboOnlineStatus(comboId, data) {
        return await (0, request_1.request)({
            url: `/v1/combos/${comboId}/online`,
            method: 'PUT',
            data
        });
    }
}
exports.ComboManagementService = ComboManagementService;
// ==================== 库存管理服务 ====================
/**
 * 库存管理服务
 * 基于swagger.json完全重构，仅包含后端支持的接口
 */
class InventoryManagementService {
    /**
     * 查询每日库存
     * GET /v1/inventory
     */
    static async getDailyInventory(date) {
        return await (0, request_1.request)({
            url: '/v1/inventory',
            method: 'GET',
            data: { date }
        });
    }
    /**
     * 更新库存
     * PUT /v1/inventory
     */
    static async updateInventory(data) {
        return await (0, request_1.request)({
            url: '/v1/inventory',
            method: 'PUT',
            data
        });
    }
    /**
     * 检查库存
     * POST /v1/inventory/check
     */
    static async checkInventory(data) {
        return await (0, request_1.request)({
            url: '/v1/inventory/check',
            method: 'POST',
            data
        });
    }
    /**
     * 获取库存统计
     * GET /v1/inventory/stats
     */
    static async getInventoryStats(params) {
        return await (0, request_1.request)({
            url: '/v1/inventory/stats',
            method: 'GET',
            data: params
        });
    }
}
exports.InventoryManagementService = InventoryManagementService;
/**
 * 搜索菜品 (原 getRecommendedDishes) - 基于 /v1/search/dishes
 * 支持分页，返回包含 has_more 的完整响应
 */
async function searchDishes(params) {
    var _a, _b, _c;
    // 首页推荐重构：使用搜索接口替代推荐接口
    // 如果没有关键词，表示获取推荐流
    const searchParams = {
        keyword: (params === null || params === void 0 ? void 0 : params.keyword) || '', // 空字符串表示推荐流
        page_id: (params === null || params === void 0 ? void 0 : params.page) || 1,
        page_size: (params === null || params === void 0 ? void 0 : params.limit) || 20,
    };
    // 仅当参数存在时才添加，避免传递 undefined 导致后端验证失败
    if (params === null || params === void 0 ? void 0 : params.merchant_id)
        searchParams.merchant_id = params.merchant_id;
    if (params === null || params === void 0 ? void 0 : params.tag_id)
        searchParams.tag_id = params.tag_id; // Added
    if (params === null || params === void 0 ? void 0 : params.user_latitude)
        searchParams.user_latitude = params.user_latitude;
    if (params === null || params === void 0 ? void 0 : params.user_longitude)
        searchParams.user_longitude = params.user_longitude;
    const response = await (0, request_1.request)({
        url: '/v1/search/dishes',
        method: 'GET',
        data: searchParams,
        useCache: searchParams.page_id === 1 && !searchParams.keyword, // 只缓存首页默认流
        cacheTTL: 1 * 60 * 1000 // 1分钟缓存 (数据即时性要求高)
    });
    // 转换响应格式以匹配 DishSearchResult
    return {
        dishes: (response.dishes || []).map(item => {
            var _a;
            return ({
                ...item,
                // 使用后端返回的商户信息，部分字段暂时缺省
                merchant_name: item.merchant_name || '未知商户',
                merchant_logo: item.merchant_logo || '',
                merchant_latitude: 0,
                merchant_longitude: 0,
                merchant_region_id: 0,
                merchant_is_open: (_a = item.merchant_is_open) !== null && _a !== void 0 ? _a : true,
                distance: item.distance || 0,
                estimated_delivery_fee: item.estimated_delivery_fee || 0,
                estimated_delivery_time: item.estimated_delivery_time || 0,
                tags: []
            });
        }),
        has_more: (_a = response.has_more) !== null && _a !== void 0 ? _a : false,
        page: (_b = response.page_id) !== null && _b !== void 0 ? _b : 1,
        total_count: (_c = response.total) !== null && _c !== void 0 ? _c : 0
    };
}
/**
 * 获取推荐套餐 - 基于 /v1/search/combos
 * 支持分页，返回包含 has_more 的完整响应
 */
async function getRecommendedCombos(params) {
    var _a, _b, _c, _d, _e, _f, _g;
    const page = (_a = params === null || params === void 0 ? void 0 : params.page) !== null && _a !== void 0 ? _a : 1;
    const pageSize = (_b = params === null || params === void 0 ? void 0 : params.limit) !== null && _b !== void 0 ? _b : 20;
    const response = await (0, request_1.request)({
        url: '/v1/search/combos',
        method: 'GET',
        data: {
            keyword: (_c = params === null || params === void 0 ? void 0 : params.keyword) !== null && _c !== void 0 ? _c : '',
            region_id: params === null || params === void 0 ? void 0 : params.region_id,
            user_latitude: params === null || params === void 0 ? void 0 : params.user_latitude,
            user_longitude: params === null || params === void 0 ? void 0 : params.user_longitude,
            page_id: page,
            page_size: pageSize
        },
        useCache: page === 1,
        cacheTTL: 3 * 60 * 1000 // 3分钟缓存
    });
    const total = (_g = (_e = (_d = response.total_count) !== null && _d !== void 0 ? _d : response.total) !== null && _e !== void 0 ? _e : (_f = response.combos) === null || _f === void 0 ? void 0 : _f.length) !== null && _g !== void 0 ? _g : 0;
    return {
        combos: response.combos || [],
        has_more: page * pageSize < total,
        page,
        total_count: total
    };
}
/**
 * 获取标签列表 - 基于 /v1/tags
 * @param type 标签类型: dish, combo, merchant, attribute, customization
 */
async function getTags(type) {
    const response = await (0, request_1.request)({
        url: '/v1/tags',
        method: 'GET',
        data: { type },
        useCache: true,
        cacheTTL: 10 * 60 * 1000 // 10分钟缓存
    });
    return response.tags || [];
}
/**
 * 搜索套餐 - 基于 /v1/search/combos
 */
async function searchCombos(params) {
    // 过滤掉 undefined 的参数
    const searchParams = {
        page_id: params.page_id || 1,
        page_size: params.page_size || 20
    };
    if (params.keyword)
        searchParams.keyword = params.keyword;
    if (params.region_id !== undefined)
        searchParams.region_id = params.region_id;
    if (params.user_latitude !== undefined)
        searchParams.user_latitude = params.user_latitude;
    if (params.user_longitude !== undefined)
        searchParams.user_longitude = params.user_longitude;
    const response = await (0, request_1.request)({
        url: '/v1/search/combos',
        method: 'GET',
        data: searchParams,
        useCache: true,
        cacheTTL: 2 * 60 * 1000 // 2分钟缓存
    });
    return response;
}
// ==================== 导出默认服务 ====================
exports.default = DishManagementService;
