/**
 * 搜索相关API接口
 * 基于swagger.json中的搜索和推荐接口
 */

import { request } from '../utils/request'
import { logger } from '../utils/logger'
import type { DishSummary } from './dish'
import type { MerchantSummary } from './merchant'

// ==================== 数据类型定义 ====================

/** 搜索商户参数 */
export interface SearchMerchantsParams extends Record<string, unknown> {
    keyword: string
    user_latitude?: number
    user_longitude?: number
    page_id: number
    page_size: number
}

/** 推荐商户参数 */
export interface RecommendMerchantsParams extends Record<string, unknown> {
    user_latitude?: number
    user_longitude?: number
    limit?: number
}

/** 搜索包间参数 */
export interface SearchRoomsParams extends Record<string, unknown> {
    keyword?: string
    date: string
    start_time: string
    end_time: string
    guest_count: number
    cuisine_type?: string
    min_price?: number
    max_price?: number
    user_latitude?: number
    user_longitude?: number
    page_id: number
    page_size: number
}

/** 推荐包间参数 */
export interface RecommendRoomsParams extends Record<string, unknown> {
    region_id?: number
    user_latitude?: number
    user_longitude?: number
    guest_count?: number
    min_price?: number
    max_price?: number
    amenities?: string
    page_id?: number
    limit?: number
}

/** 包间搜索结果 */
export interface RoomSearchResult {
    id: number
    merchant_id: number
    merchant_name: string
    merchant_logo: string
    name: string
    capacity: number
    hourly_rate: number
    minimum_spend: number
    images: string[]
    amenities: string[]
    is_available: boolean
    distance?: number
    estimated_delivery_fee?: number
}

/** 搜索历史记录 */
export interface SearchHistory {
    id: number
    keyword: string
    search_type: 'dish' | 'merchant' | 'room'
    created_at: string
}

/** 热门搜索关键词 */
export interface PopularKeyword {
    keyword: string
    search_count: number
    trend: 'up' | 'down' | 'stable'
}

/** 搜索建议 */
export interface SearchSuggestion {
    keyword: string
    type: 'dish' | 'merchant' | 'category'
    count: number
}

// ==================== API接口函数 ====================

/**
 * Robust parameter cleaner
 * Uses JSON serialization to strip undefined values reliably
 */
function cleanParams<T>(params: T): T {
    try {
        // Strip undefined
        const cleaned = JSON.parse(JSON.stringify(params))

        // Also strip explicit nulls if needed, or keeping them is fine. 
        // JSON keeps nulls. If backend dislikes null, we should remove them.
        // Let's remove nulls too for max safety against "null" string.
        if (cleaned && typeof cleaned === 'object') {
            Object.keys(cleaned).forEach(key => {
                if (cleaned[key] === null) {
                    delete cleaned[key]
                }
            })
        }
        return cleaned
    } catch (e) {
        logger.error('Param cleaning failed', e)
        return params
    }
}

/**
 * 搜索商户
 * @param params 搜索参数
 */
export async function searchMerchants(params: SearchMerchantsParams): Promise<MerchantSummary[]> {
    const data = cleanParams(params)
    if (!data.keyword) data.keyword = ''

    // Response is { merchants: [], total: ... }
    const res = await request<any>({
        url: '/v1/search/merchants',
        method: 'GET',
        data
    })
    return res.merchants || res // Fallback if API changes
}

/**
 * 获取推荐商户
 * @param params 推荐参数
 */
export async function getRecommendedMerchants(params: RecommendMerchantsParams = {}): Promise<MerchantSummary[]> {
    const res = await request<any>({
        url: '/v1/recommendations/merchants',
        method: 'GET',
        data: cleanParams(params)
    })
    return res.merchants || res
}

/**
 * 获取推荐包间
 * @param params 推荐参数
 */
