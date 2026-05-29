import {
  getFoodSafetyCaseStatusDisplay,
  operatorBasicManagementService,
  type OperatorFoodSafetyCaseDetailResponse,
  type OperatorFoodSafetyCaseItem,
  type OperatorFoodSafetyCaseStatus,
  type OperatorFoodSafetyCaseStatusTheme,
  type OperatorFoodSafetyIncidentItem
} from '../_api/operator-basic-management'

export type OperatorSafetyStatusFilter = '' | OperatorFoodSafetyCaseStatus

export interface OperatorFoodSafetyCaseView {
  id: number
  merchant_id: number
  primary_product_key: string
  primary_product_label: string
  trigger_reason: string
  status: OperatorFoodSafetyCaseStatus
  status_label: string
  status_theme: OperatorFoodSafetyCaseStatusTheme
  suspended_at: string
  created_at: string
}

export interface OperatorFoodSafetyCaseListPageData {
  cases: OperatorFoodSafetyCaseView[]
  nextPage: number
  hasMore: boolean
}

export interface OperatorFoodSafetyCaseDetailView extends OperatorFoodSafetyCaseItem {
  status_label: string
  status_theme: OperatorFoodSafetyCaseStatusTheme
  is_active: boolean
  is_resolved: boolean
}

export interface OperatorFoodSafetyIncidentView extends OperatorFoodSafetyIncidentItem {
  status_label: string
  status_theme: OperatorFoodSafetyCaseStatusTheme
}

export interface OperatorFoodSafetyDetailPageData {
  caseDetail: OperatorFoodSafetyCaseDetailView
  incidents: OperatorFoodSafetyIncidentView[]
  investigationReport: string
  merchantRectificationReport: string
  resolution: string
}

function formatProductLabel(item: OperatorFoodSafetyCaseItem): string {
  if (item.primary_product_label?.trim()) {
    return item.primary_product_label.trim()
  }
  if (item.primary_product_key?.trim()) {
    return item.primary_product_key.trim()
  }
  return '未识别问题商品'
}

function adaptFoodSafetyCase(item: OperatorFoodSafetyCaseItem): OperatorFoodSafetyCaseView {
  const statusDisplay = getFoodSafetyCaseStatusDisplay(item.status)
  return {
    id: item.id,
    merchant_id: item.merchant_id,
    primary_product_key: item.primary_product_key,
    primary_product_label: formatProductLabel(item),
    trigger_reason: item.trigger_reason,
    status: item.status,
    status_label: statusDisplay.label,
    status_theme: statusDisplay.theme,
    suspended_at: item.suspended_at,
    created_at: item.created_at
  }
}

function adaptFoodSafetyDetail(response: OperatorFoodSafetyCaseDetailResponse): OperatorFoodSafetyDetailPageData {
  const caseStatus = getFoodSafetyCaseStatusDisplay(response.case.status)
  return {
    caseDetail: {
      ...response.case,
      status_label: caseStatus.label,
      status_theme: caseStatus.theme,
      is_active: caseStatus.isActive,
      is_resolved: caseStatus.isResolved
    },
    incidents: (response.incidents || []).map((item) => {
      const incidentStatus = getFoodSafetyCaseStatusDisplay(item.status as OperatorFoodSafetyCaseStatus)
      return {
        ...item,
        status_label: incidentStatus.label,
        status_theme: incidentStatus.theme
      }
    }),
    investigationReport: response.case.investigation_report || '',
    merchantRectificationReport: response.case.merchant_rectification_report || '',
    resolution: response.case.resolution || ''
  }
}

export async function loadOperatorFoodSafetyCaseListPageData(params: {
  pageId: number
  pageSize: number
  status?: OperatorSafetyStatusFilter
}): Promise<OperatorFoodSafetyCaseListPageData> {
  const response = await operatorBasicManagementService.getFoodSafetyCases({
    page: params.pageId,
    limit: params.pageSize,
    status: params.status || undefined
  })

  return {
    cases: (response.items || []).map(adaptFoodSafetyCase),
    nextPage: params.pageId + 1,
    hasMore: Boolean(response.has_more)
  }
}

export async function loadOperatorFoodSafetyDetailPageData(id: number): Promise<OperatorFoodSafetyDetailPageData> {
  const response = await operatorBasicManagementService.getFoodSafetyCaseDetail(id)
  return adaptFoodSafetyDetail(response)
}

export async function saveOperatorFoodSafetyInvestigation(id: number, investigationReport: string): Promise<void> {
  await operatorBasicManagementService.investigateFoodSafetyCase(id, {
    investigation_report: investigationReport
  })
}

export async function saveOperatorFoodSafetyResolution(params: {
  id: number
  investigationReport?: string
  merchantRectificationReport: string
  resolution: string
}): Promise<void> {
  await operatorBasicManagementService.resolveFoodSafetyCase(params.id, {
    investigation_report: params.investigationReport,
    merchant_rectification_report: params.merchantRectificationReport,
    resolution: params.resolution
  })
}