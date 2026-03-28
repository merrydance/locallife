import dayjs from 'dayjs'
import {
  completeMerchantComplaint,
  getMerchantComplaintDetail,
  MerchantComplaintActionAck,
  MerchantComplaintItem,
  MerchantComplaintState,
  respondMerchantComplaint
} from '../../../../api/merchant-complaints'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'

interface ComplaintDetailOptions {
  id?: string
}

type ComplaintTagTheme = 'danger' | 'warning' | 'success'

interface MerchantComplaintDetailView {
  complaintId: string
  complaintState: MerchantComplaintState
  stateLabel: string
  stateTheme: ComplaintTagTheme
  amountText: string
  complaintTimeLabel: string
  complaintDetail: string
  payerOpenIdText: string
  transactionIdText: string
  outTradeNoText: string
  lastSyncedAtLabel: string
  wxpayUpdatedAtLabel: string
  respondedAtLabel: string
  completedAtLabel: string
  updatedAtLabel: string
  responseContent: string
  hasResponse: boolean
  canRespond: boolean
  canComplete: boolean
  progressCurrent: number
}

function formatMoney(amount: number) {
  return `¥${(amount / 100).toFixed(2)}`
}

function formatTime(value?: string) {
  if (!value) return '暂无'
  const parsed = dayjs(value)
  return parsed.isValid() ? parsed.format('YYYY-MM-DD HH:mm') : value
}

function formatStateLabel(state: MerchantComplaintState) {
  const map: Record<MerchantComplaintState, string> = {
    PENDING_RESPONSE: '待回复',
    PROCESSING: '处理中',
    PROCESSED: '已完结'
  }
  return map[state]
}

function getStateTheme(state: MerchantComplaintState): ComplaintTagTheme {
  if (state === 'PENDING_RESPONSE') return 'danger'
  if (state === 'PROCESSING') return 'warning'
  return 'success'
}

function getProgressCurrent(item: MerchantComplaintItem) {
  if (item.complaint_state === 'PROCESSED') return 2
  if (item.response_content || item.responded_at || item.complaint_state === 'PROCESSING') return 1
  return 0
}

function getErrorMessage(err: unknown, fallback: string) {
  if (typeof err === 'object' && err !== null && 'userMessage' in err) {
    const userMessage = (err as { userMessage?: unknown }).userMessage
    if (typeof userMessage === 'string' && userMessage.trim()) {
      return userMessage
    }
  }
  return fallback
}

function isComplaintDetailItem(value: unknown): value is MerchantComplaintItem {
  if (typeof value !== 'object' || value === null) return false
  const candidate = value as Partial<MerchantComplaintItem>
  return typeof candidate.complaint_id === 'string' && typeof candidate.complaint_state === 'string'
}

function isComplaintActionAck(value: unknown): value is MerchantComplaintActionAck {
  if (typeof value !== 'object' || value === null) return false
  const candidate = value as Partial<MerchantComplaintActionAck>
  return typeof candidate.message === 'string' && !('complaint_id' in candidate)
}

function getActionNoticeMessage(value: unknown, fallback: string) {
  return isComplaintActionAck(value) ? fallback : ''
}

function toComplaintDetail(item: MerchantComplaintItem): MerchantComplaintDetailView {
  return {
    complaintId: item.complaint_id,
    complaintState: item.complaint_state,
    stateLabel: formatStateLabel(item.complaint_state),
    stateTheme: getStateTheme(item.complaint_state),
    amountText: formatMoney(item.amount || 0),
    complaintTimeLabel: formatTime(item.complaint_time),
    complaintDetail: item.complaint_detail || '暂无投诉详情',
    payerOpenIdText: item.payer_openid || '未返回',
    transactionIdText: item.transaction_id || '未返回',
    outTradeNoText: item.out_trade_no || '未返回',
    lastSyncedAtLabel: formatTime(item.last_synced_at),
    wxpayUpdatedAtLabel: formatTime(item.wxpay_update_time),
    respondedAtLabel: formatTime(item.responded_at),
    completedAtLabel: formatTime(item.completed_at),
    updatedAtLabel: formatTime(item.updated_at),
    responseContent: item.response_content || '',
    hasResponse: Boolean(item.response_content),
    canRespond: item.complaint_state !== 'PROCESSED',
    canComplete: item.complaint_state !== 'PROCESSED',
    progressCurrent: getProgressCurrent(item)
  }
}

