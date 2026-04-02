import { responsiveBehavior } from '@/utils/responsive'
import {
  platformManagementService,
  type PlatformOperatorRuleItem,
  type PlatformProfitSharingConfigItem
} from '@/api/platform-management'
import { getErrorUserMessage } from '@/utils/user-facing'

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
    commissionSubmitting: false,
    error: null as string | null,
    total: 0,
    commissionConfigId: 0,
    platformRateInput: '',
    operatorRateInput: '',
    commissionDialogPlatformRate: '',
    commissionDialogOperatorRate: '',
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
    this.loadProfitSharingConfig()
  },

  onNavHeight(e: NavHeightEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 0 })
  },

  async loadRules() {
    if (this.data.loading) return

    this.setData({ loading: true, error: null })
    try {
      const response = await platformManagementService.getPlatformOperatorRules()
      const rawRules = response.rules || []
      const platformCommissionRule = rawRules.find((rule) => rule.key === 'PLATFORM_COMMISSION')
      const operatorCommissionRule = rawRules.find((rule) => rule.key === 'OPERATOR_COMMISSION')

      const mapped = rawRules
        .filter((rule) => rule.key !== 'PLATFORM_COMMISSION' && rule.key !== 'OPERATOR_COMMISSION')
        .map((rule) => {
        const categoryKey = normalizeCategory(rule.category)
        return {
          ...rule,
          categoryKey,
          categoryLabel: displayCategory(categoryKey),
          status: 'active' as const
        }
        })

      if (platformCommissionRule && operatorCommissionRule) {
        const platformRate = String(platformCommissionRule.value || '')
        const operatorRate = String(operatorCommissionRule.value || '')

        mapped.unshift({
          id: 'platform_rule_commission_combined',
          name: '佣金比例配置',
          key: 'COMMISSION_CONFIG',
          value: `平台 ${platformRate}% / 运营商 ${operatorRate}%`,
          unit: '',
          desc: '统一维护平台与运营商佣金比例',
          category: 'platform',
          editable: true,
          categoryKey: 'platform',
          categoryLabel: '平台维护',
          status: 'active'
        })

        this.setData({
          platformRateInput: platformRate,
          operatorRateInput: operatorRate
        })
      }

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
      const message = getErrorUserMessage(error, '加载规则失败，请稍后重试')
      this.setData({ error: message })
    } finally {
      this.setData({ loading: false })
    }
  },

  onRetry() {
    this.loadRules()
    this.loadProfitSharingConfig()
  },

  async loadProfitSharingConfig() {
    try {
      const response = await platformManagementService.listPlatformProfitSharingConfigs({
        status: 'active',
        order_source: 'all',
        page: 1,
        limit: 50
      })

      const globalConfig = (response.items || []).find(
        (item: PlatformProfitSharingConfigItem) => !item.region_id && !item.merchant_id
      )

      if (globalConfig) {
        this.setData({
          commissionConfigId: globalConfig.id,
          platformRateInput: String(globalConfig.platform_rate),
          operatorRateInput: String(globalConfig.operator_rate)
        })
      }
    } catch {
      // 分账配置加载失败时不阻断规则页；佣金展示可回退到规则接口
    }
  },

  onPlatformRateChange(e: WechatMiniprogram.CustomEvent<{ value?: string }>) {
    this.setData({ platformRateInput: String(e?.detail?.value || '') })
  },

  onOperatorRateChange(e: WechatMiniprogram.CustomEvent<{ value?: string }>) {
    this.setData({ operatorRateInput: String(e?.detail?.value || '') })
  },

  async onSaveProfitSharingConfig() {
    const { commissionSubmitting, commissionConfigId, platformRateInput, operatorRateInput } = this.data
    if (commissionSubmitting) return

    const platformRate = Number(platformRateInput)
    const operatorRate = Number(operatorRateInput)

    if (!Number.isFinite(platformRate) || !Number.isFinite(operatorRate)) {
      wx.showToast({ title: '请输入有效数字', icon: 'none' })
      return
    }
    if (platformRate < 0 || platformRate > 100 || operatorRate < 0 || operatorRate > 100) {
      wx.showToast({ title: '比例需在0-100之间', icon: 'none' })
      return
    }
    if (platformRate + operatorRate > 100) {
      wx.showToast({ title: '比例之和不能超过100', icon: 'none' })
      return
    }

    try {
      this.setData({ commissionSubmitting: true, error: null })
      const payload = {
        status: 'active',
        order_source: 'all',
        platform_rate: Math.round(platformRate),
        operator_rate: Math.round(operatorRate),
        rider_enabled: true,
        priority: 100
      }

      if (commissionConfigId > 0) {
        await platformManagementService.updatePlatformProfitSharingConfig(commissionConfigId, payload)
      } else {
        await platformManagementService.createPlatformProfitSharingConfig(payload)
      }

      await this.loadProfitSharingConfig()
      await this.loadRules()
    } catch (error: unknown) {
      const message = getErrorUserMessage(error, '保存失败，请稍后重试')
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      this.setData({ commissionSubmitting: false })
    }
  },

  onCategoryChange(e: CategoryChangeEvent) {
    const next = String(e.currentTarget.dataset.val || 'all')
    this.setData({ activeCategory: next })
  },

  onCommissionDialogPlatformRateChange(e: WechatMiniprogram.CustomEvent<{ value?: string }>) {
    this.setData({ commissionDialogPlatformRate: String(e?.detail?.value || '') })
  },

  onCommissionDialogOperatorRateChange(e: WechatMiniprogram.CustomEvent<{ value?: string }>) {
    this.setData({ commissionDialogOperatorRate: String(e?.detail?.value || '') })
  },

  onEditTap(e: RuleActionEvent) {
    const key = String(e.currentTarget.dataset.key || '')
    if (!key) return

    const rule = this.data.rules.find((item) => item.key === key)
    if (!rule) return

    if (rule.key === 'COMMISSION_CONFIG') {
      this.setData({
        showEdit: true,
        editingRule: rule,
        commissionDialogPlatformRate: this.data.platformRateInput,
        commissionDialogOperatorRate: this.data.operatorRateInput,
        newValue: ''
      })
      return
    }

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
    this.setData({
      showEdit: false,
      editingRule: null,
      newValue: '',
      commissionDialogPlatformRate: '',
      commissionDialogOperatorRate: ''
    })
  },

  async onConfirmEdit() {
    const { editingRule, newValue, submitting } = this.data
    if (!editingRule || submitting) return

    if (editingRule.key === 'COMMISSION_CONFIG') {
      const platformRate = Number(this.data.commissionDialogPlatformRate)
      const operatorRate = Number(this.data.commissionDialogOperatorRate)

      if (!Number.isFinite(platformRate) || !Number.isFinite(operatorRate)) {
        wx.showToast({ title: '请输入有效数字', icon: 'none' })
        return
      }
      if (platformRate < 0 || platformRate > 100 || operatorRate < 0 || operatorRate > 100) {
        wx.showToast({ title: '比例需在0-100之间', icon: 'none' })
        return
      }
      if (platformRate + operatorRate > 100) {
        wx.showToast({ title: '比例之和不能超过100', icon: 'none' })
        return
      }

      try {
        this.setData({ commissionSubmitting: true, submitting: true })

        let configID = this.data.commissionConfigId
        if (configID <= 0) {
          const latest = await platformManagementService.listPlatformProfitSharingConfigs({
            status: 'active',
            order_source: 'all',
            page: 1,
            limit: 50
          })
          const globalConfig = (latest.items || []).find(
            (item: PlatformProfitSharingConfigItem) => !item.region_id && !item.merchant_id
          )
          configID = globalConfig?.id || 0
        }

        const payload = {
          status: 'active',
          order_source: 'all',
          platform_rate: Math.round(platformRate),
          operator_rate: Math.round(operatorRate),
          rider_enabled: true,
          priority: 100
        }

        if (configID > 0) {
          await platformManagementService.updatePlatformProfitSharingConfig(configID, payload)
        } else {
          await platformManagementService.createPlatformProfitSharingConfig(payload)
        }

        this.setData({
          showEdit: false,
          editingRule: null,
          newValue: '',
          commissionDialogPlatformRate: '',
          commissionDialogOperatorRate: ''
        })
        await this.loadProfitSharingConfig()
        await this.loadRules()
      } catch (error: unknown) {
        const message = getErrorUserMessage(error, '更新失败，请稍后重试')
        wx.showToast({ title: message, icon: 'none' })
      } finally {
        this.setData({ commissionSubmitting: false, submitting: false })
      }
      return
    }

    if (newValue === '') return

    if (newValue === editingRule.value) {
      wx.showToast({ title: '值未变化', icon: 'none' })
      return
    }

    try {
      this.setData({ submitting: true })
      await platformManagementService.updatePlatformOperatorRule(editingRule.key, { value: newValue })
      this.setData({ showEdit: false, editingRule: null, newValue: '' })
      await this.loadRules()
    } catch (error: unknown) {
      const message = getErrorUserMessage(error, '更新失败，请稍后重试')
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      this.setData({ submitting: false })
    }
  }
})
