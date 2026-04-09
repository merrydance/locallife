import {
  getMyMerchantMembershipSettings,
  MerchantMembershipScene,
  MerchantMembershipSettingsResponse,
  updateMyMerchantMembershipSettings
} from '../../../../api/merchant'
import Toast from '../../../../miniprogram_npm/tdesign-miniprogram/toast/index'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'
import { getErrorUserMessage } from '../../../../utils/user-facing'
import { ensureMerchantConsoleAccess } from '../../../../utils/console-access'

interface MembershipFormState {
  allow_with_voucher: boolean
  allow_with_discount: boolean
  max_deduction_percent: string
  balanceScenes: Record<MerchantMembershipScene, boolean>
  bonusScenes: Record<MerchantMembershipScene, boolean>
}

const SCENE_OPTIONS: Array<{ key: MerchantMembershipScene, label: string, desc: string }> = [
  { key: 'takeout', label: '外卖订单', desc: '顾客在外卖下单时可使用会员余额或赠送金' },
  { key: 'dine_in', label: '堂食订单', desc: '顾客在线下桌台或堂食下单时可使用' },
  { key: 'reservation', label: '预订订单', desc: '顾客支付预订或预订加菜时可使用' }
]

const MEMBERSHIP_AUTO_REFRESH_WINDOW_MS = 60 * 1000

function createSceneState(selected: MerchantMembershipScene[]) {
  const selectedSet = new Set(selected)
  return {
    takeout: selectedSet.has('takeout'),
    dine_in: selectedSet.has('dine_in'),
    reservation: selectedSet.has('reservation')
  }
}

function buildForm(settings: MerchantMembershipSettingsResponse): MembershipFormState {
  return {
    allow_with_voucher: settings.allow_with_voucher,
    allow_with_discount: settings.allow_with_discount,
    max_deduction_percent: String(settings.max_deduction_percent || 0),
    balanceScenes: createSceneState(settings.balance_usable_scenes || []),
    bonusScenes: createSceneState(settings.bonus_usable_scenes || [])
  }
}

function getSelectedScenes(sceneState: Record<MerchantMembershipScene, boolean>): MerchantMembershipScene[] {
  return SCENE_OPTIONS.filter((item) => sceneState[item.key]).map((item) => item.key)
}

function hasFormChanged(current: MembershipFormState, initial: MembershipFormState) {
  return JSON.stringify(current) !== JSON.stringify(initial)
}

function shouldAutoRefresh(lastLoadedAt: number, freshnessWindowMs: number) {
  return !lastLoadedAt || Date.now() - lastLoadedAt >= freshnessWindowMs
}

const getErrorMessage = getErrorUserMessage

