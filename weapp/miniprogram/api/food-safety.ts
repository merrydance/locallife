import { request } from '../utils/request'

export type FoodSafetyIncidentType = 'foreign-object' | 'contamination' | 'expired'

export interface ReportFoodSafetyRequest extends Record<string, unknown> {
  merchant_id: number
  order_id: number
  incident_type: FoodSafetyIncidentType
  description: string
  severity_level: number
}

export interface ReportFoodSafetyResponse {
  incident_id: number
  case_id?: number
  merchant_suspended: boolean
  suspend_duration?: number
  message: string
}

export async function reportFoodSafety(data: ReportFoodSafetyRequest): Promise<ReportFoodSafetyResponse> {
  return request<ReportFoodSafetyResponse>({
    url: '/v1/food-safety/report',
    method: 'POST',
    data
  })
}