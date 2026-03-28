/**
 * 预订系统接口
 * 包含创建、查询、取消、确认预订及加菜功能
 * 对应后端 /v1/reservations 路由组
 */

import { request, API_BASE } from '../utils/request'

interface RoomTagItem {
  name?: string
}

interface RoomDetailApiResponse {
  id: number
  merchant_id?: number
  room_no?: string
  capacity?: number
  minimum_spend?: number
  images?: string[]
  tags?: Array<RoomTagItem | string>
  description?: string
}

// ==================== 数据类型定义 ====================

/**
 * 预订状态枚举
 */
export type ReservationStatus =
  | 'pending'     // 待支付
  | 'paid'        // 已支付
  | 'confirmed'   // 已确认
  | 'checked_in'  // 已签到
  | 'completed'   // 已完成
  | 'cancelled'   // 已取消
  | 'expired'     // 已过期
  | 'no_show'     // 未到店

// 用餐会话 DTO（与 /v1/dining-sessions 接口对齐）
export interface DiningSessionDTO {
  id: number
  merchant_id: number
  table_id: number
  reservation_id?: number
  user_id: number
  active_order_id?: number
  status: 'open' | 'closed'
  opened_at: string
  closed_at?: string
  created_at: string
  updated_at?: string
}

export interface DiningSessionPrecheckResponse {
  table_id: number
  reserved: boolean
  reservation_id?: number
  /** 仅表示当前登录用户是否为该预约本人，不表示商户侧可管理权限 */
  is_reservation_owner: boolean
  payment_mode?: string
  paid_amount?: number
  order_id?: number
  order_status?: string
  order_fulfillment_status?: string
}

export interface BillingGroupDTO {
  id: number
  dining_session_id: number
  status: string
  is_default: boolean
  total_amount: number
  paid_amount: number
  created_at: string
  updated_at?: string
  closed_at?: string
}

/**
 * 支付模式
 */
export type PaymentMode = 'deposit' | 'full'

/**
 * 预订来源
 */
export type ReservationSource = 'online' | 'phone' | 'walkin' | 'merchant'

/**
 * 预订菜品项
 */
export interface ReservationItem {
  id?: number
  dish_id?: number
  combo_id?: number
  quantity: number
  name?: string
  unit_price?: number
  total_price?: number
  type?: 'dish' | 'combo'
  image_url?: string
}

/**
 * 用户创建预订请求
 */
export interface CreateReservationRequest {
  table_id?: number
  date?: string              // YYYY-MM-DD
  time?: string              // HH:MM
  guest_count?: number
  // 兼容旧字段（reservation_time/party_size）
  reservation_time?: string
  party_size?: number
  merchant_id?: number
  contact_name: string
  contact_phone: string
  payment_mode?: PaymentMode
  notes?: string
  items?: ReservationItem[] // 全款模式预点菜品
}

/**
 * 商户代客创建预订请求
 */
export interface MerchantCreateReservationRequest {
  table_id: number
  date: string              // YYYY-MM-DD
  time: string              // HH:MM
  guest_count: number
  contact_name: string
  contact_phone: string
  source?: ReservationSource
  notes?: string
}

/**
 * 商户修改预订请求
 */
export interface UpdateReservationRequest {
  table_id?: number
  date?: string             // YYYY-MM-DD
  time?: string             // HH:MM
  guest_count?: number
  contact_name?: string
  contact_phone?: string
  notes?: string
}

/**
 * 预订详情响应
 */
export interface ReservationResponse {
  id: number
  table_id: number
  table_no?: string
  table_type?: string
  user_id: number
  merchant_id: number
  merchant_name?: string
  merchant_address?: string
  merchant_phone?: string
  reservation_date: string
  reservation_time: string
  guest_count: number
  contact_name: string
  contact_phone: string
  payment_mode: PaymentMode
  deposit_amount: number
  prepaid_amount: number
  refund_deadline: string
  payment_deadline: string
  status: ReservationStatus
  notes?: string
  paid_at?: string
  confirmed_at?: string
  completed_at?: string
  cancelled_at?: string
  cancel_reason?: string
  items?: ReservationItem[]
  created_at: string
  updated_at?: string
}

/**
 * 预订列表查询参数
 */
export interface ReservationListParams {
  page_id: number
  page_size: number
  status?: ReservationStatus
  date?: string  // YYYY-MM-DD
}

export interface ReservationListResponse {
  reservations: ReservationResponse[]
  total: number
  page_id: number
  page_size: number
}

/**
 * 预订统计
 */
export interface ReservationStats {
  pending_count: number
  paid_count: number
  confirmed_count: number
  checked_in_count?: number
  completed_count: number
  cancelled_count: number
  expired_count: number
  no_show_count: number
}

