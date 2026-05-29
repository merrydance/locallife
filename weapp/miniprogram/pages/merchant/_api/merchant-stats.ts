import { request } from '../../../utils/request'

export interface MerchantOverviewResponse {
  total_days: number
  total_orders: number
  total_sales: number
  total_commission: number
  avg_daily_sales: number
  print_anomalies_count: number
}

export interface TopSellingDishRow {
  dish_id: number
  dish_name: string
  dish_price: number
  total_sold: number
  total_revenue: number
}

export interface MerchantDailyStatRow {
  date: string
  order_count: number
  total_sales: number
  commission: number
  takeout_orders: number
  dine_in_orders: number
}

export type MerchantCustomerOrderBy = 'total_orders' | 'total_amount' | 'last_order_at'

export interface MerchantCustomerStatRow {
  user_id: number
  full_name: string
  phone: string
  avatar_url: string
  total_orders: number
  total_amount: number
  avg_order_amount: number
  first_order_at: string
  last_order_at: string
}

export interface MerchantCustomerStatsListResponse {
  data: MerchantCustomerStatRow[]
  total: number
  page: number
  limit: number
}

export interface MerchantCustomerFavoriteDish {
  dish_id: number
  dish_name: string
  order_count: number
  total_quantity: number
}

export interface MerchantCustomerDetailResponse extends MerchantCustomerStatRow {
  favorite_dishes: MerchantCustomerFavoriteDish[]
}

export interface MerchantHourlyStatRow {
  hour: number
  order_count: number
  avg_order_amount: number
}

export interface MerchantOrderSourceStatRow {
  order_type: string
  order_count: number
  total_sales: number
}

export interface MerchantRepurchaseRateResponse {
  total_users: number
  repeat_users: number
  repurchase_rate: number
  avg_orders_per_user: number
}

export interface MerchantCategoryStatRow {
  category_name: string
  order_count: number
  total_sales: number
  total_quantity: number
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

  static async getDailyStats(params: { start_date: string, end_date: string }): Promise<MerchantDailyStatRow[]> {
    return await request({
      url: '/v1/merchant/stats/daily',
      method: 'GET',
      data: params
    })
  }

  static async listCustomers(params: {
    order_by?: MerchantCustomerOrderBy
    page?: number
    limit?: number
  } = {}): Promise<MerchantCustomerStatsListResponse> {
    return await request({
      url: '/v1/merchant/stats/customers',
      method: 'GET',
      data: params
    })
  }

  static async getCustomerDetail(userId: number): Promise<MerchantCustomerDetailResponse> {
    return await request({
      url: `/v1/merchant/stats/customers/${userId}`,
      method: 'GET'
    })
  }

  static async getHourlyStats(params: { start_date: string, end_date: string }): Promise<MerchantHourlyStatRow[]> {
    return await request({
      url: '/v1/merchant/stats/hourly',
      method: 'GET',
      data: params
    })
  }

  static async getOrderSources(params: { start_date: string, end_date: string }): Promise<MerchantOrderSourceStatRow[]> {
    return await request({
      url: '/v1/merchant/stats/sources',
      method: 'GET',
      data: params
    })
  }

  static async getRepurchaseRate(params: { start_date: string, end_date: string }): Promise<MerchantRepurchaseRateResponse> {
    return await request({
      url: '/v1/merchant/stats/repurchase',
      method: 'GET',
      data: params
    })
  }

  static async getCategoryStats(params: { start_date: string, end_date: string }): Promise<MerchantCategoryStatRow[]> {
    return await request({
      url: '/v1/merchant/stats/categories',
      method: 'GET',
      data: params
    })
  }
}
