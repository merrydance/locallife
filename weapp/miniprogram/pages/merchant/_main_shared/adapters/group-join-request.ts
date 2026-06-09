import type { GroupJoinRequestResponse } from '../api/group-application'
import { resolveStatusTagTheme, type StatusTagTheme } from '../utils/status-tag'

export type GroupJoinRequestStatus = GroupJoinRequestResponse['status']

export type GroupJoinRequestStatusDisplay = {
  label: string
  theme: StatusTagTheme
  isPending: boolean
  isApproved: boolean
  isRejected: boolean
  isCancelled: boolean
}

export function getGroupJoinRequestStatusDisplay(status?: GroupJoinRequestStatus): GroupJoinRequestStatusDisplay {
  if (status === 'pending') {
    return {
      label: '待审核',
      theme: resolveStatusTagTheme('warning'),
      isPending: true,
      isApproved: false,
      isRejected: false,
      isCancelled: false
    }
  }

  if (status === 'approved') {
    return {
      label: '已通过',
      theme: resolveStatusTagTheme('success'),
      isPending: false,
      isApproved: true,
      isRejected: false,
      isCancelled: false
    }
  }

  if (status === 'rejected') {
    return {
      label: '已驳回',
      theme: resolveStatusTagTheme('danger'),
      isPending: false,
      isApproved: false,
      isRejected: true,
      isCancelled: false
    }
  }

  if (status === 'cancelled') {
    return {
      label: '已撤回',
      theme: resolveStatusTagTheme('neutral'),
      isPending: false,
      isApproved: false,
      isRejected: false,
      isCancelled: true
    }
  }

  return {
    label: '未知状态',
    theme: resolveStatusTagTheme('neutral'),
    isPending: false,
    isApproved: false,
    isRejected: false,
    isCancelled: false
  }
}

export function isGroupJoinRequestPending(status?: GroupJoinRequestStatus): boolean {
  return getGroupJoinRequestStatusDisplay(status).isPending
}
