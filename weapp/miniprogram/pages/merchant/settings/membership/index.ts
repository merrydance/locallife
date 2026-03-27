import {
  getMyMerchantMembershipSettings,
  MerchantMembershipScene,
  MerchantMembershipSettingsResponse,
  updateMyMerchantMembershipSettings
} from '../../../../api/merchant'
import { logger } from '../../../../utils/logger'
import { getStableBarHeights } from '../../../../utils/responsive'

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

Page({
  data: {
    navBarHeight: 88,
    sceneOptions: SCENE_OPTIONS,
    initialLoading: true,
    initialError: false,
    initialErrorMessage: '',
    loading: false,
    saving: false,
    merchantId: 0,
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

  onLoad() {
    const { navBarHeight } = getStableBarHeights()
    this.setData({ navBarHeight })
    this.loadSettings()
  },

  onShow() {
    if (!this.data.initialLoading && !this.data.saving) {
      this.loadSettings(false)
    }
  },

  onPullDownRefresh() {
    this.loadSettings(false)
  },

  async loadSettings(showLoading = true) {
    if (this.data.loading) return

    this.setData({
      loading: true,
      ...(showLoading ? { initialError: false, initialErrorMessage: '' } : {})
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
        initialErrorMessage: ''
      })
    } catch (err: unknown) {
      logger.error('Load merchant membership settings failed', err)
      const message = typeof err === 'object' && err !== null && 'userMessage' in err
        ? (err as { userMessage?: string }).userMessage || '会员设置加载失败，请重试'
        : '会员设置加载失败，请重试'

      if (this.data.initialLoading) {
        this.setData({
          initialLoading: false,
          initialError: true,
          initialErrorMessage: message
        })
      } else {
        wx.showToast({ title: message, icon: 'none' })
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
    this.setData({ form, hasChanges: hasFormChanged(form, this.data.initialForm) })
  },

  onPercentChange(e: WechatMiniprogram.CustomEvent<{ value: string }>) {
    const form = {
      ...this.data.form,
      max_deduction_percent: e.detail.value
    }
    this.setData({ form, hasChanges: hasFormChanged(form, this.data.initialForm) })
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
    this.setData({ form, hasChanges: hasFormChanged(form, this.data.initialForm) })
  },

  validateForm() {
    const percent = Number(this.data.form.max_deduction_percent)
    if (!Number.isFinite(percent) || percent < 1 || percent > 100) {
      wx.showToast({ title: '最高抵扣比例需在 1-100 之间', icon: 'none' })
      return false
    }
    return true
  },

  async onSave() {
    if (this.data.saving || !this.data.hasChanges) return
    if (!this.validateForm()) return

    this.setData({ saving: true })
    wx.showLoading({ title: '保存中...' })

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
        hasChanges: false
      })
      wx.showToast({ title: '会员设置已保存', icon: 'success' })
    } catch (err: unknown) {
      logger.error('Save merchant membership settings failed', err)
      const message = typeof err === 'object' && err !== null && 'userMessage' in err
        ? (err as { userMessage?: string }).userMessage || '保存失败，请稍后重试'
        : '保存失败，请稍后重试'
      wx.showToast({ title: message, icon: 'none' })
    } finally {
      wx.hideLoading()
      this.setData({ saving: false })
    }
  },

  onRetry() {
    this.loadSettings()
  }
})