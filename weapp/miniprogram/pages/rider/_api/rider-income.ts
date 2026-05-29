import { request } from '../../../utils/request'
import { DateRangeParams } from '../../../api/types'

export type RiderIncomeStatus = 'pending' | 'processing' | 'finished' | 'failed'

export interface RiderIncomeStatusSummary {
  status: RiderIncomeStatus
  order_count: number
  rider_amount: number
  delivery_fee: number
  rider_gross_amount: number
  rider_payment_fee: number
}

export interface RiderIncomeSummaryResponse {
  total_deliveries: number
  total_rider_income: number
  total_delivery_fee: number
  total_rider_gross_amount: number
  total_rider_payment_fee: number
  status_summary: RiderIncomeStatusSummary[]
}

export interface RiderIncomeLedgerItem {
  id: number
  payment_order_id: number
  merchant_id: number
  order_id: number
  order_no: string
  merchant_name: string
  status: RiderIncomeStatus
  total_amount: number
  delivery_fee: number
  rider_gross_amount: number
  rider_payment_fee: number
  rider_amount: number
  distributable_amount: number
  out_order_no: string
  sharing_order_id: string
  finished_at?: string
  created_at: string
}

export interface RiderIncomeLedgerResponse {
  items: RiderIncomeLedgerItem[]
  total: number
  page_id: number
  page_size: number
  has_more: boolean
}

export interface RiderIncomeDailyItem {
  date: string
  delivery_count: number
  daily_income: number
  rider_gross_amount: number
  rider_payment_fee: number
}

export interface RiderIncomeDailyResponse {
  items: RiderIncomeDailyItem[]
}

export interface RiderIncomeLedgerParams extends DateRangeParams {
  status?: RiderIncomeStatus
  page_id?: number
  page_size?: number
}

export const riderIncomeApi = {
  getSummary(params: DateRangeParams): Promise<RiderIncomeSummaryResponse> {
    return request({
      url: '/v1/rider/income/summary',
      method: 'GET',
      data: params
    })
  },

  listLedger(params: RiderIncomeLedgerParams): Promise<RiderIncomeLedgerResponse> {
    return request({
      url: '/v1/rider/income/ledger',
      method: 'GET',
      data: params
    })
  },

  getDaily(params: DateRangeParams): Promise<RiderIncomeDailyResponse> {
    return request({
      url: '/v1/rider/income/daily',
      method: 'GET',
      data: params
    })
  }
}
