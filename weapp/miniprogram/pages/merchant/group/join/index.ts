import {
  applyToJoinGroup,
  GROUP_JOIN_REQUEST_ALREADY_PENDING_CODE,
  listMyGroupJoinRequests,
  searchGroups,
  type GroupJoinRequestResponse
} from '../../_main_shared/api/group-application'
import {
  getGroupJoinRequestStatusDisplay,
  isGroupJoinRequestPending
} from '../../_main_shared/adapters/group-join-request'
import type { StatusTagTheme } from '../../_main_shared/utils/status-tag'
import { logger } from '../../../../utils/logger'
import { getErrorUserMessage } from '../../../../utils/user-facing'
import { ensureMerchantConsoleAccess } from '../../../../utils/console-access'

type NavHeightEvent = {
  detail: {
    navBarHeight: number
  }
}

type InputEvent = {
  detail: {
    value: string
  }
}

type ApplyEvent = {
  currentTarget: {
    dataset: {
      id?: number
      name?: string
    }
  }
}

type GroupJoinStatus = GroupJoinRequestResponse['status']

interface GroupJoinRequestView {
  id: number
  groupId: number
  groupName: string
  merchantId: number
  applicantUserId: number
  status: GroupJoinStatus
  statusLabel: string
  statusTheme: StatusTagTheme
  reason: string
  createdAt: string
  createdAtText: string
}

interface GroupItem {
  id: number
  name: string
  address: string
  actionText: string
  actionDisabled: boolean
  applying: boolean
  pending: boolean
  statusLabel: string
  statusTheme: StatusTagTheme
}

const getErrorMessage = getErrorUserMessage
const CONFIG_PAGE_ROUTE = 'pages/merchant/config/index'

function toNumber(value: unknown): number {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return value
  }
  if (typeof value === 'string' && value.trim()) {
    const parsed = Number(value)
    return Number.isFinite(parsed) ? parsed : 0
  }
  return 0
}

function toText(value: unknown, fallback = ''): string {
  if (typeof value !== 'string') {
    return fallback
  }
  const normalized = value.trim()
  return normalized || fallback
}

