import { getStableBarHeights } from '../../../../utils/responsive'
import {
  buildMerchantRechargeRuleStatusView,
  deleteMerchantRechargeRule,
  listMerchantRechargeRules,
  type MerchantRechargeRuleResponse,
  updateMerchantRechargeRule
} from '../../../../api/merchant'
import { ensureMerchantConsoleAccess } from '../../../../utils/console-access'
import { logger } from '../../../../utils/logger'
import { syncCurrentMerchantContext } from '../../_utils/current-merchant'
import { getErrorUserMessage } from '../../../../utils/user-facing'

interface RechargeRuleView extends MerchantRechargeRuleResponse {
  recharge_amount_yuan: string
  bonus_amount_yuan: string
  total_amount_yuan: string
  valid_range_text: string
  statusPending: boolean
  deletePending: boolean
}

const RECHARGE_RULES_AUTO_REFRESH_WINDOW_MS = 60 * 1000

function formatAmount(amount: number) {
  return (amount / 100).toFixed(2)
}

function formatDate(date: string) {
  return String(date || '').slice(0, 10)
}

function buildRuleView(rule: MerchantRechargeRuleResponse): RechargeRuleView {
  const statusView = buildMerchantRechargeRuleStatusView(rule)

  return {
    ...rule,
    status_code: statusView.code,
    status_label: statusView.label,
    status_theme: statusView.theme,
    recharge_amount_yuan: formatAmount(rule.recharge_amount),
    bonus_amount_yuan: formatAmount(rule.bonus_amount),
    total_amount_yuan: formatAmount(rule.recharge_amount + rule.bonus_amount),
    valid_range_text: `${formatDate(rule.valid_from)} 至 ${formatDate(rule.valid_until)}`,
    statusPending: false,
    deletePending: false
  }
}

function buildResultSummaryText(visibleCount: number) {
  return `当前共 ${visibleCount} 条充值规则`
}

function buildPresentationUpdate(rules: RechargeRuleView[]) {
  return {
    rules,
    resultSummaryText: buildResultSummaryText(rules.length),
    emptyDescription: '当前还没有充值规则，先新增一个'
  }
}

function shouldAutoRefresh(lastLoadedAt: number, freshnessWindowMs: number) {
  return !lastLoadedAt || Date.now() - lastLoadedAt >= freshnessWindowMs
}

function upsertRuleView(rules: RechargeRuleView[], rule: MerchantRechargeRuleResponse) {
  const nextRule = buildRuleView(rule)
  const index = rules.findIndex((item) => item.id === nextRule.id)

  if (index === -1) {
    return [nextRule, ...rules]
  }

  const nextRules = [...rules]
  nextRules[index] = nextRule
  return nextRules
}

function removeRuleView(rules: RechargeRuleView[], ruleId: number) {
  return rules.filter((item) => item.id !== ruleId)
}

