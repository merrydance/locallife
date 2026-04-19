import { generateAppBindCode } from '../api/auth'
import { buildMerchantApplymentStatusView, getMerchantApplymentStatus } from '../api/merchant-applyment'
import { getMerchantComplaintSummary } from '../api/merchant-complaints'
import {
  getMyMerchantOpenStatus,
  getMyMerchantProfile,
  updateMyMerchantOpenStatus,
  type MerchantOperatorResponse
} from '../api/merchant'
import { MerchantStatsService } from '../api/merchant-stats'
import { MerchantOrderManagementService } from '../api/order-management'

export type MerchantConsoleProfile = MerchantOperatorResponse

export function fetchMerchantConsoleProfile() {
  return getMyMerchantProfile()
}

export function fetchMerchantConsoleOpenStatus() {
  return getMyMerchantOpenStatus()
}

export function fetchMerchantConsoleOverview(startDate: string, endDate: string) {
  return MerchantStatsService.getOverview({
    start_date: startDate,
    end_date: endDate
  })
}

export function fetchMerchantConsoleOrderSummary() {
  return MerchantOrderManagementService.getOrderSummary()
}

export function fetchMerchantConsoleComplaintSummary() {
  return getMerchantComplaintSummary()
}

export function createMerchantAppBindCode() {
  return generateAppBindCode()
}

export async function fetchMerchantApplymentStatusView() {
  const applyment = await getMerchantApplymentStatus()
  return buildMerchantApplymentStatusView(applyment)
}

export function updateMerchantConsoleOpenStatus(nextIsOpen: boolean) {
  return updateMyMerchantOpenStatus(nextIsOpen)
}