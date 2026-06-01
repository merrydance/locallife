import { request } from '../utils/request'
import { normalizePaginatedResult, type PaginatedListResult, type PaginationEnvelope } from './types'

export type WantedMerchantVoteResult =
  | 'created'
  | 'voted'
  | 'already_voted'
  | 'found_in_rank'
  | 'merchant_available'

export interface WantedMerchantItem {
  id: number
  region_id: number
  name: string
  address?: string
  latitude?: number
  longitude?: number
  source: 'manual' | 'map'
  want_count: number
  rank: number
  has_voted: boolean
  last_voted_at?: string
  created_at: string
  updated_at: string
}

export interface WantedMerchantListResult extends PaginatedListResult<WantedMerchantItem> {
  items: WantedMerchantItem[]
}

interface WantedMerchantListEnvelope extends PaginationEnvelope {
  items?: WantedMerchantItem[]
}

export interface SubmitWantedMerchantParams {
  region_id: number
  source?: 'manual' | 'map'
  name: string
  address?: string
  latitude?: number
  longitude?: number
}

export interface WantedMerchantVoteResponse {
  result: WantedMerchantVoteResult
  wanted_merchant_id?: number
  merchant_id?: number
  rank?: number
  want_count?: number
}

export async function listWantedMerchants(params: {
  region_id: number
  page_id?: number
  page_size?: number
}): Promise<WantedMerchantListResult> {
  const page = params.page_id || 1
  const pageSize = params.page_size || 50
  const response = await request<WantedMerchantListEnvelope>({
    url: '/v1/wanted-merchants',
    method: 'GET',
    data: {
      region_id: params.region_id,
      page_id: page,
      page_size: pageSize
    }
  })
  const items = response.items || []
  return {
    ...normalizePaginatedResult(items, response, { page, pageSize }),
    items
  }
}

export function submitWantedMerchant(params: SubmitWantedMerchantParams): Promise<WantedMerchantVoteResponse> {
  return request<WantedMerchantVoteResponse>({
    url: '/v1/wanted-merchants/votes',
    method: 'POST',
    data: params
  })
}

export function voteWantedMerchant(params: {
  id: number
  region_id: number
}): Promise<WantedMerchantVoteResponse> {
  return request<WantedMerchantVoteResponse>({
    url: `/v1/wanted-merchants/${params.id}/votes`,
    method: 'POST',
    data: {
      region_id: params.region_id
    }
  })
}
