/**
 * 搜索和推荐接口模块
 * 基于swagger.json完全重构，提供搜索功能、推荐引擎和区域服务
 */

import { request } from '../utils/request';

// ==================== 类型定义 ====================

// 搜索相关类型
export interface SearchDishesParams extends Record<string, unknown> {
    keyword: string;
    merchant_id?: number;
    category_id?: number;
    min_price?: number;
    max_price?: number;
    sort_by?: 'price_asc' | 'price_desc' | 'sales_desc' | 'rating_desc';
    page_id?: number;  // backend expects page_id
    page_size?: number;
}

export interface SearchMerchantsParams extends Record<string, unknown> {
    keyword: string;
    latitude?: number;
    longitude?: number;
    category?: string;
    min_rating?: number;
    delivery_fee_max?: number;
    sort_by?: 'distance' | 'rating' | 'sales' | 'delivery_fee';
    page_id?: number;  // backend expects page_id
    page_size?: number;
}

export interface SearchRoomsParams extends Record<string, unknown> {
    keyword?: string;
    date: string;
    start_time: string;
    end_time: string;
    guest_count: number;
    cuisine_type?: string;
    min_price?: number;
    max_price?: number;
    latitude?: number;
    longitude?: number;
    page?: number;
    page_size?: number;
}

// 推荐相关类型
export interface RecommendationParams extends Record<string, unknown> {
    user_id?: number;
    merchant_id?: number;
    category_id?: number;
    limit?: number;
    exclude_ids?: number[];
}

export interface RecommendRoomsParams extends Record<string, unknown> {
    date: string;
    guest_count: number;
    cuisine_preference?: string;
    price_range?: 'low' | 'medium' | 'high';
    location?: {
        latitude: number;
        longitude: number;
    };
    limit?: number;
}

// 区域相关类型
export interface RegionParams extends Record<string, unknown> {
    level?: number;
    parent_id?: number;
    page?: number;
    page_size?: number;
}

export interface RegionSearchParams extends Record<string, unknown> {
    keyword: string;
    level?: number;
    limit?: number;
}

export interface RegionCheckParams extends Record<string, unknown> {
    latitude: number;
    longitude: number;
}

// 响应类型
export interface DishSearchResult {
    id: number;
    name: string;
    description: string;
    price: number;
    original_price?: number;
    image_url: string;
    merchant_id: number;
    merchant_name: string;
    category_name: string;
    sales_count: number;
    rating: number;
    is_available: boolean;
    customizations?: any[];
}

export interface MerchantSearchResult {
    id: number;
    name: string;
    description: string;
    logo_url: string;
    cover_image: string;
    category: string;
    rating: number;
    review_count: number;
    sales_count: number;
    distance?: number;
    delivery_fee?: number;
    min_order_amount: number;
    estimated_delivery_time: number;
    is_open: boolean;
    business_hours: any;
    address: string;
}

export interface RoomSearchResult {
    id: number;
    name: string;
    description: string;
    images: string[];
    merchant_id: number;
    merchant_name: string;
    capacity: {
        min_guests: number;
        max_guests: number;
    };
    price_per_hour: number;
    amenities: string[];
    cuisine_type: string;
    is_available: boolean;
    available_times: string[];
}

export interface Region {
    id: number;
    name: string;
    code: string;
    level: number;
    parent_id?: number;
    full_name: string;
    coordinates?: {
        latitude: number;
        longitude: number;
    };
    is_service_available: boolean;
}

/** 区域可用性响应 - 对齐 api.regionAvailabilityResponse */
export interface RegionAvailabilityResponse {
    region_id: number;                           // 区域ID
    name: string;                                // 区域名称
    code: string;                                // 区域代码
    is_available: boolean;                       // 是否可用
    reason?: string;                             // 不可用原因
}

/** 逆地理编码响应 - 对齐 api.reverseGeocodeResponse */
export interface ReverseGeocodeResponse {
    address?: string;                            // 地址
    city?: string;                               // 城市
    district?: string;                           // 区县
    formatted_address?: string;                  // 格式化地址
    province?: string;                           // 省份
    street?: string;                             // 街道
    street_number?: string;                      // 门牌号
}