Page({
  data: {
    navBarHeight: 88,
    complaintId: '',
    loading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    actionNoticeMessage: '',
    responsePopupVisible: false,
    responseSubmitting: false,
    completing: false,
    responseDraft: '',
    responseJumpUrl: '',
    detail: null as MerchantComplaintDetailView | null
  },

  onLoad(options: ComplaintDetailOptions) {
    const { navBarHeight } = getStableBarHeights()
    const complaintId = decodeURIComponent(options.id || '')
    this.setData({ navBarHeight, complaintId })

    if (!complaintId) {
      this.setData({
        loading: false,
        initialError: true,
        initialErrorMessage: '缺少投诉单号，无法查看详情'
      })
      return
    }

    this.loadDetail(false)
  },

  onPullDownRefresh() {
    this.loadDetail(false)
  },

  onRetry() {
    this.loadDetail(false)
  },

  onRetryRefresh() {
    this.loadDetail(true)
  },

  onCopyValue(e: WechatMiniprogram.TouchEvent) {
    const { value } = e.currentTarget.dataset as { value?: string }
    if (!value || value === '未返回') return
    wx.setClipboardData({ data: value })
  },

  onOpenResponsePopup() {
    const detail = this.data.detail
    if (!detail || !detail.canRespond) return
    this.setData({
      responsePopupVisible: true,
      responseDraft: detail.responseContent,
      responseJumpUrl: ''
    })
  },

  onCloseResponsePopup() {
    this.setData({ responsePopupVisible: false })
  },

  onResponsePopupVisibleChange(e: WechatMiniprogram.CustomEvent<{ visible: boolean }>) {
    if (!e.detail.visible) {
      this.onCloseResponsePopup()
    }
  },

  onResponseInput(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({ responseDraft: e.detail.value || '' })
  },

  onJumpUrlInput(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    this.setData({ responseJumpUrl: e.detail.value || '' })
  },

  async onSubmitResponse() {
    if (this.data.responseSubmitting) return

    const responseContent = this.data.responseDraft.trim()
    const jumpUrl = this.data.responseJumpUrl.trim()

    if (!responseContent) {
      wx.showToast({ title: '请输入回复内容', icon: 'none' })
      return
    }
    if (responseContent.length > 256) {
      wx.showToast({ title: '回复内容需控制在 256 字内', icon: 'none' })
      return
    }
    if (jumpUrl && !/^https?:\/\//.test(jumpUrl)) {
      wx.showToast({ title: '跳转链接需以 http:// 或 https:// 开头', icon: 'none' })
      return
    }

    this.setData({ responseSubmitting: true })
    wx.showLoading({ title: '提交中...' })

    try {
      const result = await respondMerchantComplaint(this.data.complaintId, {
        response_content: responseContent,
        jump_url: jumpUrl || undefined
      })
      const actionNoticeMessage = getActionNoticeMessage(
        result,
        '回复已提交到微信，页面状态稍后同步，可下拉刷新确认最新进度'
      )

      this.setData({
        responsePopupVisible: false,
        responseJumpUrl: '',
        refreshErrorMessage: '',
        actionNoticeMessage,
        detail: isComplaintDetailItem(result) ? toComplaintDetail(result) : this.data.detail
      })
      wx.showToast({ title: actionNoticeMessage ? '回复已提交，待同步' : '投诉回复已提交', icon: 'success' })
      await this.loadDetail(true)
    } catch (err) {
      logger.error('Respond merchant complaint failed', err)
      wx.showToast({ title: getErrorMessage(err, '提交回复失败，请稍后重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ responseSubmitting: false })
    }
  },

  async onCompleteComplaint() {
    if (this.data.completing || !this.data.detail?.canComplete) return

    const confirmed = await new Promise<boolean>((resolve) => {
      wx.showModal({
        title: '确认完结投诉',
        content: '确认问题已协商完成后再完结。完结后投诉会同步到微信侧处理结果。',
        confirmText: '确认完结',
        success: (res) => resolve(Boolean(res.confirm)),
        fail: () => resolve(false)
      })
    })

    if (!confirmed) return

    this.setData({ completing: true })
    wx.showLoading({ title: '提交中...' })

    try {
      const result = await completeMerchantComplaint(this.data.complaintId)
      const actionNoticeMessage = getActionNoticeMessage(
        result,
        '投诉已在微信侧完结，页面状态稍后同步，可下拉刷新确认最新进度'
      )

      this.setData({
        refreshErrorMessage: '',
        actionNoticeMessage,
        detail: isComplaintDetailItem(result) ? toComplaintDetail(result) : this.data.detail
      })
      wx.showToast({ title: actionNoticeMessage ? '已完结，待同步' : '投诉已完结', icon: 'success' })
      await this.loadDetail(true)
    } catch (err) {
      logger.error('Complete merchant complaint failed', err)
      wx.showToast({ title: getErrorMessage(err, '完结投诉失败，请稍后重试'), icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ completing: false })
    }
  },

  async loadDetail(silent: boolean) {
    if (!silent) {
      this.setData({
        loading: true,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: ''
      })
    }

    try {
      const detail = await getMerchantComplaintDetail(this.data.complaintId)
      this.setData({
        detail: toComplaintDetail(detail),
        loading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        actionNoticeMessage: ''
      })
    } catch (err) {
      logger.error('Load merchant complaint detail failed', err)
      const message = getErrorMessage(err, '投诉详情加载失败，请稍后重试')

      if (silent && this.data.detail) {
        this.setData({
          loading: false,
          refreshErrorMessage: `${message}，当前已保留上次同步结果`
        })
      } else {
        this.setData({
          loading: false,
          initialError: true,
          initialErrorMessage: message
        })
      }
    } finally {
      wx.stopPullDownRefresh()
    }
  }
})