Page({
  data: {
    navBarHeight: 88,
    sceneOptions: SCENE_OPTIONS,
    accessReady: false,
    accessDenied: false,
    accessErrorMessage: '',
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    refreshErrorMessage: '',
    loading: false,
    saving: false,
    merchantId: 0,
    lastLoadedAt: 0,
    form: buildForm({
      merchant_id: 0,
      balance_usable_scenes: [],
      bonus_usable_scenes: [],
      allow_with_voucher: false,
      allow_with_discount: false,
      max_deduction_percent: 100
    }) as MembershipFormState,
    initialForm: buildForm({
      merchant_id: 0,
      balance_usable_scenes: [],
      bonus_usable_scenes: [],
      allow_with_voucher: false,
      allow_with_discount: false,
      max_deduction_percent: 100
    }) as MembershipFormState,
    hasChanges: false
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

    this.loadSettings(true, true)
  },

  onShow() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    if (!this.data.initialLoading && !this.data.saving && !this.data.hasChanges) {
      if (shouldAutoRefresh(this.data.lastLoadedAt, MEMBERSHIP_AUTO_REFRESH_WINDOW_MS)) {
        this.loadSettings(false)
      }
    }
  },

  onPullDownRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) {
      wx.stopPullDownRefresh()
      return
    }
    if (this.data.hasChanges) {
      wx.stopPullDownRefresh()
      this.showFeedbackToast('warning', '当前有未保存修改，请先保存后再刷新')
      return
    }
    this.loadSettings(false, true)
  },

  showFeedbackToast(theme: 'success' | 'warning' | 'error', message: string, duration = 2200) {
    Toast({
      context: this,
      selector: '#t-toast',
      theme,
      message,
      placement: 'middle',
      duration,
      direction: 'column'
    })
  },

  onRetryRefresh() {
    if (!this.data.accessReady || this.data.accessDenied || this.data.accessErrorMessage) return
    this.loadSettings(false, true)
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

  async loadSettings(showLoading = true, force = false) {
    if (this.data.loading) return

    const hasExistingData = !this.data.initialLoading
    const isSilentRefresh = !showLoading && hasExistingData

    if (!force && hasExistingData && !shouldAutoRefresh(this.data.lastLoadedAt, MEMBERSHIP_AUTO_REFRESH_WINDOW_MS)) {
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
      const settings = await getMyMerchantMembershipSettings()
      const form = buildForm(settings)
      this.setData({
        merchantId: settings.merchant_id,
        form,
        initialForm: JSON.parse(JSON.stringify(form)),
        hasChanges: false,
        initialLoading: false,
        initialError: false,
        initialErrorMessage: '',
        refreshErrorMessage: '',
        lastLoadedAt: Date.now()
      })
    } catch (err: unknown) {
      logger.error('Load merchant membership settings failed', err)
      const message = getErrorMessage(err, '会员设置加载失败，请重试')

      if (this.data.initialLoading) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message
        })
      } else if (hasExistingData) {
        this.setData({ refreshErrorMessage: `${message}，当前已保留上次同步结果` })
      } else {
        this.showFeedbackToast('error', message)
      }
    } finally {
      this.setData({ loading: false })
      wx.stopPullDownRefresh()
    }
  },

  onToggleRule(e: WechatMiniprogram.CustomEvent<{ value: boolean }>) {
    const { field } = e.currentTarget.dataset as { field: 'allow_with_voucher' | 'allow_with_discount' }
    const form = {
      ...this.data.form,
      [field]: e.detail.value
    }
    this.setData({ refreshErrorMessage: '', form, hasChanges: hasFormChanged(form, this.data.initialForm) })
  },

  onPercentChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const form = {
      ...this.data.form,
      max_deduction_percent: e.detail.value
    }
    this.setData({ refreshErrorMessage: '', form, hasChanges: hasFormChanged(form, this.data.initialForm) })
  },

  onToggleScene(e: WechatMiniprogram.TouchEvent) {
    const { group, scene } = e.currentTarget.dataset as {
      group: 'balanceScenes' | 'bonusScenes'
      scene: MerchantMembershipScene
    }

    const nextGroup = {
      ...this.data.form[group],
      [scene]: !this.data.form[group][scene]
    }
    const form = {
      ...this.data.form,
      [group]: nextGroup
    }
    this.setData({ refreshErrorMessage: '', form, hasChanges: hasFormChanged(form, this.data.initialForm) })
  },

  validateForm() {
    const percent = Number(this.data.form.max_deduction_percent)
    if (!Number.isFinite(percent) || percent < 1 || percent > 100) {
      this.showFeedbackToast('warning', '最高抵扣比例需在 1-100 之间')
      return false
    }
    return true
  },

  async onSave() {
    if (this.data.saving || !this.data.hasChanges) return
    if (!this.validateForm()) return

    this.setData({ saving: true })

    try {
      const updated = await updateMyMerchantMembershipSettings({
        allow_with_voucher: this.data.form.allow_with_voucher,
        allow_with_discount: this.data.form.allow_with_discount,
        max_deduction_percent: Number(this.data.form.max_deduction_percent),
        balance_usable_scenes: getSelectedScenes(this.data.form.balanceScenes),
        bonus_usable_scenes: getSelectedScenes(this.data.form.bonusScenes)
      })
      const form = buildForm(updated)
      this.setData({
        merchantId: updated.merchant_id,
        form,
        initialForm: JSON.parse(JSON.stringify(form)),
        refreshErrorMessage: '',
        hasChanges: false,
        lastLoadedAt: Date.now()
      })
      wx.navigateBack({
        delta: 1,
        fail: () => {
          this.showFeedbackToast('success', '会员设置已保存')
        }
      })
    } catch (err: unknown) {
      logger.error('Save merchant membership settings failed', err)
      const message = getErrorMessage(err, '保存失败，请稍后重试')
      this.showFeedbackToast('error', message)
    } finally {
      this.setData({ saving: false })
    }
  },

  onRetry() {
    if (this.data.accessErrorMessage) {
      this.onRetryAccess()
      return
    }

    if (!this.data.accessReady || this.data.accessDenied) return
    this.loadSettings(true, true)
  }
})