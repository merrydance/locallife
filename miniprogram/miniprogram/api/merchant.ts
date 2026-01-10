/**
 * 商户基础管理接口
 * 基于swagger.json完全重构，仅保留后端支持的接口
 */

import { request, API_BASE } from '../utils/request'
import { getToken } from '../utils/auth'
import { logger } from '../utils/logger'
import { supabase } from '../services/supabase'

// ==================== 数据类型定义 ====================

/**
 * 商户详情响应 - 完全对齐 api.merchantResponse
 */
export interface MerchantResponse {
  id: string | number                                   // 商户ID
  name: string                                 // 商户名称
  address: string                              // 商户地址
  description: string                          // 商户描述
  phone: string                                // 商户电话
  logo_url: string                             // 商户Logo URL
  latitude: string                             // 纬度（后端定义为string）
  longitude: string                            // 经度（后端定义为string）
  status: string                               // 商户状态
  is_open: boolean                             // 是否营业
  owner_user_id: string | number                        // 商户所有者用户ID
  region_id: string | number                            // 区域ID
  created_at: string                           // 创建时间
  updated_at: string                           // 更新时间
  version?: number                             // 乐观锁版本号（可选）
}

/**
 * 商户更新请求 - 完全对齐 api.updateMerchantRequest
 */
export interface UpdateMerchantRequest extends Record<string, unknown> {
  name?: string                                // 商户名称 (2-50字符)
  address?: string                             // 商户地址 (5-200字符)
  description?: string                         // 商户描述 (最多500字符)
  phone?: string                               // 商户电话 (11位)
  logo_url?: string                            // Logo URL (最多500字符)
  latitude?: string                            // 纬度
  longitude?: string                           // 经度
  version: number                              // 乐观锁版本号（必填）
}

/**
 * 商户状态响应 - 完全对齐 api.merchantStatusResponse
 */
export interface MerchantStatusResponse {
  is_open: boolean                             // 是否营业
  message: string                              // 状态消息
  auto_close_at?: string                       // 自动打烊时间
}

/**
 * 商户状态更新请求 - 完全对齐 api.updateMerchantStatusRequest
 */
export interface UpdateMerchantStatusRequest extends Record<string, unknown> {
  is_open: boolean                             // 是否营业（必填）
  auto_close_at?: string                       // 自动打烊时间（RFC3339格式，最多50字符）
}

/**
 * 营业时间项 - 完全对齐 api.businessHourItem
 */
export interface BusinessHourItem {
  day_of_week?: number                         // 星期几 (0=周日, 1=周一, ..., 6=周六)
  open_time: string                            // 开始时间 (HH:MM格式，必填)
  close_time: string                           // 结束时间 (HH:MM格式，必填)
  is_closed?: boolean                          // 是否休息
}

/**
 * 营业时间响应项 - 完全对齐 api.businessHourResponse
 */
export interface BusinessHourResponse {
  id: number                                   // 营业时间ID
  day_of_week: number                          // 星期几
  day_name: string                             // 星期名称
  open_time: string                            // 开始时间
  close_time: string                           // 结束时间
  is_closed: boolean                           // 是否休息
}

/**
 * 设置营业时间请求 - 完全对齐 api.setBusinessHoursRequest
 */
export interface SetBusinessHoursRequest extends Record<string, unknown> {
  hours: BusinessHourItem[]                    // 一周的营业时间（1-7项，必填）
}

/**
 * 营业时间列表响应 - 完全对齐 api.businessHoursListResponse
 */
export interface BusinessHoursListResponse {
  hours: BusinessHourResponse[]                // 营业时间列表
}

/**
 * 图片上传响应 - 完全对齐 api.uploadImageResponse
 */
export interface UploadImageResponse {
  image_url: string                            // 图片URL
}

/**
 * 商户摘要信息 - 对齐 api.merchantSummary（用于搜索和推荐）
 */
export interface MerchantSummary {
  id: string | number                                   // 商户ID
  name: string                                 // 商户名称
  address: string                              // 商户地址
  description: string                          // 商户描述
  logo_url: string                             // Logo URL
  latitude: number                             // 纬度
  longitude: number                            // 经度
  distance: number                             // 距离（米）
  estimated_delivery_fee: number              // 预估配送费（分）
  monthly_sales: number                       // 近30天订单量
  region_id: string | number                            // 区域ID
  is_open: boolean                             // 是否营业
  tags: string[]                               // 商户标签
}


/**
 * 商户详情响应 - 对齐 api.merchantDetailResponse
 */