/** 逆地理编码API响应 - 对齐 api.reverseGeocodeAPIResponse */
export interface ReverseGeocodeAPIResponse {
    code: number;                                // 状态码
    message: string;                             // 消息
    data: ReverseGeocodeResponse;                // 数据
}

/** 腾讯路线API响应 - 对齐 api.tencentDirectionAPIResponse */
export interface TencentDirectionAPIResponse {
    code: number;                                // 状态码
    message: string;                             // 消息
    data: number[];                              // 路线数据
}

/** 获取推荐配置响应 - 对齐 api.getRecommendationConfigResponse */
export interface GetRecommendationConfigResponse {
    region_id: number;                           // 区域ID
    exploration_ratio: number;                   // 探索比例
    exploitation_ratio: number;                  // 利用比例
    random_ratio: number;                        // 随机比例
    auto_adjust: boolean;                        // 是否自动调整
    updated_at: string;                          // 更新时间
}

/** 更新推荐配置请求 - 对齐 api.updateRecommendationConfigRequest */
export interface UpdateRecommendationConfigRequest extends Record<string, unknown> {
    exploration_ratio?: number;                  // 探索比例（0-1）
    exploitation_ratio?: number;                 // 利用比例（0-1）
    random_ratio?: number;                       // 随机比例（0-1）
    auto_adjust?: boolean;                       // 是否自动调整
}

export interface PaginatedResponse<T> {
    data: T[];
    pagination: {
        page: number;
        page_size: number;
        total: number;
        total_pages: number;
    };
}

// ==================== API 接口函数 ====================

/**
 * 搜索菜品
 */
export const searchDishes = async (params: SearchDishesParams): Promise<PaginatedResponse<DishSearchResult>> => {
    return request({
        url: '/v1/search/dishes',
        method: 'GET',
        data: params
    });
};

/**
 * 搜索商户
 */
export const searchMerchants = async (params: SearchMerchantsParams): Promise<PaginatedResponse<MerchantSearchResult>> => {
    return request({
        url: '/v1/search/merchants',
        method: 'GET',
        data: params
    });
};

/**
 * 搜索包间
 */
export const searchRooms = async (params: SearchRoomsParams): Promise<PaginatedResponse<RoomSearchResult>> => {
    return request({
        url: '/v1/search/rooms',
        method: 'GET',
        data: params
    });
};

/**
 * 获取菜品推荐
 */
export const getRecommendedDishes = async (params: RecommendationParams): Promise<DishSearchResult[]> => {
    return request({
        url: '/v1/recommendations/dishes',
        method: 'GET',
        data: params
    });
};

/**
 * 获取商户推荐
 */
export const getRecommendedMerchants = async (params: RecommendationParams): Promise<MerchantSearchResult[]> => {
    return request({
        url: '/v1/recommendations/merchants',
        method: 'GET',
        data: params
    });
};

/**
 * 获取套餐推荐
 */
export const getRecommendedCombos = async (params: RecommendationParams): Promise<any[]> => {
    return request({
        url: '/v1/recommendations/combos',
        method: 'GET',
        data: params
    });
};

/**
 * 获取包间推荐
 */
export const getRecommendedRooms = async (params: RecommendRoomsParams): Promise<RoomSearchResult[]> => {
    return request({
        url: '/v1/recommendations/rooms',
        method: 'GET',
        data: params
    });
};

/**
 * 获取区域列表
 */
export const getRegions = async (params?: RegionParams): Promise<PaginatedResponse<Region>> => {
    return request({
        url: '/v1/regions',
        method: 'GET',
        data: params
    });
};

/**
 * 获取可服务区域列表
 */
export const getAvailableRegions = async (): Promise<Region[]> => {
    return request({
        url: '/v1/regions/available',
        method: 'GET'
    });
};

/**
 * 搜索区域
 */
export const searchRegions = async (params: RegionSearchParams): Promise<Region[]> => {
    return request({
        url: '/v1/regions/search',
        method: 'GET',
        data: params
    });
};

/**
 * 获取区域详情
 */
export const getRegionById = async (id: number): Promise<Region> => {
    return request({
        url: `/v1/regions/${id}`,
        method: 'GET'
    });
};

/**
 * 检查坐标是否在服务区域内
 */
export const checkRegionService = async (id: number, params: RegionCheckParams): Promise<{ is_available: boolean; region: Region }> => {
    return request({
        url: `/v1/regions/${id}/check`,
        method: 'GET',
        data: params
    });
};

