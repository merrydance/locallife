import { getStableBarHeights } from '../../../../../utils/responsive'
import {
  createMerchantRechargeRule,
  listMerchantRechargeRules,
  type MerchantRechargeRuleResponse,
  updateMerchantRechargeRule
} from '../../../../../api/merchant'
import { ensureMerchantConsoleAccess } from '../../../../../utils/console-access'
import { syncCurrentMerchantContext } from '../../../../../utils/current-merchant'
import { logger } from '../../../../../utils/logger'
import { getErrorUserMessage } from '../../../../../utils/user-facing'

interface RechargeRuleEditOptions {
  id?: string
}

interface RechargeRuleFormData {
  recharge_amount_yuan: string
  bonus_amount_yuan: string
  valid_from: string
  valid_until: string
  is_active: boolean
}

interface CurrencyInputState {
  fen: number
  hasValue: boolean
  isValid: boolean
}

function createDefaultFormData(): RechargeRuleFormData {
  return {
    recharge_amount_yuan: '',
    bonus_amount_yuan: '',
    valid_from: '',
    valid_until: '',
    is_active: true
  }
}

function formatDate(date: string) {
  return String(date || '').slice(0, 10)
}

function buildFormData(rule: MerchantRechargeRuleResponse): RechargeRuleFormData {
  return {
    recharge_amount_yuan: (rule.recharge_amount / 100).toFixed(2),
    bonus_amount_yuan: (rule.bonus_amount / 100).toFixed(2),
    valid_from: formatDate(rule.valid_from),
    valid_until: formatDate(rule.valid_until),
    is_active: !!rule.is_active
  }
}

function parseCurrencyInput(value: string): CurrencyInputState {
  const trimmedValue = String(value || '').trim()
  if (!trimmedValue) {
    return { fen: 0, hasValue: false, isValid: false }
  }

  if (!/^\d+(\.\d{0,2})?$/.test(trimmedValue)) {
    return { fen: 0, hasValue: true, isValid: false }
  }

  const parsedValue = Number(trimmedValue)
  if (!Number.isFinite(parsedValue)) {
    return { fen: 0, hasValue: true, isValid: false }
  }

  return {
    fen: Math.round(parsedValue * 100),
    hasValue: true,
    isValid: true
  }
}

function toRFC3339Start(date: string): string {
  return `${date}T00:00:00+08:00`
}

function toRFC3339End(date: string): string {
  return `${date}T23:59:59+08:00`
}

