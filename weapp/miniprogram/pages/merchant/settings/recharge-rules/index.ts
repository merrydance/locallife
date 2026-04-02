import dayjs from 'dayjs'
import {
  createMerchantRechargeRule,
  deleteMerchantRechargeRule,
  getMyMerchantProfile,
  listMerchantRechargeRules,
  MerchantRechargeRuleResponse,
  updateMerchantRechargeRule
} from '../../../../api/merchant'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'

type RuleStatusTheme = 'success' | 'warning' | 'danger' | 'default'

interface RuleView extends MerchantRechargeRuleResponse {
  recharge_amount_text: string
  bonus_amount_text: string
  total_amount_text: string
  valid_range_text: string
  status_label: string
  status_theme: RuleStatusTheme
}

interface RuleFormData {
  recharge_amount_yuan: string
  bonus_amount_yuan: string
  valid_from: string
  valid_until: string
  is_active: boolean
}

function formatAmount(amount: number) {
  return `¥${(amount / 100).toFixed(2)}`
}

function toRFC3339Start(date: string) {
  return `${date}T00:00:00+08:00`
}

function toRFC3339End(date: string) {
  return `${date}T23:59:59+08:00`
}

function defaultFormData(): RuleFormData {
  return {
    recharge_amount_yuan: '',
    bonus_amount_yuan: '',
    valid_from: '',
    valid_until: '',
    is_active: true
  }
}

function buildStatus(rule: MerchantRechargeRuleResponse) {
  const now = dayjs()
  const validFrom = dayjs(rule.valid_from)
  const validUntil = dayjs(rule.valid_until)

  if (!rule.is_active) {
    return { label: '已停用', theme: 'default' as RuleStatusTheme }
  }
  if (validUntil.isValid() && now.isAfter(validUntil)) {
    return { label: '已过期', theme: 'danger' as RuleStatusTheme }
  }
  if (validFrom.isValid() && now.isBefore(validFrom)) {
    return { label: '未开始', theme: 'warning' as RuleStatusTheme }
  }
  return { label: '生效中', theme: 'success' as RuleStatusTheme }
}

function buildRuleView(rule: MerchantRechargeRuleResponse): RuleView {
  const status = buildStatus(rule)
  return {
    ...rule,
    recharge_amount_text: formatAmount(rule.recharge_amount),
    bonus_amount_text: formatAmount(rule.bonus_amount),
    total_amount_text: formatAmount(rule.recharge_amount + rule.bonus_amount),
    valid_range_text: `${dayjs(rule.valid_from).format('YYYY-MM-DD')} 至 ${dayjs(rule.valid_until).format('YYYY-MM-DD')}`,
    status_label: status.label,
    status_theme: status.theme
  }
}

function toFormData(rule: MerchantRechargeRuleResponse): RuleFormData {
  return {
    recharge_amount_yuan: (rule.recharge_amount / 100).toFixed(2),
    bonus_amount_yuan: (rule.bonus_amount / 100).toFixed(2),
    valid_from: dayjs(rule.valid_from).format('YYYY-MM-DD'),
    valid_until: dayjs(rule.valid_until).format('YYYY-MM-DD'),
    is_active: rule.is_active
  }
}

const getErrorMessage = getErrorUserMessage

