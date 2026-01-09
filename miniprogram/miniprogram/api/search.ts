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

/** 搜索包间参数 - 对齐后端 searchRoomsRequest */
export interface SearchRoomsParams extends Record<string, unknown> {
    reservation_date: string           // 必填：预订日期 YYYY-MM-DD
    reservation_time: string           // 必填：预订时段 HH:MM
    min_capacity?: number              // 可选：最小容纳人数
    max_capacity?: number              // 可选：最大容纳人数
    max_minimum_spend?: number         // 可选：最大低消（分）
    tag_id?: number                    // 可选：菜系/标签ID
    region_id?: number                 // 可选：区域ID
    user_latitude?: number             // 可选：用户纬度
    user_longitude?: number            // 可选：用户经度
    page_id: number                    // 必填：页码
    page_size: number                  // 必填：每页数量
}

/** 推荐包间参数 - 对齐后端 exploreRoomsRequest */
export interface RecommendRoomsParams extends Record<string, unknown> {
    region_id?: number                 // 区域ID
    min_capacity?: number              // 最小容纳人数
    max_capacity?: number              // 最大容纳人数
    max_minimum_spend?: number         // 最大低消（分）
    user_latitude?: number             // 用户纬度
    user_longitude?: number            // 用户经度
    page_id: number                    // 页码
    page_size: number                  // 每页数量
}

/** 包间搜索结果 */
export interface RoomSearchResult {
    id: number
    merchant_id: number
    merchant_name: string
    merchant_logo: string
    merchant_address?: string
    name: string
    table_no?: string
    capacity: number
    hourly_rate: number
    minimum_spend: number
    images: string[]
    amenities: string[]
    is_available: boolean
    distance?: number
    estimated_delivery_fee?: number
    primary_image?: string
    monthly_reservations?: number
    tags?: string[]
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
 * @param params 推荐参数（已对齐后端 exploreRoomsRequest）
 */
export async function getRecommendedRooms(params: RecommendRoomsParams): Promise<RoomSearchResult[]> {
    logger.debug('Fetching Recommended Rooms', params, 'API')

    const res = await request<any>({
        url: '/v1/recommendations/rooms',
        method: 'GET',
        data: cleanParams(params)
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