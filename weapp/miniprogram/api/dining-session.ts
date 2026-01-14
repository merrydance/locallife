/**
 * 用餐会话相关 API
 * 覆盖预检、开台、会话内下单（堂食/预订到店）
 */

import { request } from '../utils/request'
import type { OrderItemRequest, OrderResponse, OrderType } from './order'

export type DiningSessionStatus = 'open' | 'closed'

export interface DiningSessionDTO {
  id: number
  merchant_id: number
  table_id: number
  reservation_id?: number
  user_id: number
  active_order_id?: number
  status: DiningSessionStatus
  opened_at: string
  closed_at?: string
  created_at: string
  updated_at?: string
}

export interface DiningSessionPrecheckResponse {
  table_id: number
  reserved: boolean
  reservation_id?: number
  is_reservation_owner: boolean
  payment_mode?: string
  paid_amount?: number
  order_id?: number
  order_status?: string
  order_fulfillment_status?: string
}

export interface OpenDiningSessionRequest {
  table_id: number
  reservation_id?: number
}

export interface OpenDiningSessionResponse {
  session: DiningSessionDTO
  cart_id?: number
  imported_items: number
}

export interface CreateDiningOrderRequest {
  merchant_id: number
  table_id: number
  items: OrderItemRequest[]
  notes?: string
  reservation_id?: number
  order_type?: OrderType
}

/** 预检桌台预订占用 */
export async function precheckDiningSession(tableId: number): Promise<DiningSessionPrecheckResponse> {
  return request({
    url: '/v1/dining-sessions/precheck',
    method: 'GET',
    data: { table_id: tableId }
  })
}

/** 开启用餐会话（若已存在开放会话，后端会直接返回） */
export async function openDiningSession(data: OpenDiningSessionRequest): Promise<OpenDiningSessionResponse> {
  return request({
    url: '/v1/dining-sessions/open',
    method: 'POST',
    data
  })
}

/** 基于用餐会话创建堂食订单（占位，调用通用订单创建接口） */
export async function createDiningOrder(payload: CreateDiningOrderRequest): Promise<OrderResponse> {
  const { merchant_id, table_id, reservation_id, items, notes, order_type = 'dine_in' } = payload
  return request({
    url: '/v1/orders',
    method: 'POST',
    data: {
      merchant_id,
      table_id,
      reservation_id,
      order_type,
      items,
      notes
    }
  })
}

export default {
  precheckDiningSession,
  openDiningSession,
  createDiningOrder
}
