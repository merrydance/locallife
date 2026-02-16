import { responsiveBehavior } from '@/utils/responsive'
import {
  platformManagementService,
  type PlatformRuleItem
} from '@/api/platform-management'

type NavHeightEvent = WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>
type TapEvent = WechatMiniprogram.CustomEvent & {
  currentTarget: {
    dataset: {
      id?: number | string
      name?: string
    }
  }
}

function getRuleStatusLabel(status: string): string {
  if (status === 'active') return '生效中'
  if (status === 'disabled') return '已停用'
  if (status === 'draft') return '草稿'
  return status || '未知'
}

Page({
  behaviors: [responsiveBehavior],
  data: {
    navBarHeight: 0,
    loading: false,
    requesting: false,
    refreshing: false,
    submitting: false,
    error: null as string | null,
    offset: 0,
    limit: 20,
    total: 0,
    hasMore: false,
    rules: [] as PlatformRuleItem[]
  },

  onLoad() {
    this.loadRules(true)
  },

  onNavHeight(e: NavHeightEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 0 })
  },

  async onRefresh() {
    this.setData({ refreshing: true })
    try {
      await this.loadRules(true)
    } finally {
      this.setData({ refreshing: false })
    }
  },

  async onLoadMore() {
    if (!this.data.hasMore || this.data.loading) {
      return
    }
    await this.loadRules(false)
  },

  async loadRules(reset: boolean) {
    if (this.data.requesting) {
      return
    }

    const offset = reset ? 0 : this.data.offset + this.data.limit
    this.setData({ loading: true, requesting: true, error: null })
    try {
      const response = await platformManagementService.getPlatformRules({
        limit: this.data.limit,
        offset
      })
      const nextRules = reset ? response.rules : this.data.rules.concat(response.rules)

      this.setData({
        rules: nextRules,
        offset,
        total: reset ? response.count : Math.max(response.count, nextRules.length),
        hasMore: response.rules.length >= this.data.limit
      })
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : '加载规则失败，请稍后重试'
      this.setData({ error: message })
      wx.showToast({ title: '加载失败', icon: 'none' })
    } finally {
      this.setData({ loading: false, requesting: false })
    }
  },

  onRetry() {
    this.loadRules(true)
  },

  getStatusLabel(status: string) {
    return getRuleStatusLabel(status)
  },

  async onDisableTap(e: TapEvent) {
    const ruleID = Number(e.currentTarget.dataset.id || 0)
    if (!ruleID || this.data.submitting) return

    const ruleName = String(e.currentTarget.dataset.name || `规则#${ruleID}`)
    const confirm = await new Promise<boolean>((resolve) => {
      wx.showModal({
        title: '停用规则',
        content: `确认停用「${ruleName}」？`,
        success: (res) => resolve(res.confirm),
        fail: () => resolve(false)
      })
    })
    if (!confirm) return

    try {
      this.setData({ submitting: true })
      await platformManagementService.disablePlatformRule(ruleID, '平台管理中心手动停用')
      wx.showToast({ title: '已停用', icon: 'success' })
      await this.loadRules(true)
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : '停用失败'
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  }
})
