export type ProfitSharingReturnStatusTheme = 'success' | 'warning' | 'danger' | 'default'

export interface ProfitSharingReturnStatusView {
  statusText: string
  statusTheme: ProfitSharingReturnStatusTheme
}

export function getProfitSharingReturnStatusView(status?: string): ProfitSharingReturnStatusView {
  const normalizedStatus = String(status || '').trim().toLowerCase()

  if (normalizedStatus === 'success' || normalizedStatus === 'return_success') {
    return { statusText: '回退成功', statusTheme: 'success' }
  }

  if (normalizedStatus === 'failed' || normalizedStatus === 'return_failed') {
    return { statusText: '回退失败', statusTheme: 'danger' }
  }

  if (normalizedStatus === 'closed') {
    return { statusText: '已关闭', statusTheme: 'default' }
  }

  if (normalizedStatus === 'pending' || normalizedStatus === 'processing' || normalizedStatus === 'pending_return') {
    return { statusText: '回退处理中', statusTheme: 'warning' }
  }

  return { statusText: '状态同步中', statusTheme: 'default' }
}