export interface MerchantDetailResponse {
  id: string | number                                   // 商户ID
  name: string                                 // 商户名称
  address: string                              // 商户地址
  description?: string                         // 商户描述
  phone: string                                // 商户电话
  logo_url?: string                            // 商户Logo URL
  latitude: number                             // 纬度
  longitude: number                            // 经度
  status: string                               // 商户状态
  is_open: boolean                             // 是否营业
  owner_user_id: string | number                        // 商户所有者用户ID
  region_id: string | number                            // 区域ID
  version: number                              // 乐观锁版本号
  created_at: string                           // 创建时间
  updated_at: string                           // 更新时间
}

/**
 * 商户列表项 - 对齐 api.merchantListItem
 */
export interface MerchantListItem {
  created_at: string
}

/**
 * 菜品数据传输对象
 */
export interface DishDTO {
  id: string
  merchant_id: string
  category_id: string
  category_name?: string
  name: string
  description?: string
  price: number
  stock: number
  status: 'ON_SHELF' | 'OFF_SHELF'
  image_url?: string
  month_sales?: number
}

/**
 * 订单菜品项
 */
export interface OrderDishItem {
  name: string
  quantity: number
  price: number
}

/**
 * 订单详情响应
 */
export interface MerchantOrderDTO {
  id: string
  order_no: string
  status: string
  order_type: 'TAKEOUT' | 'DINE_IN'
  total_amount: number
  items: OrderDishItem[]
  table_id?: string
  delivery_address?: any
  created_at: string
}

/**
 * 更新商户基本信息请求 - 对齐 api.updateMerchantBasicInfoRequest
 */
export interface UpdateMerchantBasicInfoRequest extends Record<string, unknown> {
  merchant_name?: string                       // 商户名称（2-50字符）
  business_address?: string                    // 商户地址（5-200字符）
  contact_phone?: string                       // 联系电话
  latitude?: string                            // 纬度
  longitude?: string                           // 经度
  region_id?: number                           // 区域ID
}

/**
 * 商户绑定银行卡请求 - 对齐 api.merchantBindBankRequest
 */
export interface MerchantBindBankRequest extends Record<string, unknown> {
  account_type: 'ACCOUNT_TYPE_BUSINESS' | 'ACCOUNT_TYPE_PRIVATE'  // 账户类型
  account_bank: string                         // 开户银行（最大128字符）
  account_name: string                         // 开户名称（最大128字符）
  account_number: string                       // 银行账号
  bank_address_code: string                    // 开户银行省市编码
  bank_name?: string                           // 开户银行全称（支行）
  contact_phone: string                        // 联系电话
  contact_email?: string                       // 联系邮箱
}

/**
 * 商户绑定银行卡响应 - 对齐 api.merchantBindBankResponse
 */
export interface MerchantBindBankResponse {
  applyment_id: number                         // 微信申请单号
  status: string                               // 状态
  message: string                              // 消息
}

// ==================== 商户基础管理服务 ====================

/**
 * 商户基础管理服务
 * 基于swagger.json完全重构，仅包含后端支持的接口
 */
export class MerchantManagementService {

  /**
   * 获取当前商户信息
   * GET /v1/merchants/me
   */
  static async getMerchantInfo(): Promise<MerchantResponse> {
    return await request({
      url: '/v1/merchants/me',
      method: 'GET',
      useCache: true,
      cacheTTL: 5 * 60 * 1000 // 5分钟缓存
    })
  }

  /**
   * 获取当前用户拥有的所有商户列表
   * GET /v1/merchants/my
   * 用于多店铺切换功能
   */
  static async getMyMerchants(): Promise<MerchantResponse[]> {
    return await request({
      url: '/v1/merchants/my',
      method: 'GET',
      useCache: true,
      cacheTTL: 5 * 60 * 1000 // 5分钟缓存
    })
  }

  /**
   * 更新商户信息
   * PATCH /v1/merchants/me
   * 使用乐观锁防止并发冲突
   */
  static async updateMerchantInfo(data: UpdateMerchantRequest): Promise<MerchantResponse> {
    return await request({
      url: '/v1/merchants/me',
      method: 'PATCH',
      data
    })
  }

  /**
   * 获取商户营业状态
   * GET /v1/merchants/me/status
   */
  static async getMerchantStatus(): Promise<MerchantStatusResponse> {
    return await request({
      url: '/v1/merchants/me/status',
      method: 'GET'
    })
  }