function buildEditPageUrl(ruleId?: number) {
  if (ruleId && ruleId > 0) {
    return `/pages/merchant/settings/recharge-rules/edit/index?id=${ruleId}`
  }

  return '/pages/merchant/settings/recharge-rules/edit/index'
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
    refreshErrorMessage: '',
    loading: false,
    rules: [] as RechargeRuleView[],
    resultSummaryText: '当前共 0 条充值规则',
    emptyDescription: '当前还没有充值规则，先新增一个',
    merchantId: 0,
    lastLoadedAt: 0,
    needsReloadOnShow: false,
    deleteDialogVisible: false,
    deleteDialogSubmitting: false,
    deleteDialogRuleId: 0,
    deleteDialogRuleName: ''
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

    await this.loadPageData(true, true)
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
    void this.onLoad()
  },

  async onShow() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      return
    }

    if (this.data.initialLoading || this.data.loading || this.data.deleteDialogSubmitting) {
      return
    }

    const needsReloadOnShow = !!this.data.needsReloadOnShow
    if (needsReloadOnShow) {
      this.setData({ needsReloadOnShow: false })
    }

    const merchantChanged = await this.syncMerchantContext()
    if (merchantChanged === null) {
      return
    }

    if (merchantChanged) {
      await this.loadRules(true, true)
      return
    }

    if (needsReloadOnShow && this.data.merchantId > 0) {
      await this.loadRules(false, true)
      return
    }

    if (this.data.merchantId > 0 && shouldAutoRefresh(this.data.lastLoadedAt, RECHARGE_RULES_AUTO_REFRESH_WINDOW_MS)) {
      await this.loadRules(false)
    }
  },

  onPullDownRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      wx.stopPullDownRefresh()
      return
    }

    void this.loadPageData(false, true)
  },

  onRetry() {
    if (this.data.accessErrorMessage) {
      this.onRetryAccess()
      return
    }

    if (!this.data.accessReady || this.data.accessDenied) {
      return
    }

    void this.loadPageData(true, true)
  },

  onRetryRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      wx.stopPullDownRefresh()
      return
    }

    void this.loadPageData(false, true)
  },

  async syncMerchantContext(): Promise<boolean | null> {
    try {
      const context = await syncCurrentMerchantContext({ currentMerchantId: this.data.merchantId })

      if (context.changed) {
        this.setData({
          merchantId: context.merchantId,
          lastLoadedAt: 0,
          initialLoading: true,
          initialError: false,
          initialErrorMessage: '',
          refreshErrorMessage: '',
          needsReloadOnShow: false,
          deleteDialogVisible: false,
          deleteDialogSubmitting: false,
          deleteDialogRuleId: 0,
          deleteDialogRuleName: '',
          ...buildPresentationUpdate([])
        })
        return true
      }

      if (context.merchantId !== this.data.merchantId) {
        this.setData({ merchantId: context.merchantId })
      }

      return false
    } catch (err) {
      logger.error('Sync merchant recharge rules context failed', err)
      const message = getErrorUserMessage(err, '获取商户信息失败，请重试')

      if (!this.data.lastLoadedAt && !this.data.rules.length) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message,
          refreshErrorMessage: ''
        })
      } else {
        this.setData({ refreshErrorMessage: `${message}，当前已保留上次同步结果` })
      }

      return null
    }
  },

  async loadPageData(showLoading = true, force = false) {
    const merchantChanged = await this.syncMerchantContext()
    if (merchantChanged === null) {
      wx.stopPullDownRefresh()
      return
    }

    if (!this.data.merchantId) {
      wx.stopPullDownRefresh()
      return
    }

    await this.loadRules(showLoading, force || merchantChanged)
  },

  async loadRules(showLoading = true, force = false) {
    if (this.data.loading || !this.data.merchantId) {
      wx.stopPullDownRefresh()
      return
    }

    const hasConfirmedData = this.data.rules.length > 0 || this.data.lastLoadedAt > 0
    if (!force && hasConfirmedData && !shouldAutoRefresh(this.data.lastLoadedAt, RECHARGE_RULES_AUTO_REFRESH_WINDOW_MS)) {
      wx.stopPullDownRefresh()
      return
    }

    this.setData({
      loading: true,
      ...(showLoading && !hasConfirmedData
        ? {
            initialLoading: true,
            initialError: false,
            initialErrorMessage: '',
            refreshErrorMessage: ''
          }
        : hasConfirmedData
          ? {
              initialError: false,
              initialErrorMessage: '',
              refreshErrorMessage: ''
            }
          : {})
    })

    try {
      const list = await listMerchantRechargeRules(this.data.merchantId)
      const rules = (Array.isArray(list) ? list : []).map(buildRuleView)

      this.setData({
        ...buildPresentationUpdate(rules),
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        lastLoadedAt: Date.now()
      })
    } catch (err) {
      logger.error('Load merchant recharge rules failed', err)
      const message = getErrorUserMessage(err, '加载充值规则失败，请稍后重试')

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
    if (this.data.deleteDialogSubmitting || this.data.loading) {
      return
    }

    this.setData({ refreshErrorMessage: '', needsReloadOnShow: true })

    wx.navigateTo({
      url: buildEditPageUrl(),
      fail: (err) => {
        logger.error('Navigate to recharge rule create page failed', err)
        this.setData({ needsReloadOnShow: false })
        wx.showToast({ title: '打开新建页失败，请稍后重试', icon: 'none' })
      }
    })
  },

  onEditRule(e: WechatMiniprogram.TouchEvent) {
    if (this.data.deleteDialogSubmitting || this.data.loading) {
      return
    }

    const id = Number((e.currentTarget.dataset as { id?: number | string }).id || 0)
    const rule = this.data.rules.find((item) => item.id === id)
    if (!rule || rule.statusPending || rule.deletePending) {
      return
    }

    this.setData({ refreshErrorMessage: '', needsReloadOnShow: true })

    wx.navigateTo({
      url: buildEditPageUrl(id),
      fail: (err) => {
        logger.error('Navigate to recharge rule edit page failed', err)
        this.setData({ needsReloadOnShow: false })
        wx.showToast({ title: '打开编辑页失败，请稍后重试', icon: 'none' })
      }
    })
  },

  onActionsCatch() {},

  async onToggleRuleStatus(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    const id = Number((e.currentTarget.dataset as { id?: number | string }).id || 0)
    if (!id) {
      return
    }

    const targetRule = this.data.rules.find((item) => item.id === id)
    if (!targetRule || targetRule.statusPending || targetRule.deletePending) {
      return
    }

    const targetActive = !!e.detail?.value
    if (targetActive === targetRule.is_active) {
      return
    }

    const pendingRules = this.data.rules.map((rule) => (
      rule.id === id ? { ...rule, statusPending: true } : rule
    ))

    this.setData(buildPresentationUpdate(pendingRules))

    try {
      const updatedRule = await updateMerchantRechargeRule(this.data.merchantId, id, {
        is_active: targetActive
      })

      const nextRules = upsertRuleView(pendingRules, updatedRule)
      this.setData({
        ...buildPresentationUpdate(nextRules),
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        lastLoadedAt: Date.now()
      })
      wx.showToast({ title: updatedRule.is_active ? '充值规则已启用' : '充值规则已停用', icon: 'none' })
    } catch (err) {
      logger.error('Toggle merchant recharge rule status failed', err)
      const restoredRules = pendingRules.map((rule) => (
        rule.id === id ? { ...targetRule, statusPending: false } : rule
      ))
      this.setData(buildPresentationUpdate(restoredRules))
      wx.showToast({ title: getErrorUserMessage(err, '更新状态失败，请稍后重试'), icon: 'none' })
    }
  },

  onRequestDeleteRule(e: WechatMiniprogram.TouchEvent) {
    const dataset = e.currentTarget.dataset as { id?: number | string, name?: string }
    const id = Number(dataset.id || 0)
    if (!id || this.data.deleteDialogSubmitting) {
      return
    }

    const rule = this.data.rules.find((item) => item.id === id)
    if (!rule || rule.statusPending || rule.deletePending) {
      return
    }

    this.setData({
      deleteDialogVisible: true,
      deleteDialogSubmitting: false,
      deleteDialogRuleId: id,
      deleteDialogRuleName: dataset.name || `充${rule.recharge_amount_yuan}赠${rule.bonus_amount_yuan}`
    })
  },

  onCancelDeleteDialog() {
    if (this.data.deleteDialogSubmitting) {
      return
    }

    this.setData({
      deleteDialogVisible: false,
      deleteDialogRuleId: 0,
      deleteDialogRuleName: ''
    })
  },

  async onConfirmDeleteRule() {
    if (this.data.deleteDialogSubmitting || !this.data.deleteDialogRuleId) {
      return
    }

    const ruleId = this.data.deleteDialogRuleId
    const pendingRules = this.data.rules.map((rule) => (
      rule.id === ruleId ? { ...rule, deletePending: true } : rule
    ))

    this.setData({
      ...buildPresentationUpdate(pendingRules),
      deleteDialogSubmitting: true
    })

    try {
      await deleteMerchantRechargeRule(this.data.merchantId, ruleId)
      const nextRules = removeRuleView(pendingRules, ruleId)
      this.setData({
        ...buildPresentationUpdate(nextRules),
        deleteDialogVisible: false,
        deleteDialogSubmitting: false,
        deleteDialogRuleId: 0,
        deleteDialogRuleName: '',
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        lastLoadedAt: Date.now()
      })
      wx.showToast({ title: '充值规则已删除', icon: 'none' })
    } catch (err) {
      logger.error('Delete merchant recharge rule failed', err)
      const restoredRules = pendingRules.map((rule) => (
        rule.id === ruleId ? { ...rule, deletePending: false } : rule
      ))
      this.setData({
        ...buildPresentationUpdate(restoredRules),
        deleteDialogSubmitting: false
      })
      wx.showToast({ title: getErrorUserMessage(err, '删除充值规则失败，请稍后重试'), icon: 'none' })
    }
  }
})