export interface MerchantReservationDishReference {
  reservation_id: number
  reservation_time: string
  table_no?: string
  contact_name?: string
  status: ReservationStatus
  quantity: number
}

export interface MerchantReservationDishSummaryItem {
  type: 'dish' | 'combo'
  dish_id?: number
  combo_id?: number
  name: string
  total_quantity: number
  reservation_count: number
  references: MerchantReservationDishReference[]
}

export interface MerchantReservationDishesSummaryResponse {
  date: string
  items: MerchantReservationDishSummaryItem[]
}

// ==================== 预订服务 ====================

export class ReservationService {

  // ==================== 用户端接口 ====================

  /**
   * 创建预订
   * POST /v1/reservations
   */
  static async createReservation(data: CreateReservationRequest): Promise<ReservationResponse> {
    return await request({
      url: '/v1/reservations',
      method: 'POST',
      data
    })
  }

  /**
   * 获取用户预订列表
   * GET /v1/reservations/me
   */
  static async getUserReservations(params: ReservationListParams): Promise<ReservationListResponse> {
    return await request({
      url: '/v1/reservations/me',
      method: 'GET',
      data: params
    })
  }

  /**
   * 获取预订详情
   * GET /v1/reservations/:id
   */
  static async getReservationDetail(id: number): Promise<ReservationResponse> {
    return await request({
      url: `/v1/reservations/${id}`,
      method: 'GET'
    })
  }

  /**
   * 取消预订
   * POST /v1/reservations/:id/cancel
   */
  static async cancelReservation(id: number, reason?: string): Promise<ReservationResponse> {
    return await request({
      url: `/v1/reservations/${id}/cancel`,
      method: 'POST',
      data: { reason }
    })
  }

  /**
   * 追加菜品
   * POST /v1/reservations/:id/add-dishes
   */
  static async addDishes(id: number, items: ReservationItem[]): Promise<unknown> {
    return await request({
      url: `/v1/reservations/${id}/add-dishes`,
      method: 'POST',
      data: { items }
    })
  }

  /**
   * 预订改菜（差量）
   * POST /v1/reservations/:id/modify-dishes
   */
  static async modifyDishes(id: number, items: ReservationItem[]): Promise<unknown> {
    return await request({
      url: `/v1/reservations/${id}/modify-dishes`,
      method: 'POST',
      data: { items }
    })
  }

  /**
   * 顾客到店签到
   * POST /v1/reservations/:id/checkin
   */
  static async checkIn(id: number): Promise<ReservationResponse> {
    return await request({
      url: `/v1/reservations/${id}/checkin`,
      method: 'POST'
    })
  }

  /**
   * 起菜通知
   * POST /v1/reservations/:id/start-cooking
   */
  static async startCooking(id: number): Promise<ReservationResponse> {
    return await request({
      url: `/v1/reservations/${id}/start-cooking`,
      method: 'POST'
    })
  }

  // ==================== 用餐会话接口（堂食/预订到店） ====================

  /**
   * 桌台预检：是否被预订、预订归属、最近订单
   * GET /v1/dining-sessions/precheck
   */
  static async precheckDiningSession(tableId: number): Promise<DiningSessionPrecheckResponse> {
    return await request({
      url: '/v1/dining-sessions/precheck',
      method: 'GET',
      data: { table_id: tableId }
    })
  }

  /**
   * 开启用餐会话（已存在开放会话时后端直接返回）
   * POST /v1/dining-sessions/open
   */
  static async openDiningSession(params: { table_id: number, reservation_id?: number }): Promise<{ session: DiningSessionDTO, billing_group: BillingGroupDTO, cart_id?: number, imported_items: number }> {
    return await request({
      url: '/v1/dining-sessions/open',
      method: 'POST',
      data: params
    })
  }

  // ==================== 商户端接口 ====================

  /**
   * 商户获取预订列表
   * GET /v1/reservations/merchant
   */
  static async getMerchantReservations(params: ReservationListParams): Promise<ReservationListResponse> {
    return await request({
      url: '/v1/reservations/merchant',
      method: 'GET',
      data: params
    })
  }

  /**
   * 商户获取今日预订
   * GET /v1/reservations/merchant/today
   */
  static async getTodayReservations(): Promise<{ reservations: ReservationResponse[] }> {
    return await request({
      url: '/v1/reservations/merchant/today',
      method: 'GET'
    })
  }

  /**
   * 商户获取预订统计
   * GET /v1/reservations/merchant/stats
   */
  static async getReservationStats(): Promise<ReservationStats> {
    return await request({
      url: '/v1/reservations/merchant/stats',
      method: 'GET'
    })
  }

