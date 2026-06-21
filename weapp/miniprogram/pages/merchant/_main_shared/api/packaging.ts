import { request } from '../../../../utils/request'

export type MerchantPackagingOrderType = 'takeout' | 'takeaway'

export interface MerchantPackagingSettingsResponse {
  merchant_id: number
  enabled: boolean
  required: boolean
  applicable_order_types: MerchantPackagingOrderType[]
  default_option_id?: number
}

export interface UpsertMerchantPackagingSettingsRequest extends Record<string, unknown> {
  enabled: boolean
  required: boolean
  applicable_order_types: MerchantPackagingOrderType[]
  default_option_id?: number | null
}

export interface MerchantPackagingOptionResponse {
  id: number
  merchant_id: number
  name: string
  description: string
  price: number
  is_enabled: boolean
  sort_order: number
}

export interface MerchantPackagingOptionListResponse {
  options: MerchantPackagingOptionResponse[]
  total: number
  page: number
  limit: number
  total_pages: number
}

export interface UpsertMerchantPackagingOptionRequest extends Record<string, unknown> {
  name: string
  description: string
  price: number
  is_enabled: boolean
  sort_order: number
}

export interface MerchantPackagingRequestContext {
  merchantId?: number
}

function buildMerchantPackagingRequestContext(context?: MerchantPackagingRequestContext) {
  const merchantId = Number(context?.merchantId || 0)
  if (merchantId <= 0) {
    return {
      header: undefined,
      query: undefined
    }
  }

  return {
    header: { 'X-Merchant-ID': String(merchantId) },
    query: { merchant_id: merchantId }
  }
}

export class MerchantPackagingService {
  static async getSettings(context?: MerchantPackagingRequestContext): Promise<MerchantPackagingSettingsResponse> {
    const requestContext = buildMerchantPackagingRequestContext(context)
    return request<MerchantPackagingSettingsResponse>({
      url: '/v1/merchant/packaging-settings',
      method: 'GET',
      data: requestContext.query,
      header: requestContext.header
    })
  }

  static async updateSettings(
    data: UpsertMerchantPackagingSettingsRequest,
    context?: MerchantPackagingRequestContext
  ): Promise<MerchantPackagingSettingsResponse> {
    const requestContext = buildMerchantPackagingRequestContext(context)
    return request<MerchantPackagingSettingsResponse>({
      url: '/v1/merchant/packaging-settings',
      method: 'PUT',
      data,
      header: requestContext.header
    })
  }

  static async listOptions(context?: MerchantPackagingRequestContext): Promise<MerchantPackagingOptionResponse[]> {
    const requestContext = buildMerchantPackagingRequestContext(context)
    const response = await request<MerchantPackagingOptionListResponse>({
      url: '/v1/merchant/packaging-options',
      method: 'GET',
      data: requestContext.query,
      header: requestContext.header
    })
    return response.options || []
  }

  static async createOption(
    data: UpsertMerchantPackagingOptionRequest,
    context?: MerchantPackagingRequestContext
  ): Promise<MerchantPackagingOptionResponse> {
    const requestContext = buildMerchantPackagingRequestContext(context)
    return request<MerchantPackagingOptionResponse>({
      url: '/v1/merchant/packaging-options',
      method: 'POST',
      data,
      header: requestContext.header
    })
  }

  static async updateOption(
    id: number,
    data: UpsertMerchantPackagingOptionRequest,
    context?: MerchantPackagingRequestContext
  ): Promise<MerchantPackagingOptionResponse> {
    const requestContext = buildMerchantPackagingRequestContext(context)
    return request<MerchantPackagingOptionResponse>({
      url: `/v1/merchant/packaging-options/${id}`,
      method: 'PUT',
      data,
      header: requestContext.header
    })
  }

  static async deleteOption(
    id: number,
    context?: MerchantPackagingRequestContext
  ): Promise<MerchantPackagingOptionResponse> {
    const requestContext = buildMerchantPackagingRequestContext(context)
    return request<MerchantPackagingOptionResponse>({
      url: `/v1/merchant/packaging-options/${id}`,
      method: 'DELETE',
      header: requestContext.header
    })
  }
}
