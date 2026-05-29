import { buildStatusTagView, type StatusTagTheme } from '../_main_shared/utils/status-tag'

export interface PlatformStatusView {
  label: string
  theme: StatusTagTheme
}

function buildFallbackStatusView(status?: string): PlatformStatusView {
  return buildStatusTagView(status ? '状态待确认' : '--', 'warning')
}

export function buildPlatformMerchantStatusView(status?: string): PlatformStatusView {
  const normalizedStatus = String(status || '').trim()
  const viewMap: Record<string, PlatformStatusView> = {
    active: buildStatusTagView('正常', 'success'),
    approved: buildStatusTagView('已通过', 'success'),
    suspended: buildStatusTagView('已暂停', 'danger'),
    pending: buildStatusTagView('待处理', 'warning'),
    rejected: buildStatusTagView('已拒绝', 'danger')
  }

  return viewMap[normalizedStatus] || buildFallbackStatusView(normalizedStatus)
}

export function buildPlatformOperatorStatusView(status?: string): PlatformStatusView {
  const normalizedStatus = String(status || '').trim()
  const viewMap: Record<string, PlatformStatusView> = {
    active: buildStatusTagView('运营中', 'success'),
    suspended: buildStatusTagView('已暂停', 'danger')
  }

  return viewMap[normalizedStatus] || buildFallbackStatusView(normalizedStatus)
}

export function buildPlatformRiderStatusView(status?: string): PlatformStatusView {
  const normalizedStatus = String(status || '').trim()
  const viewMap: Record<string, PlatformStatusView> = {
    active: buildStatusTagView('可接单', 'success'),
    approved: buildStatusTagView('已通过', 'success'),
    suspended: buildStatusTagView('暂停接单', 'danger')
  }

  return viewMap[normalizedStatus] || buildFallbackStatusView(normalizedStatus)
}
