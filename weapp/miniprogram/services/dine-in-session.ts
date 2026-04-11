import type {
  BillingGroupDTO,
  DiningSessionDTO,
  DiningSessionEntrySessionSummary,
  DiningSessionMenuResponse,
  OpenDiningSessionResponse
} from '../api/dining-session'
import type { ScanTableMerchantInfo, ScanTableTableInfo } from '../api/table'

const STORAGE_KEY = 'dineInSessionContext'

export interface DineInSessionContext {
  session_id: number
  billing_group_id: number
  merchant_id: number
  table_id: number
  reservation_id?: number
  table_no: string
  merchant_name: string
  merchant_logo_url?: string
  status: string
  updated_at: string
}

function buildContext(
  session: DiningSessionDTO,
  billingGroup: BillingGroupDTO,
  merchant: Pick<ScanTableMerchantInfo, 'name' | 'logo_url'>,
  table: Pick<ScanTableTableInfo, 'table_no'>
): DineInSessionContext {
  return {
    session_id: session.id,
    billing_group_id: billingGroup.id,
    merchant_id: session.merchant_id,
    table_id: session.table_id,
    reservation_id: session.reservation_id,
    table_no: table.table_no,
    merchant_name: merchant.name,
    merchant_logo_url: merchant.logo_url,
    status: session.status,
    updated_at: session.updated_at || session.created_at
  }
}

export function saveDineInSessionContext(context: DineInSessionContext) {
  wx.setStorageSync(STORAGE_KEY, context)
}

export function getDineInSessionContext(): DineInSessionContext | null {
  try {
    const context = wx.getStorageSync(STORAGE_KEY) as DineInSessionContext | null
    if (!context || !context.session_id || !context.billing_group_id) {
      return null
    }
    return context
  } catch (_error) {
    return null
  }
}

export function clearDineInSessionContext() {
  try {
    wx.removeStorageSync(STORAGE_KEY)
  } catch (_error) {
    return
  }
}

export function saveDineInSessionFromOpenResponse(
  response: OpenDiningSessionResponse,
  merchant: ScanTableMerchantInfo,
  table: ScanTableTableInfo
) {
  saveDineInSessionContext(buildContext(response.session, response.billing_group, merchant, table))
}

export function saveDineInSessionFromEntrySummary(
  summary: DiningSessionEntrySessionSummary,
  merchant: ScanTableMerchantInfo,
  table: ScanTableTableInfo
) {
  saveDineInSessionContext(buildContext(summary.session, summary.billing_group, merchant, table))
}

export function saveDineInSessionFromMenu(response: DiningSessionMenuResponse) {
  saveDineInSessionContext(buildContext(response.session, response.billing_group, response.merchant, response.table))
}