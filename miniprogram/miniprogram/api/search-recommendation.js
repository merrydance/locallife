"use strict";
/**
 * 搜索和推荐接口模块
 * 基于swagger.json完全重构，提供搜索功能、推荐引擎和区域服务
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
exports.RegionUtils = exports.RecommendationUtils = exports.SearchUtils = exports.RegionAdapter = exports.RecommendationAdapter = exports.SearchAdapter = exports.getRegionChildren = exports.checkRegionService = exports.getRegionById = exports.searchRegions = exports.getAvailableRegions = exports.getRegions = exports.getRecommendedRooms = exports.getRecommendedCombos = exports.getRecommendedMerchants = exports.getRecommendedDishes = exports.searchRooms = exports.searchMerchants = exports.searchDishes = void 0;
const request_1 = require("../utils/request");
// ==================== API 接口函数 ====================
/**
 * 搜索菜品
 */
const searchDishes = (params) => __awaiter(void 0, void 0, void 0, function* () {
    return (0, request_1.request)({
        url: '/v1/search/dishes',
        method: 'GET',
        data: params
    });
});
exports.searchDishes = searchDishes;
/**
 * 搜索商户
 */
const searchMerchants = (params) => __awaiter(void 0, void 0, void 0, function* () {
    return (0, request_1.request)({
        url: '/v1/search/merchants',
        method: 'GET',
        data: params
    });
});
exports.searchMerchants = searchMerchants;
/**
 * 搜索包间
 */
const searchRooms = (params) => __awaiter(void 0, void 0, void 0, function* () {
    return (0, request_1.request)({
        url: '/v1/search/rooms',
        method: 'GET',
        data: params
    });
});
exports.searchRooms = searchRooms;
/**
 * 获取菜品推荐
 */
const getRecommendedDishes = (params) => __awaiter(void 0, void 0, void 0, function* () {
    return (0, request_1.request)({
        url: '/v1/recommendations/dishes',
        method: 'GET',
        data: params
    });
});
exports.getRecommendedDishes = getRecommendedDishes;
/**
 * 获取商户推荐
 */
const getRecommendedMerchants = (params) => __awaiter(void 0, void 0, void 0, function* () {
    return (0, request_1.request)({
        url: '/v1/recommendations/merchants',
        method: 'GET',
        data: params
    });
});
exports.getRecommendedMerchants = getRecommendedMerchants;
/**
 * 获取套餐推荐
 */
const getRecommendedCombos = (params) => __awaiter(void 0, void 0, void 0, function* () {
    return (0, request_1.request)({
        url: '/v1/recommendations/combos',
        method: 'GET',
        data: params
    });
});
exports.getRecommendedCombos = getRecommendedCombos;
/**
 * 获取包间推荐
 */
const getRecommendedRooms = (params) => __awaiter(void 0, void 0, void 0, function* () {
    return (0, request_1.request)({
        url: '/v1/recommendations/rooms',
        method: 'GET',
        data: params
    });
});
exports.getRecommendedRooms = getRecommendedRooms;
/**
 * 获取区域列表
 */
const getRegions = (params) => __awaiter(void 0, void 0, void 0, function* () {
    return (0, request_1.request)({
        url: '/v1/regions',
        method: 'GET',
        data: params
    });
});
exports.getRegions = getRegions;
/**
 * 获取可服务区域列表
 */
const getAvailableRegions = () => __awaiter(void 0, void 0, void 0, function* () {
    return (0, request_1.request)({
        url: '/v1/regions/available',
        method: 'GET'
    });
});
exports.getAvailableRegions = getAvailableRegions;
/**
 * 搜索区域
 */
const searchRegions = (params) => __awaiter(void 0, void 0, void 0, function* () {
    return (0, request_1.request)({
        url: '/v1/regions/search',
        method: 'GET',
        data: params
    });
});
exports.searchRegions = searchRegions;
/**
 * 获取区域详情
 */
const getRegionById = (id) => __awaiter(void 0, void 0, void 0, function* () {
    return (0, request_1.request)({
        url: `/v1/regions/${id}`,
        method: 'GET'
    });
});
exports.getRegionById = getRegionById;
/**
 * 检查坐标是否在服务区域内
 */
const checkRegionService = (id, params) => __awaiter(void 0, void 0, void 0, function* () {
    return (0, request_1.request)({
        url: `/v1/regions/${id}/check`,
        method: 'GET',
        data: params
    });
});
exports.checkRegionService = checkRegionService;
/**
 * 获取区域的子区域
 */
