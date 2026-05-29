import { request } from '../../../utils/request'

export interface PlatformAlertItem {
  id: number
  alert_type: string
  level: string
  title: string
  message: string
  related_id: number
  related_type: string
  extra?: Record<string, unknown>
  timestamp: string
}

export interface ListPlatformAlertsResponse {
  alerts: PlatformAlertItem[]
  total: number
  page_id: number
  page_size: number
  has_more: boolean
}

export class PlatformAlertsService {
  async listPlatformAlerts(params: { page_id?: number, page_size?: number }): Promise<ListPlatformAlertsResponse> {
    return request<ListPlatformAlertsResponse>({
      url: '/v1/platform/alerts',
      method: 'GET',
      data: {
        page_id: params.page_id ?? 1,
        page_size: params.page_size ?? 10
      }
    })
  }
}

export const platformAlertsService = new PlatformAlertsService()
