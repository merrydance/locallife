/**
 * 用餐会话相关 API
 * 覆盖预检、开台、会话内下单（堂食/预订到店）
 */

import { request } from '../utils/request'
import type { OrderItemRequest, OrderResponse, OrderType } from './order'
import type {
  ScanTableCategoryInfo,
  ScanTableComboInfo,
  ScanTableMerchantInfo,
  ScanTablePromotionInfo,
  ScanTableTableInfo
} from './table'

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
  /** 仅表示当前登录用户是否为该预约本人，不表示商户侧可管理权限 */
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
  table_code?: string
}

export interface OpenDiningSessionResponse {
  session: DiningSessionDTO
  billing_group: BillingGroupDTO
  cart_id?: number
  imported_items: number
}

export interface TransferDiningSessionRequest {
  to_table_id: number
  table_code?: string
  reason?: string
}

export interface TransferDiningSessionResponse {
  session: DiningSessionDTO
  from_table: Record<string, unknown>
  to_table: Record<string, unknown>
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

export interface DiningSessionEntryCapabilities {
  requires_table_code: boolean
  transfer_requires_table_code: boolean
  can_order: boolean
  can_transfer: boolean
  supports_takeout_jump: boolean
  supports_reservation_jump: boolean
  supports_service_call: boolean
}

export interface DiningSessionEntrySessionSummary {
  session: DiningSessionDTO
  billing_group: BillingGroupDTO
  table_no: string
}

export interface DiningSessionEntryResponse {
  action: 'open_session' | 'resume_session' | 'transfer_session' | 'blocked'
  blocked_reason?: string
  merchant: ScanTableMerchantInfo
  table: ScanTableTableInfo
  precheck: DiningSessionPrecheckResponse
  active_session?: DiningSessionEntrySessionSummary
  transfer_session?: DiningSessionEntrySessionSummary
  capabilities: DiningSessionEntryCapabilities
}

export interface DiningSessionMenuResponse {
  session: DiningSessionDTO
  billing_group: BillingGroupDTO
  merchant: ScanTableMerchantInfo
  table: ScanTableTableInfo
  categories: ScanTableCategoryInfo[]
  combos: ScanTableComboInfo[]
  promotions: ScanTablePromotionInfo[]
}

export interface CreateDiningOrderRequest {
  merchant_id: number
  table_id: number
  items: OrderItemRequest[]
  notes?: string
  reservation_id?: number
  order_type?: OrderType
  billing_group_id?: number
}

/** 预检桌台预订占用 */
export async function precheckDiningSession(tableId: number): Promise<DiningSessionPrecheckResponse> {
  return request({
    url: '/v1/dining-sessions/precheck',
    method: 'GET',
    data: { table_id: tableId }
  })
}

export async function getDiningSessionEntry(params: {
  merchant_id?: number
  table_no?: string
  table_id?: number
}): Promise<DiningSessionEntryResponse> {
  return request({
    url: '/v1/dining-sessions/entry',
    method: 'GET',
    data: params
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

export async function getDiningSessionMenu(sessionId: number): Promise<DiningSessionMenuResponse> {
  return request({
    url: `/v1/dining-sessions/${sessionId}/menu`,
    method: 'GET'
  })
}

/** 转台（换桌） */
export async function transferDiningSessionTable(sessionId: number, data: TransferDiningSessionRequest): Promise<TransferDiningSessionResponse> {
  return request({
    url: `/v1/dining-sessions/${sessionId}/transfer-table`,
    method: 'POST',
    data
  })
}

export async function checkoutDiningSession(sessionId: number): Promise<DiningSessionDTO> {
  return request({
    url: `/v1/dining-sessions/${sessionId}/checkout`,
    method: 'POST'
  })
}

/** 基于用餐会话创建堂食订单（占位，调用通用订单创建接口） */
export async function createDiningOrder(payload: CreateDiningOrderRequest): Promise<OrderResponse> {
  const { merchant_id, table_id, reservation_id, items, notes, order_type = 'dine_in', billing_group_id } = payload
  return request({
    url: '/v1/orders',
    method: 'POST',
    data: {
      merchant_id,
      table_id,
      reservation_id,
      order_type,
      items,
      notes,
      billing_group_id
    }
  })
}

export default {
  getDiningSessionEntry,
  getDiningSessionMenu,
  precheckDiningSession,
  openDiningSession,
  transferDiningSessionTable,
  checkoutDiningSession,
  createDiningOrder
}