const getRegionChildren = (id) => __awaiter(void 0, void 0, void 0, function* () {
    return (0, request_1.request)({
        url: `/v1/regions/${id}/children`,
        method: 'GET'
    });
});
exports.getRegionChildren = getRegionChildren;
// ==================== 数据适配器 ====================
/**
 * 搜索结果适配器
 */
class SearchAdapter {
    /**
     * 适配菜品搜索结果
     */
    static adaptDishResults(results) {
        return results.map(dish => (Object.assign(Object.assign({}, dish), { price: Number(dish.price), original_price: dish.original_price ? Number(dish.original_price) : undefined, rating: Number(dish.rating), sales_count: Number(dish.sales_count), is_available: Boolean(dish.is_available) })));
    }
    /**
     * 适配商户搜索结果
     */
    static adaptMerchantResults(results) {
        return results.map(merchant => (Object.assign(Object.assign({}, merchant), { rating: Number(merchant.rating), review_count: Number(merchant.review_count), sales_count: Number(merchant.sales_count), distance: merchant.distance ? Number(merchant.distance) : undefined, delivery_fee: merchant.delivery_fee ? Number(merchant.delivery_fee) : undefined, min_order_amount: Number(merchant.min_order_amount), estimated_delivery_time: Number(merchant.estimated_delivery_time), is_open: Boolean(merchant.is_open) })));
    }
    /**
     * 适配包间搜索结果
     */
    static adaptRoomResults(results) {
        return results.map(room => (Object.assign(Object.assign({}, room), { capacity: {
                min_guests: Number(room.capacity.min_guests),
                max_guests: Number(room.capacity.max_guests)
            }, price_per_hour: Number(room.price_per_hour), is_available: Boolean(room.is_available) })));
    }
}
exports.SearchAdapter = SearchAdapter;
/**
 * 推荐系统适配器
 */
class RecommendationAdapter {
    /**
     * 构建推荐参数
     */
    static buildRecommendationParams(userId, merchantId, categoryId, limit = 10, excludeIds) {
        return {
            user_id: userId,
            merchant_id: merchantId,
            category_id: categoryId,
            limit,
            exclude_ids: excludeIds
        };
    }
    /**
     * 构建包间推荐参数
     */
    static buildRoomRecommendationParams(date, guestCount, options) {
        return {
            date,
            guest_count: guestCount,
            cuisine_preference: options === null || options === void 0 ? void 0 : options.cuisinePreference,
            price_range: options === null || options === void 0 ? void 0 : options.priceRange,
            location: options === null || options === void 0 ? void 0 : options.location,
            limit: (options === null || options === void 0 ? void 0 : options.limit) || 10
        };
    }
}
exports.RecommendationAdapter = RecommendationAdapter;
/**
 * 区域服务适配器
 */
class RegionAdapter {
    /**
     * 适配区域数据
     */
    static adaptRegion(region) {
        return Object.assign(Object.assign({}, region), { id: Number(region.id), level: Number(region.level), parent_id: region.parent_id ? Number(region.parent_id) : undefined, coordinates: region.coordinates ? {
                latitude: Number(region.coordinates.latitude),
                longitude: Number(region.coordinates.longitude)
            } : undefined, is_service_available: Boolean(region.is_service_available) });
    }
    /**
     * 构建区域层级树
     */
    static buildRegionTree(regions) {
        const regionMap = new Map();
        const rootRegions = [];
        // 创建映射
        regions.forEach(region => {
            regionMap.set(region.id, Object.assign(Object.assign({}, region), { children: [] }));
        });
        // 构建树结构
        regions.forEach(region => {
            const regionNode = regionMap.get(region.id);
            if (region.parent_id) {
                const parent = regionMap.get(region.parent_id);
                if (parent) {
                    parent.children.push(regionNode);
                }
            }
            else {
                rootRegions.push(regionNode);
            }
        });
        return rootRegions;
    }
}
exports.RegionAdapter = RegionAdapter;
// ==================== 便捷函数 ====================
/**
 * 搜索便捷函数
 */
class SearchUtils {
    /**
     * 快速搜索菜品
     */
    static quickSearchDishes(keyword, merchantId) {
        return __awaiter(this, void 0, void 0, function* () {
            const result = yield (0, exports.searchDishes)({
                keyword,
                merchant_id: merchantId,
                page_id: 1,
                page_size: 20
            });
            return SearchAdapter.adaptDishResults(result.data);
        });
    }
    /**
     * 快速搜索商户
     */
    static quickSearchMerchants(keyword, location) {
        return __awaiter(this, void 0, void 0, function* () {
            const result = yield (0, exports.searchMerchants)({
                keyword,
                latitude: location === null || location === void 0 ? void 0 : location.latitude,
                longitude: location === null || location === void 0 ? void 0 : location.longitude,
                page_id: 1,
                page_size: 20
            });
            return SearchAdapter.adaptMerchantResults(result.data);
        });
    }
    /**
     * 搜索附近商户
     */
    static searchNearbyMerchants(latitude, longitude, category) {
        return __awaiter(this, void 0, void 0, function* () {
            const result = yield (0, exports.searchMerchants)({
                keyword: '',
                latitude,
                longitude,
                category,
                sort_by: 'distance',
                page_id: 1,
                page_size: 20
            });
            return SearchAdapter.adaptMerchantResults(result.data);
        });
    }
}
exports.SearchUtils = SearchUtils;
/**
 * 推荐便捷函数
 */
