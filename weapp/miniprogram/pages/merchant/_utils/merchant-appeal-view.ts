export type MerchantAppealTagTheme = 'warning' | 'success' | 'danger' | 'primary'

type MerchantAppealStatusCode = 'submitted' | 'approved' | 'rejected'

interface MerchantAppealStatusView {
  label: string
  theme: MerchantAppealTagTheme
  isPending: boolean
}

interface MerchantAppealResultSummary {
  title: string
  description: string
}

const APPROVED_APPEAL_STATUSES = new Set<MerchantAppealStatusCode>(['approved'])

export function getMerchantAppealStatusView(status?: string, emptyLabel = '-') : MerchantAppealStatusView {
  const normalizedStatus = String(status || '').trim().toLowerCase() as MerchantAppealStatusCode | ''

  if (!normalizedStatus) {
    return {
      label: emptyLabel,
      theme: 'primary',
      isPending: false
    }
  }

  if (normalizedStatus === 'submitted') {
    return {
      label: '待审核',
      theme: 'warning',
      isPending: true
    }
  }

  if (APPROVED_APPEAL_STATUSES.has(normalizedStatus)) {
    return {
      label: '已通过',
      theme: 'success',
      isPending: false
    }
  }

  return {
    label: normalizedStatus === 'rejected' ? '已驳回' : normalizedStatus,
    theme: 'danger',
    isPending: false
  }
}

export function getMerchantAppealResultHint(status?: string): string {
  const statusView = getMerchantAppealStatusView(status)

  if (statusView.isPending) {
    return '等待平台复核，建议保留责任与证据材料。'
  }

  if (APPROVED_APPEAL_STATUSES.has(String(status || '').trim().toLowerCase() as MerchantAppealStatusCode)) {
    return '异议已通过，进入结果生效阶段。'
  }

  return '平台已驳回异议，需继续按原结果处理。'
}

export function getMerchantAppealResultSummary(status?: string): MerchantAppealResultSummary {
  const normalizedStatus = String(status || '').trim().toLowerCase() as MerchantAppealStatusCode | ''

  if (normalizedStatus === 'approved') {
    return {
      title: '异议已通过',
      description: '平台已认可商户异议，本条索赔会按复核结果重算责任或回收策略。'
    }
  }

  if (normalizedStatus === 'rejected') {
    return {
      title: '异议已驳回',
      description: '平台维持原判，商户需按当前责任结果继续处理追偿或结案。'
    }
  }

  return {
    title: '等待平台复核',
    description: '异议已经提交，平台会结合责任判定、证据和历史记录给出复核结论。'
  }
}

export function getMerchantAppealProgressCurrent(status?: string): number {
  return getMerchantAppealStatusView(status).isPending ? 1 : 2
}
