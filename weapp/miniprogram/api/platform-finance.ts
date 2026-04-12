import { request } from '../utils/request'

export interface PlatformSubMerchantLimitationRecoverySpecification {
  limitation_case_id?: string
  limitation_reason_type?: string
  limitation_reason?: string
  limitation_reason_describe?: string
  relate_limitations?: string[]
  other_relate_limitations?: string
  recover_way?: string
  recover_way_param?: string
  recover_help_url?: string
  limitation_action_type?: string
  limitation_start_date?: string
  limitation_date?: string
}

export interface PlatformSubMerchantLimitationsResponse {
  mchid: string
  limited_functions?: string[]
  other_limited_functions?: string
  recovery_specifications?: PlatformSubMerchantLimitationRecoverySpecification[]
}

class PlatformFinanceService {
  async getSubMerchantLimitations(subMchID: string): Promise<PlatformSubMerchantLimitationsResponse> {
    const normalizedSubMchID = String(subMchID || '').trim()
    if (!normalizedSubMchID) {
      throw new Error('sub_mch_id is required')
    }

    return request({
      url: `/v1/platform/finance/wechat-ecommerce/merchant-limitations/${encodeURIComponent(normalizedSubMchID)}`,
      method: 'GET'
    })
  }
}

export const platformFinanceService = new PlatformFinanceService()