class RecommendationUtils {
    /**
     * 获取个性化菜品推荐
     */
    static getPersonalizedDishes(userId_1) {
        return __awaiter(this, arguments, void 0, function* (userId, limit = 10) {
            const params = RecommendationAdapter.buildRecommendationParams(userId, undefined, undefined, limit);
            const results = yield (0, exports.getRecommendedDishes)(params);
            return SearchAdapter.adaptDishResults(results);
        });
    }
    /**
     * 获取商户内推荐菜品
     */
    static getMerchantRecommendedDishes(merchantId_1) {
        return __awaiter(this, arguments, void 0, function* (merchantId, limit = 10) {
            const params = RecommendationAdapter.buildRecommendationParams(undefined, merchantId, undefined, limit);
            const results = yield (0, exports.getRecommendedDishes)(params);
            return SearchAdapter.adaptDishResults(results);
        });
    }
    /**
     * 获取附近推荐商户
     */
    static getNearbyRecommendedMerchants() {
        return __awaiter(this, arguments, void 0, function* (limit = 10) {
            const params = RecommendationAdapter.buildRecommendationParams(undefined, undefined, undefined, limit);
            const results = yield (0, exports.getRecommendedMerchants)(params);
            return SearchAdapter.adaptMerchantResults(results);
        });
    }
}
exports.RecommendationUtils = RecommendationUtils;
/**
 * 区域便捷函数
 */
class RegionUtils {
    /**
     * 获取当前可服务区域
     */
    static getCurrentServiceRegions() {
        return __awaiter(this, void 0, void 0, function* () {
            const regions = yield (0, exports.getAvailableRegions)();
            return regions.map(region => RegionAdapter.adaptRegion(region));
        });
    }
    /**
     * 根据坐标查找服务区域
     */
    static findServiceRegionByLocation(latitude, longitude) {
        return __awaiter(this, void 0, void 0, function* () {
            try {
                const availableRegions = yield (0, exports.getAvailableRegions)();
                // 遍历可服务区域，检查坐标是否在服务范围内
                for (const region of availableRegions) {
                    try {
                        const checkResult = yield (0, exports.checkRegionService)(region.id, { latitude, longitude });
                        if (checkResult.is_available) {
                            return RegionAdapter.adaptRegion(checkResult.region);
                        }
                    }
                    catch (error) {
                        console.warn(`检查区域 ${region.id} 服务范围失败:`, error);
                    }
                }
                return null;
            }
            catch (error) {
                console.error('查找服务区域失败:', error);
                return null;
            }
        });
    }
    /**
     * 构建完整区域层级
     */
    static buildCompleteRegionHierarchy() {
        return __awaiter(this, void 0, void 0, function* () {
            const result = yield (0, exports.getRegions)({ page: 1, page_size: 1000 });
            const adaptedRegions = result.data.map(region => RegionAdapter.adaptRegion(region));
            return RegionAdapter.buildRegionTree(adaptedRegions);
        });
    }
}
exports.RegionUtils = RegionUtils;
exports.default = {
    // 搜索接口
    searchDishes: exports.searchDishes,
    searchMerchants: exports.searchMerchants,
    searchRooms: exports.searchRooms,
    // 推荐接口
    getRecommendedDishes: exports.getRecommendedDishes,
    getRecommendedMerchants: exports.getRecommendedMerchants,
    getRecommendedCombos: exports.getRecommendedCombos,
    getRecommendedRooms: exports.getRecommendedRooms,
    // 区域接口
    getRegions: exports.getRegions,
    getAvailableRegions: exports.getAvailableRegions,
    searchRegions: exports.searchRegions,
    getRegionById: exports.getRegionById,
    checkRegionService: exports.checkRegionService,
    getRegionChildren: exports.getRegionChildren,
    // 适配器
    SearchAdapter,
    RecommendationAdapter,
    RegionAdapter,
    // 便捷函数
    SearchUtils,
    RecommendationUtils,
    RegionUtils
};
