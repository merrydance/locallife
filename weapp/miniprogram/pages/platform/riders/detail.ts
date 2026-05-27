import { responsiveBehavior } from '@/utils/responsive'
import {
  platformManagementService,
  type PlatformComplaintCategory,
  type PlatformRiderDetail
} from '@/api/platform-management'
import { getErrorUserMessage } from '@/utils/user-facing'
import { resolveStatusTagTheme, type StatusTagTheme } from '@/utils/status-tag'

type NavHeightEvent = WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>

interface ComplaintCategoryView extends PlatformComplaintCategory {
  countText: string
}

interface RiderDetailView extends PlatformRiderDetail {
  regionText: string
  ageText: string
  genderText: string
  activeLabel: string
  activeTheme: StatusTagTheme
  statusLabel: string
  totalIncomeText: string
  lastMonthIncomeText: string
  complaintText: string
  categories: ComplaintCategoryView[]
}

function formatMoney(fen?: number): string {
  if (typeof fen !== 'number' || !Number.isFinite(fen)) {
    return '--'
  }
  return `¥${(fen / 100).toFixed(2)}`
}

function riderStatusLabel(status: string): string {
  switch (status) {
    case 'active':
      return '可接单'
    case 'approved':
      return '已通过'
    case 'suspended':
      return '暂停接单'
    default:
      return status || '--'
  }
}

function buildRiderDetailView(detail: PlatformRiderDetail): RiderDetailView {
  const categories = (detail.service?.complaint_categories || []).map((item) => ({
    ...item,
    countText: `${item.count || 0} 次`
  }))
  return {
    ...detail,
    regionText: detail.basic?.region_name || '--',
    ageText: typeof detail.basic?.age === 'number' ? `${detail.basic.age} 岁` : '--',
    genderText: detail.basic?.gender || '--',
    activeLabel: detail.basic?.active ? '近3天活跃' : '近3天未接单',
    activeTheme: detail.basic?.active ? resolveStatusTagTheme('success') : resolveStatusTagTheme('warning'),
    statusLabel: riderStatusLabel(detail.basic?.status || ''),
    totalIncomeText: formatMoney(detail.order_stats?.total_income),
    lastMonthIncomeText: formatMoney(detail.order_stats?.last_month_income),
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
    riderID: 0,
    detail: null as RiderDetailView | null
  },

  onLoad(options: Record<string, string>) {
    const riderID = Number(options.id || 0)
    if (!riderID) {
      this.setData({ loading: false, error: '骑手ID无效' })
      return
    }

    this.setData({ riderID })
    this.loadDetail()
  },

  onNavHeight(e: NavHeightEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 0 })
  },

  async loadDetail() {
    if (this.data.requesting || !this.data.riderID) return

    this.setData({ loading: true, requesting: true, error: null })
    try {
      const detail = await platformManagementService.getPlatformRiderDetail(this.data.riderID)
      this.setData({ detail: buildRiderDetailView(detail) })
    } catch (error: unknown) {
      this.setData({ error: getErrorUserMessage(error, '加载骑手详情失败，请稍后重试') })
    } finally {
      this.setData({ loading: false, requesting: false })
    }
  },

  onRetry() {
    this.loadDetail()
  },

  async onToggleAccepting() {
    const detail = this.data.detail
    if (!detail || this.data.submitting) return

    const pause = detail.can_pause_accepting
    const confirm = await new Promise<boolean>((resolve) => {
      wx.showModal({
        title: pause ? '暂停接单' : '恢复接单',
        content: pause ? '确认暂停该骑手接单？' : '确认恢复该骑手接单？',
        success: (res) => resolve(res.confirm),
        fail: () => resolve(false)
      })
    })
    if (!confirm) return

    try {
      this.setData({ submitting: true })
      if (pause) {
        await platformManagementService.pausePlatformRiderAccepting(detail.id)
      } else {
        await platformManagementService.resumePlatformRiderAccepting(detail.id)
      }
      await this.loadDetail()
      wx.showToast({ title: pause ? '已暂停接单' : '已恢复接单', icon: 'success' })
    } catch (error: unknown) {
      wx.showToast({ title: getErrorUserMessage(error, '操作失败，请稍后重试'), icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  }
})
