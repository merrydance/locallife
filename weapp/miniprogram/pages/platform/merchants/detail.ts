import { responsiveBehavior } from '@/utils/responsive'
import {
  platformManagementService,
  type PlatformComplaintCategory,
  type PlatformMerchantDetail
} from '@/api/platform-management'
import { getErrorUserMessage } from '@/utils/user-facing'
import { resolveStatusTagTheme, type StatusTagTheme } from '@/utils/status-tag'

type NavHeightEvent = WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>

interface ComplaintCategoryView extends PlatformComplaintCategory {
  countText: string
}

interface MerchantDetailView extends PlatformMerchantDetail {
  statusLabel: string
  statusTheme: StatusTagTheme
  regionText: string
  openLabel: string
  phoneText: string
  addressText: string
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

function merchantStatusLabel(status: string): string {
  switch (status) {
    case 'active':
      return '正常'
    case 'approved':
      return '已通过'
    case 'suspended':
      return '已暂停'
    case 'pending':
      return '待处理'
    case 'rejected':
      return '已拒绝'
    default:
      return status || '--'
  }
}

function merchantStatusTheme(status: string): StatusTagTheme {
  switch (status) {
    case 'active':
    case 'approved':
      return resolveStatusTagTheme('success')
    case 'suspended':
    case 'rejected':
      return resolveStatusTagTheme('danger')
    case 'pending':
      return resolveStatusTagTheme('warning')
    default:
      return resolveStatusTagTheme('warning')
  }
}

function buildMerchantDetailView(detail: PlatformMerchantDetail): MerchantDetailView {
  const categories = (detail.service?.complaint_categories || []).map((item) => ({
    ...item,
    countText: `${item.count || 0} 次`
  }))

  return {
    ...detail,
    statusLabel: merchantStatusLabel(detail.basic?.status || ''),
    statusTheme: merchantStatusTheme(detail.basic?.status || ''),
    regionText: detail.basic?.region_name || '--',
    openLabel: detail.basic?.is_open ? '营业中' : '休息中',
    phoneText: detail.basic?.phone || '--',
    addressText: detail.basic?.address || '--',
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
    merchantID: 0,
    detail: null as MerchantDetailView | null
  },

  onLoad(options: Record<string, string>) {
    const merchantID = Number(options.id || 0)
    if (!merchantID) {
      this.setData({ loading: false, error: '商户ID无效' })
      return
    }

    this.setData({ merchantID })
    this.loadDetail()
  },

  onNavHeight(e: NavHeightEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 0 })
  },

  async loadDetail() {
    if (this.data.requesting || !this.data.merchantID) return

    this.setData({ loading: true, requesting: true, error: null })
    try {
      const detail = await platformManagementService.getPlatformMerchantDetail(this.data.merchantID)
      this.setData({ detail: buildMerchantDetailView(detail) })
    } catch (error: unknown) {
      this.setData({ error: getErrorUserMessage(error, '加载商户详情失败，请稍后重试') })
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
        title: suspend ? '暂停商户' : '恢复商户',
        content: suspend ? '确认暂停该商户？' : '确认恢复该商户？',
        success: (res) => resolve(res.confirm),
        fail: () => resolve(false)
      })
    })
    if (!confirm) return

    try {
      this.setData({ submitting: true })
      if (suspend) {
        await platformManagementService.suspendPlatformMerchant(detail.id)
      } else {
        await platformManagementService.resumePlatformMerchant(detail.id)
      }
      await this.loadDetail()
      wx.showToast({ title: suspend ? '已暂停' : '已恢复', icon: 'success' })
    } catch (error: unknown) {
      wx.showToast({ title: getErrorUserMessage(error, '操作失败，请稍后重试'), icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  }
})
