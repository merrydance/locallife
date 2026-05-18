import { request } from '../utils/request'

export type RiderWorkbenchSectionKey =
  | 'rider_status'
  | 'current_deliveries'
  | 'order_pool'
  | 'today'
  | 'income'
  | 'deposit'
  | 'claims'
  | 'notifications'

export interface RiderWorkbenchSectionStatus {
  section: RiderWorkbenchSectionKey
  available: boolean
  message?: string
}

export interface RiderWorkbenchRiderStatus {
  status: string
  is_online: boolean
  online_status: 'offline' | 'online' | 'delivering'
  active_deliveries: number
  current_region_id: number
  required_deposit: number
  current_longitude?: number
  current_latitude?: number
  location_updated_at?: string
  can_go_online: boolean
  can_go_offline: boolean
  online_block_reason?: string
  settlement_account?: BaofuSettlementReadiness
}

export interface BaofuSettlementReadiness {
  state: string
  label: string
  payment_ready: boolean
}

export interface RiderWorkbenchDeliveryItem {
  id: number
  order_id: number
  status: 'pending' | 'assigned' | 'picking' | 'picked' | 'delivering' | 'delivered' | 'completed' | 'cancelled' | 'exception'
  delivery_fee: number
  rider_earnings: number
  pickup_address: string
  delivery_address: string
  estimated_pickup_at?: string
  estimated_delivery_at?: string
  picked_at?: string
  delivered_at?: string
  created_at: string
}

export interface RiderWorkbenchCurrentDeliveries {
  active_count: number
  items: RiderWorkbenchDeliveryItem[]
}

export interface RiderWorkbenchOrderPool {
  available_count: number
}

export interface RiderWorkbenchToday {
  date: string
  completed_deliveries: number
}

export interface RiderWorkbenchIncome {
  total_deliveries: number
  total_rider_income: number
  total_delivery_fee: number
  pending_rider_amount: number
  processing_rider_amount: number
  failed_count: number
}

export interface RiderWorkbenchDeposit {
  total_deposit: number
  frozen_deposit: number
  delivery_frozen_deposit: number
  deposit_refund_processing_amount: number
  available_deposit: number
  threshold_amount: number
}

export interface RiderWorkbenchClaims {
  pending_action_count: number
}

export interface RiderWorkbenchNotifications {
  unread_count: number
}

export interface RiderWorkbenchSummaryResponse {
  rider_status: RiderWorkbenchRiderStatus
  current_deliveries: RiderWorkbenchCurrentDeliveries
  order_pool: RiderWorkbenchOrderPool
  today: RiderWorkbenchToday
  income: RiderWorkbenchIncome
  deposit: RiderWorkbenchDeposit
  claims: RiderWorkbenchClaims
  notifications: RiderWorkbenchNotifications
  sections: RiderWorkbenchSectionStatus[]
}

export class RiderWorkbenchService {
  static async getSummary(): Promise<RiderWorkbenchSummaryResponse> {
    return await request({
      url: '/v1/rider/workbench/summary',
      method: 'GET'
    })
  }
}

export default RiderWorkbenchService
