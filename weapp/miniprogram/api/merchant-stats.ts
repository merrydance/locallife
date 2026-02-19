import { request } from '../utils/request'

export interface MerchantOverviewResponse {
  total_days: number
  total_orders: number
  total_sales: number
  total_commission: number
  avg_daily_sales: number
}

export interface TopSellingDishRow {
  dish_id: number
  dish_name: string
  dish_price: number
  total_sold: number
  total_revenue: number
}

export class MerchantStatsService {
  static async getOverview(params: { start_date: string, end_date: string }): Promise<MerchantOverviewResponse> {
    return await request({
      url: '/v1/merchant/stats/overview',
      method: 'GET',
      data: params
    })
  }

  static async getTopSellingDishes(params: { start_date: string, end_date: string, limit?: number }): Promise<TopSellingDishRow[]> {
    return await request({
      url: '/v1/merchant/stats/dishes/top',
      method: 'GET',
      data: params
    })
  }
}