  /**
   * 更新商户营业状态
   * PATCH /v1/merchants/me/status
   */
  static async updateMerchantStatus(data: UpdateMerchantStatusRequest): Promise<MerchantStatusResponse> {
    return await request({
      url: '/v1/merchants/me/status',
      method: 'PATCH',
      data
    })
  }

  /**
   * 获取商户营业时间
   * GET /v1/merchants/me/business-hours
   */
  static async getBusinessHours(): Promise<BusinessHoursListResponse> {
    return await request({
      url: '/v1/merchants/me/business-hours',
      method: 'GET'
    })
  }

  /**
   * 设置商户营业时间
   * PUT /v1/merchants/me/business-hours
   */
  static async setBusinessHours(data: SetBusinessHoursRequest): Promise<BusinessHoursListResponse> {
    return await request({
      url: '/v1/merchants/me/business-hours',
      method: 'PUT',
      data
    })
  }

  /**
   * 上传商户图片
   * POST /v1/merchants/images/upload
   * 支持营业执照、身份证、Logo等图片上传
   */
  static async uploadImage(
    filePath: string,
    category: 'business_license' | 'id_front' | 'id_back' | 'logo'
  ): Promise<UploadImageResponse> {
    const token = getToken()
    return new Promise((resolve, reject) => {
      wx.uploadFile({
        url: `${API_BASE}/v1/merchants/images/upload`,
        filePath: filePath,
        name: 'image',
        formData: { category },
        header: {
          'Authorization': `Bearer ${token}`
        },
        success: (res) => {
          if (res.statusCode === 200) {
            try {
              const data = JSON.parse(res.data)
              logger.debug('Upload Response Raw', data, 'Merchant') // DEBUG

              // Helper to normalize
              const normalize = (url: string) => {
                if (url && !url.startsWith('http')) {
                  if (url.startsWith('/')) url = url.substring(1)
                  return `${API_BASE}/${url}`
                }
                return url
              }

              if (data.code === 0 && data.data) {
                // Envelope format
                if (data.data.image_url) {
                  data.data.image_url = normalize(data.data.image_url)
                }
                resolve(data.data)
              } else if (data.image_url) {
                // Direct format (Unwrapped)
                data.image_url = normalize(data.image_url)
                resolve(data as UploadImageResponse)
              } else {
                // Fallback
                resolve(data as unknown as UploadImageResponse)
              }
            } catch (e) {
              reject(new Error('Parse upload response failed'))
            }
          } else {
            logger.error('Upload failed', res, 'Merchant')
            reject(new Error(`HTTP ${res.statusCode}`))
          }
        },
        fail: (err) => {
          logger.error('Upload network error', err, 'Merchant')
          reject(err)
        }
      })
    })
  }
}

// ==================== 顾客端商户接口 ====================

/**
 * 搜索商户 - 基于 /v1/search/merchants
 * 注意：后端要求 keyword, page_id, page_size 为必填参数
 */
export async function searchMerchants(params: {
  keyword?: string
  page_id?: number
  page_size?: number
  user_latitude?: number
  user_longitude?: number
}): Promise<MerchantSummary[]> {
  const { data, error } = await supabase.rpc('search_merchants', {
    p_keyword: params.keyword || null,
    p_user_lat: params.user_latitude,
    p_user_lng: params.user_longitude,
    p_page_id: params.page_id || 1,
    p_page_size: params.page_size || 20
  })

  if (error) {
    logger.error('searchMerchants failed', error)
    return []
  }

  return (data as any[] || []).map(item => ({
    id: item.id,
    name: item.name,
    address: item.address,
    description: item.description || '',
    logo_url: item.logo_url || '',
    latitude: item.latitude || 0,
    longitude: item.longitude || 0,
    distance: item.distance || 0,
    estimated_delivery_fee: item.estimated_delivery_fee?.final_fee || 0,
    monthly_sales: 0,
    region_id: item.region_id,
    is_open: item.is_open,
    tags: []
  }))
}

/**
 * 推荐商户响应 - 对齐 api.recommendMerchantsResponse
 */
export interface RecommendMerchantsResponse {
  merchants: MerchantSummary[]
  algorithm: string
  expired_at: string
}

/**
 * 推荐商户请求参数
 */
