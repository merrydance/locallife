import { request } from '../utils/request'
import { normalizePaginatedResult, type PaginatedListResult, type PaginationEnvelope } from './types'

export interface OperatorPendingDispatchSummary {
  region_id: number
  region_name: string
  pending_total: number
  timeout_over_3m_total: number
  oldest_wait_seconds: number
  latest_refresh_at: string
}

export interface OperatorPendingDispatchItem {
  delivery_id: number
  order_id: number
  order_no: string
  merchant_id: number
  merchant_name: string
  region_id: number
  region_name: string
  wait_seconds: number
  delivery_fee: number
  expected_pickup_at?: string
  is_timeout_over_3m: boolean
}

export interface OperatorPendingDispatchListResult extends PaginatedListResult<OperatorPendingDispatchItem> {
  items: OperatorPendingDispatchItem[]
}

type OperatorPendingDispatchListResponse = PaginationEnvelope & {
  items?: OperatorPendingDispatchItem[]
}

export class OperatorDispatchMonitorService {
  async getSummary(regionId: number) {
    return request<OperatorPendingDispatchSummary>({
      url: `/v1/operator/regions/${regionId}/delivery-pool/summary`,
      method: 'GET'
    })
  }

  async getPendingDispatches(regionId: number, params: { page_id?: number, page_size?: number }): Promise<OperatorPendingDispatchListResult> {
    const pageId = params.page_id ?? 1
    const pageSize = params.page_size ?? 20

    const response = await request<OperatorPendingDispatchListResponse>({
      url: `/v1/operator/regions/${regionId}/delivery-pool`,
      method: 'GET',
      data: {
        page: pageId,
        limit: pageSize
      }
    })

    const items = Array.isArray(response?.items) ? response.items : []
    return {
      ...normalizePaginatedResult(items, response, { page: pageId, pageSize }),
      items
    }
  }
}

export const operatorDispatchMonitorService = new OperatorDispatchMonitorService()