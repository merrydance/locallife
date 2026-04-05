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
import { ensureMerchantConsoleAccess } from '../../../../utils/console-access'

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

const RECHARGE_RULES_AUTO_REFRESH_WINDOW_MS = 60 * 1000

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

function upsertRuleView(rules: RuleView[], rule: MerchantRechargeRuleResponse) {
  const nextRule = buildRuleView(rule)
  const index = rules.findIndex((item) => item.id === nextRule.id)

  if (index === -1) {
    return [nextRule, ...rules]
  }

  const nextRules = [...rules]
  nextRules[index] = nextRule
  return nextRules
}

function removeRuleView(rules: RuleView[], ruleId: number) {
  return rules.filter((item) => item.id !== ruleId)
}

function shouldAutoRefresh(lastLoadedAt: number, freshnessWindowMs: number) {
  return !lastLoadedAt || Date.now() - lastLoadedAt >= freshnessWindowMs
}

const getErrorMessage = getErrorUserMessage

Page({
  data: {
    navBarHeight: 88,
    accessReady: false,
    accessDenied: false,
    accessErrorMessage: '',
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    actionNoticeMessage: '',
    refreshErrorMessage: '',
    loading: false,
    submitting: false,
    merchantId: 0,
    lastLoadedAt: 0,
    rules: [] as RuleView[],
    formVisible: false,
    isEdit: false,
    editId: 0,
    form: defaultFormData()
  },

  async onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })

    const accessResult = await ensureMerchantConsoleAccess()
    this.setData({
      accessReady: true,
      accessDenied: accessResult.status === 'denied',
      accessErrorMessage: accessResult.status === 'error' ? accessResult.message : ''
    })
    if (accessResult.status !== 'granted') {
      this.setData({ initialLoading: false })
      return
    }

    this.loadPageData(true, true)
  },

  onRetryAccess() {
    this.setData({
      accessReady: false,
      accessDenied: false,
      accessErrorMessage: '',
      initialLoading: true,
      initialError: false,
      initialErrorMessage: ''
    })
    this.onLoad()
  },

  onShow() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    if (this.data.merchantId > 0 && !this.data.initialLoading && !this.data.submitting) {
      if (shouldAutoRefresh(this.data.lastLoadedAt, RECHARGE_RULES_AUTO_REFRESH_WINDOW_MS)) {
        this.loadRules(false)
      }
    }
  },

  onPullDownRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    this.loadPageData(false, true)
  },

  onRetry() {
    if (this.data.accessErrorMessage) {
      this.onRetryAccess()
      return
    }

    if (!this.data.accessReady || this.data.accessDenied) return
    this.loadPageData(true, true)
  },

  onRetryRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    this.loadPageData(false, true)
  },

  async ensureMerchantId() {
    if (this.data.merchantId > 0) {
      return this.data.merchantId
    }

    try {
      const cached = wx.getStorageSync('current_merchant') as { id?: number, merchant_id?: number } | null
      const cachedMerchantId = Number(cached?.id || cached?.merchant_id || 0)
      if (cachedMerchantId > 0) {
        this.setData({ merchantId: cachedMerchantId })
        return cachedMerchantId
      }

      const profile = await getMyMerchantProfile()
      const merchantId = Number(profile.id || 0)
      if (merchantId > 0) {
        this.setData({ merchantId })
        return merchantId
      }
      throw new Error('invalid merchant id')
    } catch (err) {
      logger.error('Init merchant recharge rules context failed', err)
      this.setData({
        initialLoading: false,
        initialError: true,
        initialErrorMessage: getErrorMessage(err, '获取商户信息失败，请重试'),
        refreshErrorMessage: ''
      })
      return 0
    }
  },

  async loadPageData(showLoading = true, force = false) {
    const merchantId = await this.ensureMerchantId()
    if (!merchantId) {
      wx.stopPullDownRefresh()
      return
    }

    await this.loadRules(showLoading, force)
  },

  async loadRules(showLoading = true, force = false) {
    if (this.data.loading || !this.data.merchantId) return

    const hasConfirmedData = this.data.lastLoadedAt > 0
    const isSilentRefresh = !showLoading && hasConfirmedData

    if (!force && hasConfirmedData && !shouldAutoRefresh(this.data.lastLoadedAt, RECHARGE_RULES_AUTO_REFRESH_WINDOW_MS)) {
      wx.stopPullDownRefresh()
      return
    }

    this.setData({
      loading: true,
      ...(showLoading
        ? { initialError: false, initialErrorMessage: '', refreshErrorMessage: '' }
        : isSilentRefresh
          ? { refreshErrorMessage: '' }
          : {})
    })

    try {
      const list = await listMerchantRechargeRules(this.data.merchantId)
      const rules = (Array.isArray(list) ? list : []).map(buildRuleView)
      this.setData({
        rules,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        lastLoadedAt: Date.now()
      })
    } catch (err) {
      logger.error('Load merchant recharge rules failed', err)
      const message = getErrorMessage(err, '加载充值规则失败，请稍后重试')

      if (this.data.initialLoading || !hasConfirmedData) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message,
          refreshErrorMessage: ''
        })
      } else {
        this.setData({ refreshErrorMessage: `${message}，当前已保留上次同步结果` })
      }
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  onAddRule() {
    this.setData({
      actionNoticeMessage: '',
      refreshErrorMessage: '',
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
      actionNoticeMessage: '',
      refreshErrorMessage: '',
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
    this.setData({
      actionNoticeMessage: '',
      refreshErrorMessage: '',
      [`form.${field}`]: e.detail.value
    })
  },

  onDateChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const { field } = e.currentTarget.dataset as { field?: 'valid_from' | 'valid_until' }
    if (!field) return
    this.setData({
      actionNoticeMessage: '',
      refreshErrorMessage: '',
      [`form.${field}`]: e.detail.value
    })
  },

  onSwitchChange(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    this.setData({
      actionNoticeMessage: '',
      refreshErrorMessage: '',
      'form.is_active': Boolean(e.detail.value)
    })
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
    if (this.data.submitting) return

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

      const wasEdit = this.data.isEdit
      let updatedRule: MerchantRechargeRuleResponse

      if (wasEdit && this.data.editId) {
        updatedRule = await updateMerchantRechargeRule(this.data.merchantId, this.data.editId, {
          ...payload,
          is_active: this.data.form.is_active
        })
      } else {
        updatedRule = await createMerchantRechargeRule(this.data.merchantId, payload)
      }

      this.setData({
        rules: upsertRuleView(this.data.rules, updatedRule),
        formVisible: false,
        isEdit: false,
        editId: 0,
        form: defaultFormData(),
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        actionNoticeMessage: wasEdit ? '充值规则已更新。' : '充值规则已创建。',
        refreshErrorMessage: '',
        lastLoadedAt: Date.now()
      })
      void this.loadRules(false, true)
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
          const updatedRule = await updateMerchantRechargeRule(this.data.merchantId, id, { is_active: !active })
          this.setData({
            rules: upsertRuleView(this.data.rules, updatedRule),
            initialLoading: false,
            initialError: false,
            initialErrorMessage: '',
            actionNoticeMessage: updatedRule.is_active ? '充值规则已启用。' : '充值规则已停用。',
            refreshErrorMessage: '',
            lastLoadedAt: Date.now()
          })
          void this.loadRules(false, true)
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
          this.setData({
            rules: removeRuleView(this.data.rules, id),
            initialLoading: false,
            initialError: false,
            initialErrorMessage: '',
            actionNoticeMessage: '充值规则已删除。',
            refreshErrorMessage: '',
            lastLoadedAt: Date.now()
          })
          void this.loadRules(false, true)
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