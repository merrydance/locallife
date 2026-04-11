export type AdminApprovalCategory = 'draft' | 'pending' | 'approved' | 'rejected' | 'unknown'

export type AdminApprovalTheme = 'success' | 'warning' | 'danger' | 'primary' | 'default'

export type AdminApprovalFilter = 'draft' | 'pending' | 'submitted' | 'approved' | 'rejected'

export type AdminApprovalSort = 'approved_first' | 'rejected_first' | 'submitted_first'

export type AdminApprovalDisplay = {
  category: AdminApprovalCategory
  label: string
  theme: AdminApprovalTheme
  isPending: boolean
  isApproved: boolean
  isRejected: boolean
  isDraft: boolean
}

const PENDING_APPROVAL_STATUSES = new Set(['submitted', 'pending', 'reviewing', 'pending_approval'])

export function getAdminApprovalCategory(status?: string): AdminApprovalCategory {
  if (!status) {
    return 'unknown'
  }

  if (status === 'approved') {
    return 'approved'
  }

  if (status === 'rejected') {
    return 'rejected'
  }

  if (status === 'draft') {
    return 'draft'
  }

  if (PENDING_APPROVAL_STATUSES.has(status)) {
    return 'pending'
  }

  return 'unknown'
}

export function getAdminApprovalStatusLabel(status?: string): string {
  const category = getAdminApprovalCategory(status)

  if (category === 'approved') return '已通过'
  if (category === 'rejected') return '已驳回'
  if (category === 'pending') return '待审核'
  if (category === 'draft') return '草稿'
  return status || '未知状态'
}

export function getAdminApprovalStatusDisplay(
  status?: string,
  options: {
    draftTheme?: AdminApprovalTheme
    unknownTheme?: AdminApprovalTheme
  } = {}
): AdminApprovalDisplay {
  const category = getAdminApprovalCategory(status)

  if (category === 'approved') {
    return { category, label: '已通过', theme: 'success', isPending: false, isApproved: true, isRejected: false, isDraft: false }
  }

  if (category === 'rejected') {
    return { category, label: '已驳回', theme: 'danger', isPending: false, isApproved: false, isRejected: true, isDraft: false }
  }

  if (category === 'pending') {
    return { category, label: '待审核', theme: 'warning', isPending: true, isApproved: false, isRejected: false, isDraft: false }
  }

  if (category === 'draft') {
    return {
      category,
      label: '草稿',
      theme: options.draftTheme || 'primary',
      isPending: false,
      isApproved: false,
      isRejected: false,
      isDraft: true
    }
  }

  return {
    category,
    label: status || '未知状态',
    theme: options.unknownTheme || 'primary',
    isPending: false,
    isApproved: false,
    isRejected: false,
    isDraft: false
  }
}

export function matchesAdminApprovalFilter(status: string | undefined, filter: AdminApprovalFilter): boolean {
  const category = getAdminApprovalCategory(status)

  if (filter === 'submitted') {
    return category === 'pending'
  }

  return category === filter
}

export function getAdminApprovalStatusPriority(status: string | undefined, sortBy: AdminApprovalSort): number {
  const category = getAdminApprovalCategory(status)

  if (sortBy === 'approved_first') {
    if (category === 'approved') return 0
    if (category === 'pending') return 1
    if (category === 'rejected') return 2
    if (category === 'draft') return 3
    return 4
  }

  if (sortBy === 'rejected_first') {
    if (category === 'rejected') return 0
    if (category === 'pending') return 1
    if (category === 'approved') return 2
    if (category === 'draft') return 3
    return 4
  }

  if (category === 'pending') return 0
  if (category === 'approved') return 1
  if (category === 'rejected') return 2
  if (category === 'draft') return 3
  return 4
}

export function buildAdminApprovalStats<T>(
  list: T[],
  getStatus: (item: T) => string | undefined
): Record<AdminApprovalCategory, number> {
  return list.reduce<Record<AdminApprovalCategory, number>>(
    (stats, item) => {
      const category = getAdminApprovalCategory(getStatus(item))
      stats[category] += 1
      return stats
    },
    {
      draft: 0,
      pending: 0,
      approved: 0,
      rejected: 0,
      unknown: 0
    }
  )
}