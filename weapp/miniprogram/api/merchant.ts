/**
 * 商户基础管理接口 (仅保留顾客端搜索和推荐)
 */

import { request } from '../utils/request'
import type { CustomizationGroup } from './dish'

// ==================== 数据类型定义 ====================

/**
 * 商户摘要信息 - 对齐 api.merchantSummary（用于搜索和推荐）
 */
export interface MerchantSummary {
  id: number                                   // 商户ID
  name: string                                 // 商户名称
  address?: string                             // 商户地址
  description?: string                         // 商户描述
  logo_url?: string                            // Logo URL
  distance?: number                            // 距离（米）
  estimated_delivery_fee?: number              // 预估配送费（分）
  total_orders?: number                        // 总销量（搜索返回）
  monthly_sales?: number                       // 近30天订单量（旧字段）
  region_id?: number                           // 区域ID
  status?: string                              // 商户状态
  is_open?: boolean                            // 是否营业（仅部分接口）
  tags?: string[]                              // 商户标签（仅部分接口）
}

/** 搜索商户返回项 - 对齐后端 searchMerchantResponse */
export interface SearchMerchantItem {
  id: number
  name: string
  description?: string
  address?: string
  phone?: string
  logo_url: string
  status: string
  is_open?: boolean
  region_id: number
  total_orders?: number
  distance?: number
  estimated_delivery_fee?: number
  tags?: string[]
}

export interface SearchMerchantsResponse {
  merchants: SearchMerchantItem[]
  total?: number
  total_count?: number
  page_id?: number
  page_size?: number
}

// ==================== 顾客端商户接口 ====================

/**
 * 搜索商户 - 基于 /v1/search/merchants
 */
export async function searchMerchants(params: {
  keyword?: string
  region_id?: number
  page_id?: number
  page_size?: number
  user_latitude?: number
  user_longitude?: number
}): Promise<MerchantSummary[]> {
  const requestParams: Record<string, unknown> = {
    keyword: params.keyword || '',
    page_id: params.page_id || 1,
    page_size: params.page_size || 20
  }

  if (params.user_latitude !== undefined && params.user_latitude !== null) {
    requestParams.user_latitude = params.user_latitude
  }
  if (params.user_longitude !== undefined && params.user_longitude !== null) {
    requestParams.user_longitude = params.user_longitude
  }

  if (params.region_id !== undefined && params.region_id !== null) {
    requestParams.region_id = params.region_id
  }

  const response = await request<SearchMerchantsResponse>({
    url: '/v1/search/merchants',
    method: 'GET',
    data: requestParams,
    useCache: true,
    cacheTTL: 2 * 60 * 1000
  })

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
    is_open: item.is_open
  }))
}

/**
 * 推荐商户请求参数
 */
export interface RecommendMerchantsParams {
  region_id?: number
  user_latitude?: number
  user_longitude?: number
  limit?: number
  page?: number
}

/**
 * 推荐商户结果（包含分页信息）
 */
export interface RecommendMerchantsResult {
  merchants: MerchantSummary[]
  has_more: boolean
  page: number
  total_count: number
}

/**
 * 获取推荐商户
 */
export async function getRecommendedMerchants(params?: RecommendMerchantsParams): Promise<RecommendMerchantsResult> {
  const page = params?.page ?? 1
  const pageSize = params?.limit ?? 20
  const response = await request<{ merchants: MerchantSummary[], total?: number, total_count?: number, page_id?: number, page_size?: number }>({
    url: '/v1/search/merchants',
    method: 'GET',
    data: {
      keyword: '',
      region_id: params?.region_id,
      user_latitude: params?.user_latitude,
      user_longitude: params?.user_longitude,
      page_id: page,
      page_size: pageSize
    },
    useCache: page === 1,
    cacheTTL: 3 * 60 * 1000
  })
  const total = response.total_count ?? response.total ?? response.merchants?.length ?? 0
  return {
    merchants: response.merchants || [],
    has_more: page * pageSize < total,
    page,
    total_count: total
  }
}

/**
 * 消费者端的证照/优惠规则定义
 */
export interface PublicDiscountRule {
  id: number
  name: string
  min_order_amount: number
  discount_amount: number
}

export interface PublicVoucher {
  id: number
  name: string
  amount: number
  min_order_amount: number
}

export interface PublicDeliveryPromotion {
  id: number
  name: string
  min_order_amount: number
  discount_amount: number
}

