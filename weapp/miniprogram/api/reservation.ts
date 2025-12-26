/**
 * 预订系统接口
 * 包含创建、查询、取消、确认预订及加菜功能
 */

import { request } from '../utils/request'

// ==================== 数据类型定义 ====================

/**
 * 预订状态枚举
 */
export type ReservationStatus =
  | 'pending'     // 待确认
  | 'confirmed'   // 已确认
  | 'completed'   // 已完成
  | 'cancelled'   // 已取消
  | 'no_show'     // 未到店

/**
 * 预订菜品项
 */
export interface ReservationItem {
  dish_id: number
  quantity: number
  name?: string
  price?: number
  image_url?: string
}

/**
 * 创建预订请求
 */
export interface CreateReservationRequest {
  merchant_id: number
  reservation_time: string // YYYY-MM-DD HH:mm:ss
  party_size: number
  contact_name: string
  contact_phone: string
  notes?: string
  items?: ReservationItem[]
}

/**
 * 预订详情响应
 */
export interface ReservationResponse {
  id: number
  reservation_no: string
  merchant_id: number
  merchant_name: string
  merchant_address?: string
  merchant_image?: string
  user_id: number
  status: ReservationStatus
  reservation_time: string
  party_size: number
  contact_name: string
  contact_phone: string
  notes?: string
  items?: ReservationItem[]
  total_amount?: number
  deposit_amount?: number
  created_at: string
  updated_at: string
  cancel_reason?: string
}

/**
 * 预订列表查询参数
 */
export interface ReservationListParams {
  page_id: number
  page_size: number
  status?: ReservationStatus
}

/**
 * 预订列表响应
 */
export interface ReservationListResponse {
  reservations: ReservationResponse[]
  total: number
}

// ==================== 预订服务 ====================

export class ReservationService {

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
   * 获取预订列表
   * GET /v1/reservations
   */
  static async getReservations(params: ReservationListParams): Promise<ReservationListResponse> {
    return await request({
      url: '/v1/user/reservations',
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
  static async cancelReservation(id: number, reason: string): Promise<ReservationResponse> {
    return await request({
      url: `/v1/reservations/${id}/cancel`,
      method: 'POST',
      data: { reason }
    })
  }

  /**
   * 添加预订菜品
   * POST /v1/reservations/:id/items
   */
  static async addReservationDishes(id: number, items: ReservationItem[]): Promise<ReservationResponse> {
    return await request({
      url: `/v1/reservations/${id}/items`,
      method: 'POST',
      data: { items }
    })
  }

  // ==================== 商户端接口 ====================

  /**
   * 商户确认预订
   * POST /v1/merchant/reservations/:id/confirm
   */
  static async confirmReservation(id: number): Promise<ReservationResponse> {
    return await request({
      url: `/v1/merchant/reservations/${id}/confirm`,
      method: 'POST'
    })
  }

  /**
   * 商户拒绝预订
   * POST /v1/merchant/reservations/:id/reject
   */
  static async rejectReservation(id: number, reason: string): Promise<ReservationResponse> {
    return await request({
      url: `/v1/merchant/reservations/${id}/reject`,
      method: 'POST',
      data: { reason }
    })
  }

  /**
   * 商户标记未到店
   * POST /v1/merchant/reservations/:id/no-show
   */
  static async markNoShow(id: number): Promise<ReservationResponse> {
    return await request({
      url: `/v1/merchant/reservations/${id}/no-show`,
      method: 'POST'
    })
  }

  /**
   * 商户完成预订
   * POST /v1/merchant/reservations/:id/complete
   */
  static async completeReservation(id: number): Promise<ReservationResponse> {
    return await request({
      url: `/v1/merchant/reservations/${id}/complete`,
      method: 'POST'
    })
  }
}

export default ReservationService