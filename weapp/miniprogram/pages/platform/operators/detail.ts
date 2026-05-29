import { responsiveBehavior } from '@/utils/responsive'
import {
  platformManagementService,
  type PlatformComplaintCategory,
  type PlatformOperatorDetail
} from '../_api/platform-management'
import { getErrorUserMessage } from '@/utils/user-facing'
import { type StatusTagTheme } from '../_main_shared/utils/status-tag'
import { buildPlatformOperatorStatusView } from '../_utils/platform-status-view'

type NavHeightEvent = WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>

interface ComplaintCategoryView extends PlatformComplaintCategory {
  countText: string
}

interface OperatorDetailView extends PlatformOperatorDetail {
  statusLabel: string
  statusTheme: StatusTagTheme
  regionText: string
  merchantText: string
  lastMonthRevenueText: string
  complaintText: string
  categories: ComplaintCategoryView[]
}

function formatMoney(fen?: number): string {
  if (typeof fen !== 'number' || !Number.isFinite(fen)) {
    return '--'
  }
  return `¥${(fen / 100).toFixed(2)}`
}

function buildOperatorDetailView(detail: PlatformOperatorDetail): OperatorDetailView {
  const categories = (detail.service?.complaint_categories || []).map((item) => ({
    ...item,
    countText: `${item.count || 0} 次`
  }))
  const statusView = buildPlatformOperatorStatusView(detail.status)
  return {
    ...detail,
    statusLabel: statusView.label,
    statusTheme: statusView.theme,
    regionText: `${detail.region_count || 0} 个`,
    merchantText: `${detail.merchant_count || 0} 家`,
    lastMonthRevenueText: formatMoney(detail.order_stats?.last_month_income),
    complaintText: `${detail.service?.complaint_count || 0} 次`,
    categories
  }
}

Page({
  behaviors: [responsiveBehavior],
  data: {
    navBarHeight: 0,
    loading: true,
    requesting: false,
    submitting: false,
    error: null as string | null,
    actionResultText: '',
    actionResultNote: '',
    operatorID: 0,
    detail: null as OperatorDetailView | null
  },

  onLoad(options: Record<string, string>) {
    const operatorID = Number(options.id || 0)
    if (!operatorID) {
      this.setData({ loading: false, error: '运营商ID无效' })
      return
    }

    this.setData({ operatorID })
    this.loadDetail()
  },

  onNavHeight(e: NavHeightEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 0 })
  },

  async loadDetail() {
    if (this.data.requesting || !this.data.operatorID) return

    this.setData({ loading: true, requesting: true, error: null })
    try {
      const detail = await platformManagementService.getPlatformOperatorDetail(this.data.operatorID)
      this.setData({ detail: buildOperatorDetailView(detail) })
    } catch (error: unknown) {
      this.setData({ error: getErrorUserMessage(error, '加载运营商详情失败，请稍后重试') })
    } finally {
      this.setData({ loading: false, requesting: false })
    }
  },

  onRetry() {
    this.loadDetail()
  },

  async onToggleStatus() {
    const detail = this.data.detail
    if (!detail || this.data.submitting) return

    const suspend = detail.can_suspend
    const confirm = await new Promise<boolean>((resolve) => {
      wx.showModal({
        title: suspend ? '暂停运营商' : '恢复运营商',
        content: suspend ? '确认暂停该运营商？' : '确认恢复该运营商？',
        success: (res) => resolve(res.confirm),
        fail: () => resolve(false)
      })
    })
    if (!confirm) return

    try {
      this.setData({ submitting: true })
      this.setData({
        actionResultText: suspend ? '已提交暂停运营商' : '已提交恢复运营商',
        actionResultNote: '正在回读后端状态'
      })
      await platformManagementService.updatePlatformOperatorStatus(detail.id, suspend ? 'suspended' : 'active')
      await this.loadDetail()
      this.setData({
        actionResultText: suspend ? '运营商已暂停' : '运营商已恢复',
        actionResultNote: `当前状态：${this.data.detail?.statusLabel || '已同步'}`
      })
    } catch (error: unknown) {
      this.setData({
        actionResultText: '状态同步失败',
        actionResultNote: getErrorUserMessage(error, '操作失败，请稍后重试')
      })
    } finally {
      this.setData({ submitting: false })
    }
  }
})
