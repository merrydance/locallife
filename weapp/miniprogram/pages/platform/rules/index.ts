import { responsiveBehavior } from '@/utils/responsive'
import {
  platformManagementService,
  type PlatformOperatorRuleItem
} from '@/api/platform-management'

type NavHeightEvent = WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>
type CategoryChangeEvent = WechatMiniprogram.TouchEvent & {
  currentTarget: {
    dataset: {
      val?: string
    }
  }
}
type RuleActionEvent = WechatMiniprogram.TouchEvent & {
  currentTarget: {
    dataset: {
      key?: string
      name?: string
    }
  }
}

interface PlatformRuleCategory {
  label: string
  value: string
  icon: string
}

interface PlatformRuleViewItem extends PlatformOperatorRuleItem {
  categoryKey: string
  categoryLabel: string
  status: 'active'
}

function normalizeCategory(value?: string): string {
  const raw = String(value || '').trim().toLowerCase()
  if (!raw) return 'platform'
  return raw
}

function displayCategory(value: string): string {
  const key = normalizeCategory(value)
  if (key === 'platform') return '平台维护'
  if (key === 'finance') return '结算'
  return '平台维护'
}

Page({
  behaviors: [responsiveBehavior],
  data: {
    navBarHeight: 0,
    loading: false,
    submitting: false,
    error: null as string | null,
    total: 0,
    activeCategory: 'all',
    categories: [
      { label: '全部', value: 'all', icon: 'app' }
    ] as PlatformRuleCategory[],
    rules: [] as PlatformRuleViewItem[],
    categorizedRules: {
      all: [] as PlatformRuleViewItem[]
    } as Record<string, PlatformRuleViewItem[]>,
    showEdit: false,
    editingRule: null as PlatformRuleViewItem | null,
    newValue: ''
  },

  onLoad() {
    this.loadRules()
  },

  onNavHeight(e: NavHeightEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 0 })
  },

  async loadRules() {
    if (this.data.loading) return

    this.setData({ loading: true, error: null })
    try {
      const response = await platformManagementService.getPlatformOperatorRules()
      const mapped = (response.rules || []).map((rule) => {
        const categoryKey = normalizeCategory(rule.category)
        return {
          ...rule,
          categoryKey,
          categoryLabel: displayCategory(categoryKey),
          status: 'active' as const
        }
      })

      const categoryKeys = Array.from(new Set(mapped.map((item) => item.categoryKey)))
      const categories: PlatformRuleCategory[] = [
        { label: '全部', value: 'all', icon: 'app' },
        ...categoryKeys.map((key) => ({
          label: displayCategory(key),
          value: key,
          icon: key === 'finance' ? 'money' : 'setting'
        }))
      ]

      const categorized: Record<string, PlatformRuleViewItem[]> = { all: mapped }
      categoryKeys.forEach((key) => {
        categorized[key] = mapped.filter((item) => item.categoryKey === key)
      })

      this.setData({
        rules: mapped,
        categorizedRules: categorized,
        categories,
        total: mapped.length,
        activeCategory: categories.some((c) => c.value === this.data.activeCategory) ? this.data.activeCategory : 'all'
      })
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : '加载规则失败，请稍后重试'
      this.setData({ error: message })
      wx.showToast({ title: '加载失败', icon: 'none' })
    } finally {
      this.setData({ loading: false })
    }
  },

  onRetry() {
    this.loadRules()
  },

  onCategoryChange(e: CategoryChangeEvent) {
    const next = String(e.currentTarget.dataset.val || 'all')
    this.setData({ activeCategory: next })
  },

  onEditTap(e: RuleActionEvent) {
    const key = String(e.currentTarget.dataset.key || '')
    if (!key) return

    const rule = this.data.rules.find((item) => item.key === key)
    if (!rule) return

    this.setData({
      showEdit: true,
      editingRule: rule,
      newValue: rule.value
    })
  },

  onValueChange(e: WechatMiniprogram.CustomEvent<{ value?: string }>) {
    const value = e?.detail?.value
    this.setData({ newValue: typeof value === 'string' ? value : '' })
  },

  onCloseEdit() {
    this.setData({ showEdit: false, editingRule: null, newValue: '' })
  },

  async onConfirmEdit() {
    const { editingRule, newValue, submitting } = this.data
    if (!editingRule || submitting || newValue === '') return

    if (newValue === editingRule.value) {
      wx.showToast({ title: '值未变化', icon: 'none' })
      return
    }

    try {
      this.setData({ submitting: true })
      await platformManagementService.updatePlatformOperatorRule(editingRule.key, { value: newValue })
      wx.showToast({ title: '更新成功', icon: 'success' })
      this.setData({ showEdit: false, editingRule: null, newValue: '' })
      await this.loadRules()
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : '更新失败'
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  }
})
