import { responsiveBehavior } from '@/utils/responsive'
import {
  platformManagementService,
  type PlatformOperationalConfigItem
} from '@/api/platform-management'
import { getErrorUserMessage } from '@/utils/user-facing'

type NavHeightEvent = WechatMiniprogram.CustomEvent<{ navBarHeight?: number }>

type PlatformRuleKey =
  | 'PLATFORM_COMMISSION'
  | 'OPERATOR_COMMISSION'
  | 'RIDER_DEPOSIT'

type RuleValueMap = Record<PlatformRuleKey, string>

interface SaveRuleUpdate {
  key: PlatformRuleKey
  value: string
}

const EMPTY_RULE_VALUES: RuleValueMap = {
  PLATFORM_COMMISSION: '',
  OPERATOR_COMMISSION: '',
  RIDER_DEPOSIT: ''
}

function findRuleValue(rules: PlatformOperationalConfigItem[], key: PlatformRuleKey): string {
  const rule = rules.find((item) => item.key === key)
  return String(rule?.value || '')
}

function buildRuleValueMap(rules: PlatformOperationalConfigItem[]): RuleValueMap {
  return {
    PLATFORM_COMMISSION: findRuleValue(rules, 'PLATFORM_COMMISSION'),
    OPERATOR_COMMISSION: findRuleValue(rules, 'OPERATOR_COMMISSION'),
    RIDER_DEPOSIT: findRuleValue(rules, 'RIDER_DEPOSIT')
  }
}

function hasValueChanged(currentValue: string, nextValue: string): boolean {
  const currentText = String(currentValue || '').trim()
  const nextText = String(nextValue || '').trim()

  if (!currentText && !nextText) {
    return false
  }

  const currentNumber = Number(currentText)
  const nextNumber = Number(nextText)

  if (Number.isFinite(currentNumber) && Number.isFinite(nextNumber)) {
    return Math.abs(currentNumber - nextNumber) >= 0.0001
  }

  return currentText !== nextText
}

function normalizeIntegerRate(value: number): string {
  return String(Math.round(value))
}

function normalizeMoneyValue(value: number): string {
  if (Number.isInteger(value)) {
    return String(value)
  }

  return value.toFixed(2).replace(/(\.\d*?[1-9])0+$/, '$1').replace(/\.0+$/, '')
}

Page({
  behaviors: [responsiveBehavior],
  data: {
    navBarHeight: 0,
    loading: false,
    commissionSubmitting: false,
    depositSubmitting: false,
    error: null as string | null,
    total: 0,
    ruleValues: { ...EMPTY_RULE_VALUES } as RuleValueMap,
    platformRateInput: '',
    operatorRateInput: '',
    riderDepositInput: ''
  },

  onLoad() {
    this.loadRules()
  },

  onNavHeight(e: NavHeightEvent) {
    this.setData({ navBarHeight: e.detail.navBarHeight || 0 })
  },

  async loadRules(options: { showLoading?: boolean } = {}) {
    const { showLoading = true } = options
    if (showLoading && this.data.loading) return

    if (showLoading) {
      this.setData({ loading: true, error: null })
    }

    try {
      const response = await platformManagementService.getPlatformOperationalConfigs()
      const rules = response.rules || []
      const ruleValues = buildRuleValueMap(rules)

      this.setData({
        total: rules.length,
        ruleValues,
        platformRateInput: ruleValues.PLATFORM_COMMISSION,
        operatorRateInput: ruleValues.OPERATOR_COMMISSION,
        riderDepositInput: ruleValues.RIDER_DEPOSIT,
        error: null
      })
    } catch (error: unknown) {
      const message = getErrorUserMessage(error, '加载规则失败，请稍后重试')
      this.setData({ error: message })
    } finally {
      if (showLoading) {
        this.setData({ loading: false })
      }
    }
  },

  onRetry() {
    this.loadRules()
  },

  onPlatformRateChange(e: WechatMiniprogram.CustomEvent<{ value?: string }>) {
    this.setData({ platformRateInput: String(e?.detail?.value || '') })
  },

  onOperatorRateChange(e: WechatMiniprogram.CustomEvent<{ value?: string }>) {
    this.setData({ operatorRateInput: String(e?.detail?.value || '') })
  },

  onRiderDepositChange(e: WechatMiniprogram.CustomEvent<{ value?: string }>) {
    this.setData({ riderDepositInput: String(e?.detail?.value || '') })
  },

  async applyRuleUpdates(updates: SaveRuleUpdate[]) {
    let hasUpdated = false

    try {
      for (const update of updates) {
        await platformManagementService.updatePlatformOperationalConfig(update.key, { value: update.value })
        hasUpdated = true
      }

      await this.loadRules({ showLoading: false })
    } catch (error) {
      if (hasUpdated) {
        await this.loadRules({ showLoading: false })
      }
      throw error
    }
  },

  async onSaveCommissionConfig() {
    const { commissionSubmitting, platformRateInput, operatorRateInput, ruleValues } = this.data
    if (commissionSubmitting) return

    const platformRate = Number(platformRateInput)
    const operatorRate = Number(operatorRateInput)

    if (!Number.isInteger(platformRate) || !Number.isInteger(operatorRate)) {
      wx.showToast({ title: '请输入 0-100 的整数比例', icon: 'none' })
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

    const normalizedPlatformRate = normalizeIntegerRate(platformRate)
    const normalizedOperatorRate = normalizeIntegerRate(operatorRate)
    const updates: SaveRuleUpdate[] = []

    if (hasValueChanged(ruleValues.PLATFORM_COMMISSION, normalizedPlatformRate)) {
      updates.push({ key: 'PLATFORM_COMMISSION', value: normalizedPlatformRate })
    }
    if (hasValueChanged(ruleValues.OPERATOR_COMMISSION, normalizedOperatorRate)) {
      updates.push({ key: 'OPERATOR_COMMISSION', value: normalizedOperatorRate })
    }

    if (updates.length === 0) {
      wx.showToast({ title: '值未变化', icon: 'none' })
      return
    }

    try {
      this.setData({ commissionSubmitting: true })
      await this.applyRuleUpdates(updates)
      wx.showToast({ title: '佣金配置已保存', icon: 'success' })
    } catch (error: unknown) {
      const message = getErrorUserMessage(error, '保存失败，请稍后重试')
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      this.setData({ commissionSubmitting: false })
    }
  },

  async onSaveDepositConfig() {
    const { depositSubmitting, riderDepositInput, ruleValues } = this.data
    if (depositSubmitting) return

    const riderDeposit = Number(riderDepositInput)

    if (!Number.isFinite(riderDeposit)) {
      wx.showToast({ title: '请输入有效金额', icon: 'none' })
      return
    }
    if (riderDeposit < 0) {
      wx.showToast({ title: '金额不能小于0', icon: 'none' })
      return
    }

    const normalizedRiderDeposit = normalizeMoneyValue(riderDeposit)
    const updates: SaveRuleUpdate[] = []

    if (hasValueChanged(ruleValues.RIDER_DEPOSIT, normalizedRiderDeposit)) {
      updates.push({ key: 'RIDER_DEPOSIT', value: normalizedRiderDeposit })
    }

    if (updates.length === 0) {
      wx.showToast({ title: '值未变化', icon: 'none' })
      return
    }

    try {
      this.setData({ depositSubmitting: true })
      await this.applyRuleUpdates(updates)
      wx.showToast({ title: '骑手押金已保存', icon: 'success' })
    } catch (error: unknown) {
      const message = getErrorUserMessage(error, '保存失败，请稍后重试')
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      this.setData({ depositSubmitting: false })
    }
  }
})
