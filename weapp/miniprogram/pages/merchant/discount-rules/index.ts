import dayjs from 'dayjs'
import {
  createMerchantDiscountRule,
  deleteMerchantDiscountRule,
  getMyMerchantProfile,
  listMerchantDiscountRules,
  MerchantDiscountRuleResponse,
  updateMerchantDiscountRule
} from '../../../api/merchant'
import { logger } from '../../../utils/logger'
import { getStableBarHeights } from '../../../utils/responsive'
import { getErrorUserMessage } from '../../../utils/user-facing'

type DiscountStatusTheme = 'success' | 'warning' | 'danger' | 'default'

interface DiscountRuleView extends MerchantDiscountRuleResponse {
  min_order_amount_text: string
  discount_amount_text: string
  valid_range_text: string
  stacking_text: string
  status_label: string
  status_theme: DiscountStatusTheme
}

interface DiscountRuleFormData {
  name: string
  description: string
  min_order_amount_yuan: string
  discount_amount_yuan: string
  can_stack_with_voucher: boolean
  can_stack_with_membership: boolean
  stacking_group: string
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

function defaultFormData(): DiscountRuleFormData {
  return {
    name: '',
    description: '',
    min_order_amount_yuan: '',
    discount_amount_yuan: '',
    can_stack_with_voucher: false,
    can_stack_with_membership: false,
    stacking_group: '',
    valid_from: '',
    valid_until: '',
    is_active: true
  }
}

function buildStatus(rule: MerchantDiscountRuleResponse) {
  const now = dayjs()
  const validFrom = dayjs(rule.valid_from)
  const validUntil = dayjs(rule.valid_until)

  if (!rule.is_active) {
    return { label: '已停用', theme: 'default' as DiscountStatusTheme }
  }
  if (validUntil.isValid() && now.isAfter(validUntil)) {
    return { label: '已过期', theme: 'danger' as DiscountStatusTheme }
  }
  if (validFrom.isValid() && now.isBefore(validFrom)) {
    return { label: '未开始', theme: 'warning' as DiscountStatusTheme }
  }
  return { label: '生效中', theme: 'success' as DiscountStatusTheme }
}

function buildStackingText(rule: MerchantDiscountRuleResponse) {
  const tags: string[] = []
  if (rule.can_stack_with_voucher) tags.push('可叠加代金券')
  if (rule.can_stack_with_membership) tags.push('可叠加会员权益')
  if (rule.stacking_group) tags.push(`分组 ${rule.stacking_group}`)
  return tags.length ? tags.join(' · ') : '默认不与其他优惠叠加'
}

function buildRuleView(rule: MerchantDiscountRuleResponse): DiscountRuleView {
  const status = buildStatus(rule)
  return {
    ...rule,
    min_order_amount_text: formatAmount(rule.min_order_amount),
    discount_amount_text: formatAmount(rule.discount_amount),
    valid_range_text: `${dayjs(rule.valid_from).format('YYYY-MM-DD')} 至 ${dayjs(rule.valid_until).format('YYYY-MM-DD')}`,
    stacking_text: buildStackingText(rule),
    status_label: status.label,
    status_theme: status.theme
  }
}

function toFormData(rule: MerchantDiscountRuleResponse): DiscountRuleFormData {
  return {
    name: rule.name,
    description: rule.description || '',
    min_order_amount_yuan: (rule.min_order_amount / 100).toFixed(2),
    discount_amount_yuan: (rule.discount_amount / 100).toFixed(2),
    can_stack_with_voucher: rule.can_stack_with_voucher,
    can_stack_with_membership: rule.can_stack_with_membership,
    stacking_group: rule.stacking_group || '',
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
    rules: [] as DiscountRuleView[],
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
      logger.error('Init merchant discount rules context failed', err)
      wx.showToast({ title: '获取商户信息失败', icon: 'none' })
    }
  },

