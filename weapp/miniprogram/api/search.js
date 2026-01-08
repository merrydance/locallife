"use strict";
/**
 * 搜索相关API接口
 * 基于swagger.json中的搜索和推荐接口
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
var __rest = (this && this.__rest) || function (s, e) {
    var t = {};
    for (var p in s) if (Object.prototype.hasOwnProperty.call(s, p) && e.indexOf(p) < 0)
        t[p] = s[p];
    if (s != null && typeof Object.getOwnPropertySymbols === "function")
        for (var i = 0, p = Object.getOwnPropertySymbols(s); i < p.length; i++) {
            if (e.indexOf(p[i]) < 0 && Object.prototype.propertyIsEnumerable.call(s, p[i]))
                t[p[i]] = s[p[i]];
        }
    return t;
};
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
function searchMerchants(params) {
    return __awaiter(this, void 0, void 0, function* () {
        const data = cleanParams(params);
        if (!data.keyword)
            data.keyword = '';
        // Response is { merchants: [], total: ... }
        const res = yield (0, request_1.request)({
            url: '/v1/search/merchants',
            method: 'GET',
            data
        });
        return res.merchants || res; // Fallback if API changes
    });
}
/**
 * 获取推荐商户
 * @param params 推荐参数
 */
function getRecommendedMerchants() {
    return __awaiter(this, arguments, void 0, function* (params = {}) {
        const res = yield (0, request_1.request)({
            url: '/v1/recommendations/merchants',
            method: 'GET',
            data: cleanParams(params)
        });
        return res.merchants || res;
    });
}
/**
 * 获取推荐包间
 * @param params 推荐参数（已对齐后端 exploreRoomsRequest）
 */
function getRecommendedRooms(params) {
    return __awaiter(this, void 0, void 0, function* () {
        logger_1.logger.debug('Fetching Recommended Rooms', params, 'API');
        const res = yield (0, request_1.request)({
            url: '/v1/recommendations/rooms',
            method: 'GET',
            data: cleanParams(params)
        });
        return res.rooms || res;
    });
}
/**
 * 搜索包间
 * @param params 搜索参数
 */
function searchRooms(params) {
    return __awaiter(this, void 0, void 0, function* () {
        const res = yield (0, request_1.request)({
            url: '/v1/search/rooms',
            method: 'GET',
            data: cleanParams(params)
        });
        return res.rooms || res;
    });
}
/**
 * 获取搜索建议
 * @param keyword 关键词前缀
 * @param type 搜索类型
 */
function getSearchSuggestions(keyword, type) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: '/v1/search/suggestions',
            method: 'GET',
            data: { keyword, type }
        });
    });
}
/**
 * 获取热门搜索关键词
 * @param type 搜索类型
 */
function getPopularKeywords(type) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: '/v1/search/popular',
            method: 'GET',
            data: { type }
        });
    });
}
/**
 * 获取搜索历史
 * @param limit 返回数量限制
 */
function getSearchHistory() {
    return __awaiter(this, arguments, void 0, function* (limit = 10) {
        return (0, request_1.request)({
            url: '/v1/search/history',
            method: 'GET',
            data: { limit }
        });
    });
}
/**
 * 清除搜索历史
 */
function clearSearchHistory() {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: '/v1/search/history',
            method: 'DELETE'
        });
    });
}
/**
 * 删除单条搜索历史
 * @param historyId 历史记录ID
 */
function deleteSearchHistory(historyId) {
    return __awaiter(this, void 0, void 0, function* () {
        return (0, request_1.request)({
            url: `/v1/search/history/${historyId}`,
            method: 'DELETE'
        });
    });
}
/**
 * 综合搜索（同时搜索菜品和商户）
 * @param keyword 搜索关键词
 * @param params 搜索参数
 */
function unifiedSearch(keyword_1) {
    return __awaiter(this, arguments, void 0, function* (keyword, params = {}) {
        const { dish_limit = 10, merchant_limit = 10 } = params, locationParams = __rest(params, ["dish_limit", "merchant_limit"]);
        const cleanedLoc = cleanParams(locationParams);
        // 并行搜索菜品和商户
        const [dishResults, merchantResults] = yield Promise.all([
            (0, request_1.request)({
                url: '/v1/search/dishes',
                method: 'GET',
                data: Object.assign({ keyword, page_id: 1, page_size: dish_limit }, cleanedLoc)
            }),
            (0, request_1.request)({
                url: '/v1/search/merchants',
                method: 'GET',
                data: Object.assign({ keyword, page_id: 1, page_size: merchant_limit }, cleanedLoc)
            })
        ]);
        return {
            dishes: dishResults.dishes || dishResults,
            merchants: merchantResults.merchants || merchantResults,
            total_dishes: dishResults.total || dishResults.length,
            total_merchants: merchantResults.total || merchantResults.length
        };
    });
}
// ==================== 兼容性别名 ====================
/** @deprecated 使用 searchMerchants 替代 */
exports.getMerchants = searchMerchants;
/** @deprecated 使用 getRecommendedMerchants 替代 */
exports.getRecommendations = getRecommendedMerchants;
