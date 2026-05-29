import { MerchantStatsService } from '../_api/merchant-stats'
import { MerchantOrderManagementService } from '../_api/order-management'

export function fetchMerchantDashboardOverview(startDate: string, endDate: string) {
  return MerchantStatsService.getOverview({
    start_date: startDate,
    end_date: endDate
  })
}

export function fetchMerchantDashboardOrderSummary() {
  return MerchantOrderManagementService.getOrderSummary()
}