/**
 * 获取区域的子区域
 */
export const getRegionChildren = async (id: number): Promise<Region[]> => {
    return request({
        url: `/v1/regions/${id}/children`,
        method: 'GET'
    });
};

// ==================== 数据适配器 ====================

/**
 * 搜索结果适配器
 */
export class SearchAdapter {
    /**
     * 适配菜品搜索结果
     */
    static adaptDishResults(results: DishSearchResult[]): DishSearchResult[] {
        return results.map(dish => ({
            ...dish,
            price: Number(dish.price),
            original_price: dish.original_price ? Number(dish.original_price) : undefined,
            rating: Number(dish.rating),
            sales_count: Number(dish.sales_count),
            is_available: Boolean(dish.is_available)
        }));
    }

    /**
     * 适配商户搜索结果
     */
    static adaptMerchantResults(results: MerchantSearchResult[]): MerchantSearchResult[] {
        return results.map(merchant => ({
            ...merchant,
            rating: Number(merchant.rating),
            review_count: Number(merchant.review_count),
            sales_count: Number(merchant.sales_count),
            distance: merchant.distance ? Number(merchant.distance) : undefined,
            delivery_fee: merchant.delivery_fee ? Number(merchant.delivery_fee) : undefined,
            min_order_amount: Number(merchant.min_order_amount),
            estimated_delivery_time: Number(merchant.estimated_delivery_time),
            is_open: Boolean(merchant.is_open)
        }));
    }

    /**
     * 适配包间搜索结果
     */
    static adaptRoomResults(results: RoomSearchResult[]): RoomSearchResult[] {
        return results.map(room => ({
            ...room,
            capacity: {
                min_guests: Number(room.capacity.min_guests),
                max_guests: Number(room.capacity.max_guests)
            },
            price_per_hour: Number(room.price_per_hour),
            is_available: Boolean(room.is_available)
        }));
    }
}

/**
 * 推荐系统适配器
 */
export class RecommendationAdapter {
    /**
     * 构建推荐参数
     */
    static buildRecommendationParams(
        userId?: number,
        merchantId?: number,
        categoryId?: number,
        limit: number = 10,
        excludeIds?: number[]
    ): RecommendationParams {
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
    static buildRoomRecommendationParams(
        date: string,
        guestCount: number,
        options?: {
            cuisinePreference?: string;
            priceRange?: 'low' | 'medium' | 'high';
            location?: { latitude: number; longitude: number };
            limit?: number;
        }
    ): RecommendRoomsParams {
        return {
            date,
            guest_count: guestCount,
            cuisine_preference: options?.cuisinePreference,
            price_range: options?.priceRange,
            location: options?.location,
            limit: options?.limit || 10
        };
    }
}

/**
 * 区域服务适配器
 */
export class RegionAdapter {
    /**
     * 适配区域数据
     */
    static adaptRegion(region: Region): Region {
        return {
            ...region,
            id: Number(region.id),
            level: Number(region.level),
            parent_id: region.parent_id ? Number(region.parent_id) : undefined,
            coordinates: region.coordinates ? {
                latitude: Number(region.coordinates.latitude),
                longitude: Number(region.coordinates.longitude)
            } : undefined,
            is_service_available: Boolean(region.is_service_available)
        };
    }

    /**
     * 构建区域层级树
     */
    static buildRegionTree(regions: Region[]): Region[] {
        const regionMap = new Map<number, Region & { children?: Region[] }>();
        const rootRegions: Region[] = [];

        // 创建映射
        regions.forEach(region => {
            regionMap.set(region.id, { ...region, children: [] });
        });

        // 构建树结构
        regions.forEach(region => {
            const regionNode = regionMap.get(region.id)!;
            if (region.parent_id) {
                const parent = regionMap.get(region.parent_id);
                if (parent) {
                    parent.children!.push(regionNode);
                }
            } else {
                rootRegions.push(regionNode);
            }
        });

        return rootRegions;
    }
}

// ==================== 便捷函数 ====================

/**
 * 搜索便捷函数
 */
export class SearchUtils {
    /**
     * 快速搜索菜品
     */
    static async quickSearchDishes(keyword: string, merchantId?: number): Promise<DishSearchResult[]> {
        const result = await searchDishes({
            keyword,
            merchant_id: merchantId,
            page_id: 1,
            page_size: 20
        });
        return SearchAdapter.adaptDishResults(result.data);
    }

