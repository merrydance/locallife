import { request } from '../utils/request'

export type MerchantComplaintState = 'PENDING_RESPONSE' | 'PROCESSING' | 'PROCESSED'

export interface MerchantComplaintItem {
  id: number
  complaint_id: string
  complaint_time: string
  payer_openid?: string
  complaint_detail: string
  complaint_state: MerchantComplaintState
  transaction_id?: string
  out_trade_no?: string
  amount: number
  response_content?: string
  responded_at?: string
  completed_at?: string
  last_synced_at: string
  wxpay_update_time?: string
  created_at: string
  updated_at: string
}

export interface MerchantComplaintListResponse {
  complaints: MerchantComplaintItem[]
  page: number
  limit: number
}

export interface ListMerchantComplaintsParams {
  state?: MerchantComplaintState
  page?: number
  limit?: number
}

export interface RespondMerchantComplaintRequest {
  response_content: string
  jump_url?: string
}

export interface MerchantComplaintActionAck {
  message: string
}

export function listMerchantComplaints(params: ListMerchantComplaintsParams = {}) {
  return request<MerchantComplaintListResponse>({
    url: '/v1/merchant/complaints',
    method: 'GET',
    data: params
  })
}

export function getMerchantComplaintDetail(complaintId: string) {
  return request<MerchantComplaintItem>({
    url: `/v1/merchant/complaints/${encodeURIComponent(complaintId)}`,
    method: 'GET'
  })
}

export function respondMerchantComplaint(complaintId: string, data: RespondMerchantComplaintRequest) {
  return request<MerchantComplaintItem | MerchantComplaintActionAck>({
    url: `/v1/merchant/complaints/${encodeURIComponent(complaintId)}/response`,
    method: 'POST',
    data
  })
}

export function completeMerchantComplaint(complaintId: string) {
  return request<MerchantComplaintItem | MerchantComplaintActionAck>({
    url: `/v1/merchant/complaints/${encodeURIComponent(complaintId)}/complete`,
    method: 'POST'
  })
}