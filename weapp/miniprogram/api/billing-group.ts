/**
 * 账单组相关 API
 */

import { request } from '../utils/request'

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

export interface BillingGroupOrderDTO {
  id: number
  billing_group_id: number
  order_id: number
  amount: number
  status: string
  created_at: string
  updated_at?: string
}

export async function createBillingGroup(dining_session_id: number): Promise<BillingGroupDTO> {
  return request({
    url: '/v1/billing-groups',
    method: 'POST',
    data: { dining_session_id }
  })
}

export async function joinBillingGroup(id: number): Promise<BillingGroupDTO> {
  return request({
    url: `/v1/billing-groups/${id}/join`,
    method: 'POST'
  })
}

export async function listBillingGroups(dining_session_id: number): Promise<{ billing_groups: BillingGroupDTO[] }> {
  return request({
    url: '/v1/billing-groups',
    method: 'GET',
    data: { dining_session_id }
  })
}

export async function listBillingGroupOrders(id: number): Promise<{ orders: BillingGroupOrderDTO[] }> {
  return request({
    url: `/v1/billing-groups/${id}/orders`,
    method: 'GET'
  })
}

export default {
  createBillingGroup,
  joinBillingGroup,
  listBillingGroups,
  listBillingGroupOrders
}
