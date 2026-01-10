/**
 * 搜索相关API接口
 * 迁移至 Supabase RPC 实现
 */

import { supabase } from '../services/supabase'
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

/** 搜索菜品参数 */
export interface SearchDishesParams extends Record<string, unknown> {
    keyword: string
    tag_id?: string
    user_latitude?: number
    user_longitude?: number
    page_id: number
    page_size: number
}

/** 搜索建议 */
export interface SearchSuggestion {
    keyword: string
    type: 'dish' | 'merchant' | 'category'
    count: number
}

// RPC 返回原始数据类型定义
interface MerchantRPCResult {
  id: string
  name: string
  description: string
  logo_url: string
  phone: string
  address: string
  latitude: number
  longitude: number
  status: string
  region_id: string
  is_open: boolean
  distance: number
  estimated_delivery_fee: { final_fee: number; base_fee: number; distance_fee: number }
  total_count: number
}

interface DishRPCResult {
  id: string
  merchant_id: string
  name: string
  price: number
  member_price?: number
  image_url: string
  is_available: boolean
  merchant_name: string
  merchant_logo: string
  merchant_is_open: boolean
  distance: number
  estimated_delivery_time: number
  estimated_delivery_fee: { final_fee: number; base_fee: number; distance_fee: number }
  monthly_sales: number
  repurchase_rate?: number
  total_count: number
}

// 汇总结果增强类型
type MerchantWithMeta = MerchantSummary & { total_count?: number }
type DishWithMeta = DishSummary & { total_count?: number }

// ==================== API接口函数 ====================

/**
 * 搜索商户 (对接 Supabase search_merchants RPC)
 */
export async function searchMerchants(params: SearchMerchantsParams): Promise<MerchantWithMeta[]> {
    const { data, error } = await supabase.rpc<MerchantRPCResult[]>('search_merchants', {
        p_keyword: params.keyword || null,
        p_user_lat: params.user_latitude,
        p_user_lng: params.user_longitude,
        p_page_id: params.page_id,
        p_page_size: params.page_size
    })

    if (error) {
        logger.error('searchMerchants failed', error)
        throw error
    }

    // Map RPC result to MerchantSummary
    return (data || []).map(item => ({
        id: item.id,
        name: item.name,
        description: item.description,
        logo_url: item.logo_url,
        phone: item.phone,
        address: item.address,
        latitude: item.latitude || 0,
        longitude: item.longitude || 0,
        status: item.status,
        region_id: item.region_id,
        is_open: item.is_open,
        distance: item.distance,
        estimated_delivery_fee: item.estimated_delivery_fee?.final_fee || 0,
        monthly_sales: 0, 
        tags: [], 
        total_count: item.total_count
    } as MerchantWithMeta))
}

/**
 * 搜索菜品 (对接 Supabase search_dishes RPC)
 */
export async function searchDishes(params: SearchDishesParams): Promise<DishWithMeta[]> {
    const { data, error } = await supabase.rpc<DishRPCResult[]>('search_dishes', {
        p_keyword: params.keyword || null,
        p_tag_id: params.tag_id || null,
        p_user_lat: params.user_latitude,
        p_user_lng: params.user_longitude,
        p_page_id: params.page_id,
        p_page_size: params.page_size
    })

    if (error) {
        logger.error('searchDishes failed', error)
        throw error
    }

    return (data || []).map(item => ({
        id: item.id,
        merchant_id: item.merchant_id,
        name: item.name,
        price: item.price,
        member_price: item.member_price,
        image_url: item.image_url,
        is_available: item.is_available,
        merchant_name: item.merchant_name,
        merchant_logo: item.merchant_logo,
        merchant_is_open: item.merchant_is_open,
        merchant_latitude: 0, 
        merchant_longitude: 0, 
        merchant_region_id: '', 
        distance: item.distance,
        estimated_delivery_time: item.estimated_delivery_time,
        estimated_delivery_fee: item.estimated_delivery_fee?.final_fee || 0,
        monthly_sales: item.monthly_sales,
        repurchase_rate: item.repurchase_rate,
        tags: [], 
        total_count: item.total_count
    } as DishWithMeta))
}

/**
 * 获取推荐商户 (暂用 search_merchants 无参调用)
 */
export async function getRecommendedMerchants(params: { user_latitude?: number; user_longitude?: number; limit?: number } = {}): Promise<MerchantWithMeta[]> {
    return searchMerchants({
        keyword: '',
        user_latitude: params.user_latitude,
        user_longitude: params.user_longitude,
        page_id: 1,
        page_size: params.limit || 10
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
 * 综合搜索 (并行调用 Dishes 与 Merchants RPC)
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
    const [dishes, merchants] = await Promise.all([
        searchDishes({
            keyword,
            user_latitude: params.user_latitude,
            user_longitude: params.user_longitude,
            page_id: 1,
            page_size: params.dish_limit || 10
        }),
        searchMerchants({
            keyword,
            user_latitude: params.user_latitude,
            user_longitude: params.user_longitude,
            page_id: 1,
            page_size: params.merchant_limit || 10
        })
    ])

    return {
        dishes,
        merchants,
        total_dishes: dishes.length > 0 ? dishes[0].total_count || 0 : 0,
        total_merchants: merchants.length > 0 ? merchants[0].total_count || 0 : 0
    }
}

// ==================== 历史记录占位 (前端本地实现或随后迁移) ====================

export async function getSearchHistory(): Promise<string[]> { return [] }
export async function clearSearchHistory(): Promise<void> { }
export async function getPopularKeywords(): Promise<string[]> { return [] }

// ==================== 兼容性别名 ====================
export const getMerchants = searchMerchants
export const getRecommendations = getRecommendedMerchants