function formatDateTime(value?: string): string {
  if (!value) {
    return ''
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }

  const pad = (part: number) => String(part).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function toJoinRequestView(request: GroupJoinRequestResponse): GroupJoinRequestView {
  const groupName = toText(request.group_name, `集团 ${request.group_id}`)
  const statusDisplay = getGroupJoinRequestStatusDisplay(request.status)
  return {
    id: request.id,
    groupId: request.group_id,
    groupName,
    merchantId: request.merchant_id,
    applicantUserId: request.applicant_user_id,
    status: request.status,
    statusLabel: statusDisplay.label,
    statusTheme: statusDisplay.theme,
    reason: toText(request.reason),
    createdAt: request.created_at,
    createdAtText: formatDateTime(request.created_at)
  }
}

function findLatestPendingJoinRequest(requests: GroupJoinRequestView[]) {
  return requests.find((request) => isGroupJoinRequestPending(request.status)) || null
}

function buildGroupItems(
  rawGroups: Array<Record<string, unknown> | GroupItem>,
  joinRequests: GroupJoinRequestView[],
  applyingGroupId: number
): GroupItem[] {
  const pendingByGroup = new Map<number, GroupJoinRequestView>()
  for (const request of joinRequests) {
    if (isGroupJoinRequestPending(request.status) && !pendingByGroup.has(request.groupId)) {
      pendingByGroup.set(request.groupId, request)
    }
  }

  return rawGroups.map((raw) => {
    const id = toNumber(raw.id)
    const pendingRequest = pendingByGroup.get(id)
    const applying = applyingGroupId > 0 && applyingGroupId === id
    return {
      id,
      name: toText(raw.name, '未命名集团'),
      address: toText(raw.address, '暂无地址'),
      actionText: pendingRequest ? '已申请' : '申请加入',
      actionDisabled: id <= 0 || !!pendingRequest,
      applying,
      pending: !!pendingRequest,
      statusLabel: pendingRequest?.statusLabel || '',
      statusTheme: pendingRequest?.statusTheme || 'default'
    }
  })
}

function upsertJoinRequest(
  requests: GroupJoinRequestView[],
  request: GroupJoinRequestView
): GroupJoinRequestView[] {
  const next = [request, ...requests.filter((item) => item.id !== request.id)]
  return next.sort((left, right) => right.id - left.id)
}

function getErrorCode(error: unknown): number | undefined {
  if (!error || typeof error !== 'object') {
    return undefined
  }
  const knownError = error as { statusCode?: unknown, code?: unknown }
  const code = typeof knownError.code === 'number' ? knownError.code : Number(knownError.code)
  if (Number.isFinite(code)) {
    return code
  }
  const statusCode = typeof knownError.statusCode === 'number'
    ? knownError.statusCode
    : Number(knownError.statusCode)
  return Number.isFinite(statusCode) ? statusCode : undefined
}

function isDuplicateJoinRequestError(error: unknown): boolean {
  return getErrorCode(error) === GROUP_JOIN_REQUEST_ALREADY_PENDING_CODE
}

Page({
  data: {
    navBarHeight: 88,
    accessReady: false,
    accessDenied: false,
    accessErrorMessage: '',
    searchErrorMessage: '',
    joinRequestsErrorMessage: '',
    keyword: '',
    rawGroups: [] as Array<Record<string, unknown>>,
    groups: [] as GroupItem[],
    joinRequests: [] as GroupJoinRequestView[],
    pendingJoinRequest: null as GroupJoinRequestView | null,
    joinRequestsLoading: false,
    searched: false,
    loading: false,
    applying: false,
    applyingGroupId: 0,
    dialogVisible: false,
    selectedGroupId: 0,
    selectedGroupName: '',
    applyReason: ''
  },

  async onLoad() {
    const accessResult = await ensureMerchantConsoleAccess()
    this.setData({
      accessReady: true,
      accessDenied: accessResult.status === 'denied',
      accessErrorMessage: accessResult.status === 'error' ? accessResult.message : ''
    })

    if (accessResult.status === 'granted') {
      await this.refreshJoinRequests({ initial: true })
    }
  },

  async onShow() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      return
    }
    await this.refreshJoinRequests({ silent: true })
  },

  onRetryAccess() {
    this.setData({ accessReady: false, accessDenied: false, accessErrorMessage: '' })
    this.onLoad()
  },

  onNavHeight(e: NavHeightEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight })
  },

  onSearchChange(e: InputEvent) {
    this.setData({ keyword: e.detail.value })
  },

  async onRetryJoinRequests() {
    await this.refreshJoinRequests({ initial: true })
  },

  async refreshJoinRequests(options: { initial?: boolean, silent?: boolean } = {}) {
    if (!options.silent) {
      this.setData({ joinRequestsLoading: !!options.initial, joinRequestsErrorMessage: '' })
    }

    try {
      const requests = await listMyGroupJoinRequests()
      const viewRequests = (requests || []).map(toJoinRequestView)
      this.setData({
        joinRequests: viewRequests,
        pendingJoinRequest: findLatestPendingJoinRequest(viewRequests),
        groups: buildGroupItems(this.data.rawGroups, viewRequests, this.data.applyingGroupId),
        joinRequestsErrorMessage: '',
        joinRequestsLoading: false
      })
    } catch (e) {
      const message = getErrorMessage(e, '申请状态同步失败，请稍后重试')
      const hasTrustedState = this.data.joinRequests.length > 0 || !!this.data.pendingJoinRequest
      this.setData({
        joinRequestsErrorMessage: options.silent && hasTrustedState ? `${message}，当前保留上次结果` : message,
        joinRequestsLoading: false
      })
      logger.error('Refresh group join requests failed', e)
    }
  },

  async onSearchSubmit() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    if (!this.data.keyword.trim()) return
    this.setData({ loading: true, searched: false, rawGroups: [], groups: [], searchErrorMessage: '' })
    try {
      const res = await searchGroups(this.data.keyword)
      this.setData({
        rawGroups: res || [],
        groups: buildGroupItems(res || [], this.data.joinRequests, this.data.applyingGroupId),
        searched: true,
        searchErrorMessage: '',
        loading: false
      })
    } catch (e) {
      this.setData({
        loading: false,
        searched: true,
        searchErrorMessage: getErrorMessage(e, '搜索失败，请稍后重试')
      })
      logger.error('Search groups failed', e)
    }
  },

  onApply(e: ApplyEvent) {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    if (this.data.applying) return

    const { id, name } = e.currentTarget.dataset
    if (typeof id !== 'number' || id <= 0) {
      wx.showToast({ title: '集团信息异常', icon: 'none' })
      return
    }
    const pendingRequest = this.data.joinRequests.find((request) => (
      request.groupId === id && isGroupJoinRequestPending(request.status)
    ))
    if (pendingRequest) {
      this.setData({ pendingJoinRequest: pendingRequest })
      wx.showToast({ title: '已提交该集团申请，请等待审核', icon: 'none' })
      return
    }
    this.setData({
      selectedGroupId: id,
      selectedGroupName: typeof name === 'string' ? name : '',
      dialogVisible: true,
      applyReason: ''
    })
  },

  onReasonChange(e: InputEvent) {
    this.setData({ applyReason: e.detail.value })
  },

  closeDialog() {
    if (this.data.applying) return
    this.setData({ dialogVisible: false })
  },

  async confirmApply() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    if (this.data.applying) return
    if (this.data.selectedGroupId <= 0) {
      wx.showToast({ title: '集团信息异常', icon: 'none' })
      return
    }

    let shouldShowSubmittedModal = false
    this.setData({
      applying: true,
      applyingGroupId: this.data.selectedGroupId,
      groups: buildGroupItems(this.data.rawGroups, this.data.joinRequests, this.data.selectedGroupId)
    })
    wx.showLoading({ title: '提交申请...' })
    try {
      const created = await applyToJoinGroup(this.data.selectedGroupId, {
        reason: this.data.applyReason
      })
      const createdView = toJoinRequestView({
        ...created,
        group_name: created.group_name || this.data.selectedGroupName
      })
      const nextRequests = upsertJoinRequest(this.data.joinRequests, createdView)
      this.setData({
        joinRequests: nextRequests,
        pendingJoinRequest: findLatestPendingJoinRequest(nextRequests),
        groups: buildGroupItems(this.data.rawGroups, nextRequests, this.data.selectedGroupId),
        joinRequestsErrorMessage: '',
        dialogVisible: false
      })
      await this.refreshJoinRequests({ silent: true })
      shouldShowSubmittedModal = true
    } catch (e: unknown) {
      if (isDuplicateJoinRequestError(e)) {
        await this.refreshJoinRequests({ silent: true })
        this.setData({ dialogVisible: false })
        shouldShowSubmittedModal = true
      } else {
        wx.showToast({ title: getErrorMessage(e, '申请失败，请稍后重试'), icon: 'none' })
      }
    } finally {
      wx.hideLoading()
      this.setData({
        applying: false,
        applyingGroupId: 0,
        groups: buildGroupItems(this.data.rawGroups, this.data.joinRequests, 0)
      })
    }

    if (shouldShowSubmittedModal) {
      this.showSubmittedModal()
    }
  },

  showSubmittedModal() {
    const groupName = this.data.pendingJoinRequest?.groupName || this.data.selectedGroupName
    wx.showModal({
      title: '申请已提交',
      content: `加入 ${groupName} 的申请已发送，请联系集团管理员审核。`,
      showCancel: false,
      success: () => {
        const pages = getCurrentPages()
        const previousPage = pages[pages.length - 2]

        if (previousPage?.route === CONFIG_PAGE_ROUTE) {
          wx.navigateBack()
          return
        }

        wx.redirectTo({ url: '/pages/merchant/config/index' })
      }
    })
  }
})
