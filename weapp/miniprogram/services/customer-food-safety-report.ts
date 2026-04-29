import { reportFoodSafety, type FoodSafetyIncidentType, type ReportFoodSafetyResponse } from '../api/food-safety'
import { getOrderDetail, isFoodSafetyReportableOrder } from '../api/order'

export interface CustomerFoodSafetyOrderView {
  orderId: number
  orderNo: string
  merchantId: number
  merchantName: string
  reportable: boolean
}

export interface SubmitCustomerFoodSafetyReportParams {
  merchantId: number
  orderId: number
  incidentType: FoodSafetyIncidentType
  description: string
  severityLevel: number
}

export async function loadCustomerFoodSafetyOrderView(orderId: number): Promise<CustomerFoodSafetyOrderView> {
  const order = await getOrderDetail(orderId)
  return {
    orderId: order.id,
    orderNo: order.order_no,
    merchantId: order.merchant_id,
    merchantName: order.merchant_name || '商户',
    reportable: isFoodSafetyReportableOrder(order)
  }
}

export async function submitCustomerFoodSafetyReport(params: SubmitCustomerFoodSafetyReportParams): Promise<ReportFoodSafetyResponse> {
  return reportFoodSafety({
    merchant_id: params.merchantId,
    order_id: params.orderId,
    incident_type: params.incidentType,
    description: params.description,
    severity_level: params.severityLevel
  })
}