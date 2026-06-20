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

export class MerchantPackagingService {
  static async getSettings(): Promise<MerchantPackagingSettingsResponse> {
    return request<MerchantPackagingSettingsResponse>({
      url: '/v1/merchant/packaging-settings',
      method: 'GET'
    })
  }

  static async updateSettings(
    data: UpsertMerchantPackagingSettingsRequest
  ): Promise<MerchantPackagingSettingsResponse> {
    return request<MerchantPackagingSettingsResponse>({
      url: '/v1/merchant/packaging-settings',
      method: 'PUT',
      data
    })
  }

  static async listOptions(): Promise<MerchantPackagingOptionResponse[]> {
    const response = await request<MerchantPackagingOptionListResponse>({
      url: '/v1/merchant/packaging-options',
      method: 'GET'
    })
    return response.options || []
  }

  static async createOption(
    data: UpsertMerchantPackagingOptionRequest
  ): Promise<MerchantPackagingOptionResponse> {
    return request<MerchantPackagingOptionResponse>({
      url: '/v1/merchant/packaging-options',
      method: 'POST',
      data
    })
  }

  static async updateOption(
    id: number,
    data: UpsertMerchantPackagingOptionRequest
  ): Promise<MerchantPackagingOptionResponse> {
    return request<MerchantPackagingOptionResponse>({
      url: `/v1/merchant/packaging-options/${id}`,
      method: 'PUT',
      data
    })
  }

  static async deleteOption(id: number): Promise<MerchantPackagingOptionResponse> {
    return request<MerchantPackagingOptionResponse>({
      url: `/v1/merchant/packaging-options/${id}`,
      method: 'DELETE'
    })
  }
}