Page({
  data: {
    navBarHeight: 88,
    loading: false,
    submitting: false,
    merchantId: 0,
    rules: [] as RuleView[],
    formVisible: false,
    isEdit: false,
    editId: 0,
    form: defaultFormData()
  },

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.initMerchantId()
  },

  onShow() {
    if (this.data.merchantId > 0) {
      this.loadRules()
    }
  },

  onPullDownRefresh() {
    this.loadRules()
  },

  async initMerchantId() {
    try {
      const cached = wx.getStorageSync('current_merchant') as { id?: number, merchant_id?: number } | null
      const cachedMerchantId = Number(cached?.id || cached?.merchant_id || 0)
      if (cachedMerchantId > 0) {
        this.setData({ merchantId: cachedMerchantId })
        await this.loadRules()
        return
      }

      const profile = await getMyMerchantProfile()
      this.setData({ merchantId: profile.id })
      await this.loadRules()
    } catch (err) {
      logger.error('Init merchant recharge rules context failed', err)
      wx.showToast({ title: '获取商户信息失败', icon: 'none' })
    }
  },

  async loadRules() {
    if (this.data.loading || !this.data.merchantId) return
    this.setData({ loading: true })
    try {
      const list = await listMerchantRechargeRules(this.data.merchantId)
      const rules = (Array.isArray(list) ? list : []).map(buildRuleView)
      this.setData({ rules })
    } catch (err) {
      logger.error('Load merchant recharge rules failed', err)
      wx.showToast({ title: getErrorMessage(err, '加载充值规则失败，请稍后重试'), icon: 'none' })
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  onAddRule() {
    this.setData({
      formVisible: true,
      isEdit: false,
      editId: 0,
      form: defaultFormData()
    })
  },

  onEditRule(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    const rule = this.data.rules.find((item) => item.id === id)
    if (!rule) return

    this.setData({
      formVisible: true,
      isEdit: true,
      editId: id,
      form: toFormData(rule)
    })
  },

  onCloseForm() {
    this.setData({ formVisible: false })
  },

  onTextInput(e: WechatMiniprogram.Input) {
    const { field } = e.currentTarget.dataset as { field?: keyof RuleFormData }
    if (!field) return
    this.setData({ [`form.${field}`]: e.detail.value })
  },

  onDateChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const { field } = e.currentTarget.dataset as { field?: 'valid_from' | 'valid_until' }
    if (!field) return
    this.setData({ [`form.${field}`]: e.detail.value })
  },

  onSwitchChange(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    this.setData({ 'form.is_active': Boolean(e.detail.value) })
  },

  validateForm() {
    const { form } = this.data
    if (!form.recharge_amount_yuan || Number(form.recharge_amount_yuan) <= 0) return '请输入有效的充值金额'
    if (!form.bonus_amount_yuan && form.bonus_amount_yuan !== '0') return '请输入赠送金额，没有赠送请填 0'
    if (Number(form.bonus_amount_yuan) < 0) return '赠送金额不能小于 0'
    if (!form.valid_from) return '请选择开始日期'
    if (!form.valid_until) return '请选择结束日期'
    if (form.valid_until < form.valid_from) return '结束日期不能早于开始日期'
    return ''
  },

  async onSubmitForm() {
    const errorMessage = this.validateForm()
    if (errorMessage) {
      wx.showToast({ title: errorMessage, icon: 'none' })
      return
    }

    this.setData({ submitting: true })
    wx.showLoading({ title: '保存中...' })
    try {
      const payload = {
        recharge_amount: Math.round(Number(this.data.form.recharge_amount_yuan) * 100),
        bonus_amount: Math.round(Number(this.data.form.bonus_amount_yuan) * 100),
        valid_from: toRFC3339Start(this.data.form.valid_from),
        valid_until: toRFC3339End(this.data.form.valid_until)
      }

      if (this.data.isEdit && this.data.editId) {
        await updateMerchantRechargeRule(this.data.merchantId, this.data.editId, {
          ...payload,
          is_active: this.data.form.is_active
        })
      } else {
        await createMerchantRechargeRule(this.data.merchantId, payload)
      }

      this.setData({ formVisible: false })
      await this.loadRules()
    } catch (err) {
      logger.error('Submit merchant recharge rule failed', err)
      wx.showToast({ title: getErrorMessage(err, this.data.isEdit ? '更新充值规则失败，请稍后重试' : '创建充值规则失败，请稍后重试'), icon: 'none' })
    } finally {
      this.setData({ submitting: false })
      wx.hideLoading()
    }
  },

  onToggleRuleStatus(e: WechatMiniprogram.TouchEvent) {
    const { id, active } = e.currentTarget.dataset as { id?: number, active?: boolean }
    if (!id || typeof active !== 'boolean') return

    wx.showModal({
      title: active ? '停用充值规则' : '启用充值规则',
      content: `${active ? '停用后顾客将看不到该充值活动' : '启用后顾客可在会员充值入口看到该活动'}。`,
      success: async (res) => {
        if (!res.confirm) return
        wx.showLoading({ title: '处理中...' })
        try {
          await updateMerchantRechargeRule(this.data.merchantId, id, { is_active: !active })
          await this.loadRules()
        } catch (err) {
          logger.error('Toggle merchant recharge rule status failed', err)
          wx.showToast({ title: getErrorMessage(err, '更新状态失败，请稍后重试'), icon: 'none' })
        } finally {
          wx.hideLoading()
        }
      }
    })
  },

  onDeleteRule(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return

    wx.showModal({
      title: '确认删除',
      content: '删除后不可恢复，确认删除这条充值规则吗？',
      confirmColor: '#e34d59',
      success: async (res) => {
        if (!res.confirm) return
        wx.showLoading({ title: '删除中...' })
        try {
          await deleteMerchantRechargeRule(this.data.merchantId, id)
          await this.loadRules()
        } catch (err) {
          logger.error('Delete merchant recharge rule failed', err)
          wx.showToast({ title: getErrorMessage(err, '删除充值规则失败，请稍后重试'), icon: 'none' })
        } finally {
          wx.hideLoading()
        }
      }
    })
  }
})