export interface RecommendMerchantsParams {
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
 * 获取推荐商户 - 基于 /v1/recommendations/merchants
 * 支持分页，返回包含 has_more 的完整响应
 */
export async function getRecommendedMerchants(params?: RecommendMerchantsParams): Promise<RecommendMerchantsResult> {
  const response = await request<RecommendMerchantsResponse & { has_more?: boolean; page?: number; total_count?: number }>({
    url: '/v1/recommendations/merchants',
    method: 'GET',
    data: params,
    useCache: params?.page === 1 || !params?.page,
    cacheTTL: 3 * 60 * 1000 // 3分钟缓存
  })
  return {
    merchants: response.merchants || [],
    has_more: response.has_more ?? false,
    page: response.page ?? 1,
    total_count: response.total_count ?? 0
  }
}

/**
 * 消费者端商户详情响应 - 对齐 api.publicMerchantDetailResponse
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
  id: string | number                                   // 商户ID
  name: string                                 // 商户名称
  description?: string                         // 商户描述
  logo_url?: string                            // Logo URL
  cover_image?: string                         // 门头照/招牌图
  phone: string                                // 商户电话
  address: string                              // 商户地址
  latitude: number                             // 纬度
  longitude: number                            // 经度
  region_id: string | number                            // 区域ID
  is_open: boolean                             // 是否营业
  tags: string[]                               // 商户标签（如：快餐、川菜）
  monthly_sales: number                        // 近30天订单量
  trust_score: number                          // 信誉分
  avg_prep_minutes: number                     // 平均出餐时间（分钟）
  business_license_image_url?: string          // 营业执照图片
  food_permit_url?: string                     // 食品经营许可证
  business_hours?: {                           // 营业时间
    day_of_week: number                        // 0=周日, 1=周一, ..., 6=周六
    open_time: string                          // HH:MM
    close_time: string                         // HH:MM
    is_closed: boolean                         // 是否休息
  }[]
  discount_rules?: PublicDiscountRule[]         // 满减规则
  vouchers?: PublicVoucher[]                   // 代金券
  delivery_promotions?: PublicDeliveryPromotion[] // 配送费优惠
}


/**
 * 获取商户详情（消费者端）
 * GET /v1/public/merchants/:id
 * 返回包含标签、营业时间、证照等完整信息
 */
export async function getPublicMerchantDetail(merchantId: string | number): Promise<PublicMerchantDetail> {
  return await request({
    url: `/v1/public/merchants/${merchantId}`,
    method: 'GET',
    useCache: true,
    cacheTTL: 5 * 60 * 1000 // 5分钟缓存
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
  member_price?: number
  image_url?: string
  category_id: number
  category_name: string
  monthly_sales: number
  prepare_time: number
  tags: string[]
}

/**
 * 菜品列表响应
 */
export interface PublicMerchantDishesResponse {
  categories: PublicDishCategory[]
  dishes: PublicDish[]
}

/**
 * 获取商户菜品列表（消费者端）
 * GET /v1/public/merchants/:id/dishes
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
}

/**
 * 套餐列表响应
 */
export interface PublicMerchantCombosResponse {
  combos: PublicCombo[]
}

/**
 * 获取商户套餐列表（消费者端）
 * GET /v1/public/merchants/:id/combos
 */
export async function getPublicMerchantCombos(merchantId: number): Promise<PublicMerchantCombosResponse> {
  return await request({
    url: `/v1/public/merchants/${merchantId}/combos`,
    method: 'GET',
    useCache: true,
    cacheTTL: 5 * 60 * 1000
  })
}

// ==================== 商户基础管理适配器 ====================

/**
 * 商户基础管理数据适配器
 * 处理前端展示数据和后端API数据之间的转换
 */
export class MerchantManagementAdapter {

  /**
   * 格式化商户状态显示文本
   */
  static formatMerchantStatus(status: string): string {
    const statusMap: Record<string, string> = {
      'active': '正常营业',
      'inactive': '暂停营业',
      'suspended': '已暂停',
      'pending': '待审核'
    }
    return statusMap[status] || status
  }

  /**
   * 格式化营业状态显示文本
   */
  static formatBusinessStatus(isOpen: boolean): string {
    return isOpen ? '营业中' : '已打烊'
  }

  /**
   * 格式化星期显示文本
   */
  static formatDayOfWeek(dayOfWeek: number): string {
    const dayNames = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']
    return dayNames[dayOfWeek] || `星期${dayOfWeek}`
  }

  /**
   * 生成默认营业时间（周一到周日 9:00-21:00）
   */
  static generateDefaultBusinessHours(): BusinessHourItem[] {
    const defaultHours: BusinessHourItem[] = []

    for (let i = 0; i < 7; i++) {
      defaultHours.push({
        day_of_week: i,
        open_time: '09:00',
        close_time: '21:00',
        is_closed: false
      })
    }

    return defaultHours
  }

  /**
   * 验证营业时间数据
   */
  static validateBusinessHours(hours: BusinessHourItem[]): {
    isValid: boolean
    errors: string[]
  } {
    const errors: string[] = []

    if (!hours || hours.length === 0) {
      errors.push('营业时间不能为空')
      return { isValid: false, errors }
    }

    if (hours.length > 7) {
      errors.push('营业时间最多7天')
    }

    hours.forEach((hour, index) => {
      if (!hour.open_time || !hour.close_time) {
        errors.push(`第${index + 1}项营业时间缺少开始或结束时间`)
      }

      if (hour.open_time && hour.close_time) {
        const openTime = new Date(`2000-01-01 ${hour.open_time}:00`)
        const closeTime = new Date(`2000-01-01 ${hour.close_time}:00`)

        if (openTime >= closeTime) {
          errors.push(`第${index + 1}项营业时间：开始时间不能晚于或等于结束时间`)
        }
      }

      if (hour.day_of_week !== undefined && (hour.day_of_week < 0 || hour.day_of_week > 6)) {
        errors.push(`第${index + 1}项营业时间：星期数值无效`)
      }
    })

    return {
      isValid: errors.length === 0,
      errors
    }
  }

  /**
   * 检查当前是否在营业时间内
   */
  static isCurrentlyOpen(businessHours: BusinessHourResponse[]): boolean {
    const now = new Date()
    const currentDay = now.getDay() // 0=周日, 1=周一, ..., 6=周六
    const currentTime = now.toTimeString().slice(0, 5) // HH:MM格式

    const todayHours = businessHours.find(hour => hour.day_of_week === currentDay)

    if (!todayHours || todayHours.is_closed) {
      return false
    }

    return currentTime >= todayHours.open_time && currentTime <= todayHours.close_time
  }

  /**
   * 获取下次营业时间
   */
  static getNextOpenTime(businessHours: BusinessHourResponse[]): string | null {
    const now = new Date()
    const currentDay = now.getDay()
    const currentTime = now.toTimeString().slice(0, 5)

    // 检查今天剩余时间
    const todayHours = businessHours.find(hour => hour.day_of_week === currentDay)
    if (todayHours && !todayHours.is_closed && currentTime < todayHours.open_time) {
      return `今天 ${todayHours.open_time}`
    }

    // 检查未来7天
    for (let i = 1; i <= 7; i++) {
      const checkDay = (currentDay + i) % 7
      const dayHours = businessHours.find(hour => hour.day_of_week === checkDay)

      if (dayHours && !dayHours.is_closed) {
        const dayName = this.formatDayOfWeek(checkDay)
        return `${dayName} ${dayHours.open_time}`
      }
    }

    return null
  }
}

// ==================== 导出默认服务 ====================


export default MerchantManagementService

export const getMerchants = searchMerchants

/**
 * 获取商户订单列表
 */
export function getMerchantOrders(merchantId: string, status?: string): Promise<MerchantOrderDTO[]> {
  return request({
    url: `/merchant/${merchantId}/orders`,
    method: 'GET',
    data: { status }
  })
}

/**
 * 获取商户菜品列表响应类型
 */
export interface MerchantDishesResponse {
  categories: Array<{
    id: number
    name: string
    sort_order: number
  }>
  dishes: DishDTO[]
}

/**
 * 获取商户菜品列表
 */
export function getMerchantDishes(merchantId: string): Promise<MerchantDishesResponse> {
  return request({
    url: `/v1/public/merchants/${merchantId}/dishes`,
    method: 'GET'
  })
}

/**
 * 接单
 */
export function acceptOrder(merchantId: string, orderId: string): Promise<void> {
  return request({
    url: `/merchant/orders/${orderId}/accept`,
    method: 'POST'
  })
}

/**
 * 拒单
 */
export function rejectOrder(orderId: string, reason: string): Promise<void> {
  return request({
    url: `/merchant/orders/${orderId}/reject`,
    method: 'POST',
    data: { reason }
  })
}

/**
 * 出餐
 */
export function readyOrder(orderId: string): Promise<void> {
  return request({
    url: `/merchant/orders/${orderId}/ready`,
    method: 'POST'
  })
}

/**
 * 更新/新增菜品
 */
export function upsertDish(merchantId: string, dish: any): Promise<void> {
  return request({
    url: `/merchant/${merchantId}/dishes`,
    method: 'POST',
    data: dish
  })
}