  async loadRules() {
    if (this.data.loading || !this.data.merchantId) return
    this.setData({ loading: true })
    try {
      const response = await listMerchantDiscountRules(this.data.merchantId, 1, 50)
      this.setData({ rules: (response.rules || []).map(buildRuleView) })
    } catch (err) {
      logger.error('Load merchant discount rules failed', err)
      wx.showToast({ title: getErrorMessage(err, '加载满减规则失败，请稍后重试'), icon: 'none' })
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  onAddRule() {
    this.setData({ formVisible: true, isEdit: false, editId: 0, form: defaultFormData() })
  },

  onEditRule(e: WechatMiniprogram.TouchEvent) {
    const { id } = e.currentTarget.dataset as { id?: number }
    if (!id) return
    const rule = this.data.rules.find((item) => item.id === id)
    if (!rule) return
    this.setData({ formVisible: true, isEdit: true, editId: id, form: toFormData(rule) })
  },

  onCloseForm() {
    this.setData({ formVisible: false })
  },

  onTextInput(e: WechatMiniprogram.Input) {
    const { field } = e.currentTarget.dataset as { field?: keyof DiscountRuleFormData }
    if (!field) return
    this.setData({ [`form.${field}`]: e.detail.value })
  },

  onDateChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const { field } = e.currentTarget.dataset as { field?: 'valid_from' | 'valid_until' }
    if (!field) return
    this.setData({ [`form.${field}`]: e.detail.value })
  },

  onSwitchChange(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    const { field } = e.currentTarget.dataset as { field?: 'can_stack_with_voucher' | 'can_stack_with_membership' | 'is_active' }
    if (!field) return
    this.setData({ [`form.${field}`]: Boolean(e.detail.value) })
  },

  validateForm() {
    const { form } = this.data
    const minOrderAmount = Number(form.min_order_amount_yuan)
    const discountAmount = Number(form.discount_amount_yuan)

    if (!form.name.trim()) return '请填写规则名称'
    if (!Number.isFinite(minOrderAmount) || minOrderAmount <= 0) return '请输入有效的门槛金额'
    if (!Number.isFinite(discountAmount) || discountAmount <= 0) return '请输入有效的优惠金额'
    if (discountAmount >= minOrderAmount) return '优惠金额需小于门槛金额'
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
        name: this.data.form.name.trim(),
        description: this.data.form.description.trim() || undefined,
        min_order_amount: Math.round(Number(this.data.form.min_order_amount_yuan) * 100),
        discount_amount: Math.round(Number(this.data.form.discount_amount_yuan) * 100),
        can_stack_with_voucher: this.data.form.can_stack_with_voucher,
        can_stack_with_membership: this.data.form.can_stack_with_membership,
        stacking_group: this.data.form.stacking_group.trim() || undefined,
        valid_from: toRFC3339Start(this.data.form.valid_from),
        valid_until: toRFC3339End(this.data.form.valid_until)
      }

      if (this.data.isEdit && this.data.editId) {
        await updateMerchantDiscountRule(this.data.merchantId, this.data.editId, {
          ...payload,
          is_active: this.data.form.is_active
        })
      } else {
        await createMerchantDiscountRule(this.data.merchantId, payload)
      }

      this.setData({ formVisible: false })
      await this.loadRules()
    } catch (err) {
      logger.error('Submit merchant discount rule failed', err)
      wx.showToast({ title: getErrorMessage(err, this.data.isEdit ? '更新满减规则失败，请稍后重试' : '创建满减规则失败，请稍后重试'), icon: 'none' })
    } finally {
      this.setData({ submitting: false })
      wx.hideLoading()
    }
  },

  onToggleRuleStatus(e: WechatMiniprogram.TouchEvent) {
    const { id, active } = e.currentTarget.dataset as { id?: number, active?: boolean }
    if (!id || typeof active !== 'boolean') return
    wx.showLoading({ title: '处理中...' })
    updateMerchantDiscountRule(this.data.merchantId, id, { is_active: !active })
      .then(() => this.loadRules())
      .catch((err) => {
        logger.error('Toggle merchant discount rule status failed', err)
        wx.showToast({ title: getErrorMessage(err, '更新状态失败，请稍后重试'), icon: 'none' })
      })
      .finally(() => wx.hideLoading())
  },

  onDeleteRule(e: WechatMiniprogram.TouchEvent) {
    const { id, name } = e.currentTarget.dataset as { id?: number, name?: string }
    if (!id) return
    wx.showModal({
      title: '确认删除',
      content: `删除「${name || '该规则'}」后不可恢复，确认继续吗？`,
      confirmColor: '#e34d59',
      success: async (res) => {
        if (!res.confirm) return
        wx.showLoading({ title: '删除中...' })
        try {
          await deleteMerchantDiscountRule(this.data.merchantId, id)
          await this.loadRules()
        } catch (err) {
          logger.error('Delete merchant discount rule failed', err)
          wx.showToast({ title: getErrorMessage(err, '删除满减规则失败，请稍后重试'), icon: 'none' })
        } finally {
          wx.hideLoading()
        }
      }
    })
  }
})