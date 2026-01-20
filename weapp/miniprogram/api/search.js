"use strict";
/**
 * 搜索相关API接口
 * 基于swagger.json中的搜索和推荐接口
 */
Object.defineProperty(exports, "__esModule", { value: true });
exports.getRecommendations = exports.getMerchants = void 0;
exports.searchMerchants = searchMerchants;
exports.getRecommendedMerchants = getRecommendedMerchants;
exports.getRecommendedRooms = getRecommendedRooms;
exports.searchRooms = searchRooms;
exports.getSearchSuggestions = getSearchSuggestions;
exports.getPopularKeywords = getPopularKeywords;
exports.getSearchHistory = getSearchHistory;
exports.clearSearchHistory = clearSearchHistory;
exports.deleteSearchHistory = deleteSearchHistory;
exports.unifiedSearch = unifiedSearch;
const request_1 = require("../utils/request");
const logger_1 = require("../utils/logger");
// ==================== API接口函数 ====================
/**
 * Robust parameter cleaner
 * Uses JSON serialization to strip undefined values reliably
 */
function cleanParams(params) {
    try {
        // Strip undefined
        const cleaned = JSON.parse(JSON.stringify(params));
        // Also strip explicit nulls if needed, or keeping them is fine. 
        // JSON keeps nulls. If backend dislikes null, we should remove them.
        // Let's remove nulls too for max safety against "null" string.
        if (cleaned && typeof cleaned === 'object') {
            Object.keys(cleaned).forEach(key => {
                if (cleaned[key] === null) {
                    delete cleaned[key];
                }
            });
        }
        return cleaned;
    }
    catch (e) {
        logger_1.logger.error('Param cleaning failed', e);
        return params;
    }
}
/**
 * 搜索商户
 * @param params 搜索参数
 */
async function searchMerchants(params) {
    const data = cleanParams(params);
    if (!data.keyword)
        data.keyword = '';
    // Response is { merchants: [], total: ... }
    const res = await (0, request_1.request)({
        url: '/v1/search/merchants',
        method: 'GET',
        data
    });
    return res.merchants || res; // Fallback if API changes
}
/**
 * 获取推荐商户
 * @param params 推荐参数
 */
async function getRecommendedMerchants(params = {}) {
    const pageSize = params.limit || 20;
    const res = await searchMerchants({
        keyword: '',
        region_id: params.region_id,
        user_latitude: params.user_latitude,
        user_longitude: params.user_longitude,
        page_id: 1,
        page_size: pageSize
    });
    return res;
}
/**
 * 获取推荐包间
 * @param params 推荐参数（已对齐后端 exploreRoomsRequest）
 */
async function getRecommendedRooms(params) {
    logger_1.logger.debug('Fetching Recommended Rooms', params, 'API');
    const now = new Date();
    const yyyy = now.getFullYear();
    const mm = String(now.getMonth() + 1).padStart(2, '0');
    const dd = String(now.getDate()).padStart(2, '0');
    const defaultDate = `${yyyy}-${mm}-${dd}`;
    const defaultTime = '12:00';
    const res = await searchRooms({
        reservation_date: params.reservation_date || defaultDate,
        reservation_time: params.reservation_time || defaultTime,
        region_id: params.region_id,
        min_capacity: params.min_capacity,
        max_capacity: params.max_capacity,
        max_minimum_spend: params.max_minimum_spend,
        user_latitude: params.user_latitude,
        user_longitude: params.user_longitude,
        page_id: params.page_id,
        page_size: params.page_size
    });
    return res;
}
/**
 * 搜索包间
 * @param params 搜索参数
 */
async function searchRooms(params) {
    const res = await (0, request_1.request)({
        url: '/v1/search/rooms',
        method: 'GET',
        data: cleanParams(params)
    });
    return res.rooms || res;
}
/**
 * 获取搜索建议
 * @param keyword 关键词前缀
 * @param type 搜索类型
 */
async function getSearchSuggestions(keyword, type) {
    return (0, request_1.request)({
        url: '/v1/search/suggestions',
        method: 'GET',
        data: { keyword, type }
    });
}
/**
 * 获取热门搜索关键词
 * @param type 搜索类型
 */
async function getPopularKeywords(type) {
    return (0, request_1.request)({
        url: '/v1/search/popular',
        method: 'GET',
        data: { type }
    });
}
/**
 * 获取搜索历史
 * @param limit 返回数量限制
 */
async function getSearchHistory(limit = 10) {
    return (0, request_1.request)({
        url: '/v1/search/history',
        method: 'GET',
        data: { limit }
    });
}
/**
 * 清除搜索历史
 */
async function clearSearchHistory() {
    return (0, request_1.request)({
        url: '/v1/search/history',
        method: 'DELETE'
    });
}
/**
 * 删除单条搜索历史
 * @param historyId 历史记录ID
 */
async function deleteSearchHistory(historyId) {
    return (0, request_1.request)({
        url: `/v1/search/history/${historyId}`,
        method: 'DELETE'
    });
}
/**
 * 综合搜索（同时搜索菜品和商户）
 * @param keyword 搜索关键词
 * @param params 搜索参数
 */
async function unifiedSearch(keyword, params = {}) {
    const { dish_limit = 10, merchant_limit = 10, ...locationParams } = params;
    const cleanedLoc = cleanParams(locationParams);
    // 并行搜索菜品和商户
    const [dishResults, merchantResults] = await Promise.all([
        (0, request_1.request)({
            url: '/v1/search/dishes',
            method: 'GET',
            data: {
                keyword,
                page_id: 1,
                page_size: dish_limit,
                ...cleanedLoc
            }
        }),
        (0, request_1.request)({
            url: '/v1/search/merchants',
            method: 'GET',
            data: {
                keyword,
                page_id: 1,
                page_size: merchant_limit,
                ...cleanedLoc
            }
        })
    ]);
    return {
        dishes: dishResults.dishes || dishResults,
        merchants: merchantResults.merchants || merchantResults,
        total_dishes: dishResults.total || dishResults.length,
        total_merchants: merchantResults.total || merchantResults.length
    };
}
// ==================== 兼容性别名 ====================
/** @deprecated 使用 searchMerchants 替代 */
exports.getMerchants = searchMerchants;
/** @deprecated 使用 getRecommendedMerchants 替代 */
exports.getRecommendations = getRecommendedMerchants;