export interface PublicMerchantDetail {
  id: number
  name: string
  description?: string
  logo_url?: string
  cover_image?: string
  phone: string
  address: string
  latitude: number
  longitude: number
  region_id: number
  is_open: boolean
  tags: string[]
  monthly_sales: number
  avg_prep_minutes: number
  business_license_image_url?: string
  food_permit_url?: string
  business_hours?: {
    day_of_week: number
    open_time: string
    close_time: string
    is_closed: boolean
  }[]
  discount_rules?: PublicDiscountRule[]
  vouchers?: PublicVoucher[]
  delivery_promotions?: PublicDeliveryPromotion[]
}

/**
 * 获取商户详情（消费者端）
 */
export async function getPublicMerchantDetail(merchantId: number): Promise<PublicMerchantDetail> {
  return await request({
    url: `/v1/public/merchants/${merchantId}`,
    method: 'GET',
    useCache: true,
    cacheTTL: 5 * 60 * 1000
  })
}

/**
 * 菜品分类项
 */
export interface PublicDishCategory {
  id: number
  name: string
  sort_order: number
}

/**
 * 菜品项
 */
export interface PublicDish {
  id: number
  name: string
  description?: string
  price: number
  original_price?: number
  member_price?: number
  image_url?: string
  category_id: number
  category_name: string
  monthly_sales: number
  prepare_time: number
  tags: string[]
  customization_groups?: CustomizationGroup[]
}

export type DishDTO = PublicDish

/**
 * 菜品列表响应
 */
export interface PublicMerchantDishesResponse {
  categories: PublicDishCategory[]
  dishes: PublicDish[]
}

/**
 * 获取商户菜品列表（消费者端）
 */
export async function getPublicMerchantDishes(merchantId: number): Promise<PublicMerchantDishesResponse> {
  return await request({
    url: `/v1/public/merchants/${merchantId}/dishes`,
    method: 'GET',
    useCache: true,
    cacheTTL: 5 * 60 * 1000
  })
}

/**
 * 套餐菜品项
 */
export interface ComboDishItem {
  dish_id: number
  dish_name: string
  quantity: number
}

/**
 * 套餐项
 */
export interface PublicCombo {
  id: number
  name: string
  description?: string
  image_url?: string
  combo_price: number
  original_price: number
  dishes: ComboDishItem[]
  tags?: string[]
  dish_images?: string[]
}

/**
 * 套餐列表响应
 */
export interface PublicMerchantCombosResponse {
  combos: PublicCombo[]
}

/**
 * 获取商户套餐列表（消费者端）
 */
export async function getPublicMerchantCombos(merchantId: number): Promise<PublicMerchantCombosResponse> {
  return await request({
    url: `/v1/public/merchants/${merchantId}/combos`,
    method: 'GET',
    useCache: true,
    cacheTTL: 5 * 60 * 1000
  })
}

// ==================== 数据适配器 (仅保留顾客端相关) ====================

export class MerchantManagementAdapter {
  static formatDayOfWeek(dayOfWeek: number): string {
    const dayNames = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']
    return dayNames[dayOfWeek] || `星期${dayOfWeek}`
  }

  static isCurrentlyOpen(businessHours: { day_of_week: number, open_time: string, close_time: string, is_closed: boolean }[]): boolean {
    const now = new Date()
    const currentDay = now.getDay()
    const currentTime = now.toTimeString().slice(0, 5)

    const todayHours = businessHours.find((hour) => hour.day_of_week === currentDay)

    if (!todayHours || todayHours.is_closed) {
      return false
    }

    return currentTime >= todayHours.open_time && currentTime <= todayHours.close_time
  }
}

export const getMerchantDishes = getPublicMerchantDishes
export const getMerchants = searchMerchants

// ==================== 商户工作台接口（商户自身管理用） ====================

/**
 * 商户详情响应（商户工作台使用，包含 version）
 */
export interface MerchantOperatorResponse {
  id: number
  owner_user_id: number
  region_id: number
  name: string
  description?: string
  logo_url?: string
  phone: string
  address: string
  latitude?: string
  longitude?: string
  status: string
  is_open: boolean
  version: number
  created_at: string
  updated_at: string
}

/**
 * 获取当前登录商户信息（商户工作台）
 * GET /v1/merchants/me
 */
export function getMyMerchantProfile() {
  return request<MerchantOperatorResponse>({
    url: '/v1/merchants/me',
    method: 'GET'
  })
}

/**
 * 更新当前商户 Logo（商户工作台）
 * PATCH /v1/merchants/me
 * @param logoUrl 图片相对路径（rawUrl）
 * @param version 乐观锁版本号，必须从 GET /v1/merchants/me 获取
 */
export function updateMyMerchantLogo(logoUrl: string, version: number) {
  return request<MerchantOperatorResponse>({
    url: '/v1/merchants/me',
    method: 'PATCH',
    data: { logo_url: logoUrl, version }
  })
}