    /**
     * 快速搜索商户
     */
    static async quickSearchMerchants(keyword: string, location?: { latitude: number; longitude: number }): Promise<MerchantSearchResult[]> {
        const result = await searchMerchants({
            keyword,
            latitude: location?.latitude,
            longitude: location?.longitude,
            page_id: 1,
            page_size: 20
        });
        return SearchAdapter.adaptMerchantResults(result.data);
    }

    /**
     * 搜索附近商户
     */
    static async searchNearbyMerchants(
        latitude: number,
        longitude: number,
        category?: string
    ): Promise<MerchantSearchResult[]> {
        const result = await searchMerchants({
            keyword: '',
            latitude,
            longitude,
            category,
            sort_by: 'distance',
            page_id: 1,
            page_size: 20
        });
        return SearchAdapter.adaptMerchantResults(result.data);
    }
}

/**
 * 推荐便捷函数
 */
export class RecommendationUtils {
    /**
     * 获取个性化菜品推荐
     */
    static async getPersonalizedDishes(userId: number, limit: number = 10): Promise<DishSearchResult[]> {
        const params = RecommendationAdapter.buildRecommendationParams(userId, undefined, undefined, limit);
        const results = await getRecommendedDishes(params);
        return SearchAdapter.adaptDishResults(results);
    }

    /**
     * 获取商户内推荐菜品
     */
    static async getMerchantRecommendedDishes(merchantId: number, limit: number = 10): Promise<DishSearchResult[]> {
        const params = RecommendationAdapter.buildRecommendationParams(undefined, merchantId, undefined, limit);
        const results = await getRecommendedDishes(params);
        return SearchAdapter.adaptDishResults(results);
    }

    /**
     * 获取附近推荐商户
     */
    static async getNearbyRecommendedMerchants(limit: number = 10): Promise<MerchantSearchResult[]> {
        const params = RecommendationAdapter.buildRecommendationParams(undefined, undefined, undefined, limit);
        const results = await getRecommendedMerchants(params);
        return SearchAdapter.adaptMerchantResults(results);
    }
}

/**
 * 区域便捷函数
 */
export class RegionUtils {
    /**
     * 获取当前可服务区域
     */
    static async getCurrentServiceRegions(): Promise<Region[]> {
        const regions = await getAvailableRegions();
        return regions.map(region => RegionAdapter.adaptRegion(region));
    }

    /**
     * 根据坐标查找服务区域
     */
    static async findServiceRegionByLocation(
        latitude: number,
        longitude: number
    ): Promise<Region | null> {
        try {
            const availableRegions = await getAvailableRegions();

            // 遍历可服务区域，检查坐标是否在服务范围内
            for (const region of availableRegions) {
                try {
                    const checkResult = await checkRegionService(region.id, { latitude, longitude });
                    if (checkResult.is_available) {
                        return RegionAdapter.adaptRegion(checkResult.region);
                    }
                } catch (error) {
                    console.warn(`检查区域 ${region.id} 服务范围失败:`, error);
                }
            }

            return null;
        } catch (error) {
            console.error('查找服务区域失败:', error);
            return null;
        }
    }

    /**
     * 构建完整区域层级
     */
    static async buildCompleteRegionHierarchy(): Promise<Region[]> {
        const result = await getRegions({ page: 1, page_size: 1000 });
        const adaptedRegions = result.data.map(region => RegionAdapter.adaptRegion(region));
        return RegionAdapter.buildRegionTree(adaptedRegions);
    }
}

export default {
    // 搜索接口
    searchDishes,
    searchMerchants,
    searchRooms,

    // 推荐接口
    getRecommendedDishes,
    getRecommendedMerchants,
    getRecommendedCombos,
    getRecommendedRooms,

    // 区域接口
    getRegions,
    getAvailableRegions,
    searchRegions,
    getRegionById,
    checkRegionService,
    getRegionChildren,

    // 适配器
    SearchAdapter,
    RecommendationAdapter,
    RegionAdapter,

    // 便捷函数
    SearchUtils,
    RecommendationUtils,
    RegionUtils
};