Page({
  data: {
    navBarHeight: 88,
    accessReady: false,
    accessDenied: false,
    accessErrorMessage: '',
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    submitting: false,
    isEdit: false,
    merchantId: 0,
    ruleId: 0,
    formData: createDefaultFormData() as RechargeRuleFormData
  },

  async onLoad(options: RechargeRuleEditOptions) {
    const { navBarHeight } = getStableBarHeights()
    const ruleId = Number(options.id || 0)

    this.setData({
      navBarHeight,
      isEdit: ruleId > 0,
      ruleId
    })

    await this.bootstrapPage()
  },

  async bootstrapPage() {
    this.setData({
      accessReady: false,
      accessDenied: false,
      accessErrorMessage: '',
      initialLoading: true,
      initialError: false,
      initialErrorMessage: '',
      submitting: false
    })

    const accessResult = await ensureMerchantConsoleAccess()
    if (accessResult.status !== 'granted') {
      this.setData({
        accessReady: true,
        accessDenied: accessResult.status === 'denied',
        accessErrorMessage: accessResult.status === 'error' ? accessResult.message : '',
        initialLoading: false
      })
      return
    }

    try {
      const merchantContext = await syncCurrentMerchantContext()
      if (!merchantContext.merchantId) {
        throw new Error('invalid merchant id')
      }

      this.setData({
        accessReady: true,
        accessDenied: false,
        accessErrorMessage: '',
        merchantId: merchantContext.merchantId
      })

      if (!this.data.isEdit) {
        this.setData({
          initialLoading: false,
          initialError: false,
          initialErrorMessage: '',
          formData: createDefaultFormData()
        })
        return
      }

      await this.loadRuleDetail(merchantContext.merchantId)
    } catch (err) {
      logger.error('Bootstrap recharge rule edit page failed', err)
      this.setData({
        accessReady: true,
        initialLoading: false,
        initialError: true,
        initialErrorMessage: getErrorUserMessage(err, '获取商户信息失败，请稍后重试')
      })
    }
  },

  async loadRuleDetail(merchantId?: number) {
    const targetMerchantId = Number(merchantId || this.data.merchantId)

    this.setData({
      initialLoading: true,
      initialError: false,
      initialErrorMessage: ''
    })

    try {
      const rules = await listMerchantRechargeRules(targetMerchantId)
      const targetRule = (Array.isArray(rules) ? rules : []).find((item) => item.id === this.data.ruleId)

      if (!targetRule) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: '未找到该充值规则，可能已被删除'
        })
        return
      }

      this.setData({
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        formData: buildFormData(targetRule)
      })
    } catch (err) {
      logger.error('Load recharge rule detail failed', err)
      this.setData({
        initialLoading: false,
        initialError: true,
        initialErrorMessage: getErrorUserMessage(err, '充值规则详情加载失败，请稍后重试')
      })
    }
  },

  onRetryAccess() {
    void this.bootstrapPage()
  },

  onRetry() {
    void this.bootstrapPage()
  },

  onFormInputChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const { field } = e.currentTarget.dataset as { field?: keyof RechargeRuleFormData }
    if (!field) {
      return
    }

    this.setData({ [`formData.${field}`]: String(e.detail.value || '') })
  },

  onDateChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const { field } = e.currentTarget.dataset as { field?: keyof RechargeRuleFormData }
    if (!field) {
      return
    }

    this.setData({ [`formData.${field}`]: String(e.detail.value || '') })
  },

  onSwitchChange(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    const { field } = e.currentTarget.dataset as { field?: keyof RechargeRuleFormData }
    if (!field) {
      return
    }

    this.setData({ [`formData.${field}`]: !!e.detail.value })
  },

  async onSubmit() {
    if (this.data.submitting || this.data.initialLoading || this.data.initialError) {
      return
    }

    const { formData, isEdit, merchantId, ruleId } = this.data
    const rechargeAmount = parseCurrencyInput(formData.recharge_amount_yuan)
    if (!rechargeAmount.hasValue || !rechargeAmount.isValid || rechargeAmount.fen <= 0) {
      wx.showToast({ title: '请输入有效的充值金额', icon: 'none' })
      return
    }

    const bonusAmount = parseCurrencyInput(formData.bonus_amount_yuan)
    if (!bonusAmount.hasValue || !bonusAmount.isValid || bonusAmount.fen < 0) {
      wx.showToast({ title: '请输入有效的赠送金额', icon: 'none' })
      return
    }

    if (!formData.valid_from) {
      wx.showToast({ title: '请选择开始日期', icon: 'none' })
      return
    }

    if (!formData.valid_until) {
      wx.showToast({ title: '请选择结束日期', icon: 'none' })
      return
    }

    if (formData.valid_until < formData.valid_from) {
      wx.showToast({ title: '结束日期不能早于开始日期', icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    wx.showLoading({ title: '保存中...' })

    try {
      if (isEdit && ruleId > 0) {
        await updateMerchantRechargeRule(merchantId, ruleId, {
          recharge_amount: rechargeAmount.fen,
          bonus_amount: bonusAmount.fen,
          valid_from: toRFC3339Start(formData.valid_from),
          valid_until: toRFC3339End(formData.valid_until),
          is_active: formData.is_active
        })
      } else {
        await createMerchantRechargeRule(merchantId, {
          recharge_amount: rechargeAmount.fen,
          bonus_amount: bonusAmount.fen,
          valid_from: toRFC3339Start(formData.valid_from),
          valid_until: toRFC3339End(formData.valid_until)
        })
      }

      wx.showToast({ title: isEdit ? '充值规则已更新' : '充值规则已创建', icon: 'success' })
      wx.navigateBack()
    } catch (err) {
      logger.error('Submit recharge rule failed', err)
      wx.showToast({
        title: getErrorUserMessage(err, isEdit ? '更新失败，请稍后重试' : '创建失败，请稍后重试'),
        icon: 'none'
      })
    } finally {
      wx.hideLoading()
      this.setData({ submitting: false })
    }
  }
})