import dayjs from 'dayjs'
import {
  listMerchantComplaints,
  MerchantComplaintItem,
  MerchantComplaintState
} from '../../../api/merchant-complaints'
import { logger } from '../../../utils/logger'
import { getStableBarHeights } from '../../../utils/responsive'
import { getErrorUserMessage } from '../../../utils/user-facing'

type ComplaintTab = 'all' | MerchantComplaintState
type ComplaintTagTheme = 'danger' | 'warning' | 'success'

interface MerchantComplaintCardView {
  complaintId: string
  stateLabel: string
  stateTheme: ComplaintTagTheme
  amountText: string
  complaintTimeLabel: string
  orderNoLabel: string
  detailPreview: string
  responseStatusLabel: string
  actionHint: string
}

function formatMoney(amount: number) {
  return `¥${(amount / 100).toFixed(2)}`
}

function formatStateLabel(state: ComplaintTab) {
  const map: Record<ComplaintTab, string> = {
    all: '全部投诉',
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

function formatResponseStatus(item: MerchantComplaintItem) {
  if (item.completed_at || item.complaint_state === 'PROCESSED') {
    return '投诉已完结'
  }
  if (item.response_content) {
    return '已回复，等待微信侧同步处理'
  }
  return '尚未回复，建议尽快处理'
}

function formatActionHint(item: MerchantComplaintItem) {
  if (item.complaint_state === 'PENDING_RESPONSE') {
    return '进入详情后回复投诉，避免超时升级'
  }
  if (item.complaint_state === 'PROCESSING') {
    return item.response_content ? '可补充说明或在确认后完结投诉' : '建议补充商户回复并跟进处理状态'
  }
  return '进入详情查看回复记录与完结时间'
}

function toComplaintCard(item: MerchantComplaintItem): MerchantComplaintCardView {
  return {
    complaintId: item.complaint_id,
    stateLabel: formatStateLabel(item.complaint_state),
    stateTheme: getStateTheme(item.complaint_state),
    amountText: formatMoney(item.amount || 0),
    complaintTimeLabel: dayjs(item.complaint_time).format('MM-DD HH:mm'),
    orderNoLabel: item.out_trade_no || item.transaction_id || '未关联到订单号',
    detailPreview: item.complaint_detail || '暂无投诉详情',
    responseStatusLabel: formatResponseStatus(item),
    actionHint: formatActionHint(item)
  }
}

const getErrorMessage = getErrorUserMessage

Page({
  data: {
    navBarHeight: 88,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    loading: false,
    loadingMore: false,
    currentTab: 'all' as ComplaintTab,
    currentTabLabel: formatStateLabel('all'),
    complaints: [] as MerchantComplaintCardView[],
    page: 0,
    limit: 20,
    hasMore: false,
    loadedCount: 0,
    respondedCount: 0,
    completedCount: 0
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.loadComplaints({ reset: true })
  },

  onShow() {
    if (!this.data.initialLoading && !this.data.loading && !this.data.loadingMore) {
      this.loadComplaints({ reset: true, silent: true })
    }
  },

  onPullDownRefresh() {
    this.loadComplaints({ reset: true, silent: this.data.complaints.length > 0 })
  },

  onTabChange(e: WechatMiniprogram.CustomEvent<{ value: ComplaintTab }>) {
    const currentTab = e.detail.value
    this.setData({ currentTab, currentTabLabel: formatStateLabel(currentTab) })
    this.loadComplaints({ reset: true })
  },

  onViewDetail(e: WechatMiniprogram.TouchEvent) {
    const { complaintId } = e.currentTarget.dataset as { complaintId?: string }
    if (!complaintId) return
    wx.navigateTo({ url: `/pages/merchant/complaints/detail/index?id=${encodeURIComponent(complaintId)}` })
  },

  onLoadMore() {
    if (!this.data.hasMore || this.data.loadingMore) return
    this.loadComplaints({ reset: false })
  },

  onRetry() {
    this.loadComplaints({ reset: true })
  },

  onRetryRefresh() {
    this.loadComplaints({ reset: true, silent: true })
  },

  async loadComplaints(options: { reset: boolean, silent?: boolean }) {
    const { reset, silent = false } = options
    if (reset && this.data.loading) return
    if (!reset && (this.data.loadingMore || !this.data.hasMore)) return

    const nextPage = reset ? 1 : this.data.page + 1
    const hasExistingComplaints = this.data.complaints.length > 0

    this.setData(reset
      ? (silent && hasExistingComplaints
          ? {
              loading: true,
              refreshErrorMessage: ''
            }
          : {
              loading: true,
              initialError: false,
              initialErrorMessage: '',
              refreshErrorMessage: '',
              complaints: [],
              page: 0,
              hasMore: false,
              loadedCount: 0,
              respondedCount: 0,
              completedCount: 0
            })
      : {
          loadingMore: true
        })

    try {
      const response = await listMerchantComplaints({
        state: this.data.currentTab === 'all' ? undefined : this.data.currentTab,
        page: nextPage,
        limit: this.data.limit
      })

      const nextItems = Array.isArray(response.complaints)
        ? response.complaints.map(toComplaintCard)
        : []
      const complaints = reset ? nextItems : this.data.complaints.concat(nextItems)

      this.setData({
        complaints,
        page: nextPage,
        hasMore: nextItems.length >= this.data.limit,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        loadedCount: complaints.length,
        respondedCount: complaints.filter((item) => item.responseStatusLabel !== '尚未回复，建议尽快处理').length,
        completedCount: complaints.filter((item) => item.stateLabel === '已完结').length
      })
    } catch (err) {
      logger.error('Load merchant complaints failed', err)
      const message = getErrorMessage(err, '投诉列表加载失败，请稍后重试')

      if (reset) {
        if (silent && hasExistingComplaints) {
          this.setData({
            initialLoading: false,
            refreshErrorMessage: `${message}，当前已保留上次同步结果`
          })
        } else {
          this.setData({
            initialLoading: false,
            initialError: true,
            initialErrorMessage: message,
            complaints: [],
            page: 0,
            hasMore: false,
            loadedCount: 0,
            respondedCount: 0,
            completedCount: 0
          })
        }
      } else {
        wx.showToast({ title: message, icon: 'none' })
      }
    } finally {
      this.setData({ loading: false, loadingMore: false })
      wx.stopPullDownRefresh()
    }
  }
})