export async function getRecommendedRooms(params: RecommendRoomsParams = {}): Promise<RoomSearchResult[]> {
    const {
        limit = 10,
        page_id = 1,
        guest_count,
        max_price,
        // min_price is not supported by recommend API currently based on swagger
        ...rest
    } = cleanParams(params)

    const queryParams: any = {
        page_id,
        page_size: limit,
        ...rest
    }

    if (guest_count) {
        queryParams.min_capacity = guest_count
    }

    // Interpret max_price as max_minimum_spend if provided
    if (max_price) {
        queryParams.max_minimum_spend = max_price // sending raw value for now to match status quo, unless I verify currency.
    }

    logger.debug('Fetching Recommended Rooms', queryParams, 'API')

    const res = await request<any>({
        url: '/v1/recommendations/rooms',
        method: 'GET',
        data: queryParams
    })
    return res.rooms || res
}

/**
 * 搜索包间
 * @param params 搜索参数
 */
export async function searchRooms(params: SearchRoomsParams): Promise<RoomSearchResult[]> {
    const res = await request<any>({
        url: '/v1/search/rooms',
        method: 'GET',
        data: cleanParams(params)
    })
    return res.rooms || res
}

/**
 * 获取搜索建议
 * @param keyword 关键词前缀
 * @param type 搜索类型
 */
export async function getSearchSuggestions(keyword: string, type?: 'dish' | 'merchant'): Promise<SearchSuggestion[]> {
    return request({
        url: '/v1/search/suggestions',
        method: 'GET',
        data: { keyword, type }
    })
}

/**
 * 获取热门搜索关键词
 * @param type 搜索类型
 */
export async function getPopularKeywords(type?: 'dish' | 'merchant'): Promise<PopularKeyword[]> {
    return request({
        url: '/v1/search/popular',
        method: 'GET',
        data: { type }
    })
}

/**
 * 获取搜索历史
 * @param limit 返回数量限制
 */
export async function getSearchHistory(limit: number = 10): Promise<SearchHistory[]> {
    return request({
        url: '/v1/search/history',
        method: 'GET',
        data: { limit }
    })
}

/**
 * 清除搜索历史
 */
export async function clearSearchHistory(): Promise<void> {
    return request({
        url: '/v1/search/history',
        method: 'DELETE'
    })
}

/**
 * 删除单条搜索历史
 * @param historyId 历史记录ID
 */
export async function deleteSearchHistory(historyId: number): Promise<void> {
    return request({
        url: `/v1/search/history/${historyId}`,
        method: 'DELETE'
    })
}

// ==================== 综合搜索接口 ====================

/** 综合搜索结果 */
export interface UnifiedSearchResult {
    dishes: DishSummary[]
    merchants: MerchantSummary[]
    total_dishes: number
    total_merchants: number
}

/**
 * 综合搜索（同时搜索菜品和商户）
 * @param keyword 搜索关键词
 * @param params 搜索参数
 */
export async function unifiedSearch(
    keyword: string,
    params: {
        user_latitude?: number
        user_longitude?: number
        dish_limit?: number
        merchant_limit?: number
    } = {}
): Promise<UnifiedSearchResult> {
    const { dish_limit = 10, merchant_limit = 10, ...locationParams } = params
    const cleanedLoc = cleanParams(locationParams)

    // 并行搜索菜品和商户
    const [dishResults, merchantResults] = await Promise.all([
        request({
            url: '/v1/search/dishes',
            method: 'GET',
            data: {
                keyword,
                page_id: 1,
                page_size: dish_limit,
                ...cleanedLoc
            }
        }) as Promise<any>,
        request({
            url: '/v1/search/merchants',
            method: 'GET',
            data: {
                keyword,
                page_id: 1,
                page_size: merchant_limit,
                ...cleanedLoc
            }
        }) as Promise<any>
    ])

    return {
        dishes: dishResults.dishes || dishResults,
        merchants: merchantResults.merchants || merchantResults,
        total_dishes: dishResults.total || dishResults.length,
        total_merchants: merchantResults.total || merchantResults.length
    }
}

// ==================== 兼容性别名 ====================

/** @deprecated 使用 searchMerchants 替代 */
export const getMerchants = searchMerchants

/** @deprecated 使用 getRecommendedMerchants 替代 */
export const getRecommendations = getRecommendedMerchants