  /**
   * 商户按天获取预订菜品/套餐汇总
   * GET /v1/reservations/merchant/dishes
   */
  static async getMerchantReservationDishes(date: string): Promise<MerchantReservationDishesSummaryResponse> {
    return await request({
      url: '/v1/reservations/merchant/dishes',
      method: 'GET',
      data: { date }
    })
  }

  /**
   * 商户代客创建预订
   * POST /v1/reservations/merchant/create
   */
  static async merchantCreateReservation(data: MerchantCreateReservationRequest): Promise<ReservationResponse> {
    return await request({
      url: '/v1/reservations/merchant/create',
      method: 'POST',
      data
    })
  }

  /**
   * 商户修改预订
   * PUT /v1/reservations/:id/update
   */
  static async updateReservation(id: number, data: UpdateReservationRequest): Promise<ReservationResponse> {
    return await request({
      url: `/v1/reservations/${id}/update`,
      method: 'PUT',
      data
    })
  }

  /**
   * 商户确认预订
   * POST /v1/reservations/:id/confirm
   */
  static async confirmReservation(id: number): Promise<ReservationResponse> {
    return await request({
      url: `/v1/reservations/${id}/confirm`,
      method: 'POST'
    })
  }

  /**
   * 商户完成预订
   * POST /v1/reservations/:id/complete
   */
  static async completeReservation(id: number): Promise<ReservationResponse> {
    return await request({
      url: `/v1/reservations/${id}/complete`,
      method: 'POST'
    })
  }

  /**
   * 商户标记未到店
   * POST /v1/reservations/:id/no-show
   */
  static async markNoShow(id: number): Promise<ReservationResponse> {
    return await request({
      url: `/v1/reservations/${id}/no-show`,
      method: 'POST'
    })
  }
}

// ==================== 便捷导出函数 ====================

// 用户端
export const createReservation = ReservationService.createReservation
export const getUserReservations = ReservationService.getUserReservations
export const getReservationDetail = ReservationService.getReservationDetail
export const cancelReservation = ReservationService.cancelReservation
export const addDishesToReservation = ReservationService.addDishes
export const modifyDishesToReservation = ReservationService.modifyDishes
export const checkInReservation = ReservationService.checkIn
export const startCookingReservation = ReservationService.startCooking
export const precheckDiningSession = ReservationService.precheckDiningSession
export const openDiningSession = ReservationService.openDiningSession

// 商户端
export const getMerchantReservations = ReservationService.getMerchantReservations
export const getTodayReservations = ReservationService.getTodayReservations
export const getReservationStats = ReservationService.getReservationStats
export const getMerchantReservationDishes = ReservationService.getMerchantReservationDishes
export const merchantCreateReservation = ReservationService.merchantCreateReservation
export const updateReservation = ReservationService.updateReservation
export const confirmReservationByMerchant = ReservationService.confirmReservation
export const completeReservationByMerchant = ReservationService.completeReservation
export const markReservationNoShow = ReservationService.markNoShow

// ==================== 包间相关接口 ====================

/**
 * 包间信息（用于预订详情页）
 */
export interface Room {
  id: number
  merchant_id: number       // 商户ID（全款模式需要）
  name: string
  capacity: number
  min_spend: number         // 最低消费（分）
  deposit: number           // 预定定金（分）
  images: string[]          // 图片列表
  facilities: string[]      // 设施服务
  description: string
}

/**
 * 获取包间详情（用于预订详情页）
 * 从后端 /v1/rooms/:id 获取数据并映射字段
 */
export async function getRoomDetail(id: string): Promise<Room> {
  const response = await request({
    url: `/v1/rooms/${id}`,
    method: 'GET'
  }) as RoomDetailApiResponse

  // 映射后端 RoomDetailResponse 到页面 Room 格式
  // 图片URL已经是完整路径或以/开头，直接使用
  const processImageUrl = (url: string) => {
    if (!url) return ''
    if (url.startsWith('http')) return url
    if (url.startsWith('/')) return `${API_BASE}${url}`
    return url
  }

  return {
    id: response.id,
    merchant_id: response.merchant_id || 0,
    name: response.room_no || '包间',
    capacity: response.capacity || 0,
    min_spend: response.minimum_spend || 0,
    // 定金逻辑与后端一致：有最低消费则定金=最低消费，否则默认100元
    deposit: (response.minimum_spend || 0) > 0 ? (response.minimum_spend || 0) : 10000,
    images: (response.images || []).map((url: string) => processImageUrl(url)),
    facilities: (response.tags || []).map((tag) =>
      typeof tag === 'string' ? tag : tag.name || ''
    ), // 标签作为设施
    description: response.description || ''
  }
}

export default ReservationService