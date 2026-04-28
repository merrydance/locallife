import { getMerchantComplaintSummary } from '../api/merchant-complaints'
import { MerchantStatsService } from '../api/merchant-stats'
import { MerchantOrderManagementService } from '../api/order-management'

export function fetchMerchantDashboardOverview(startDate: string, endDate: string) {
  return MerchantStatsService.getOverview({
    start_date: startDate,
    end_date: endDate
  })
}

export function fetchMerchantDashboardOrderSummary() {
  return MerchantOrderManagementService.getOrderSummary()
}

export function fetchMerchantDashboardComplaintSummary() {
  return getMerchantComplaintSummary()
}